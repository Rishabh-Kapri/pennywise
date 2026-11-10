package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeJobService struct {
	input service.JobListInput
	err   error
}

func (f *fakeJobService) List(_ context.Context, input service.JobListInput) (service.JobList, error) {
	f.input = input
	return service.JobList{Enabled: true}, f.err
}

func (f *fakeJobService) Get(context.Context, uuid.UUID) (service.JobDetail, error) {
	return service.JobDetail{}, f.err
}

func (f *fakeJobService) Retry(context.Context, uuid.UUID) error           { return f.err }
func (f *fakeJobService) Cancel(context.Context, uuid.UUID) error          { return f.err }
func (f *fakeJobService) RunAgain(context.Context, uuid.UUID) error        { return f.err }
func (f *fakeJobService) Continue(context.Context, uuid.UUID) error        { return f.err }
func (f *fakeJobService) Dismiss(context.Context, uuid.UUID) error         { return f.err }
func (f *fakeJobService) RetryFailed(context.Context, string) (int, error) { return 2, f.err }

func jobRouter(svc service.JobService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewJobHandler(svc)
	r := gin.New()
	r.GET("/jobs", h.List)
	r.GET("/jobs/:id", h.Get)
	r.POST("/jobs/:id/retry", h.Retry)
	r.POST("/jobs/:id/cancel", h.Cancel)
	r.POST("/jobs/retry-failed", h.RetryFailed)
	r.POST("/jobs/:id/run-again", h.RunAgain)
	r.POST("/jobs/:id/dismiss", h.Dismiss)
	return r
}

func TestJobRoutes(t *testing.T) {
	id := uuid.New().String()
	tests := []struct {
		name   string
		method string
		path   string
		err    error
		want   int
	}{
		{"list", http.MethodGet, "/jobs?status=retrying", nil, http.StatusOK},
		{"bad status", http.MethodGet, "/jobs?status=nope", nil, http.StatusBadRequest},
		{"bad job id filter", http.MethodGet, "/jobs?jobId=nope", nil, http.StatusBadRequest},
		{"bad since", http.MethodGet, "/jobs?since=yesterday", nil, http.StatusBadRequest},
		{"bad id", http.MethodGet, "/jobs/nope", nil, http.StatusBadRequest},
		{"hidden job", http.MethodGet, "/jobs/" + id, errs.New(errs.CodeJobNotFound, "no"), http.StatusNotFound},
		{"retry", http.MethodPost, "/jobs/" + id + "/retry", nil, http.StatusOK},
		{"retry a live job", http.MethodPost, "/jobs/" + id + "/retry", errs.New(errs.CodeJobInvalidState, "no"), http.StatusConflict},
		{"cancel", http.MethodPost, "/jobs/" + id + "/cancel", nil, http.StatusOK},
		{"store down", http.MethodPost, "/jobs/" + id + "/cancel", errs.New(errs.CodeJobUpdateFailed, "no"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			jobRouter(&fakeJobService{err: tt.err}).ServeHTTP(w, req)
			if w.Code != tt.want {
				t.Fatalf("got %d: %s", w.Code, w.Body)
			}
		})
	}
}

func TestJobListPassesFilters(t *testing.T) {
	jobID := uuid.New()
	svc := &fakeJobService{}
	w := httptest.NewRecorder()
	path := "/jobs?status=dead&type=message.extract&queue=cipher&errorCode=X&limit=5&offset=10" +
		"&since=2026-11-01T00:00:00Z&jobId=" + jobID.String()
	jobRouter(svc).ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))

	got := svc.input
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"enabled":true`) {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	if got.Status != "dead" || got.Type != "message.extract" || got.Queue != "cipher" || got.ErrorCode != "X" ||
		got.Limit != 5 || got.Offset != 10 || got.Since == nil || got.JobID == nil || *got.JobID != jobID {
		t.Fatalf("%+v", got)
	}
}
