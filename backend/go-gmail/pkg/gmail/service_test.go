package gmail

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/config"
	"golang.org/x/oauth2"
	gmailv1 "google.golang.org/api/gmail/v1"
)

// ---------------------------------------------------------------------------
// Mock: OAuthConfig + TokenFetcher
// ---------------------------------------------------------------------------

type mockOAuthConfig struct {
	tokenFetcher TokenFetcher
}

func (m *mockOAuthConfig) TokenSource(_ context.Context, _ *oauth2.Token) TokenFetcher {
	return m.tokenFetcher
}

type mockTokenFetcher struct {
	token *oauth2.Token
	err   error
}

func (m *mockTokenFetcher) Token() (*oauth2.Token, error) {
	return m.token, m.err
}

// ---------------------------------------------------------------------------
// Mock: GmailAPI
// ---------------------------------------------------------------------------

type mockGmailAPI struct {
	stopWatchErr   error
	setupWatchRes  *gmailv1.WatchResponse
	setupWatchErr  error
	listHistoryRes *gmailv1.ListHistoryResponse
	listHistoryErr error
	getMessageRes  *gmailv1.Message
	getMessageErr  error
}

func (m *mockGmailAPI) StopWatch(_ context.Context, _ string) error {
	return m.stopWatchErr
}

func (m *mockGmailAPI) SetupWatch(_ context.Context, _ string, _ *gmailv1.WatchRequest) (*gmailv1.WatchResponse, error) {
	return m.setupWatchRes, m.setupWatchErr
}

func (m *mockGmailAPI) ListHistory(_ context.Context, _ string, _ uint64) (*gmailv1.ListHistoryResponse, error) {
	return m.listHistoryRes, m.listHistoryErr
}

func (m *mockGmailAPI) GetMessage(_ context.Context, _ string, _ string) (*gmailv1.Message, error) {
	return m.getMessageRes, m.getMessageErr
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestService(api GmailAPI, tokenErr error) *Service {
	tok := &oauth2.Token{AccessToken: "test-access", RefreshToken: "test-refresh"}
	if tokenErr != nil {
		tok = nil
	}
	return &Service{
		config: &config.Config{ProjectID: "test-project", PubsubTopic: "test-topic"},
		oauthConfig: &mockOAuthConfig{
			tokenFetcher: &mockTokenFetcher{token: tok, err: tokenErr},
		},
		gmailAPIFactory: func(_ context.Context, _ *oauth2.Token, _ OAuthConfig) (GmailAPI, error) {
			if api == nil {
				return nil, errors.New("factory error")
			}
			return api, nil
		},
	}
}

// ---------------------------------------------------------------------------
// TestNewService
// ---------------------------------------------------------------------------

func TestNewService(t *testing.T) {
	svc := NewService()
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.gmailAPIFactory == nil {
		t.Error("expected gmailAPIFactory to be set")
	}
	if svc.oauthConfig == nil {
		t.Error("expected oauthConfig to be set")
	}
}

// ---------------------------------------------------------------------------
// TestGetOauth2Config
// ---------------------------------------------------------------------------

func TestGetOauth2Config(t *testing.T) {
	cfg := getOauth2Config("")
	if cfg == nil {
		t.Fatal("expected non-nil oauth2 config")
	}
}

// ---------------------------------------------------------------------------
// TestWatchHandler
// ---------------------------------------------------------------------------

func TestWatchHandler(t *testing.T) {
	ctx := context.Background()

	t.Run("Token fetch error", func(t *testing.T) {
		svc := newTestService(&mockGmailAPI{}, errors.New("token error"))
		_, _, err := svc.WatchHandler(ctx, GmailSyncRequest{Email: "a@b.com", RefreshToken: "rt"})
		if err == nil {
			t.Error("expected error from token fetch failure")
		}
	})

	t.Run("IsStop=true, stopWatch succeeds", func(t *testing.T) {
		svc := newTestService(&mockGmailAPI{stopWatchErr: nil}, nil)
		histID, exp, err := svc.WatchHandler(ctx, GmailSyncRequest{
			Email: "a@b.com", RefreshToken: "rt", IsStop: true,
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if histID != 0 || exp != 0 {
			t.Errorf("expected 0,0 got %d,%d", histID, exp)
		}
	})

	t.Run("IsStop=true, stopWatch returns factory error", func(t *testing.T) {
		svc := newTestService(nil, nil) // nil api triggers factory error
		_, _, err := svc.WatchHandler(ctx, GmailSyncRequest{
			Email: "a@b.com", RefreshToken: "rt", IsStop: true,
		})
		if err == nil {
			t.Error("expected error from factory failure")
		}
	})

	t.Run("IsStop=true, stopWatch API error", func(t *testing.T) {
		svc := newTestService(&mockGmailAPI{stopWatchErr: errors.New("stop error")}, nil)
		_, _, err := svc.WatchHandler(ctx, GmailSyncRequest{
			Email: "a@b.com", RefreshToken: "rt", IsStop: true,
		})
		if err == nil {
			t.Error("expected error from stopWatch failure")
		}
	})

	t.Run("IsStop=false, setupWatch succeeds", func(t *testing.T) {
		svc := newTestService(&mockGmailAPI{
			setupWatchRes: &gmailv1.WatchResponse{HistoryId: 42, Expiration: 9999},
		}, nil)
		histID, exp, err := svc.WatchHandler(ctx, GmailSyncRequest{
			Email: "a@b.com", RefreshToken: "rt",
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if histID != 42 {
			t.Errorf("expected histID 42, got %d", histID)
		}
		if exp != 9999 {
			t.Errorf("expected expiration 9999, got %d", exp)
		}
	})

	t.Run("IsStop=false, setupWatch factory error", func(t *testing.T) {
		svc := newTestService(nil, nil)
		_, _, err := svc.WatchHandler(ctx, GmailSyncRequest{Email: "a@b.com", RefreshToken: "rt"})
		if err == nil {
			t.Error("expected error from factory failure")
		}
	})

	t.Run("IsStop=false, setupWatch API error", func(t *testing.T) {
		svc := newTestService(&mockGmailAPI{setupWatchErr: errors.New("watch error")}, nil)
		_, _, err := svc.WatchHandler(ctx, GmailSyncRequest{Email: "a@b.com", RefreshToken: "rt"})
		if err == nil {
			t.Error("expected error from setupWatch API failure")
		}
	})
}

// ---------------------------------------------------------------------------
// TestGetMessageHistory
// ---------------------------------------------------------------------------

func TestGetMessageHistory(t *testing.T) {
	ctx := context.Background()
	oauthCfg := &mockOAuthConfig{tokenFetcher: &mockTokenFetcher{token: &oauth2.Token{}}}
	token := &oauth2.Token{AccessToken: "tok"}

	t.Run("Factory error", func(t *testing.T) {
		svc := newTestService(nil, nil) // nil api => factory error
		_, err := svc.GetMessageHistory(ctx, "a@b.com", 1, token, oauthCfg)
		if err == nil {
			t.Error("expected factory error")
		}
	})

	t.Run("ListHistory error", func(t *testing.T) {
		svc := newTestService(&mockGmailAPI{listHistoryErr: errors.New("list err")}, nil)
		_, err := svc.GetMessageHistory(ctx, "a@b.com", 1, token, oauthCfg)
		if err == nil {
			t.Error("expected listHistory error")
		}
	})

	t.Run("Empty history returns empty slice", func(t *testing.T) {
		svc := newTestService(&mockGmailAPI{
			listHistoryRes: &gmailv1.ListHistoryResponse{History: nil},
		}, nil)
		data, err := svc.GetMessageHistory(ctx, "a@b.com", 1, token, oauthCfg)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(data) != 0 {
			t.Errorf("expected empty, got %d", len(data))
		}
	})

	t.Run("GetMessage error returns nil,nil", func(t *testing.T) {
		svc := newTestService(&mockGmailAPI{
			listHistoryRes: &gmailv1.ListHistoryResponse{
				History: []*gmailv1.History{
					{MessagesAdded: []*gmailv1.HistoryMessageAdded{
						{Message: &gmailv1.Message{Id: "msg1"}},
					}},
				},
			},
			getMessageErr: errors.New("get error"),
		}, nil)
		data, err := svc.GetMessageHistory(ctx, "a@b.com", 1, token, oauthCfg)
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		if data != nil {
			t.Errorf("expected nil data on message fetch error, got %v", data)
		}
	})

	t.Run("Duplicate message IDs are deduplicated", func(t *testing.T) {
		svc := newTestService(&mockGmailAPI{
			listHistoryRes: &gmailv1.ListHistoryResponse{
				History: []*gmailv1.History{
					{MessagesAdded: []*gmailv1.HistoryMessageAdded{
						{Message: &gmailv1.Message{Id: "msg1"}},
						{Message: &gmailv1.Message{Id: "msg1"}}, // duplicate
					}},
				},
			},
			getMessageRes: &gmailv1.Message{
				Id:      "msg1",
				Payload: &gmailv1.MessagePart{Parts: nil, Headers: nil},
			},
		}, nil)
		data, err := svc.GetMessageHistory(ctx, "a@b.com", 1, token, oauthCfg)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(data) != 1 {
			t.Errorf("expected 1 deduplicated message, got %d", len(data))
		}
	})

	t.Run("HTML part is decoded and non-HTML parts skipped", func(t *testing.T) {
		htmlContent := "<p>Dear Customer</p>"
		encoded := base64.URLEncoding.EncodeToString([]byte(htmlContent))
		svc := newTestService(&mockGmailAPI{
			listHistoryRes: &gmailv1.ListHistoryResponse{
				History: []*gmailv1.History{
					{MessagesAdded: []*gmailv1.HistoryMessageAdded{
						{Message: &gmailv1.Message{Id: "msg1"}},
					}},
				},
			},
			getMessageRes: &gmailv1.Message{
				Id: "msg1",
				Payload: &gmailv1.MessagePart{
					Parts: []*gmailv1.MessagePart{
						{MimeType: "text/plain", Body: &gmailv1.MessagePartBody{Data: "ignored"}},
						{MimeType: "text/html", Body: &gmailv1.MessagePartBody{Data: encoded}},
					},
					Headers: []*gmailv1.MessagePartHeader{
						{Name: "Subject", Value: "TXN Alert"},
					},
				},
			},
		}, nil)
		data, err := svc.GetMessageHistory(ctx, "a@b.com", 1, token, oauthCfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data) != 1 {
			t.Fatalf("expected 1 message, got %d", len(data))
		}
		if data[0].Body != htmlContent {
			t.Errorf("expected decoded HTML body, got %q", data[0].Body)
		}
		if data[0].MessageId != "msg1" {
			t.Errorf("expected MessageId 'msg1', got %q", data[0].MessageId)
		}
	})

	t.Run("Invalid base64 in HTML part logs and continues", func(t *testing.T) {
		svc := newTestService(&mockGmailAPI{
			listHistoryRes: &gmailv1.ListHistoryResponse{
				History: []*gmailv1.History{
					{MessagesAdded: []*gmailv1.HistoryMessageAdded{
						{Message: &gmailv1.Message{Id: "msg2"}},
					}},
				},
			},
			getMessageRes: &gmailv1.Message{
				Id: "msg2",
				Payload: &gmailv1.MessagePart{
					Parts: []*gmailv1.MessagePart{
						{MimeType: "text/html", Body: &gmailv1.MessagePartBody{Data: "!!!invalid-base64!!!"}},
					},
				},
			},
		}, nil)
		data, err := svc.GetMessageHistory(ctx, "a@b.com", 1, token, oauthCfg)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// Body will be empty (decode failed) but message is still returned
		if len(data) != 1 {
			t.Errorf("expected 1 message, got %d", len(data))
		}
	})
}

// ---------------------------------------------------------------------------
// TestInit
// ---------------------------------------------------------------------------

func TestInit(t *testing.T) {
	result := Init()
	if result != nil {
		t.Errorf("expected Init() to return nil, got %v", result)
	}
}

// ---------------------------------------------------------------------------
// TestIsTransactionEmail
// ---------------------------------------------------------------------------

func makeHeader(name, value string) *gmailv1.MessagePartHeader {
	return &gmailv1.MessagePartHeader{Name: name, Value: value}
}

func TestIsTransactionEmail(t *testing.T) {
	svc := &Service{}

	tests := []struct {
		name    string
		headers []*gmailv1.MessagePartHeader
		want    bool
	}{
		{
			"Subject with txn keyword",
			[]*gmailv1.MessagePartHeader{makeHeader("Subject", "TXN Alert for your account")},
			true,
		},
		{
			"Subject with alert : update",
			[]*gmailv1.MessagePartHeader{makeHeader("Subject", "Alert : Update on your HDFC account")},
			true,
		},
		{
			"Subject with alert :  update (double space)",
			[]*gmailv1.MessagePartHeader{makeHeader("Subject", "Alert :  Update for Card")},
			true,
		},
		{
			"Subject with credit card",
			[]*gmailv1.MessagePartHeader{makeHeader("Subject", "Thank you for using your Credit Card")},
			true,
		},
		{
			"Subject with debited",
			[]*gmailv1.MessagePartHeader{makeHeader("Subject", "Amount Debited from your account")},
			true,
		},
		{
			"Subject with instaalert",
			[]*gmailv1.MessagePartHeader{makeHeader("Subject", "InstaAlert: transaction notification")},
			true,
		},
		{
			"Subject with view: account",
			[]*gmailv1.MessagePartHeader{makeHeader("Subject", "View: Account Statement")},
			true,
		},
		{
			"From header with txn keyword",
			[]*gmailv1.MessagePartHeader{
				makeHeader("From", "bank-txn@hdfc.com"),
				makeHeader("Subject", "Bank Update"),
			},
			true,
		},
		{
			"Non-transaction email",
			[]*gmailv1.MessagePartHeader{
				makeHeader("Subject", "Weekly Newsletter"),
				makeHeader("From", "newsletter@example.com"),
			},
			false,
		},
		{
			"Empty headers",
			[]*gmailv1.MessagePartHeader{},
			false,
		},
		{
			"Irrelevant header name",
			[]*gmailv1.MessagePartHeader{makeHeader("X-Custom", "txn alert debited")},
			false,
		},
		{
			"Case insensitive subject",
			[]*gmailv1.MessagePartHeader{makeHeader("Subject", "CREDIT CARD STATEMENT")},
			true,
		},
		{
			"Multiple headers, only one matches",
			[]*gmailv1.MessagePartHeader{
				makeHeader("To", "user@example.com"),
				makeHeader("Subject", "HDFC Bank InstaAlert"),
				makeHeader("Date", "Mon, 01 Jan 2025"),
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.IsTransactionEmail(tt.headers)
			if got != tt.want {
				t.Errorf("IsTransactionEmail() = %v, want %v", got, tt.want)
			}
		})
	}
}
