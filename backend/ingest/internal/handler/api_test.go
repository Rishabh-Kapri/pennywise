package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/ports"
	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/auth"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

var testBudget = uuid.New()

// serve sends one request through the registered routes, as a caller of
// testBudget. principal, when set, stands in for the auth middleware.
func serve(h *APIHandler, principal *auth.Principal, method, path, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.Register(r.Group("", func(c *gin.Context) {
		ctx := utils.WithBudgetID(c.Request.Context(), testBudget)
		if principal != nil {
			ctx = auth.WithPrincipal(ctx, principal)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	return w
}

type fakeIngest struct {
	got []ports.InboundMessage
	err error
}

func (f *fakeIngest) Accept(
	_ context.Context, budgets []uuid.UUID, msg ports.InboundMessage, _ ...job.EnqueueOption,
) (service.Accepted, error) {
	if len(budgets) != 1 || budgets[0] != testBudget {
		return service.Accepted{}, errs.New(errs.CodeInternalError, "wrong budgets %v", budgets)
	}
	f.got = append(f.got, msg)
	if f.err != nil {
		return service.Accepted{}, f.err
	}
	return service.Accepted{ExternalID: msg.ExternalID, Jobs: []service.Queued{{BudgetID: testBudget, JobID: uuid.New()}}}, nil
}

func TestSms(t *testing.T) {
	post := func(f *fakeIngest, body string) *httptest.ResponseRecorder {
		return serve(NewAPIHandler(f, nil), nil, http.MethodPost, "/sms", body)
	}

	t.Run("queues an sms for the budget", func(t *testing.T) {
		f := &fakeIngest{}
		w := post(f, `{"sender":"AX-HDFCBK","body":"Rs 10 debited","receivedAt":"2026-11-03T10:00:00Z"}`)
		if w.Code != http.StatusAccepted || !strings.Contains(w.Body.String(), `"jobs"`) {
			t.Fatalf("status = %d: %s", w.Code, w.Body)
		}
		m := f.got[0]
		if m.Source != model.SourceSMS || m.Provider != ports.ProviderAndroid || m.ExternalID != "" ||
			m.ReceivedAt.Location() != time.UTC || m.ReceivedAt.IsZero() {
			t.Fatalf("message = %+v", m)
		}
	})

	t.Run("ignores how the app caught it", func(t *testing.T) {
		f := &fakeIngest{}
		post(f, `{"sender":"AX-HDFCBK","body":"Rs 10","receivedAt":"2026-11-03T10:00:00Z","source":"live"}`)
		post(f, `{"sender":"AX-HDFCBK","body":"Rs 10","receivedAt":"2026-11-03T15:30:00+05:30","source":"inbox"}`)
		if f.got[0] != f.got[1] {
			t.Fatalf("messages differ: %+v vs %+v", f.got[0], f.got[1])
		}
	})

	for name, body := range map[string]string{
		"no sender": `{"body":"Rs 10","receivedAt":"2026-11-03T10:00:00Z"}`,
		"no body":   `{"sender":"AX-HDFCBK","receivedAt":"2026-11-03T10:00:00Z"}`,
		"no time":   `{"sender":"AX-HDFCBK","body":"Rs 10"}`,
	} {
		t.Run(name+" is rejected", func(t *testing.T) {
			f := &fakeIngest{}
			if w := post(f, body); w.Code != http.StatusBadRequest || len(f.got) != 0 {
				t.Fatalf("status = %d, calls = %d", w.Code, len(f.got))
			}
		})
	}
}

func TestIngest(t *testing.T) {
	post := func(f *fakeIngest, body string) *httptest.ResponseRecorder {
		return serve(NewAPIHandler(f, nil), nil, http.MethodPost, "/messages", body)
	}

	t.Run("with external id", func(t *testing.T) {
		f := &fakeIngest{}
		w := post(f, `{"externalId":"x1","sender":"bank","body":"Rs 10 debited"}`)
		if w.Code != http.StatusAccepted {
			t.Fatalf("status = %d: %s", w.Code, w.Body)
		}
		var resp service.Accepted
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.ExternalID != "x1" || len(resp.Jobs) != 1 {
			t.Fatalf("response %s: %v", w.Body, err)
		}
		m := f.got[0]
		if m.Source != model.SourceAPI || m.Provider != ports.ProviderAPI || m.ExternalID != "x1" || m.ReceivedAt.IsZero() {
			t.Fatalf("message = %+v", m)
		}
	})

	t.Run("no id and no time is rejected", func(t *testing.T) {
		f := &fakeIngest{}
		if w := post(f, `{"sender":"bank","body":"Rs 10 debited"}`); w.Code != http.StatusBadRequest || len(f.got) != 0 {
			t.Fatalf("status = %d, calls = %d", w.Code, len(f.got))
		}
	})

	t.Run("missing body is rejected", func(t *testing.T) {
		if w := post(&fakeIngest{}, `{"externalId":"x","sender":"bank"}`); w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", w.Code)
		}
	})

	t.Run("too large", func(t *testing.T) {
		big := `{"externalId":"x","sender":"bank","body":"` + strings.Repeat("a", maxRequestBytes) + `"}`
		if w := post(&fakeIngest{}, big); w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", w.Code)
		}
	})

	t.Run("no queue is 503", func(t *testing.T) {
		f := &fakeIngest{err: errs.New(errs.CodeIngestQueueUnavailable, "no queue")}
		if w := post(f, `{"externalId":"x","sender":"bank","body":"b"}`); w.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d", w.Code)
		}
	})
}

func TestReadOnlyKeysCantWrite(t *testing.T) {
	readOnly := &auth.Principal{Kind: auth.APIKeyKind, APIKey: &model.APIKey{Scopes: []model.Scope{model.ScopeRead}}}
	f := &fakeIngest{}
	h := NewAPIHandler(f, &fakeMailbox{})

	w := serve(h, readOnly, http.MethodPost, "/messages", `{"externalId":"x","sender":"bank","body":"b"}`)
	if w.Code != http.StatusForbidden || len(f.got) != 0 {
		t.Fatalf("write: status = %d, calls = %d", w.Code, len(f.got))
	}
	if w := serve(h, readOnly, http.MethodGet, "/connections", ""); w.Code != http.StatusOK {
		t.Fatalf("read: status = %d", w.Code)
	}
}

type fakeMailbox struct {
	service.MailboxService
	budget, connection uuid.UUID
	provider, arg      string // arg: the consent code or the account
	paused             *bool
	disconnected       bool
	err                error
}

func (f *fakeMailbox) Connect(
	_ context.Context, budget uuid.UUID, provider, code string,
) (*model.MailConnection, error) {
	f.budget, f.provider, f.arg = budget, provider, code
	if f.err != nil {
		return nil, f.err
	}
	return &model.MailConnection{Provider: provider, Account: "me@example.com", Credentials: []byte("secret")}, nil
}

func (f *fakeMailbox) ListConnections(context.Context, uuid.UUID) ([]model.MailConnection, error) {
	return nil, f.err
}

func (f *fakeMailbox) BackfillCandidates(
	context.Context, uuid.UUID, uuid.UUID,
) ([]ports.InboundMessage, error) {
	return []ports.InboundMessage{{ExternalID: "m1", Subject: "Debit", Body: "secret body"}}, f.err
}

func (f *fakeMailbox) QueueBackfill(
	context.Context, uuid.UUID, uuid.UUID, []string,
) (job.EnqueueResult, error) {
	return job.EnqueueResult{JobID: uuid.New()}, f.err
}

func (f *fakeMailbox) Test(
	_ context.Context, budget uuid.UUID, provider, account string,
) (job.EnqueueResult, error) {
	f.budget, f.provider, f.arg = budget, provider, account
	return job.EnqueueResult{JobID: uuid.New()}, f.err
}

func (f *fakeMailbox) SetPaused(
	_ context.Context, budget, connection uuid.UUID, paused bool,
) (*model.MailConnection, error) {
	f.budget, f.connection, f.paused = budget, connection, &paused
	return &model.MailConnection{ID: connection, Account: "me@gmail.com"}, nil
}

func (f *fakeMailbox) Disconnect(_ context.Context, budget, connection uuid.UUID) error {
	f.budget, f.connection, f.disconnected = budget, connection, true
	return nil
}

func mailboxRequest(m *fakeMailbox, method, path, body string) *httptest.ResponseRecorder {
	return serve(NewAPIHandler(&fakeIngest{}, m), nil, method, path, body)
}

func TestTestMailbox(t *testing.T) {
	t.Run("queues for the mailbox, gmail by default", func(t *testing.T) {
		m := &fakeMailbox{}
		w := mailboxRequest(m, http.MethodPost, "/test", `{"account":"me@example.com"}`)
		if w.Code != http.StatusAccepted || m.budget != testBudget || m.provider != "gmail" || m.arg != "me@example.com" {
			t.Fatalf("status = %d: %s, %+v", w.Code, w.Body, m)
		}
	})

	t.Run("missing account", func(t *testing.T) {
		m := &fakeMailbox{}
		if w := mailboxRequest(m, http.MethodPost, "/test", `{}`); w.Code != http.StatusBadRequest || m.arg != "" {
			t.Fatalf("status = %d", w.Code)
		}
	})

	t.Run("mailbox not on the budget", func(t *testing.T) {
		m := &fakeMailbox{err: errs.New(errs.CodeConnectionNotFound, "no")}
		if w := mailboxRequest(m, http.MethodPost, "/test", `{"account":"other@example.com"}`); w.Code != http.StatusNotFound {
			t.Fatalf("status = %d", w.Code)
		}
	})
}

func TestConnect(t *testing.T) {
	t.Run("connects and never returns credentials", func(t *testing.T) {
		m := &fakeMailbox{}
		w := mailboxRequest(m, http.MethodPost, "/connections", `{"code":"abc"}`)
		if w.Code != http.StatusCreated || m.budget != testBudget || m.provider != "gmail" || m.arg != "abc" {
			t.Fatalf("status = %d, %+v", w.Code, m)
		}
		if strings.Contains(w.Body.String(), "secret") || !strings.Contains(w.Body.String(), "me@example.com") {
			t.Fatalf("body = %s", w.Body)
		}
	})

	t.Run("missing code", func(t *testing.T) {
		if w := mailboxRequest(&fakeMailbox{}, http.MethodPost, "/connections", `{}`); w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", w.Code)
		}
	})

	t.Run("no refresh token asks the user to act", func(t *testing.T) {
		m := &fakeMailbox{err: errs.New(errs.CodeMailNoRefreshToken, "none")}
		if w := mailboxRequest(m, http.MethodPost, "/connections", `{"code":"abc"}`); w.Code != http.StatusConflict {
			t.Fatalf("status = %d", w.Code)
		}
	})
}

func TestBackfill(t *testing.T) {
	path := "/connections/" + uuid.NewString() + "/backfill"

	t.Run("candidates leave out the body", func(t *testing.T) {
		w := mailboxRequest(&fakeMailbox{}, http.MethodGet, path, "")
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"externalId":"m1"`) ||
			strings.Contains(w.Body.String(), "secret body") {
			t.Fatalf("status = %d: %s", w.Code, w.Body)
		}
	})
	t.Run("queues the chosen emails", func(t *testing.T) {
		w := mailboxRequest(&fakeMailbox{}, http.MethodPost, path, `{"externalIds":["a","b"]}`)
		if w.Code != http.StatusAccepted || !strings.Contains(w.Body.String(), `"jobId"`) {
			t.Fatalf("status = %d: %s", w.Code, w.Body)
		}
	})
	t.Run("needs at least one email", func(t *testing.T) {
		if w := mailboxRequest(&fakeMailbox{}, http.MethodPost, path, `{"externalIds":[]}`); w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", w.Code)
		}
	})
	t.Run("too many emails", func(t *testing.T) {
		m := &fakeMailbox{err: errs.New(errs.CodeIngestInvalidMessage, "too many")}
		if w := mailboxRequest(m, http.MethodPost, path, `{"externalIds":["a"]}`); w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", w.Code)
		}
	})
	t.Run("bad connection id", func(t *testing.T) {
		if w := mailboxRequest(&fakeMailbox{}, http.MethodGet, "/connections/nope/backfill", ""); w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", w.Code)
		}
	})
}

func TestUpdateConnection(t *testing.T) {
	id := uuid.New()
	path := "/connections/" + id.String()

	for _, paused := range []bool{true, false} {
		m := &fakeMailbox{}
		w := mailboxRequest(m, http.MethodPatch, path, `{"paused":`+map[bool]string{true: "true", false: "false"}[paused]+`}`)
		if w.Code != http.StatusOK || m.paused == nil || *m.paused != paused || m.budget != testBudget || m.connection != id {
			t.Fatalf("paused %v: status = %d, mailbox = %+v", paused, w.Code, m)
		}
	}

	t.Run("an empty body changes nothing", func(t *testing.T) {
		m := &fakeMailbox{}
		if w := mailboxRequest(m, http.MethodPatch, path, `{}`); w.Code != http.StatusBadRequest || m.paused != nil {
			t.Fatalf("status = %d, mailbox = %+v", w.Code, m)
		}
	})
}

func TestDisconnect(t *testing.T) {
	id := uuid.New()
	m := &fakeMailbox{}
	w := mailboxRequest(m, http.MethodDelete, "/connections/"+id.String(), "")
	if w.Code != http.StatusNoContent || !m.disconnected || m.budget != testBudget || m.connection != id {
		t.Fatalf("status = %d, mailbox = %+v", w.Code, m)
	}
}
