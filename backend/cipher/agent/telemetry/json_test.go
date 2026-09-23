package telemetry

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONKeepsTruncatedPayloadParseable(t *testing.T) {
	value := map[string]string{"prompt": strings.Repeat("🙂\\\"", 10_000)}
	encoded, err := JSON(value, 500)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > 500 || !json.Valid([]byte(encoded)) {
		t.Fatalf("invalid bounded JSON: length=%d", len(encoded))
	}
	var preview struct {
		Truncated bool `json:"truncated"`
	}
	if err := json.Unmarshal([]byte(encoded), &preview); err != nil || !preview.Truncated {
		t.Fatalf("missing truncation marker: %s", encoded)
	}
}
