package utils

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestSecretBoxRoundTrip(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	box, err := NewSecretBox(key)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.Seal("sk-secret")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sealed), "sk-secret") {
		t.Fatal("sealed value contains the plaintext")
	}
	got, err := box.Open(sealed)
	if err != nil || got != "sk-secret" {
		t.Fatalf("Open = %q, %v", got, err)
	}

	sealed[len(sealed)-1] ^= 1
	if _, err := box.Open(sealed); err == nil {
		t.Error("expected tampered secret to fail")
	}
}

func TestNewSecretBoxRejectsBadKeys(t *testing.T) {
	if _, err := NewSecretBox(""); err != ErrSecretBoxKeyMissing {
		t.Errorf("empty key: %v", err)
	}
	short := base64.StdEncoding.EncodeToString([]byte("short"))
	if _, err := NewSecretBox(short); err == nil {
		t.Error("expected short key to fail")
	}
}
