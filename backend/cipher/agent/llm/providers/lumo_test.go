package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

func TestLumoResponses(t *testing.T) {
	for _, key := range []string{"", "test-key"} {
		t.Run("key="+key, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/responses" || r.Method != http.MethodPost {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				wantAuth := ""
				if key != "" {
					wantAuth = "Bearer " + key
				}
				if r.Header.Get("Authorization") != wantAuth {
					t.Error("incorrect authentication")
				}
				var req openAIReq
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
				}
				if req.Model != "lumo" {
					t.Errorf("model = %q", req.Model)
				}
				if req.Reasoning == nil || req.Reasoning.Effort != "high" {
					t.Errorf("Lumo thinking was not requested: %+v", req.Reasoning)
				}
				if req.Stream {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "event: response.reasoning_text.delta\ndata: {\"delta\":\"Checking the tool.\"}\n\n")
					fmt.Fprint(w, "event: response.output_item.added\ndata: {\"output_index\":0,\"item\":{\"type\":\"function_call\",\"id\":\"item_1\",\"call_id\":\"call_1\",\"name\":\"lookup\"}}\n\n")
					fmt.Fprint(w, "event: response.completed\ndata: {\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":2,\"output_tokens\":1}}}\n\n")
					return
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"resp_1","model":"lumo","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"Hello"}]}]}`)
			}))
			defer server.Close()
			t.Setenv("LUMO_BASE_URL", server.URL+"/v1/")
			t.Setenv("LUMO_API_KEY", key)
			client, err := NewLumoClient()
			if err != nil {
				t.Fatal(err)
			}
			res, err := client.Chat(context.Background(), sharedModel.ChatRequest{Model: "lumo"})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Message.Content) != 1 || res.Message.Content[0].Text != "Hello" {
				t.Fatalf("unexpected response: %+v", res)
			}
			completed, tool, reasoning := false, false, false
			for chunk := range client.Stream(context.Background(), sharedModel.ChatRequest{Model: "lumo"}) {
				if chunk.Type == sharedModel.ChunkEventReasoning && chunk.Text == "Checking the tool." {
					reasoning = true
				}
				if chunk.Type == sharedModel.ChunkEventToolCallStart {
					tool = true
					if chunk.ToolCallID != "call_1" {
						t.Errorf("call ID = %q", chunk.ToolCallID)
					}
				}
				if chunk.Type == sharedModel.ChunkEventCompleted {
					completed = true
					if chunk.Usage.TotalTokens != 3 || chunk.Usage.CacheReadTokens != 0 {
						t.Errorf("usage = %+v", chunk.Usage)
					}
				}
			}
			if !completed || !tool || !reasoning {
				t.Fatal("missing stream events")
			}
		})
	}
}

func TestLumoRequiresValidBaseURL(t *testing.T) {
	for _, base := range []string{"", "localhost:3003", "ftp://localhost:3003", "http://localhost:3003?key=value"} {
		t.Setenv("LUMO_BASE_URL", base)
		if _, err := NewLumoClient(); err == nil {
			t.Errorf("accepted invalid base URL %q", base)
		}
	}
}
