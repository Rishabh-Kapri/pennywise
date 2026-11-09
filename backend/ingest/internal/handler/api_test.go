package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/ports"
	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

type fakeIngest struct {
	got []ports.InboundMessage
	err error
}

func (f *fakeIngest) Accept(
	_ context.Context, msg ports.InboundMessage, _ ...job.EnqueueOption,
) ([]service.Queued, error) {
	f.got = append(f.got, msg)
	if f.err != nil {
		return nil, f.err
	}
	return []service.Queued{{BudgetID: msg.BudgetID[0], JobID: uuid.New()}}, nil
}

var testBudget = uuid.New()

func post(t *testing.T, svc service.IngestService, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/", func(c *gin.Context) {
		// stands in for BudgetIdMiddleware
		c.Request = c.Request.WithContext(utils.WithBudgetID(c.Request.Context(), testBudget))
		c.Next()
	}, NewAPIHandler(svc, nil).Ingest)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	return w
}

func postSms(t *testing.T, svc service.IngestService, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/", func(c *gin.Context) {
		// stands in for BudgetIdMiddleware
		c.Request = c.Request.WithContext(utils.WithBudgetID(c.Request.Context(), testBudget))
		c.Next()
	}, NewAPIHandler(svc, nil).Sms)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	return w
}

func TestAPISms(t *testing.T) {
	t.Run("queues an sms for the budget", func(t *testing.T) {
		f := &fakeIngest{}
		w := postSms(t, f, `{"sender":"AX-HDFCBK","body":"Rs 10 debited","receivedAt":"2026-11-03T10:00:00Z"}`)
		if w.Code != http.StatusAccepted {
			t.Fatalf("status = %d: %s", w.Code, w.Body)
		}
		m := f.got[0]
		if m.Source != ports.SmsSource || m.Provider != SmsProvider || m.ExternalID == "" ||
			len(m.BudgetID) != 1 || m.BudgetID[0] != testBudget || m.ReceivedAt.IsZero() {
			t.Fatalf("message = %+v", m)
		}
	})

	t.Run("live and inbox copies share an id", func(t *testing.T) {
		f := &fakeIngest{}
		postSms(t, f, `{"sender":"AX-HDFCBK","body":"Rs 10","receivedAt":"2026-11-03T10:00:00Z","source":"live"}`)
		postSms(t, f, `{"sender":"AX-HDFCBK","body":"Rs 10","receivedAt":"2026-11-03T15:30:00+05:30","source":"inbox"}`)
		if f.got[0].ExternalID != f.got[1].ExternalID {
			t.Fatalf("ids = %q, %q", f.got[0].ExternalID, f.got[1].ExternalID)
		}
	})

	t.Run("the same text later is a new message", func(t *testing.T) {
		f := &fakeIngest{}
		postSms(t, f, `{"sender":"AX-HDFCBK","body":"Rs 10","receivedAt":"2026-11-03T10:00:00Z"}`)
		postSms(t, f, `{"sender":"AX-HDFCBK","body":"Rs 10","receivedAt":"2026-11-03T10:05:00Z"}`)
		if f.got[0].ExternalID == f.got[1].ExternalID {
			t.Fatalf("both ids = %q", f.got[0].ExternalID)
		}
	})

	for name, body := range map[string]string{
		"no sender": `{"body":"Rs 10","receivedAt":"2026-11-03T10:00:00Z"}`,
		"no body":   `{"sender":"AX-HDFCBK","receivedAt":"2026-11-03T10:00:00Z"}`,
		"no time":   `{"sender":"AX-HDFCBK","body":"Rs 10"}`,
	} {
		t.Run(name+" is rejected", func(t *testing.T) {
			f := &fakeIngest{}
			w := postSms(t, f, body)
			if w.Code != http.StatusBadRequest || len(f.got) != 0 {
				t.Fatalf("status = %d, calls = %d", w.Code, len(f.got))
			}
		})
	}
}

func TestAPIIngest(t *testing.T) {
	t.Run("with external id", func(t *testing.T) {
		f := &fakeIngest{}
		w := post(t, f, `{"externalId":"x1","sender":"bank","body":"Rs 10 debited"}`)
		if w.Code != http.StatusAccepted {
			t.Fatalf("status = %d: %s", w.Code, w.Body)
		}
		m := f.got[0]
		if m.Source != ports.ApiSource || m.Provider != "api" || m.ExternalID != "x1" ||
			len(m.BudgetID) != 1 || m.BudgetID[0] != testBudget || m.ReceivedAt.IsZero() {
			t.Fatalf("message = %+v", m)
		}
	})

	t.Run("derived id is stable for a retry", func(t *testing.T) {
		f := &fakeIngest{}
		body := `{"sender":"bank","body":"Rs 10 debited","receivedAt":"2026-11-03T10:00:00+05:30"}`
		post(t, f, body)
		post(t, f, body)
		if f.got[0].ExternalID == "" || f.got[0].ExternalID != f.got[1].ExternalID {
			t.Fatalf("ids = %q, %q", f.got[0].ExternalID, f.got[1].ExternalID)
		}
		var resp struct{ ExternalID string }
		w := post(t, f, body)
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.ExternalID != f.got[0].ExternalID {
			t.Fatalf("response id = %q", resp.ExternalID)
		}
	})

	t.Run("no id and no time is rejected", func(t *testing.T) {
		f := &fakeIngest{}
		w := post(t, f, `{"sender":"bank","body":"Rs 10 debited"}`)
		if w.Code != http.StatusBadRequest || len(f.got) != 0 {
			t.Fatalf("status = %d, calls = %d", w.Code, len(f.got))
		}
	})

	t.Run("missing body is rejected", func(t *testing.T) {
		w := post(t, &fakeIngest{}, `{"externalId":"x","sender":"bank"}`)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", w.Code)
		}
	})

	t.Run("too large", func(t *testing.T) {
		big := `{"externalId":"x","sender":"bank","body":"` + strings.Repeat("a", maxRequestBytes) + `"}`
		w := post(t, &fakeIngest{}, big)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", w.Code)
		}
	})

	t.Run("no queue is 503", func(t *testing.T) {
		f := &fakeIngest{err: errs.New(errs.CodeIngestQueueUnavailable, "no queue")}
		w := post(t, f, `{"externalId":"x","sender":"bank","body":"b"}`)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d", w.Code)
		}
	})
}

type fakeMailbox struct {
	service.MailboxService
	budget            uuid.UUID
	provider, account string
	err               error
}

func (f *fakeMailbox) Connect(
	_ context.Context, budget uuid.UUID, provider, code string,
) (*model.MailConnection, error) {
	f.budget, f.provider, f.account = budget, provider, code
	if f.err != nil {
		return nil, f.err
	}
	return &model.MailConnection{Provider: provider, Account: "me@example.com", Credentials: []byte("secret")}, nil
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
	f.budget, f.provider, f.account = budget, provider, account
	return job.EnqueueResult{JobID: uuid.New()}, f.err
}

func postTest(t *testing.T, m *fakeMailbox, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/", func(c *gin.Context) {
		c.Request = c.Request.WithContext(utils.WithBudgetID(c.Request.Context(), testBudget))
		c.Next()
	}, NewAPIHandler(nil, m).Test)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	return w
}

func TestAPITest(t *testing.T) {
	t.Run("queues for the mailbox, gmail by default", func(t *testing.T) {
		m := &fakeMailbox{}
		w := postTest(t, m, `{"account":"me@example.com"}`)
		if w.Code != http.StatusAccepted {
			t.Fatalf("status = %d: %s", w.Code, w.Body)
		}
		if m.budget != testBudget || m.provider != "gmail" || m.account != "me@example.com" {
			t.Fatalf("%+v", m)
		}
	})

	t.Run("missing account", func(t *testing.T) {
		m := &fakeMailbox{}
		if w := postTest(t, m, `{}`); w.Code != http.StatusBadRequest || m.account != "" {
			t.Fatalf("status = %d", w.Code)
		}
	})

	t.Run("mailbox not on the budget", func(t *testing.T) {
		m := &fakeMailbox{err: errs.New(errs.CodeConnectionNotFound, "no")}
		if w := postTest(t, m, `{"account":"other@example.com"}`); w.Code != http.StatusNotFound {
			t.Fatalf("status = %d", w.Code)
		}
	})
}

func TestAPIConnect(t *testing.T) {
	run := func(m *fakeMailbox, body string) *httptest.ResponseRecorder {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.POST("/", func(c *gin.Context) {
			c.Request = c.Request.WithContext(utils.WithBudgetID(c.Request.Context(), testBudget))
			c.Next()
		}, NewAPIHandler(nil, m).Connect)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
		return w
	}

	t.Run("connects and never returns credentials", func(t *testing.T) {
		m := &fakeMailbox{}
		w := run(m, `{"code":"abc"}`)
		if w.Code != http.StatusCreated || m.budget != testBudget || m.provider != "gmail" || m.account != "abc" {
			t.Fatalf("status = %d, %+v", w.Code, m)
		}
		if strings.Contains(w.Body.String(), "secret") || !strings.Contains(w.Body.String(), "me@example.com") {
			t.Fatalf("body = %s", w.Body)
		}
	})

	t.Run("missing code", func(t *testing.T) {
		if w := run(&fakeMailbox{}, `{}`); w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", w.Code)
		}
	})

	t.Run("no refresh token asks the user to act", func(t *testing.T) {
		m := &fakeMailbox{err: errs.New(errs.CodeMailNoRefreshToken, "none")}
		if w := run(m, `{"code":"abc"}`); w.Code != http.StatusConflict {
			t.Fatalf("status = %d", w.Code)
		}
	})
}

func TestAPIBackfill(t *testing.T) {
	gin.SetMode(gin.TestMode)
	run := func(method, id, body string) *httptest.ResponseRecorder {
		r := gin.New()
		h := NewAPIHandler(nil, &fakeMailbox{})
		budget := func(c *gin.Context) {
			c.Request = c.Request.WithContext(utils.WithBudgetID(c.Request.Context(), testBudget))
			c.Next()
		}
		r.GET("/:id", budget, h.BackfillCandidates)
		r.POST("/:id", budget, h.Backfill)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, "/"+id, strings.NewReader(body)))
		return w
	}
	id := uuid.NewString()

	t.Run("candidates leave out the body", func(t *testing.T) {
		w := run(http.MethodGet, id, "")
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"externalId":"m1"`) ||
			strings.Contains(w.Body.String(), "secret body") {
			t.Fatalf("status = %d: %s", w.Code, w.Body)
		}
	})
	t.Run("queues the chosen emails", func(t *testing.T) {
		w := run(http.MethodPost, id, `{"externalIds":["a","b"]}`)
		if w.Code != http.StatusAccepted || !strings.Contains(w.Body.String(), `"jobId"`) {
			t.Fatalf("status = %d: %s", w.Code, w.Body)
		}
	})
	t.Run("needs at least one email", func(t *testing.T) {
		if w := run(http.MethodPost, id, `{"externalIds":[]}`); w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", w.Code)
		}
	})
	t.Run("bad connection id", func(t *testing.T) {
		if w := run(http.MethodGet, "nope", ""); w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", w.Code)
		}
	})
}

type pauseMailbox struct {
	fakeMailbox
	budget, connection uuid.UUID
	paused             bool
	disconnected       bool
}

func (m *pauseMailbox) SetPaused(
	_ context.Context, budget, connection uuid.UUID, paused bool,
) (*model.MailConnection, error) {
	m.budget, m.connection, m.paused = budget, connection, paused
	return &model.MailConnection{ID: connection, Account: "me@gmail.com"}, nil
}

func (m *pauseMailbox) Disconnect(_ context.Context, budget, connection uuid.UUID) error {
	m.budget, m.connection, m.disconnected = budget, connection, true
	return nil
}

func connectionRequest(t *testing.T, m *pauseMailbox, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewAPIHandler(&fakeIngest{}, m)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(utils.WithBudgetID(c.Request.Context(), testBudget))
		c.Next()
	})
	r.PATCH("/connections/:id", h.UpdateConnection)
	r.DELETE("/connections/:id", h.Disconnect)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	return w
}

func TestAPIUpdateConnection(t *testing.T) {
	id := uuid.New()

	t.Run("pauses", func(t *testing.T) {
		m := &pauseMailbox{}
		w := connectionRequest(t, m, http.MethodPatch, "/connections/"+id.String(), `{"paused":true}`)
		if w.Code != http.StatusOK || !m.paused || m.budget != testBudget || m.connection != id {
			t.Fatalf("status = %d, mailbox = %+v", w.Code, m)
		}
	})

	t.Run("resumes on an explicit false", func(t *testing.T) {
		m := &pauseMailbox{paused: true}
		w := connectionRequest(t, m, http.MethodPatch, "/connections/"+id.String(), `{"paused":false}`)
		if w.Code != http.StatusOK || m.paused {
			t.Fatalf("status = %d, mailbox = %+v", w.Code, m)
		}
	})

	t.Run("an empty body changes nothing", func(t *testing.T) {
		m := &pauseMailbox{}
		w := connectionRequest(t, m, http.MethodPatch, "/connections/"+id.String(), `{}`)
		if w.Code != http.StatusBadRequest || m.connection != uuid.Nil {
			t.Fatalf("status = %d, mailbox = %+v", w.Code, m)
		}
	})
}

func TestAPIDisconnect(t *testing.T) {
	id := uuid.New()
	m := &pauseMailbox{}
	w := connectionRequest(t, m, http.MethodDelete, "/connections/"+id.String(), "")
	if w.Code != http.StatusNoContent || !m.disconnected || m.budget != testBudget || m.connection != id {
		t.Fatalf("status = %d, mailbox = %+v", w.Code, m)
	}
}
