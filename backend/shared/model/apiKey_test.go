package model

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestValidAPIKeyFormat(t *testing.T) {
	random := strings.Repeat("A", base64.RawURLEncoding.EncodedLen(APIKeyLength))
	cases := map[string]bool{
		APIKeyPrefix + "_" + APIKeyVersion + "_" + random: true,
		"bad_" + APIKeyVersion + "_" + random:             false,
		APIKeyPrefix + "_v99_" + random:                   false,
		APIKeyPrefix + "_" + APIKeyVersion + "_tooshort":  false,
		"onlyone_segment":                                 false,
		"badkey":                                          false,
	}
	for key, want := range cases {
		if got := ValidAPIKeyFormat(key); got != want {
			t.Errorf("ValidAPIKeyFormat(%q) = %v, want %v", key, got, want)
		}
	}
}
