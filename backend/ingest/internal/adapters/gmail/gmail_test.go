package gmail

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	gmailv1 "google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"

	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/ports"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

// fakeMailbox serves canned Gmail API responses.
type fakeMailbox struct {
	found       []*gmailv1.Message
	query       string
	maxResults  int64
	headerCalls atomic.Int32
	address     string
	profileErr  error
	pages       map[string]*gmailv1.ListHistoryResponse // by page token
	historyErr  error
	messages    map[string]*gmailv1.Message
	watchRes    *gmailv1.WatchResponse
	watchReq    *gmailv1.WatchRequest
	attachments map[string]string // attachment ID -> base64url data
}

func (f *fakeMailbox) attachment(_ context.Context, _ string, attachmentID string) (*gmailv1.MessagePartBody, error) {
	data, ok := f.attachments[attachmentID]
	if !ok {
		return nil, &googleapi.Error{Code: http.StatusNotFound}
	}
	return &gmailv1.MessagePartBody{Data: data}, nil
}

func (f *fakeMailbox) watch(_ context.Context, req *gmailv1.WatchRequest) (*gmailv1.WatchResponse, error) {
	f.watchReq = req
	return f.watchRes, nil
}
func (f *fakeMailbox) stop(context.Context) error { return nil }
func (f *fakeMailbox) profile(context.Context) (string, error) {
	return f.address, f.profileErr
}
func (f *fakeMailbox) history(_ context.Context, _ uint64, pageToken string) (*gmailv1.ListHistoryResponse, error) {
	if f.historyErr != nil {
		return nil, f.historyErr
	}
	return f.pages[pageToken], nil
}
func (f *fakeMailbox) message(_ context.Context, id string) (*gmailv1.Message, error) {
	m, ok := f.messages[id]
	if !ok {
		return nil, &googleapi.Error{Code: http.StatusNotFound}
	}
	return m, nil
}
func (f *fakeMailbox) headers(_ context.Context, id string) (*gmailv1.Message, error) {
	f.headerCalls.Add(1)
	return f.message(context.Background(), id)
}
func (f *fakeMailbox) search(_ context.Context, query string, maxResults int64) ([]*gmailv1.Message, error) {
	f.query, f.maxResults = query, maxResults
	return f.found, nil
}

func newBox(t *testing.T) *utils.SecretBox {
	t.Helper()
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	box, err := utils.NewSecretBox(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatal(err)
	}
	return box
}

func newAdapter(t *testing.T, mb *fakeMailbox) (*adapter, model.MailConnection) {
	t.Helper()
	box := newBox(t)
	sealed, err := SealCredentials(box, Credentials{RefreshToken: "rt", OAuthClientType: model.GoogleOAuthClientTypeWeb})
	if err != nil {
		t.Fatal(err)
	}
	a := &adapter{
		cfg: Config{WebClientID: "id", WebClientSecret: "secret", ProjectID: "p", PubsubTopic: "t", Box: box},
		open: func(context.Context, string, oauth2.TokenSource) (mailbox, error) {
			return mb, nil
		},
	}
	conn := model.MailConnection{
		ID: uuid.New(), Provider: "gmail", Account: "me@gmail.com",
		BudgetIDs: []uuid.UUID{uuid.New(), uuid.New()}, Credentials: sealed, Cursor: "100",
	}
	return a, conn
}

func htmlMessage(id, subject, html string) *gmailv1.Message {
	return &gmailv1.Message{
		Id: id, InternalDate: 1_700_000_000_000,
		Payload: &gmailv1.MessagePart{
			MimeType: "multipart/alternative",
			Headers: []*gmailv1.MessagePartHeader{
				{Name: "From", Value: "alerts@bank.example"}, {Name: "subject", Value: subject},
			},
			Parts: []*gmailv1.MessagePart{
				{MimeType: "text/plain", Body: &gmailv1.MessagePartBody{Data: base64.URLEncoding.EncodeToString([]byte("plain"))}},
				{MimeType: "text/html", Body: &gmailv1.MessagePartBody{Data: base64.URLEncoding.EncodeToString([]byte(html))}},
			},
		},
	}
}

func added(ids ...string) *gmailv1.History {
	h := &gmailv1.History{}
	for _, id := range ids {
		h.MessagesAdded = append(h.MessagesAdded, &gmailv1.HistoryMessageAdded{Message: &gmailv1.Message{Id: id}})
	}
	return h
}

func TestFetchSinceReadsEveryPageAndAdvancesCursor(t *testing.T) {
	mb := &fakeMailbox{
		pages: map[string]*gmailv1.ListHistoryResponse{
			"":   {History: []*gmailv1.History{added("m1", "m2")}, HistoryId: 150, NextPageToken: "p2"},
			"p2": {History: []*gmailv1.History{added("m2", "m3", "gone")}, HistoryId: 180},
		},
		messages: map[string]*gmailv1.Message{
			"m1": htmlMessage("m1", "Debited", "<b>Rs 499</b>"),
			"m2": htmlMessage("m2", "Credited", "<b>Rs 10</b>"),
			"m3": htmlMessage("m3", "Txn", "<b>Rs 1</b>"),
			// "gone" was deleted after being listed: skipped, not an error
		},
	}
	a, conn := newAdapter(t, mb)

	msgs, cursor, err := a.FetchSince(context.Background(), conn)
	if err != nil {
		t.Fatal(err)
	}
	if cursor != "180" {
		t.Fatalf("cursor = %q, want 180", cursor)
	}
	if len(msgs) != 3 {
		t.Fatalf("got %d messages, want 3 (m2 once, gone skipped)", len(msgs))
	}
	m := msgs[0]
	if m.ExternalID != "m1" || m.Source != ports.EmailSource || m.Provider != "gmail" ||
		m.Sender != "alerts@bank.example" || m.Subject != "Debited" || m.Body != "<b>Rs 499</b>" ||
		len(m.BudgetID) != 2 || m.ReceivedAt.UnixMilli() != 1_700_000_000_000 {
		t.Fatalf("message = %+v", m)
	}
}

func TestFetchSinceErrors(t *testing.T) {
	t.Run("expired cursor", func(t *testing.T) {
		a, conn := newAdapter(t, &fakeMailbox{historyErr: &googleapi.Error{Code: http.StatusNotFound}})
		_, cursor, err := a.FetchSince(context.Background(), conn)
		if !errs.HasCode(err, errs.CodeMailCursorExpired) || cursor != "" {
			t.Fatalf("got (%q, %v)", cursor, err)
		}
	})
	t.Run("no cursor yet", func(t *testing.T) {
		a, conn := newAdapter(t, &fakeMailbox{})
		conn.Cursor = ""
		_, _, err := a.FetchSince(context.Background(), conn)
		if !errs.HasCode(err, errs.CodeMailInvalidConnection) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("message fetch fails: no cursor, so nothing is skipped", func(t *testing.T) {
		mb := &fakeMailbox{
			pages:    map[string]*gmailv1.ListHistoryResponse{"": {History: []*gmailv1.History{added("m1")}, HistoryId: 150}},
			messages: map[string]*gmailv1.Message{},
		}
		a, conn := newAdapter(t, mb)
		a.open = func(context.Context, string, oauth2.TokenSource) (mailbox, error) {
			return &failingMessages{mb}, nil
		}
		_, cursor, err := a.FetchSince(context.Background(), conn)
		if !errs.HasCode(err, errs.CodeMailRequestFailed) || errs.StatusOf(err) != 500 || cursor != "" {
			t.Fatalf("got (%q, %v)", cursor, err)
		}
	})
	t.Run("credentials sealed with another key", func(t *testing.T) {
		a, conn := newAdapter(t, &fakeMailbox{})
		a.cfg.Box = newBox(t)
		_, _, err := a.FetchSince(context.Background(), conn)
		if !errs.HasCode(err, errs.CodeMailInvalidConnection) {
			t.Fatalf("err = %v", err)
		}
	})
}

type failingMessages struct{ *fakeMailbox }

func (failingMessages) message(context.Context, string) (*gmailv1.Message, error) {
	return nil, &googleapi.Error{Code: http.StatusInternalServerError}
}

func TestWatchReturnsCursorAndExpiry(t *testing.T) {
	mb := &fakeMailbox{watchRes: &gmailv1.WatchResponse{HistoryId: 42, Expiration: 1_800_000_000_000}}
	a, conn := newAdapter(t, mb)
	cursor, expires, err := a.Watch(context.Background(), conn)
	if err != nil {
		t.Fatal(err)
	}
	if cursor != "42" || expires.UnixMilli() != 1_800_000_000_000 {
		t.Fatalf("got (%q, %v)", cursor, expires)
	}
	if mb.watchReq.TopicName != "projects/p/topics/t" {
		t.Fatalf("topic = %q", mb.watchReq.TopicName)
	}
}

func TestBodyHandlesSingleAndNestedParts(t *testing.T) {
	enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	single := &gmailv1.MessagePart{MimeType: "text/html", Body: &gmailv1.MessagePartBody{Data: enc("<p>one</p>")}}
	if got := body(single); got != "<p>one</p>" {
		t.Fatalf("single-part body = %q", got)
	}
	nested := &gmailv1.MessagePart{MimeType: "multipart/mixed", Parts: []*gmailv1.MessagePart{
		{MimeType: "multipart/alternative", Parts: []*gmailv1.MessagePart{
			{MimeType: "text/plain", Body: &gmailv1.MessagePartBody{Data: enc("plain")}},
		}},
	}}
	if got := body(nested); got != "plain" {
		t.Fatalf("plain fallback = %q", got)
	}
}

func connectAdapter(t *testing.T, mb *fakeMailbox, token *oauth2.Token, exchangeErr error) *adapter {
	t.Helper()
	a, _ := newAdapter(t, mb)
	a.cfg.WebClientID, a.cfg.WebClientSecret = "id", "secret"
	a.exchange = func(_ context.Context, cfg *oauth2.Config, code string) (*oauth2.Token, error) {
		if cfg.RedirectURL != webRedirectURL || code != "the-code" {
			t.Fatalf("exchange got redirect %q code %q", cfg.RedirectURL, code)
		}
		return token, exchangeErr
	}
	return a
}

func TestConnect(t *testing.T) {
	ctx := context.Background()

	t.Run("seals a web credential for the granted address", func(t *testing.T) {
		a := connectAdapter(t, &fakeMailbox{address: "Me@Gmail.com"}, &oauth2.Token{RefreshToken: "rt"}, nil)
		got, err := a.Connect(ctx, "the-code")
		if err != nil {
			t.Fatal(err)
		}
		if got.Account != "me@gmail.com" {
			t.Fatalf("account = %q", got.Account)
		}
		creds, err := a.openCredentials(model.MailConnection{Credentials: got.Credentials})
		if err != nil {
			t.Fatal(err)
		}
		if creds.RefreshToken != "rt" || creds.OAuthClientType != model.GoogleOAuthClientTypeWeb {
			t.Fatalf("%+v", creds)
		}
	})

	t.Run("no refresh token", func(t *testing.T) {
		a := connectAdapter(t, &fakeMailbox{}, &oauth2.Token{AccessToken: "at"}, nil)
		if _, err := a.Connect(ctx, "the-code"); !errs.HasCode(err, errs.CodeMailNoRefreshToken) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("google refuses the code", func(t *testing.T) {
		a := connectAdapter(t, &fakeMailbox{}, nil, &oauth2.RetrieveError{})
		if _, err := a.Connect(ctx, "the-code"); !errs.HasCode(err, errs.CodeMailAuthFailed) {
			t.Fatalf("got %v", err)
		}
	})
}

func TestSearchListsEverythingWithoutBodies(t *testing.T) {
	mb := &fakeMailbox{
		found: []*gmailv1.Message{{Id: "new"}, {Id: "gone"}, {Id: "old"}},
		messages: map[string]*gmailv1.Message{
			"new": {Id: "new", Payload: &gmailv1.MessagePart{}},
			"old": {Id: "old", Payload: &gmailv1.MessagePart{}},
		},
	}
	a, conn := newAdapter(t, mb)

	got, err := a.Search(context.Background(), conn, ports.Query{Days: 14, MaxResults: 9999})
	if err != nil {
		t.Fatal(err)
	}
	if mb.query != "newer_than:14d" || mb.maxResults != maxSearchResults {
		t.Fatalf("query %q, max %d", mb.query, mb.maxResults)
	}
	if mb.headerCalls.Load() != 3 {
		t.Fatalf("loaded headers %d times", mb.headerCalls.Load())
	}
	if len(got) != 2 || got[0].ExternalID != "new" || got[1].ExternalID != "old" {
		t.Fatalf("%+v", got)
	}
}

func TestFetchLoadsTheChosenMessagesInFull(t *testing.T) {
	mb := &fakeMailbox{messages: map[string]*gmailv1.Message{"a": {Id: "a", Payload: &gmailv1.MessagePart{}}}}
	a, conn := newAdapter(t, mb)

	got, err := a.Fetch(context.Background(), conn, []string{"a", "gone"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ExternalID != "a" || mb.headerCalls.Load() != 0 {
		t.Fatalf("%+v, header calls %d", got, mb.headerCalls.Load())
	}
}

func TestAttachmentsListedAndDownloaded(t *testing.T) {
	msg := &gmailv1.Message{Id: "m1", Payload: &gmailv1.MessagePart{
		MimeType: "multipart/mixed",
		Parts: []*gmailv1.MessagePart{
			{MimeType: "text/html", Body: &gmailv1.MessagePartBody{Data: base64.URLEncoding.EncodeToString([]byte("<p>Your statement</p>"))}},
			{MimeType: "application/pdf", Filename: "statement.pdf", Body: &gmailv1.MessagePartBody{AttachmentId: "att-1", Size: 9}},
		},
	}}
	mb := &fakeMailbox{
		messages:    map[string]*gmailv1.Message{"m1": msg},
		attachments: map[string]string{"att-1": base64.URLEncoding.EncodeToString([]byte("%PDF-1.4\n"))},
	}
	a, conn := newAdapter(t, mb)

	in := toInbound(conn, msg)
	if len(in.Attachments) != 1 || !in.Attachments[0].IsPDF() || in.Attachments[0].FileName != "statement.pdf" {
		t.Fatalf("attachments = %+v", in.Attachments)
	}
	if in.MailAccount != conn.Account {
		t.Errorf("mail account = %q, want %q", in.MailAccount, conn.Account)
	}

	data, err := a.Attachment(context.Background(), conn, "m1", "statement.pdf")
	if err != nil || string(data) != "%PDF-1.4\n" {
		t.Fatalf("attachment = %q, %v", data, err)
	}
	if _, err := a.Attachment(context.Background(), conn, "m1", "other.pdf"); err == nil {
		t.Error("a file the message doesn't have was found")
	}
}
