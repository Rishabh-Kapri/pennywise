package handler

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

type StreamHandler struct {
	OnTextDelta     func(textDelta string)
	OnToolCallStart func(ctx context.Context, tool sharedModel.ToolCall)
	OnToolCall      func(ctx context.Context, tool sharedModel.ToolCall)
	OnDone          func(usage sharedModel.Usage)
	OnComplete      func()
	OnError         func()
}

type eventToolCall struct {
	id   string
	name string
	args strings.Builder
}

func appendCompletedToolCall(
	stepResult *sharedModel.StepResult,
	callback StreamHandler,
	ctx context.Context,
	activeTool *eventToolCall,
) bool {
	if activeTool == nil {
		return false
	}

	toolArgs := activeTool.args.String()
	if toolArgs == "" {
		toolArgs = "{}"
	}
	if !json.Valid([]byte(toolArgs)) {
		return false
	}

	tool := sharedModel.ToolCall{
		ID:        activeTool.id,
		Name:      activeTool.name,
		Arguments: json.RawMessage(toolArgs),
	}
	if callback.OnToolCall != nil {
		callback.OnToolCall(ctx, tool)
	}
	stepResult.ToolCalls = append(stepResult.ToolCalls, tool)
	return true
}

/* ProcessStream accumulates the stream events into a single StepResult for that LLM call
* Additionally it can call supplied callbacks for side effects, like logging, telemtry etc.
 */
func ProcessStream(
	ctx context.Context,
	req *sharedModel.ChatRequest,
	events <-chan sharedModel.StreamChunk,
	handlerCallback StreamHandler,
) sharedModel.StepResult {
	log := logger.Logger(ctx)
	stepResult := sharedModel.StepResult{MaxTokens: req.MaxTokens}
	var text strings.Builder
	var hasFunctionCall bool

	activeTools := make(map[int]*eventToolCall)

	// this blocks until the stream is closed or the context is cancelled
	// used to accumulate the stream events into a single StepResult
	for {
		select {
		case event, ok := <-events:
			// log.Info("received event from channel", "event", event)
			if !ok {
				log.Info("channel closed")
				if stepResult.StopReason == "" {
					stepResult.StopReason = sharedModel.StopReasonError
				}
				return stepResult
			}
			switch event.Type {
			case sharedModel.ChunkEventText:
				text.WriteString(event.Text)
				if handlerCallback.OnTextDelta != nil {
					handlerCallback.OnTextDelta(event.Text)
				}

			case sharedModel.ChunkEventToolCallStart:
				hasFunctionCall = true
				// create new eventTool
				eventTool := activeTools[event.OutputIndex]
				if eventTool == nil {
					eventTool = &eventToolCall{}
					activeTools[event.OutputIndex] = eventTool
				}
				if event.ToolCallID != "" {
					eventTool.id = event.ToolCallID
				}
				if event.ToolName != "" {
					eventTool.name = event.ToolName
				}
				if handlerCallback.OnToolCallStart != nil {
					handlerCallback.OnToolCallStart(ctx, sharedModel.ToolCall{
						ID:   event.ToolCallID,
						Name: event.ToolName,
					})
				}

			case sharedModel.ChunkEventToolCallDelta:
				activeTool := activeTools[event.OutputIndex]
				if activeTool == nil {
					activeTool = &eventToolCall{}
					activeTools[event.OutputIndex] = activeTool
				}
				activeTool.args.WriteString(event.ToolArgsDelta)

			case sharedModel.ChunkEventToolCall:
				// tool call stream is done
				activeTool := activeTools[event.OutputIndex]
				if appendCompletedToolCall(&stepResult, handlerCallback, ctx, activeTool) {
					activeTool.args.Reset()
					delete(activeTools, event.OutputIndex)
				}

			case sharedModel.ChunkEventCompleted:
				for outputIndex, activeTool := range activeTools {
					if appendCompletedToolCall(&stepResult, handlerCallback, ctx, activeTool) {
						activeTool.args.Reset()
						delete(activeTools, outputIndex)
					}
				}
				// handle tool call here too
				if handlerCallback.OnDone != nil {
					handlerCallback.OnDone(event.Usage)
				}
				stepResult.Usage = event.Usage
				stepResult.Text = text.String()
				text.Reset()
				if hasFunctionCall || len(stepResult.ToolCalls) > 0 {
					stepResult.StopReason = sharedModel.StopReasonToolUse
				} else if event.StopReason != "" {
					stepResult.StopReason = event.StopReason
				} else {
					stepResult.StopReason = sharedModel.StopReasonEndTurn
				}
				return stepResult

			case sharedModel.ChunkEventError:
				if handlerCallback.OnError != nil {
					handlerCallback.OnError()
				}
				stepResult.StopReason = sharedModel.StopReasonError
				return stepResult
			}
		case <-ctx.Done():
			stepResult.Err = ctx.Err()
			stepResult.StopReason = sharedModel.StopReasonError
			return stepResult
		}
	}
}
