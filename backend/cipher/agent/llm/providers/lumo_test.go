package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/handler"
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
				if req.Model != "lumo-max" {
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
				fmt.Fprint(w, `{"id":"resp_1","model":"lumo-max","status":"completed","output":[{"type":"reasoning","content":[{"type":"reasoning_text","text":"Checking the answer."}]},{"type":"message","content":[{"type":"output_text","text":"Hello"}]}]}`)
			}))
			defer server.Close()
			t.Setenv("LUMO_BASE_URL", server.URL+"/v1/")
			t.Setenv("LUMO_API_KEY", key)
			client, err := NewLumoClient()
			if err != nil {
				t.Fatal(err)
			}
			res, err := client.Chat(context.Background(), sharedModel.ChatRequest{Model: "lumo-max"})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Message.Content) != 1 || res.Message.Content[0].Text != "Hello" {
				t.Fatalf("unexpected response: %+v", res)
			}
			if res.Reasoning != "Checking the answer." {
				t.Fatalf("reasoning = %q", res.Reasoning)
			}
			completed, tool, reasoning := false, false, false
			for chunk := range client.Stream(context.Background(), sharedModel.ChatRequest{Model: "lumo-max"}) {
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

func TestLumoStreamReportsProviderErrors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response func(http.ResponseWriter)
		want     string
	}{
		{
			name: "stream error event",
			response: func(w http.ResponseWriter) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "event: error\ndata: {\"type\":\"error\",\"code\":\"server_error\",\"message\":\"Lumo session expired\"}\n\n")
			},
			want: "Lumo session expired",
		},
		{
			name: "HTTP request error",
			response: func(w http.ResponseWriter) {
				w.WriteHeader(http.StatusBadGateway)
				fmt.Fprint(w, "Lumo unavailable")
			},
			want: "Lumo unavailable",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				tc.response(w)
			}))
			defer server.Close()
			t.Setenv("LUMO_BASE_URL", server.URL)
			client, err := NewLumoClient()
			if err != nil {
				t.Fatal(err)
			}
			req := sharedModel.ChatRequest{Model: "lumo-max"}
			result := handler.ProcessStream(context.Background(), &req, client.Stream(context.Background(), req), handler.StreamHandler{})
			if result.Err == nil || !strings.Contains(result.Err.Error(), tc.want) {
				t.Fatalf("stream error = %v, want %q", result.Err, tc.want)
			}
		})
	}
}

func TestLumoStreamFallsBackToLiteAfterPremiumQuota(t *testing.T) {
	var modelsMu sync.Mutex
	var models []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		modelsMu.Lock()
		models = append(models, req.Model)
		modelsMu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		if req.Model == "lumo-max" {
			fmt.Fprint(w, "event: response.created\ndata: {\"response\":{\"status\":\"in_progress\"}}\n\n")
			fmt.Fprint(w, "event: response.output_item.added\ndata: {\"output_index\":0,\"item\":{\"type\":\"reasoning\"}}\n\n")
			fmt.Fprint(w, "event: error\ndata: {\"type\":\"error\",\"code\":\"server_error\",\"message\":\"Error: You have reached your daily quota for premium models\"}\n\n")
			return
		}
		fmt.Fprint(w, "event: response.output_text.delta\ndata: {\"delta\":\"Lite answer\"}\n\n")
		fmt.Fprint(w, "event: response.completed\ndata: {\"response\":{\"model\":\"lumo-lite\",\"status\":\"completed\"}}\n\n")
	}))
	defer server.Close()
	t.Setenv("LUMO_BASE_URL", server.URL)
	client, err := NewLumoClient()
	if err != nil {
		t.Fatal(err)
	}
	req := sharedModel.ChatRequest{Model: "lumo-max"}
	result := handler.ProcessStream(context.Background(), &req, client.Stream(context.Background(), req), handler.StreamHandler{})
	if result.Err != nil || result.Text != "Lite answer" || result.Model != "lumo-lite" {
		t.Fatalf("stream result = %+v", result)
	}
	modelsMu.Lock()
	defer modelsMu.Unlock()
	if len(models) != 2 || models[0] != "lumo-max" || models[1] != "lumo-lite" {
		t.Fatalf("requested models = %v", models)
	}
}

func TestLumoStreamDoesNotRetryAfterOutput(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: response.output_text.delta\ndata: {\"delta\":\"Partial answer\"}\n\n")
		fmt.Fprint(w, "event: error\ndata: {\"type\":\"error\",\"message\":\"You have reached your daily quota for premium models\"}\n\n")
	}))
	defer server.Close()
	t.Setenv("LUMO_BASE_URL", server.URL)
	client, err := NewLumoClient()
	if err != nil {
		t.Fatal(err)
	}
	req := sharedModel.ChatRequest{Model: "lumo-max"}
	result := handler.ProcessStream(context.Background(), &req, client.Stream(context.Background(), req), handler.StreamHandler{})
	if result.Err == nil || requests.Load() != 1 {
		t.Fatalf("stream result = %+v, requests = %d", result, requests.Load())
	}
}

func TestLumoChatFallsBackToLiteAfterPremiumQuota(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		requests.Add(1)
		if req.Model == "lumo-max" {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":"You have reached your daily quota for premium models"}`)
			return
		}
		fmt.Fprint(w, `{"model":"lumo-lite","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"Lite answer"}]}]}`)
	}))
	defer server.Close()
	t.Setenv("LUMO_BASE_URL", server.URL)
	client, err := NewLumoClient()
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Chat(context.Background(), sharedModel.ChatRequest{Model: "lumo-max"})
	if err != nil || response == nil || response.Model != "lumo-lite" || requests.Load() != 2 {
		t.Fatalf("response = %+v, error = %v, requests = %d", response, err, requests.Load())
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
