package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseLLMTargets(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []LLMTarget
	}{
		{
			name: "empty falls back to the default chain",
			raw:  "",
			want: nil,
		},
		{
			name: "provider without a model defers to the registry default",
			raw:  "openrouter",
			want: []LLMTarget{{Provider: "openrouter"}},
		},
		{
			name: "ordered chain keeps the configured order",
			raw:  "ollama=gemma4:12b,openrouter=google/gemini-2.5-flash",
			want: []LLMTarget{
				{Provider: "ollama", Model: "gemma4:12b"},
				{Provider: "openrouter", Model: "google/gemini-2.5-flash"},
			},
		},
		{
			name: "surrounding whitespace and empty entries are ignored",
			raw:  " ollama = gemma4:12b , , anthropic ",
			want: []LLMTarget{
				{Provider: "ollama", Model: "gemma4:12b"},
				{Provider: "anthropic"},
			},
		},
		{
			name: "only the first separator splits, so models keep = : and /",
			raw:  "openrouter=vendor/model:tag=v2",
			want: []LLMTarget{{Provider: "openrouter", Model: "vendor/model:tag=v2"}},
		},
		{
			name: "entries without a provider are dropped",
			raw:  "=gemma4,ollama",
			want: []LLMTarget{{Provider: "ollama"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, parseLLMTargets(tt.raw))
		})
	}
}
