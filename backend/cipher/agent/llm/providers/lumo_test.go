package providers

import (
	"context"
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
)

func TestNormalizeLumoBaseURL(t *testing.T) {
	cases := map[string]string{
		"https://lumo.up.railway.app":          "https://lumo.up.railway.app",
		"https://lumo.up.railway.app/":         "https://lumo.up.railway.app",
		"https://lumo.up.railway.app/v1":       "https://lumo.up.railway.app",
		"https://lumo.up.railway.app/v1/":      "https://lumo.up.railway.app",
		"  http://lumo.railway.internal:3003 ": "http://lumo.railway.internal:3003",
		"":                                     "",
	}

	for raw, want := range cases {
		if got := normalizeLumoBaseURL(raw); got != want {
			t.Fatalf("normalizeLumoBaseURL(%q) = %q, want %q", raw, got, want)
		}
	}
}

// The lumo provider reuses the Responses client, so the thing worth pinning is
// that it targets lumo-tamer's /v1/responses rather than OpenRouter's /api/v1.
func TestLumoClientPostsToResponsesPath(t *testing.T) {
	body := mustJSON(t, openRouterRes{
		ID:     "resp_1",
		Model:  "lumo",
		Status: "completed",
		Output: []openRouterOutputItem{
			{
				Type:    "message",
				Role:    sharedModel.RoleAssistant,
				Content: []openRouterContentBlock{{Type: "output_text", Text: "hi"}},
			},
		},
	})
	testTransport := &openRouterTestTransport{responseBody: body}
	client := &responsesClient{
		provider: "lumo",
		path:     lumoResponsesPath,
		httpClient: transport.NewClient(
			"lumo",
			testTransport,
			transport.WithPropagateInternalHeaders(false),
		),
	}

	res, err := client.Chat(context.Background(), sharedModel.ChatRequest{
		Model: LumoDefaultModel,
		Messages: []sharedModel.AgentMessage{
			{Role: sharedModel.RoleUser, Content: []sharedModel.ContentBlock{{Type: "text", Text: "hello"}}},
		},
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	if testTransport.req.Path != "/v1/responses" {
		t.Fatalf("path = %s, want /v1/responses", testTransport.req.Path)
	}
	if len(res.Message.Content) != 1 || res.Message.Content[0].Text != "hi" {
		t.Fatalf("content = %+v, want single text block %q", res.Message.Content, "hi")
	}
}
