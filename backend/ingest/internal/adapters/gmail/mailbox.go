package gmail

import (
	"context"

	"golang.org/x/oauth2"
	gmailv1 "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// mailbox is the slice of the Gmail API the adapter uses, for one account.
// The real one wraps the Gmail SDK; tests use a fake.
type mailbox interface {
	profile(ctx context.Context) (string, error)
	watch(ctx context.Context, req *gmailv1.WatchRequest) (*gmailv1.WatchResponse, error)
	stop(ctx context.Context) error
	history(ctx context.Context, startID uint64, pageToken string) (*gmailv1.ListHistoryResponse, error)
	message(ctx context.Context, id string) (*gmailv1.Message, error)
	// headers loads a message without its body.
	headers(ctx context.Context, id string) (*gmailv1.Message, error)
	search(ctx context.Context, query string, maxResults int64) ([]*gmailv1.Message, error)
}

// openMailbox connects to one account. ts refreshes the access token from
// the stored refresh token on demand.
type openMailbox func(ctx context.Context, account string, ts oauth2.TokenSource) (mailbox, error)

func openSDKMailbox(ctx context.Context, account string, ts oauth2.TokenSource) (mailbox, error) {
	svc, err := gmailv1.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	return &sdkMailbox{svc: svc, account: account}, nil
}

type sdkMailbox struct {
	svc     *gmailv1.Service
	account string
}

func (m *sdkMailbox) profile(ctx context.Context) (string, error) {
	res, err := m.svc.Users.GetProfile(m.account).Context(ctx).Do()
	if err != nil {
		return "", err
	}
	return res.EmailAddress, nil
}

func (m *sdkMailbox) watch(ctx context.Context, req *gmailv1.WatchRequest) (*gmailv1.WatchResponse, error) {
	return m.svc.Users.Watch(m.account, req).Context(ctx).Do()
}

func (m *sdkMailbox) stop(ctx context.Context) error {
	return m.svc.Users.Stop(m.account).Context(ctx).Do()
}

func (m *sdkMailbox) history(
	ctx context.Context,
	startID uint64,
	pageToken string,
) (*gmailv1.ListHistoryResponse, error) {
	call := m.svc.Users.History.List(m.account).
		StartHistoryId(startID).
		HistoryTypes("messageAdded").
		Context(ctx)
	if pageToken != "" {
		call = call.PageToken(pageToken)
	}
	return call.Do()
}

func (m *sdkMailbox) message(ctx context.Context, id string) (*gmailv1.Message, error) {
	return m.svc.Users.Messages.Get(m.account, id).Context(ctx).Do()
}

func (m *sdkMailbox) headers(ctx context.Context, id string) (*gmailv1.Message, error) {
	return m.svc.Users.Messages.Get(m.account, id).
		Format("metadata").
		MetadataHeaders("From", "Subject").
		Context(ctx).
		Do()
}

// search reads every page until maxResults messages are found.
func (m *sdkMailbox) search(ctx context.Context, query string, maxResults int64) ([]*gmailv1.Message, error) {
	var (
		found     []*gmailv1.Message
		pageToken string
	)
	for {
		call := m.svc.Users.Messages.
			List(m.account).
			Q(query).
			MaxResults(maxResults - int64(len(found))).
			Context(ctx)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		res, err := call.Do()
		if err != nil {
			return nil, err
		}
		found = append(found, res.Messages...)
		if res.NextPageToken == "" || int64(len(found)) >= maxResults {
			return found, nil
		}
		pageToken = res.NextPageToken
	}
}
