package providers

import (
	"context"
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
)

type anthropicStreamTestTransport struct {
	streamReq    *transport.Request
	streamEvents []transport.SSEEvent
}

func (t *anthropicStreamTestTransport) Send(
	ctx context.Context,
	req *transport.Request,
) (transport.Response, error) {
	return transport.Response{}, nil
}

func (t *anthropicStreamTestTransport) Stream(
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

func TestAnthropicStreamTextAndUsage(t *testing.T) {
	streamTransport := &anthropicStreamTestTransport{
		streamEvents: []transport.SSEEvent{
			anthropicSSE("message_start", `{"type":"message_start","message":{"id":"msg_1","model":"claude-sonnet-4-6","usage":{"input_tokens":12,"output_tokens":1}}}`),
			anthropicSSE("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
			anthropicSSE("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hel"}}`),
			anthropicSSE("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"lo"}}`),
			anthropicSSE("content_block_stop", `{"type":"content_block_stop","index":0}`),
			anthropicSSE("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}`),
			anthropicSSE("message_stop", `{"type":"message_stop"}`),
		},
	}
	client := newTestAnthropicClient(streamTransport)

	chunks := collectAnthropicStream(client.Stream(context.Background(), sharedModel.ChatRequest{
		Model: "claude-sonnet-4-6",
		Messages: []sharedModel.AgentMessage{
			{
				Role: sharedModel.RoleUser,
				Content: []sharedModel.ContentBlock{
					{Type: "text", Text: "hello"},
				},
			},
		},
	}))

	if got, want := len(chunks), 5; got != want {
		t.Fatalf("chunk count = %d, want %d: %#v", got, want, chunks)
	}
	if chunks[0].Type != sharedModel.ChunkEventStarted {
		t.Fatalf("first chunk = %s, want %s", chunks[0].Type, sharedModel.ChunkEventStarted)
	}
	if chunks[1].Type != sharedModel.ChunkEventText || chunks[1].Text != "Hel" {
		t.Fatalf("second chunk = %#v, want text Hel", chunks[1])
	}
	if chunks[2].Type != sharedModel.ChunkEventText || chunks[2].Text != "lo" {
		t.Fatalf("third chunk = %#v, want text lo", chunks[2])
	}
	if chunks[3].Type != sharedModel.ChunkEventToolCall {
		t.Fatalf("fourth chunk = %s, want %s", chunks[3].Type, sharedModel.ChunkEventToolCall)
	}
	if chunks[4].Type != sharedModel.ChunkEventCompleted {
		t.Fatalf("final chunk = %s, want %s", chunks[4].Type, sharedModel.ChunkEventCompleted)
	}
	if got, want := chunks[4].Usage.InputTokens, 12; got != want {
		t.Fatalf("input tokens = %d, want %d", got, want)
	}
	if got, want := chunks[4].Usage.OutputTokens, 7; got != want {
		t.Fatalf("output tokens = %d, want %d", got, want)
	}
	if got, want := chunks[4].Usage.TotalTokens, 19; got != want {
		t.Fatalf("total tokens = %d, want %d", got, want)
	}

	payload, ok := streamTransport.streamReq.Payload.(anthropicReq)
	if !ok {
		t.Fatalf("payload type = %T, want anthropicReq", streamTransport.streamReq.Payload)
	}
	if !payload.Stream {
		t.Fatal("stream payload did not set stream=true")
	}
}

func TestAnthropicStreamToolUse(t *testing.T) {
	streamTransport := &anthropicStreamTestTransport{
		streamEvents: []transport.SSEEvent{
			anthropicSSE("message_start", `{"type":"message_start","message":{"id":"msg_1","model":"claude-sonnet-4-6","usage":{"input_tokens":20,"output_tokens":1}}}`),
			anthropicSSE("content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_123","name":"get_budget_info","input":{}}}`),
			anthropicSSE("content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"scope\""}}`),
			anthropicSSE("content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":":\"summary\"}"}}`),
			anthropicSSE("content_block_stop", `{"type":"content_block_stop","index":1}`),
			anthropicSSE("message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":9}}`),
			anthropicSSE("message_stop", `{"type":"message_stop"}`),
		},
	}
	client := newTestAnthropicClient(streamTransport)

	chunks := collectAnthropicStream(client.Stream(context.Background(), sharedModel.ChatRequest{
		Model: "claude-sonnet-4-6",
		Messages: []sharedModel.AgentMessage{
			{
				Role: sharedModel.RoleUser,
				Content: []sharedModel.ContentBlock{
					{Type: "text", Text: "summarize"},
				},
			},
		},
	}))

	if got, want := len(chunks), 6; got != want {
		t.Fatalf("chunk count = %d, want %d: %#v", got, want, chunks)
	}
	if chunks[1].Type != sharedModel.ChunkEventToolCallStart {
		t.Fatalf("tool start chunk = %s, want %s", chunks[1].Type, sharedModel.ChunkEventToolCallStart)
	}
	if got, want := chunks[1].ToolCallID, "toolu_123"; got != want {
		t.Fatalf("tool id = %s, want %s", got, want)
	}
	if got, want := chunks[1].ToolName, "get_budget_info"; got != want {
		t.Fatalf("tool name = %s, want %s", got, want)
	}
	if got, want := chunks[1].OutputIndex, 1; got != want {
		t.Fatalf("tool output index = %d, want %d", got, want)
	}
	if chunks[2].Type != sharedModel.ChunkEventToolCallDelta || chunks[2].ToolArgsDelta != `{"scope"` {
		t.Fatalf("first tool delta = %#v", chunks[2])
	}
	if chunks[3].Type != sharedModel.ChunkEventToolCallDelta || chunks[3].ToolArgsDelta != `:"summary"}` {
		t.Fatalf("second tool delta = %#v", chunks[3])
	}
	if chunks[4].Type != sharedModel.ChunkEventToolCall || chunks[4].OutputIndex != 1 {
		t.Fatalf("tool done chunk = %#v", chunks[4])
	}
	if chunks[5].Type != sharedModel.ChunkEventCompleted {
		t.Fatalf("final chunk = %s, want %s", chunks[5].Type, sharedModel.ChunkEventCompleted)
	}
}

func newTestAnthropicClient(streamTransport transport.Transport) *anthropicClient {
	return &anthropicClient{
		httpClient: transport.NewClient(
			"anthropic",
			streamTransport,
			transport.WithPropagateInternalHeaders(false),
		),
	}
}

func anthropicSSE(event string, data string) transport.SSEEvent {
	return transport.SSEEvent{
		Event: event,
		Data:  []byte(data),
	}
}

func collectAnthropicStream(events <-chan sharedModel.StreamChunk) []sharedModel.StreamChunk {
	var chunks []sharedModel.StreamChunk
	for event := range events {
		chunks = append(chunks, event)
	}
	return chunks
}
