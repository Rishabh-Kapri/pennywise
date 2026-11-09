package factory

import (
	"context"
	"strings"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/auth"
)

func TestParseServiceTokens(t *testing.T) {
	got, err := parseServiceTokens(" api:a1 , cipher:c1,,gmail:g:1 ")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got["api"] != "a1" || got["cipher"] != "c1" || got["gmail"] != "g:1" {
		t.Fatalf("got %v", got)
	}
	for _, bad := range []string{"api", "api:", ":secret-tok", "api:secret-a,api:secret-b"} {
		_, err := parseServiceTokens(bad)
		if err == nil {
			t.Fatalf("%q: want error", bad)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Fatalf("%q: error leaks a token: %v", bad, err)
		}
	}
}

func TestServiceAuthenticatorFallsBackToSharedToken(t *testing.T) {
	a, err := NewServiceAuthenticator(context.Background(), ServiceAuthConfig{OwnToken: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.Authenticate(context.Background(), auth.Credentials{ServiceToken: "shared"})
	if err != nil || p == nil || p.Kind != auth.ServiceKind || p.Service != "" {
		t.Fatalf("got (%+v, %v)", p, err)
	}
}
