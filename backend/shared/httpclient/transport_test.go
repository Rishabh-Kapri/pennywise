package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
)

func TestNon2xxErrorCarriesStatus(t *testing.T) {
	for _, tc := range []struct {
		name, body, wantSuffix string
	}{
		{"with body", "slow down", ": slow down"},
		{"empty body", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			c := transport.NewClient("test", NewHttpTransport(srv.URL), transport.WithPropagateInternalHeaders(false))
			_, err := transport.Post[map[string]any](context.Background(), c, "/v1/x", nil, map[string]string{"a": "b"})
			if err == nil {
				t.Fatal("want an error for a 429")
			}
			if got := errs.StatusOf(err); got != http.StatusTooManyRequests {
				t.Fatalf("StatusOf = %d, want 429", got)
			}
			want := "HTTP_CLIENT_ERROR: POST request for " + srv.URL + "/v1/x failed with status code: 429" + tc.wantSuffix
			if err.Error() != want {
				t.Fatalf("message changed:\n got %q\nwant %q", err.Error(), want)
			}
		})
	}
}

func TestNetworkErrorHasNoStatus(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing listening any more

	c := transport.NewClient("test", NewHttpTransport(url), transport.WithPropagateInternalHeaders(false))
	_, err := transport.Post[map[string]any](context.Background(), c, "/v1/x", nil, nil)
	if err == nil {
		t.Fatal("want a connection error")
	}
	if got := errs.StatusOf(err); got != 0 {
		t.Fatalf("StatusOf = %d, want 0 for a network failure", got)
	}
}
