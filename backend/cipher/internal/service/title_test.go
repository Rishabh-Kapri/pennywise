package service

import (
	"strings"
	"testing"
)

func TestTitleChatRequestModelParsing(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		wantProvider string
		wantModel    string
	}{
		{
			name:         "provider and model",
			model:        "openai/gpt-4o",
			wantProvider: "openai",
			wantModel:    "gpt-4o",
		},
		{
			// OpenRouter model ids contain a slash of their own; only the first
			// separator may be consumed.
			name:         "provider and model with slash in model name",
			model:        "openrouter/anthropic/claude-haiku-4.5",
			wantProvider: "openrouter",
			wantModel:    "anthropic/claude-haiku-4.5",
		},
		{
			// A bare model name must resolve against the registry default provider
			// rather than panicking on a missing split element.
			name:         "bare model name",
			model:        "claude-sonnet-4-6",
			wantProvider: "",
			wantModel:    "claude-sonnet-4-6",
		},
		{
			// Unset config: both provider and model come from the registry.
			name:         "empty model",
			model:        "",
			wantProvider: "",
			wantModel:    "",
		},
		{
			name:         "surrounding whitespace",
			model:        "  openai/gpt-4o  ",
			wantProvider: "openai",
			wantModel:    "gpt-4o",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := titleChatRequest(tc.model, "How much did I spend on groceries?", nil)

			if req.Provider != tc.wantProvider {
				t.Errorf("provider = %q, want %q", req.Provider, tc.wantProvider)
			}
			if req.Model != tc.wantModel {
				t.Errorf("model = %q, want %q", req.Model, tc.wantModel)
			}
			if req.MaxTokens != titleMaxTokens {
				t.Errorf("maxTokens = %d, want %d", req.MaxTokens, titleMaxTokens)
			}
			if len(req.Messages) != 2 {
				t.Fatalf("message count = %d, want 2", len(req.Messages))
			}
		})
	}
}

func TestFallbackTitle(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "short message used verbatim",
			message: "How much did I spend?",
			want:    "How much did I spend?",
		},
		{
			name:    "whitespace collapsed",
			message: "  How   much\ndid I spend?  ",
			want:    "How much did I spend?",
		},
		{
			name:    "empty message yields no title",
			message: "   ",
			want:    "",
		},
		{
			name:    "long message truncated",
			message: strings.Repeat("a", fallbackTitleMaxChars+10),
			want:    strings.Repeat("a", fallbackTitleMaxChars) + "...",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := fallbackTitle(tc.message); got != tc.want {
				t.Errorf("fallbackTitle() = %q, want %q", got, tc.want)
			}
		})
	}
}

// Truncation must not split a multi-byte rune, which byte slicing would.
func TestFallbackTitleTruncatesOnRuneBoundary(t *testing.T) {
	message := strings.Repeat("₹", fallbackTitleMaxChars+10)

	got := fallbackTitle(message)

	want := strings.Repeat("₹", fallbackTitleMaxChars) + "..."
	if got != want {
		t.Errorf("fallbackTitle() = %q, want %q", got, want)
	}
}
