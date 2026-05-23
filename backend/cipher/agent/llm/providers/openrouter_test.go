package providers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/handler"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
)

type openRouterTestTransport struct {
	req          *transport.Request
	responseBody []byte
	streamReq    *transport.Request
	streamEvents []transport.SSEEvent
}

func (t *openRouterTestTransport) Send(
	ctx context.Context,
	req *transport.Request,
) (transport.Response, error) {
	t.req = req
	return transport.Response{
		StatusCode: 200,
		Body:       t.responseBody,
	}, nil
}

func (t *openRouterTestTransport) Stream(
	ctx context.Context,
	req *transport.Request,
) (transport.StreamResponse, error) {
	t.streamReq = req
	events := make(chan transport.SSEEvent)
	go func() {
		defer close(events)
		for _, event := range t.streamEvents {
			select {
			case events <- event:
			case <-ctx.Done():
				return
			}
		}
	}()

	return transport.StreamResponse{
		StatusCode: 200,
		Events:     events,
	}, nil
}

func TestOpenRouterChatUsesResponsesAPIAndNormalizesToolCalls(t *testing.T) {
	body := mustJSON(t, openRouterRes{
		ID:     "resp_1",
		Model:  "anthropic/claude-haiku-4.5",
		Status: "completed",
		Output: []openRouterOutputItem{
			{
				Type:      "function_call",
				ID:        "fc_1",
				CallID:    "call_1",
				Name:      "get_today",
				Arguments: "{}",
			},
		},
		Usage: openRouterUsage{
			PromptTokens:     10,
			CompletionTokens: 3,
			TotalTokens:      13,
		},
	})
	streamTransport := &openRouterTestTransport{responseBody: body}
	client := newTestOpenRouterClient(streamTransport)

	res, err := client.Chat(context.Background(), sharedModel.ChatRequest{
		Model: "anthropic/claude-haiku-4.5",
		Messages: []sharedModel.AgentMessage{
			{
				Role:    sharedModel.RoleSystem,
				Content: []sharedModel.ContentBlock{{Type: "text", Text: "system prompt"}},
			},
			{
				Role:    sharedModel.RoleUser,
				Content: []sharedModel.ContentBlock{{Type: "text", Text: "today?"}},
			},
		},
		Tools: []sharedModel.ToolDefiniton{
			{
				Name:        "get_today",
				Description: "Get today's date",
				InputSchema: sharedModel.ToolSchema{Type: "object"},
			},
		},
		ToolChoice: []sharedModel.ToolChoice{{Type: sharedModel.ToolChoiceAuto}},
		MaxTokens:  256,
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	if streamTransport.req.Path != openRouterResponsesPath {
		t.Fatalf("path = %s, want %s", streamTransport.req.Path, openRouterResponsesPath)
	}
	payload, ok := streamTransport.req.Payload.(openRouterReq)
	if !ok {
		t.Fatalf("payload type = %T, want openRouterReq", streamTransport.req.Payload)
	}
	if payload.Instructions != "system prompt" {
		t.Fatalf("instructions = %q, want system prompt", payload.Instructions)
	}
	if len(payload.Input) != 1 || payload.Input[0].Type != "message" || payload.Input[0].Content[0].Type != "input_text" {
		t.Fatalf("input payload = %#v", payload.Input)
	}
	if len(payload.Tools) != 1 || payload.Tools[0].Name != "get_today" {
		t.Fatalf("tools payload = %#v", payload.Tools)
	}
	if res.StopReason != sharedModel.StopReasonToolUse {
		t.Fatalf("stop reason = %s, want %s", res.StopReason, sharedModel.StopReasonToolUse)
	}
	if got, want := len(res.Message.ToolCalls), 1; got != want {
		t.Fatalf("tool call count = %d, want %d", got, want)
	}
	if got, want := res.Message.ToolCalls[0].ID, "call_1"; got != want {
		t.Fatalf("tool call id = %s, want %s", got, want)
	}
	if got, want := res.Usage.InputTokens, 10; got != want {
		t.Fatalf("input tokens = %d, want %d", got, want)
	}
}

func TestOpenRouterStreamProcessesResponsesToolEvents(t *testing.T) {
	streamTransport := &openRouterTestTransport{
		streamEvents: []transport.SSEEvent{
			openRouterSSE(`{"type":"response.created","response":{"id":"resp_1","object":"response","status":"in_progress"}}`),
			openRouterSSE(`{"type":"response.output_item.added","response_id":"resp_1","output_index":1,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"get_today","arguments":""}}`),
			openRouterSSE(`{"type":"response.function_call_arguments.delta","response_id":"resp_1","output_index":1,"delta":"{\"scope\""}`),
			openRouterSSE(`{"type":"response.function_call_arguments.delta","response_id":"resp_1","output_index":1,"delta":":\"today\"}"}`),
			openRouterSSE(`{"type":"response.function_call_arguments.done","response_id":"resp_1","output_index":1,"arguments":"{\"scope\":\"today\"}"}`),
			openRouterSSE(`{"type":"response.done","response":{"id":"resp_1","object":"response","status":"completed","usage":{"input_tokens":9,"output_tokens":4,"total_tokens":13}}}`),
			openRouterSSE(`[DONE]`),
		},
	}
	client := newTestOpenRouterClient(streamTransport)

	step := handler.ProcessStream(
		context.Background(),
		&sharedModel.ChatRequest{},
		client.Stream(context.Background(), sharedModel.ChatRequest{
			Model: "anthropic/claude-haiku-4.5",
			Messages: []sharedModel.AgentMessage{
				{Role: sharedModel.RoleUser, Content: []sharedModel.ContentBlock{{Type: "text", Text: "today?"}}},
			},
		}),
		handler.StreamHandler{},
	)

	if streamTransport.streamReq.Path != openRouterResponsesPath {
		t.Fatalf("path = %s, want %s", streamTransport.streamReq.Path, openRouterResponsesPath)
	}
	payload, ok := streamTransport.streamReq.Payload.(openRouterReq)
	if !ok {
		t.Fatalf("payload type = %T, want openRouterReq", streamTransport.streamReq.Payload)
	}
	if !payload.Stream {
		t.Fatal("stream payload did not set stream=true")
	}
	if step.StopReason != sharedModel.StopReasonToolUse {
		t.Fatalf("stop reason = %s, want %s", step.StopReason, sharedModel.StopReasonToolUse)
	}
	if got, want := len(step.ToolCalls), 1; got != want {
		t.Fatalf("tool call count = %d, want %d: %#v", got, want, step.ToolCalls)
	}
	if got, want := string(step.ToolCalls[0].Arguments), `{"scope":"today"}`; got != want {
		t.Fatalf("tool args = %s, want %s", got, want)
	}
	if got, want := step.Usage.TotalTokens, 13; got != want {
		t.Fatalf("total tokens = %d, want %d", got, want)
	}
}

func TestOpenRouterStreamProcessesContentPartDelta(t *testing.T) {
	streamTransport := &openRouterTestTransport{
		streamEvents: []transport.SSEEvent{
			openRouterSSE(`{"type":"response.created","response":{"id":"resp_1","object":"response","status":"in_progress"}}`),
			openRouterSSE(`{"type":"response.content_part.delta","response_id":"resp_1","output_index":0,"content_index":0,"delta":"Hel"}`),
			openRouterSSE(`{"type":"response.content_part.delta","response_id":"resp_1","output_index":0,"content_index":0,"delta":"lo"}`),
			openRouterSSE(`{"type":"response.done","response":{"id":"resp_1","object":"response","status":"completed","usage":{"input_tokens":2,"output_tokens":1,"total_tokens":3}}}`),
		},
	}
	client := newTestOpenRouterClient(streamTransport)

	step := handler.ProcessStream(
		context.Background(),
		&sharedModel.ChatRequest{},
		client.Stream(context.Background(), sharedModel.ChatRequest{
			Model: "anthropic/claude-haiku-4.5",
			Messages: []sharedModel.AgentMessage{
				{Role: sharedModel.RoleUser, Content: []sharedModel.ContentBlock{{Type: "text", Text: "hello"}}},
			},
		}),
		handler.StreamHandler{},
	)

	if step.StopReason != sharedModel.StopReasonEndTurn {
		t.Fatalf("stop reason = %s, want %s", step.StopReason, sharedModel.StopReasonEndTurn)
	}
	if got, want := step.Text, "Hello"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

func newTestOpenRouterClient(testTransport transport.Transport) *openRouterClient {
	return &openRouterClient{
		httpClient: transport.NewClient(
			"openrouter",
			testTransport,
			transport.WithPropagateInternalHeaders(false),
		),
	}
}

func openRouterSSE(data string) transport.SSEEvent {
	return transport.SSEEvent{
		Data: []byte(data),
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	return data
}
