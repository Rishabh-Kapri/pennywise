package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

func ollamaServer(t *testing.T, dims int, path *string, auth *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*path, *auth = r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": [][]float64{make([]float64, dims)}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestUserEmbedderOllama(t *testing.T) {
	var path, auth string
	srv := ollamaServer(t, EmbeddingDimensions, &path, &auth)
	e, err := NewUserEmbedder(model.LLMProviderOllama, srv.URL, "", srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	vector, err := e.Embed(context.Background(), "bge-m3", "text")
	if err != nil || len(vector) != EmbeddingDimensions {
		t.Fatalf("got %d, %v", len(vector), err)
	}
	if path != "/api/embed" || auth != "" {
		t.Fatalf("path %q, auth %q", path, auth)
	}
}

func TestUserEmbedderRejectsTheWrongSize(t *testing.T) {
	var path, auth string
	srv := ollamaServer(t, 768, &path, &auth)
	e, err := NewUserEmbedder(model.LLMProviderOllama, srv.URL, "k", srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Embed(context.Background(), "nomic", "text"); !errs.HasCode(err, errs.CodeInternalError) {
		t.Fatalf("err = %v", err)
	}
	if auth != "Bearer k" {
		t.Fatalf("auth = %q", auth)
	}
}

func TestUserEmbedderRefusesOtherProviders(t *testing.T) {
	if _, err := NewUserEmbedder(model.LLMProviderAnthropic, "", "k", nil); !errs.HasCode(err, errs.CodeAIProviderNotConfigured) {
		t.Fatalf("err = %v", err)
	}
}
