package logger

import (
	"encoding/json"
	"log/slog"
	"testing"
)

type testPayload struct {
	EmailText string  `json:"emailText"`
	Amount    float64 `json:"amount"`
}

type testStringer struct{}

func (testStringer) String() string {
	return "test-stringer"
}

func TestNormalizeValuePrettifiesJSONString(t *testing.T) {
	normalized := normalizeValue(slog.StringValue(`{"emailText":"hello","amount":-12.5}`))

	if normalized.Kind() != slog.KindString {
		t.Fatalf("expected string value, got %v", normalized.Kind())
	}

	want := "{\n  \"emailText\": \"hello\",\n  \"amount\": -12.5\n}"
	if normalized.String() != want {
		t.Fatalf("expected pretty JSON string %q, got %q", want, normalized.String())
	}
}

func TestNormalizeValuePrettifiesStructs(t *testing.T) {
	normalized := normalizeValue(slog.AnyValue(testPayload{
		EmailText: "hello",
		Amount:    -12.5,
	}))

	if normalized.Kind() != slog.KindString {
		t.Fatalf("expected string value, got %v", normalized.Kind())
	}

	want := "{\n  \"emailText\": \"hello\",\n  \"amount\": -12.5\n}"
	if normalized.String() != want {
		t.Fatalf("expected pretty JSON string %q, got %q", want, normalized.String())
	}
}

func TestNormalizeValueLeavesStringersUntouched(t *testing.T) {
	normalized := normalizeValue(slog.AnyValue(testStringer{}))

	if normalized.Kind() != slog.KindAny {
		t.Fatalf("expected Any kind, got %v", normalized.Kind())
	}

	if _, ok := normalized.Any().(testStringer); !ok {
		t.Fatalf("expected original stringer value to be preserved, got %T", normalized.Any())
	}
}

func TestNormalizeValuePrettifiesJSONRawMessage(t *testing.T) {
	normalized := normalizeValue(slog.AnyValue(json.RawMessage(`{"merchant":"Fuel","amount":2000}`)))

	if normalized.Kind() != slog.KindString {
		t.Fatalf("expected string value, got %v", normalized.Kind())
	}

	want := "{\n  \"merchant\": \"Fuel\",\n  \"amount\": 2000\n}"
	if normalized.String() != want {
		t.Fatalf("expected pretty JSON string %q, got %q", want, normalized.String())
	}
}

func TestNormalizeAttrShortensContextKeys(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{name: "correlation id", key: "correlation_id", want: "cid"},
		{name: "budget id", key: "budget_id", want: "budget"},
		{name: "legacy budget id", key: "budgetId", want: "budget"},
		{name: "header budget id", key: "X-Budget-ID", want: "budget"},
		{name: "user id", key: "user_id", want: "user"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			attr := normalizeAttr(slog.String(test.key, "value"))
			if attr.Key != test.want {
				t.Fatalf("expected key %q, got %q", test.want, attr.Key)
			}
		})
	}
}
