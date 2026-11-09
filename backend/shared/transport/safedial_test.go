package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestIsPublicAddr(t *testing.T) {
	for addr, want := range map[string]bool{
		"8.8.8.8":         true,
		"2606:4700::1111": true,
		"127.0.0.1":       false,
		"10.1.2.3":        false,
		"172.16.0.1":      false,
		"192.168.1.1":     false,
		"169.254.169.254": false, // cloud metadata
		"100.64.0.1":      false,
		"0.0.0.0":         false,
		"::1":             false,
		"fd00::1":         false,
		"::ffff:10.0.0.1": false,
	} {
		if got := isPublicAddr(netip.MustParseAddr(addr)); got != want {
			t.Errorf("isPublicAddr(%s) = %v, want %v", addr, got, want)
		}
	}
}

func TestUserURLClientRefusesLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	if _, err := NewUserURLClient(false).Do(req); err == nil {
		t.Fatal("expected loopback dial to be refused")
	}
	res, err := NewUserURLClient(true).Do(req)
	if err != nil {
		t.Fatalf("allowPrivate should permit loopback: %v", err)
	}
	res.Body.Close()
}
