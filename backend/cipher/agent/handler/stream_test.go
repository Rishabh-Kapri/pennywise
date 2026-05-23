package handler

import (
	"context"
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

func TestProcessStreamIgnoresToolDoneWithoutStart(t *testing.T) {
	events := make(chan sharedModel.StreamChunk, 3)
	events <- sharedModel.StreamChunk{Type: sharedModel.ChunkEventText, Text: "hello"}
	events <- sharedModel.StreamChunk{Type: sharedModel.ChunkEventToolCall, OutputIndex: 0}
	events <- sharedModel.StreamChunk{Type: sharedModel.ChunkEventCompleted}
	close(events)

	result := ProcessStream(context.Background(), &sharedModel.ChatRequest{}, events, StreamHandler{})
	if result.StopReason != sharedModel.StopReasonEndTurn {
		t.Fatalf("stop reason = %s, want %s", result.StopReason, sharedModel.StopReasonEndTurn)
	}
	if result.Text != "hello" {
		t.Fatalf("text = %q, want hello", result.Text)
	}
	if len(result.ToolCalls) != 0 {
		t.Fatalf("tool calls = %#v, want none", result.ToolCalls)
	}
}

func TestProcessStreamRecordsRequestMaxTokens(t *testing.T) {
	events := make(chan sharedModel.StreamChunk, 1)
	events <- sharedModel.StreamChunk{Type: sharedModel.ChunkEventCompleted}
	close(events)

	result := ProcessStream(context.Background(), &sharedModel.ChatRequest{MaxTokens: 4096}, events, StreamHandler{})
	if got, want := result.MaxTokens, 4096; got != want {
		t.Fatalf("max tokens = %d, want %d", got, want)
	}
}

func TestProcessStreamUsesCompletedChunkStopReason(t *testing.T) {
	events := make(chan sharedModel.StreamChunk, 1)
	events <- sharedModel.StreamChunk{
		Type:       sharedModel.ChunkEventCompleted,
		StopReason: sharedModel.StopReasonMaxTokens,
	}
	close(events)

	result := ProcessStream(context.Background(), &sharedModel.ChatRequest{}, events, StreamHandler{})
	if result.StopReason != sharedModel.StopReasonMaxTokens {
		t.Fatalf("stop reason = %s, want %s", result.StopReason, sharedModel.StopReasonMaxTokens)
	}
}

func TestProcessStreamDefaultsEmptyToolArgsToObject(t *testing.T) {
	events := make(chan sharedModel.StreamChunk, 3)
	events <- sharedModel.StreamChunk{
		Type:        sharedModel.ChunkEventToolCallStart,
		ToolCallID:  "toolu_123",
		ToolName:    "get_today",
		OutputIndex: 1,
	}
	events <- sharedModel.StreamChunk{Type: sharedModel.ChunkEventToolCall, OutputIndex: 1}
	events <- sharedModel.StreamChunk{Type: sharedModel.ChunkEventCompleted}
	close(events)

	result := ProcessStream(context.Background(), &sharedModel.ChatRequest{}, events, StreamHandler{})
	if result.StopReason != sharedModel.StopReasonToolUse {
		t.Fatalf("stop reason = %s, want %s", result.StopReason, sharedModel.StopReasonToolUse)
	}
	if got, want := len(result.ToolCalls), 1; got != want {
		t.Fatalf("tool call count = %d, want %d", got, want)
	}
	if got, want := string(result.ToolCalls[0].Arguments), "{}"; got != want {
		t.Fatalf("tool args = %s, want %s", got, want)
	}
}
