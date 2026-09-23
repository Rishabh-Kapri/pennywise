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

func TestLumoMaxResponsesStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var req openAIReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if req.Model != "lumo-max" || req.Reasoning == nil || req.Reasoning.Effort != "high" || !req.Stream {
			t.Errorf("unexpected Lumo request: %+v", req)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: response.output_item.added\ndata: {\"output_index\":0,\"item\":{\"type\":\"function_call\",\"id\":\"item_1\",\"call_id\":\"call_1\",\"name\":\"lookup\"}}\n\n")
		fmt.Fprint(w, "event: response.completed\ndata: {\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":2,\"output_tokens\":1}}}\n\n")
	}))
	defer server.Close()
	t.Setenv("LUMO_BASE_URL", server.URL+"/v1/")
	t.Setenv("LUMO_API_KEY", "")

	client, err := NewLumoClient()
	if err != nil {
		t.Fatal(err)
	}
	toolSeen, completed := false, false
	for chunk := range client.Stream(context.Background(), sharedModel.ChatRequest{Provider: "lumo", Model: "lumo-max"}) {
		if chunk.Type == sharedModel.ChunkEventToolCallStart {
			toolSeen = chunk.ToolCallID == "call_1"
		}
		if chunk.Type == sharedModel.ChunkEventCompleted {
			completed = true
		}
	}
	if !toolSeen || !completed {
		t.Fatalf("missing tool or completion event: tool=%t completed=%t", toolSeen, completed)
	}
}
