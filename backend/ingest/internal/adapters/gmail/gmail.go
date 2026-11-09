// Package gmail is the Gmail adapter for ports.MailProvider. It is the Gmail
// code from go-gmail (pkg/gmail, pkg/auth, pkg/temporal) moved behind the
// port: watch, history fetch and backfill search.
package gmail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/sync/errgroup"
	gmailv1 "google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"

	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/ports"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

const (
	providerName = "gmail"
	// currentUser is Gmail's name for the account a token belongs to.
	currentUser = "me"
	// webRedirectURL is what Google's popup flow uses in place of a real URL.
	webRedirectURL = "postmessage"
)

// maxSearchDays and maxSearchResults bound a search; Gmail's own limit is the
// account's whole history, which would be read one message at a time.
const (
	maxSearchDays    = 30
	maxSearchResults = 500
	// fetchConcurrency stays well inside Gmail's per-user rate limit.
	fetchConcurrency = 10
)

var scopes = []string{"https://mail.google.com/", "https://www.googleapis.com/auth/userinfo.email"}

// Config is the app-level Gmail setup, the same for every connection.
type Config struct {
	WebClientID     string
	WebClientSecret string
	AndroidClientID string
	// ProjectID and PubsubTopic name the topic Gmail pushes to.
	ProjectID   string
	PubsubTopic string
	// Box opens Connection.Credentials.
	Box *utils.SecretBox
}

// Credentials is what Connection.Credentials holds for Gmail, as sealed
// JSON. Google refresh tokens don't change when an access token is
// refreshed, so the adapter never has to write them back.
type Credentials struct {
	RefreshToken    string                      `json:"refreshToken"`
	OAuthClientType model.GoogleOAuthClientType `json:"oauthClientType"`
}

// SealCredentials produces Connection.Credentials for a newly connected
// mailbox.
func SealCredentials(box *utils.SecretBox, c Credentials) ([]byte, error) {
	raw, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	return box.Seal(string(raw))
}

// exchangeCode swaps a consent screen's one-time code for tokens.
type exchangeCode func(ctx context.Context, cfg *oauth2.Config, code string) (*oauth2.Token, error)

type adapter struct {
	cfg      Config
	open     openMailbox
	exchange exchangeCode
}

func New(cfg Config) ports.MailProvider {
	return &adapter{
		cfg:  cfg,
		open: openSDKMailbox,
		exchange: func(ctx context.Context, cfg *oauth2.Config, code string) (*oauth2.Token, error) {
			return cfg.Exchange(ctx, code)
		},
	}
}

// Connect implements [ports.MailProvider] for the web client. The address comes
// from Gmail, not the caller, so a user can only connect a mailbox they granted.
func (a *adapter) Connect(ctx context.Context, code string) (ports.ConnectedMailbox, error) {
	if a.cfg.Box == nil {
		return ports.ConnectedMailbox{}, errs.New(errs.CodeMailNotConfigured, "gmail adapter has no credentials key")
	}
	oauthCfg, err := a.oauthConfig(model.GoogleOAuthClientTypeWeb)
	if err != nil {
		return ports.ConnectedMailbox{}, err
	}
	oauthCfg.RedirectURL = webRedirectURL

	var noConn model.MailConnection
	token, err := a.exchange(ctx, oauthCfg, code)
	if err != nil {
		return ports.ConnectedMailbox{}, googleError("exchange code", noConn, err)
	}
	// Google only sends one when the user sees the consent screen.
	if token.RefreshToken == "" {
		return ports.ConnectedMailbox{}, errs.New(errs.CodeMailNoRefreshToken, "google returned no refresh token")
	}

	mb, err := a.open(ctx, currentUser, oauthCfg.TokenSource(ctx, token))
	if err != nil {
		return ports.ConnectedMailbox{}, googleError("open mailbox", noConn, err)
	}
	account, err := mb.profile(ctx)
	if err != nil {
		return ports.ConnectedMailbox{}, googleError("read mailbox address", noConn, err)
	}

	sealed, err := SealCredentials(a.cfg.Box, Credentials{
		RefreshToken:    token.RefreshToken,
		OAuthClientType: model.GoogleOAuthClientTypeWeb,
	})
	if err != nil {
		return ports.ConnectedMailbox{}, errs.Wrap(errs.CodeInternalError, "seal gmail credentials", err)
	}
	return ports.ConnectedMailbox{Account: strings.ToLower(account), Credentials: sealed}, nil
}

// Watch implements [ports.MailProvider]: Gmail pushes new INBOX mail to the
// Pub/Sub topic until expiresAt (about a week), so call it again before then.
func (a *adapter) Watch(ctx context.Context, conn model.MailConnection) (string, time.Time, error) {
	if a.cfg.ProjectID == "" || a.cfg.PubsubTopic == "" {
		return "", time.Time{}, errs.New(errs.CodeMailNotConfigured, "gmail watch needs PROJECT_ID and PUBSUB_TOPIC")
	}
	mb, err := a.mailboxFor(ctx, conn)
	if err != nil {
		return "", time.Time{}, err
	}
	res, err := mb.watch(ctx, &gmailv1.WatchRequest{
		LabelIds:          []string{"INBOX"},
		LabelFilterAction: "include",
		TopicName:         fmt.Sprintf("projects/%s/topics/%s", a.cfg.ProjectID, a.cfg.PubsubTopic),
	})
	if err != nil {
		return "", time.Time{}, googleError("watch mailbox", conn, err)
	}
	return strconv.FormatUint(res.HistoryId, 10), time.UnixMilli(res.Expiration), nil
}

// StopWatch implements [ports.MailProvider].
func (a *adapter) StopWatch(ctx context.Context, conn model.MailConnection) error {
	mb, err := a.mailboxFor(ctx, conn)
	if err != nil {
		return err
	}
	if err := mb.stop(ctx); err != nil {
		return googleError("stop watch", conn, err)
	}
	return nil
}

// FetchSince implements [ports.MailProvider]. The cursor is a Gmail history
// ID. Every page of history is read, so nothing is skipped when many
// messages arrived at once. On any error no cursor is returned: the caller
// keeps the old one and the same messages are fetched again next time
// (duplicates are dropped by the job idempotency key on ExternalID).
func (a *adapter) FetchSince(ctx context.Context, conn model.MailConnection) ([]ports.InboundMessage, string, error) {
	start, err := strconv.ParseUint(conn.Cursor, 10, 64)
	if err != nil {
		return nil, "", errs.NewWith(errs.CodeMailInvalidConnection,
			"connection has no usable cursor; call Watch first", connDetails(conn))
	}
	mb, err := a.mailboxFor(ctx, conn)
	if err != nil {
		return nil, "", err
	}

	var (
		ids       []string
		seen      = map[string]bool{}
		latest    = start
		pageToken string
	)
	for {
		page, err := mb.history(ctx, start, pageToken)
		if isStatus(err, http.StatusNotFound) {
			// Gmail keeps about a week of history; older cursors are gone.
			return nil, "", errs.Wrap(errs.CodeMailCursorExpired, "history cursor expired", err, connDetails(conn))
		}
		if err != nil {
			return nil, "", googleError("list history", conn, err)
		}
		for _, h := range page.History {
			for _, added := range h.MessagesAdded {
				if added.Message == nil || seen[added.Message.Id] {
					continue
				}
				seen[added.Message.Id] = true
				ids = append(ids, added.Message.Id)
			}
		}
		if page.HistoryId > latest {
			latest = page.HistoryId
		}
		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}

	messages, err := a.fetchMessages(ctx, conn, ids, mb.message)
	if err != nil {
		return nil, "", err
	}
	return messages, strconv.FormatUint(latest, 10), nil
}

// Search implements [ports.MailProvider]: everything in the mailbox newer than
// q.Days, newest first. Gmail leaves out Spam and Trash.
func (a *adapter) Search(ctx context.Context, conn model.MailConnection, q ports.Query) ([]ports.InboundMessage, error) {
	days := min(max(q.Days, 1), maxSearchDays)
	maxResults := min(max(q.MaxResults, 1), maxSearchResults)

	mb, err := a.mailboxFor(ctx, conn)
	if err != nil {
		return nil, err
	}
	found, err := mb.search(ctx, fmt.Sprintf("newer_than:%dd", days), maxResults)
	if err != nil {
		return nil, googleError("search messages", conn, err)
	}
	ids := make([]string, 0, len(found))
	for _, m := range found {
		ids = append(ids, m.Id)
	}
	return a.fetchMessages(ctx, conn, ids, mb.headers)
}

// Fetch implements [ports.MailProvider].
func (a *adapter) Fetch(ctx context.Context, conn model.MailConnection, ids []string) ([]ports.InboundMessage, error) {
	mb, err := a.mailboxFor(ctx, conn)
	if err != nil {
		return nil, err
	}
	return a.fetchMessages(ctx, conn, ids, mb.message)
}

// fetchMessages loads each message with load, keeping the order of ids. A
// message deleted since it was listed is skipped; any other failure fails the
// whole fetch.
func (a *adapter) fetchMessages(
	ctx context.Context,
	conn model.MailConnection,
	ids []string,
	load func(ctx context.Context, id string) (*gmailv1.Message, error),
) ([]ports.InboundMessage, error) {
	loaded := make([]*gmailv1.Message, len(ids))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(fetchConcurrency)
	for i, id := range ids {
		group.Go(func() error {
			msg, err := load(groupCtx, id)
			if isStatus(err, http.StatusNotFound) {
				logger.Logger(ctx).Info("gmail message gone before fetch", "message_id", id, "connection_id", conn.ID)
				return nil
			}
			if err != nil {
				return googleError("get message", conn, err)
			}
			loaded[i] = msg
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	messages := make([]ports.InboundMessage, 0, len(ids))
	for _, msg := range loaded {
		if msg != nil {
			messages = append(messages, toInbound(conn, msg))
		}
	}
	return messages, nil
}

// mailboxFor opens the connection's credentials and connects to its mailbox.
func (a *adapter) mailboxFor(ctx context.Context, conn model.MailConnection) (mailbox, error) {
	creds, err := a.openCredentials(conn)
	if err != nil {
		return nil, err
	}
	oauthCfg, err := a.oauthConfig(creds.OAuthClientType)
	if err != nil {
		return nil, err
	}
	ts := oauthCfg.TokenSource(ctx, &oauth2.Token{RefreshToken: creds.RefreshToken})
	mb, err := a.open(ctx, conn.Account, ts)
	if err != nil {
		return nil, googleError("open mailbox", conn, err)
	}
	return mb, nil
}

func (a *adapter) openCredentials(conn model.MailConnection) (Credentials, error) {
	if a.cfg.Box == nil {
		return Credentials{}, errs.New(errs.CodeMailNotConfigured, "gmail adapter has no credentials key")
	}
	raw, err := a.cfg.Box.Open(conn.Credentials)
	if err != nil {
		return Credentials{}, errs.Wrap(errs.CodeMailInvalidConnection, "open credentials", err, connDetails(conn))
	}
	var creds Credentials
	if err := json.Unmarshal([]byte(raw), &creds); err != nil || creds.RefreshToken == "" {
		return Credentials{}, errs.NewWith(errs.CodeMailInvalidConnection,
			"credentials are not gmail credentials", connDetails(conn))
	}
	return creds, nil
}

// oauthConfig is go-gmail's per-client-type OAuth setup. Android tokens were
// issued to a public client, so they refresh without a secret.
func (a *adapter) oauthConfig(clientType model.GoogleOAuthClientType) (*oauth2.Config, error) {
	if model.NormalizeGoogleOAuthClientType(clientType) == model.GoogleOAuthClientTypeAndroid {
		if a.cfg.AndroidClientID == "" {
			return nil, errs.New(errs.CodeMailNotConfigured, "GOOGLE_ANDROID_CLIENT_ID not set")
		}
		return &oauth2.Config{ClientID: a.cfg.AndroidClientID, Endpoint: google.Endpoint, Scopes: scopes}, nil
	}
	if a.cfg.WebClientID == "" || a.cfg.WebClientSecret == "" {
		return nil, errs.New(errs.CodeMailNotConfigured, "GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET not set")
	}
	return &oauth2.Config{
		ClientID:     a.cfg.WebClientID,
		ClientSecret: a.cfg.WebClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       scopes,
	}, nil
}

// googleError gives a Gmail/OAuth failure a mail code. A refused refresh
// token is CodeMailAuthFailed: the user has to reconnect, retrying won't help.
func googleError(action string, conn model.MailConnection, err error) error {
	if retrieve, ok := errors.AsType[*oauth2.RetrieveError](err); ok {
		errOpts := []errs.Option{connDetails(conn)}
		if retrieve.Response != nil {
			errOpts = append(errOpts, errs.WithStatus(retrieve.Response.StatusCode))
		}
		return errs.Wrap(errs.CodeMailAuthFailed, action+": google refused the stored grant", err, errOpts...)
	}
	if apiErr, ok := errors.AsType[*googleapi.Error](err); ok {
		return errs.Wrap(errs.CodeMailRequestFailed, action, err, connDetails(conn), errs.WithStatus(apiErr.Code))
	}

	return errs.Wrap(errs.CodeMailRequestFailed, action, err, connDetails(conn))
}

func isStatus(err error, status int) bool {
	var apiErr *googleapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == status
}

func connDetails(conn model.MailConnection) errs.Option {
	return errs.WithDetails(map[string]any{"connection_id": conn.ID, "provider": providerName})
}
