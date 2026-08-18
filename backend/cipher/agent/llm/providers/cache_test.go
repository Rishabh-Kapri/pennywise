package providers

import (
	"encoding/json"
	"strings"
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

// The static half of the system prompt must carry a cache breakpoint and the
// dynamic half must not — a breakpoint after the volatile block would cache
// content that changes daily, which never hits.
func TestAnthropicSystemBlocksCarryCacheBreakpoint(t *testing.T) {
	client := &anthropicClient{name: "chat"}

	req := client.toAnthropicReq(sharedModel.ChatRequest{
		Model: "claude-sonnet-4-6",
		Messages: []sharedModel.AgentMessage{
			{
				Role: sharedModel.RoleSystem,
				Content: []sharedModel.ContentBlock{
					{Type: "text", Text: "static instructions", Cacheable: true},
					{Type: "text", Text: "today is 2026-07-27"},
				},
			},
			{
				Role:    sharedModel.RoleUser,
				Content: []sharedModel.ContentBlock{{Type: "text", Text: "hello"}},
			},
		},
	})

	if len(req.System) != 2 {
		t.Fatalf("system block count = %d, want 2", len(req.System))
	}
	if req.System[0].CacheControl == nil || req.System[0].CacheControl.Type != "ephemeral" {
		t.Errorf("static system block should carry an ephemeral breakpoint, got %#v", req.System[0].CacheControl)
	}
	if req.System[1].CacheControl != nil {
		t.Errorf("dynamic system block must not carry a breakpoint, got %#v", req.System[1].CacheControl)
	}
}

// Tools render before the system prompt, so the breakpoint belongs on the last
// tool to cache the whole array.
func TestAnthropicToolsCarryCacheBreakpointOnLastTool(t *testing.T) {
	tools := toAnthropicTools([]sharedModel.ToolDefiniton{
		{Name: "get_today"},
		{Name: "get_budget_info"},
		{Name: "execute_sql"},
	})

	if len(tools) != 3 {
		t.Fatalf("tool count = %d, want 3", len(tools))
	}
	for i, tool := range tools[:len(tools)-1] {
		if tool.CacheControl != nil {
			t.Errorf("tool[%d] (%s) should not carry a breakpoint", i, tool.Name)
		}
	}
	if last := tools[len(tools)-1]; last.CacheControl == nil {
		t.Errorf("last tool (%s) should carry the cache breakpoint", last.Name)
	}
}

func TestAnthropicToolsWithNoToolsIsEmpty(t *testing.T) {
	if got := toAnthropicTools(nil); len(got) != 0 {
		t.Errorf("tool count = %d, want 0", len(got))
	}
}

func TestAnthropicUsageCarriesCacheCounters(t *testing.T) {
	usage := toModelUsage(anthropicUsage{
		InputTokens:              100,
		OutputTokens:             50,
		CacheReadInputTokens:     900,
		CacheCreationInputTokens: 20,
	})

	if usage.CacheReadTokens != 900 {
		t.Errorf("cacheReadTokens = %d, want 900", usage.CacheReadTokens)
	}
	if usage.CacheWriteTokens != 20 {
		t.Errorf("cacheWriteTokens = %d, want 20", usage.CacheWriteTokens)
	}
	if usage.TotalTokens != 150 {
		t.Errorf("totalTokens = %d, want 150", usage.TotalTokens)
	}
}

// OpenRouter has no instructions string any more: system content is an input
// item so it can carry a breakpoint for Anthropic models served through it.
func TestOpenRouterSystemInputCarriesCacheBreakpoint(t *testing.T) {
	input := toOpenRouterInput([]sharedModel.AgentMessage{
		{
			Role: sharedModel.RoleSystem,
			Content: []sharedModel.ContentBlock{
				{Type: "text", Text: "static instructions", Cacheable: true},
				{Type: "text", Text: "today is 2026-07-27"},
			},
		},
		{
			Role:    sharedModel.RoleUser,
			Content: []sharedModel.ContentBlock{{Type: "text", Text: "hello"}},
		},
	})

	if len(input) != 2 {
		t.Fatalf("input item count = %d, want 2", len(input))
	}
	system := input[0]
	if system.Role != sharedModel.RoleSystem {
		t.Fatalf("first input item role = %q, want system", system.Role)
	}
	if len(system.Content) != 2 {
		t.Fatalf("system content block count = %d, want 2", len(system.Content))
	}
	if system.Content[0].CacheControl == nil || system.Content[0].CacheControl.Type != "ephemeral" {
		t.Errorf("static block should carry an ephemeral breakpoint, got %#v", system.Content[0].CacheControl)
	}
	if system.Content[1].CacheControl != nil {
		t.Errorf("dynamic block must not carry a breakpoint, got %#v", system.Content[1].CacheControl)
	}
}

// The Responses API caches automatically; prompt_cache_key only pins a
// conversation to a consistent cache, so it must follow the conversation id.
func TestOpenAIRequestCarriesPromptCacheKey(t *testing.T) {
	client := &openAIClient{}

	req := client.toOpenAIReq(sharedModel.ChatRequest{
		Model:    "gpt-4o",
		Metadata: map[string]string{"conversationId": "conv-123"},
		Messages: []sharedModel.AgentMessage{
			{Role: sharedModel.RoleUser, Content: []sharedModel.ContentBlock{{Type: "text", Text: "hello"}}},
		},
	})

	if req.PromptCacheKey != "conv-123" {
		t.Errorf("promptCacheKey = %q, want conv-123", req.PromptCacheKey)
	}
}

// OpenRouter reports cache writes under whichever spelling the upstream
// provider uses, and modelled none of them until now -- so every run through
// OpenRouter recorded cacheWriteTokens as 0 whether or not the prefix was
// cached. That made a real cache miss indistinguishable from an unreported
// counter, which is the failure this covers.
func TestOpenRouterUsageCarriesCacheWriteUnderEitherSpelling(t *testing.T) {
	tests := []struct {
		name  string
		usage openRouterUsage
		want  int
	}{
		{
			name: "anthropic top-level spelling",
			usage: openRouterUsage{
				InputTokens:              100,
				OutputTokens:             50,
				CacheCreationInputTokens: 3200,
			},
			want: 3200,
		},
		{
			name: "input_tokens_details spelling",
			usage: openRouterUsage{
				InputTokens:        100,
				OutputTokens:       50,
				InputTokensDetails: openRouterTokenDetails{CacheCreationTokens: 3200},
			},
			want: 3200,
		},
		{
			name: "prompt_tokens_details spelling",
			usage: openRouterUsage{
				PromptTokens:        100,
				CompletionTokens:    50,
				PromptTokensDetails: openRouterTokenDetails{CacheCreationTokens: 3200},
			},
			want: 3200,
		},
		{
			name:  "absent stays zero",
			usage: openRouterUsage{InputTokens: 100, OutputTokens: 50},
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toOpenRouterUsage(tt.usage).CacheWriteTokens; got != tt.want {
				t.Errorf("cacheWriteTokens = %d, want %d", got, tt.want)
			}
		})
	}
}

// The raw usage bytes are kept so the counters we do not model stay visible in
// logs. Without this, discovering a new spelling requires guessing at it.
func TestOpenRouterUsageRetainsRawPayload(t *testing.T) {
	body := []byte(`{"input_tokens":100,"output_tokens":50,"some_unmodelled_cache_field":7}`)

	var usage openRouterUsage
	if err := json.Unmarshal(body, &usage); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if usage.InputTokens != 100 {
		t.Errorf("inputTokens = %d, want 100", usage.InputTokens)
	}
	if !strings.Contains(string(usage.raw), "some_unmodelled_cache_field") {
		t.Errorf("raw payload lost the unmodelled field: %s", usage.raw)
	}
}
