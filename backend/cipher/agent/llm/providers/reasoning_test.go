package providers

import (
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

func reasoningReq(model, provider, effort string) sharedModel.ChatRequest {
	return sharedModel.ChatRequest{
		Model:           model,
		Provider:        provider,
		ReasoningEffort: effort,
		Messages: []sharedModel.AgentMessage{
			{Role: sharedModel.RoleUser, Content: []sharedModel.ContentBlock{{Type: "text", Text: "hello"}}},
		},
	}
}

// Unset effort must leave the request untouched so every existing step keeps
// the provider's default; a set effort is passed through.
func TestOpenRouterReasoningEffort(t *testing.T) {
	client := &openRouterClient{}

	if got := client.toOpenRouterReq(reasoningReq("deepseek/x", "openrouter", "")); got.Reasoning != nil {
		t.Errorf("reasoning = %#v, want nil when no effort is requested", got.Reasoning)
	}
	got := client.toOpenRouterReq(reasoningReq("deepseek/x", "openrouter", "low"))
	if got.Reasoning == nil || got.Reasoning.Effort != "low" {
		t.Errorf("reasoning = %#v, want effort low", got.Reasoning)
	}
}

func TestOpenAIReasoningEffortOverridesDefault(t *testing.T) {
	client := &openAIClient{}

	// lumo defaults to high effort; an explicit request wins
	got := client.toOpenAIReq(reasoningReq("lumo-lite", "lumo", "low"))
	if got.Reasoning == nil || got.Reasoning.Effort != "low" {
		t.Errorf("lumo reasoning = %#v, want effort low", got.Reasoning)
	}
	if got := client.toOpenAIReq(reasoningReq("lumo-lite", "lumo", "")); got.Reasoning == nil || got.Reasoning.Effort != "high" {
		t.Errorf("lumo default reasoning = %#v, want effort high", got.Reasoning)
	}
	if got := client.toOpenAIReq(reasoningReq("gpt-4o", "openai", "")); got.Reasoning != nil {
		t.Errorf("gpt-4o reasoning = %#v, want nil", got.Reasoning)
	}
	got = client.toOpenAIReq(reasoningReq("gpt-5", "openai", "medium"))
	if got.Reasoning == nil || got.Reasoning.Effort != "medium" || got.Reasoning.Summary != "auto" {
		t.Errorf("gpt-5 reasoning = %#v, want summary auto + effort medium", got.Reasoning)
	}
}

// A thinking model on Ollama spends the output budget reasoning, so structured
// replies and low-effort requests turn thinking off; plain chat keeps the
// model's default.
func TestOllamaThinking(t *testing.T) {
	client := &ollamaClient{}

	if got := client.toOllamaReq(reasoningReq("qwen3.5:9b", "ollama", "")); got.Think != nil {
		t.Errorf("think = %v, want unset for plain chat", *got.Think)
	}

	low := client.toOllamaReq(reasoningReq("qwen3.5:9b", "ollama", "low"))
	if low.Think == nil || *low.Think {
		t.Errorf("think = %v, want false for low effort", low.Think)
	}

	structured := reasoningReq("qwen3.5:9b", "ollama", "")
	structured.Format = "json"
	if got := client.toOllamaReq(structured); got.Think == nil || *got.Think {
		t.Errorf("think = %v, want false for a JSON reply", got.Think)
	}
}
