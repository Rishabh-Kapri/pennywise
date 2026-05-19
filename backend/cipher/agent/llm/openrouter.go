package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
)

const openRouterResponsesPath = "/api/v1/responses"

type openRouterClient struct {
	httpClient *transport.Client
}

type openRouterReq struct {
	Model           string                `json:"model"`
	Input           []openRouterInputItem `json:"input"`
	Instructions    string                `json:"instructions,omitempty"`
	Tools           []openAITool          `json:"tools,omitempty"`
	ToolChoice      any                   `json:"tool_choice,omitempty"`
	Temperature     float32               `json:"temperature,omitempty"`
	MaxOutputTokens int                   `json:"max_output_tokens,omitempty"`
	Metadata        map[string]string     `json:"metadata,omitempty"`
	Stream          bool                  `json:"stream,omitempty"`
}

type openRouterInputItem struct {
	Type      string                   `json:"type"`
	Role      sharedModel.Role         `json:"role,omitempty"`
	ID        string                   `json:"id,omitempty"`
	Status    string                   `json:"status,omitempty"`
	Content   []openRouterContentBlock `json:"content,omitempty"`
	CallID    string                   `json:"call_id,omitempty"`
	Name      string                   `json:"name,omitempty"`
	Arguments string                   `json:"arguments,omitempty"`
	Output    string                   `json:"output,omitempty"`
}

type openRouterContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type openRouterRes struct {
	ID                string                   `json:"id"`
	Model             string                   `json:"model"`
	Output            []openRouterOutputItem   `json:"output"`
	Usage             openRouterUsage          `json:"usage"`
	Status            string                   `json:"status"`
	IncompleteDetails *openAIIncompleteDetails `json:"incomplete_details,omitempty"`
	Error             *openRouterError         `json:"error,omitempty"`
}

type openRouterOutputItem struct {
	ID        string                   `json:"id"`
	Type      string                   `json:"type"`
	Status    string                   `json:"status,omitempty"`
	Role      sharedModel.Role         `json:"role,omitempty"`
	Content   []openRouterContentBlock `json:"content,omitempty"`
	CallID    string                   `json:"call_id,omitempty"`
	Name      string                   `json:"name,omitempty"`
	Arguments string                   `json:"arguments,omitempty"`
}

type openRouterUsage struct {
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	TotalTokens      int `json:"total_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type openRouterError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type openRouterStreamEvent struct {
	Type        string               `json:"type"`
	Response    openRouterRes        `json:"response"`
	OutputIndex int                  `json:"output_index"`
	Item        openRouterOutputItem `json:"item"`
	Delta       string               `json:"delta"`
	Arguments   string               `json:"arguments"`
	Error       *openRouterError     `json:"error,omitempty"`
}

func NewOpenRouterClient() (LLM, error) {
	cfg := config.Load()
	if cfg.OpenRouterAPIKey == "" {
		return nil, errs.New(errs.CodeInternalError, "no openrouter api key found")
	}

	headers := map[string][]string{
		"content-type":  {"application/json"},
		"Authorization": {fmt.Sprintf("Bearer %s", cfg.OpenRouterAPIKey)},
	}

	httpTransport := httpclient.NewHttpTransport("https://openrouter.ai")

	return &openRouterClient{
		httpClient: transport.NewClient(
			"openrouter",
			httpTransport,
			transport.WithDefaultHeaders(headers),
			transport.WithPropagateInternalHeaders(false),
		),
	}, nil
}

func (c *openRouterClient) toOpenRouterReq(req sharedModel.ChatRequest) openRouterReq {
	input, instructions := toOpenRouterInput(req.Messages)
	return openRouterReq{
		Model:           req.Model,
		Input:           input,
		Instructions:    instructions,
		Tools:           toOpenAITools(req.Tools),
		ToolChoice:      toOpenAIToolChoice(req.ToolChoice),
		Temperature:     req.Temperature,
		MaxOutputTokens: req.MaxTokens,
		Metadata:        req.Metadata,
		Stream:          req.Stream,
	}
}

func toOpenRouterInput(messages []sharedModel.AgentMessage) ([]openRouterInputItem, string) {
	out := make([]openRouterInputItem, 0, len(messages))
	var instructions strings.Builder

	for i, msg := range messages {
		if msg.Role == sharedModel.RoleSystem {
			text := contentBlocksText(msg.Content)
			if text != "" {
				if instructions.Len() > 0 {
					instructions.WriteString("\n\n")
				}
				instructions.WriteString(text)
			}
			continue
		}

		if msg.ToolResult != nil {
			callID := msg.ToolResult.ToolCallId
			out = append(out, openRouterInputItem{
				Type:   "function_call_output",
				ID:     openRouterHistoryID("fc_output", i, callID),
				CallID: callID,
				Output: contentBlocksText(msg.ToolResult.Content),
			})
			continue
		}

		if len(msg.Content) > 0 {
			contentType := "input_text"
			item := openRouterInputItem{
				Type:    "message",
				Role:    msg.Role,
				Content: toOpenRouterContent(msg.Content, contentType),
			}
			if msg.Role == sharedModel.RoleAssistant {
				item.ID = openRouterHistoryID("msg", i, "")
				item.Status = "completed"
				item.Content = toOpenRouterContent(msg.Content, "output_text")
			}
			out = append(out, item)
		}

		for j, call := range msg.ToolCalls {
			callID := call.ID
			out = append(out, openRouterInputItem{
				Type:      "function_call",
				ID:        openRouterHistoryID("fc", i+j, callID),
				CallID:    callID,
				Name:      call.Name,
				Arguments: openRouterArguments(call.Arguments),
			})
		}
	}

	return out, instructions.String()
}

func toOpenRouterContent(blocks []sharedModel.ContentBlock, blockType string) []openRouterContentBlock {
	out := make([]openRouterContentBlock, 0, len(blocks))
	for _, block := range blocks {
		if block.Type != "text" || block.Text == "" {
			continue
		}
		out = append(out, openRouterContentBlock{
			Type: blockType,
			Text: block.Text,
		})
	}
	return out
}

func openRouterArguments(args json.RawMessage) string {
	if len(args) == 0 {
		return "{}"
	}
	return string(args)
}

func openRouterHistoryID(prefix string, index int, fallback string) string {
	if fallback == "" {
		return fmt.Sprintf("%s_%d", prefix, index)
	}

	var b strings.Builder
	for _, r := range fallback {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return fmt.Sprintf("%s_%d", prefix, index)
	}
	return fmt.Sprintf("%s_%s", prefix, b.String())
}

func (c *openRouterClient) fromOpenRouterRes(res openRouterRes) (sharedModel.ChatResponse, error) {
	if res.Error != nil && res.Error.Message != "" {
		return sharedModel.ChatResponse{}, errs.New(errs.CodeInternalError, "openrouter: %s", res.Error.Message)
	}
	if len(res.Output) == 0 {
		return sharedModel.ChatResponse{}, errs.New(errs.CodeInternalError, "openrouter: no output returned")
	}

	content := make([]sharedModel.ContentBlock, 0)
	toolCalls := make([]sharedModel.ToolCall, 0)
	for _, output := range res.Output {
		switch output.Type {
		case "message":
			for _, item := range output.Content {
				if item.Type == "output_text" && item.Text != "" {
					content = append(content, sharedModel.ContentBlock{Type: "text", Text: item.Text})
				}
			}
		case "function_call":
			toolCalls = append(toolCalls, sharedModel.ToolCall{
				ID:        openRouterCallID(output),
				Name:      output.Name,
				Arguments: json.RawMessage(openRouterArguments(json.RawMessage(output.Arguments))),
			})
		}
	}

	return sharedModel.ChatResponse{
		ID:    res.ID,
		Model: res.Model,
		Message: sharedModel.AgentMessage{
			Role:      sharedModel.RoleAssistant,
			Content:   content,
			ToolCalls: toolCalls,
		},
		Usage:       toOpenRouterUsage(res.Usage),
		StopReason:  toOpenRouterStopReason(res),
		RawProvider: res,
	}, nil
}

func openRouterCallID(item openRouterOutputItem) string {
	if item.CallID != "" {
		return item.CallID
	}
	return item.ID
}

func toOpenRouterUsage(usage openRouterUsage) sharedModel.Usage {
	inputTokens := usage.InputTokens
	if inputTokens == 0 {
		inputTokens = usage.PromptTokens
	}

	outputTokens := usage.OutputTokens
	if outputTokens == 0 {
		outputTokens = usage.CompletionTokens
	}

	totalTokens := usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = inputTokens + outputTokens
	}

	return sharedModel.Usage{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  totalTokens,
	}
}

func toOpenRouterStopReason(res openRouterRes) sharedModel.StopReason {
	for _, output := range res.Output {
		if output.Type == "function_call" {
			return sharedModel.StopReasonToolUse
		}
	}

	if res.IncompleteDetails != nil && res.IncompleteDetails.Reason == "max_output_tokens" {
		return sharedModel.StopReasonMaxTokens
	}

	switch res.Status {
	case "completed":
		return sharedModel.StopReasonEndTurn
	case "incomplete":
		return sharedModel.StopReasonMaxTokens
	default:
		return sharedModel.StopReasonError
	}
}

func (c *openRouterClient) Chat(ctx context.Context, req sharedModel.ChatRequest) (*sharedModel.ChatResponse, error) {
	log := logger.Logger(ctx)
	openRouterReq := c.toOpenRouterReq(req)
	res, err := transport.Post[openRouterRes](ctx, c.httpClient, openRouterResponsesPath, nil, openRouterReq)
	if err != nil {
		log.Error("error while sending openrouter /api/v1/responses", "error", err)
		return nil, err
	}

	chatRes, err := c.fromOpenRouterRes(res)
	if err != nil {
		return nil, err
	}

	return &chatRes, nil
}

func (c *openRouterClient) Stream(ctx context.Context, req sharedModel.ChatRequest) <-chan sharedModel.StreamChunk {
	log := logger.Logger(ctx)
	openRouterReq := c.toOpenRouterReq(req)
	openRouterReq.Stream = true
	events := make(chan sharedModel.StreamChunk, 1)

	headers := map[string][]string{
		"Accept": {"text/event-stream"},
	}
	res, err := transport.StreamPost(ctx, c.httpClient, openRouterResponsesPath, headers, openRouterReq)
	if err != nil {
		log.Error("error while sending openrouter streaming /api/v1/responses", "error", err)
		events <- sharedModel.StreamChunk{
			Type: sharedModel.ChunkEventError,
			Text: err.Error(),
		}
		close(events)
		return events
	}

	go func() {
		defer close(events)

		usage := sharedModel.Usage{}
		toolArgs := map[int]*strings.Builder{}
		startedTools := map[int]bool{}
		completedTools := map[int]bool{}
		sawTextDelta := false

		sendToolArgs := func(outputIndex int, delta string) bool {
			if delta == "" {
				return true
			}
			builder := toolArgs[outputIndex]
			if builder == nil {
				builder = &strings.Builder{}
				toolArgs[outputIndex] = builder
			}
			builder.WriteString(delta)
			return sendOpenRouterChunk(ctx, events, sharedModel.StreamChunk{
				Type:          sharedModel.ChunkEventToolCallDelta,
				ToolArgsDelta: delta,
				OutputIndex:   outputIndex,
			})
		}

		sendToolDone := func(outputIndex int) bool {
			if completedTools[outputIndex] {
				return true
			}
			completedTools[outputIndex] = true
			return sendOpenRouterChunk(ctx, events, sharedModel.StreamChunk{
				Type:        sharedModel.ChunkEventToolCall,
				OutputIndex: outputIndex,
			})
		}

		for event := range res.Events {
			data := strings.TrimSpace(string(event.Data))
			if data == "" {
				continue
			}
			if data == "[DONE]" {
				sendOpenRouterChunk(ctx, events, sharedModel.StreamChunk{
					Type:  sharedModel.ChunkEventCompleted,
					Usage: usage,
				})
				return
			}

			var ev openRouterStreamEvent
			if err := json.Unmarshal([]byte(data), &ev); err != nil {
				log.Error("error while unmarshalling openrouter stream event", "event", event.Event, "error", err)
				sendOpenRouterChunk(ctx, events, sharedModel.StreamChunk{
					Type: sharedModel.ChunkEventError,
					Text: err.Error(),
				})
				return
			}

			eventType := event.Event
			if eventType == "" {
				eventType = ev.Type
			}

			switch eventType {
			case "response.created", "response.in_progress":
				if !sendOpenRouterChunk(ctx, events, sharedModel.StreamChunk{Type: sharedModel.ChunkEventStarted}) {
					return
				}

			case "response.content_part.delta", "response.output_text.delta":
				if ev.Delta == "" {
					continue
				}
				sawTextDelta = true
				if !sendOpenRouterChunk(ctx, events, sharedModel.StreamChunk{
					Type: sharedModel.ChunkEventText,
					Text: ev.Delta,
				}) {
					return
				}

			case "response.output_item.added":
				if ev.Item.Type != "function_call" {
					continue
				}
				startedTools[ev.OutputIndex] = true
				if !sendOpenRouterChunk(ctx, events, sharedModel.StreamChunk{
					Type:        sharedModel.ChunkEventToolCallStart,
					ToolCallID:  openRouterCallID(ev.Item),
					ToolName:    ev.Item.Name,
					OutputIndex: ev.OutputIndex,
				}) {
					return
				}

			case "response.function_call_arguments.delta":
				if !sendToolArgs(ev.OutputIndex, ev.Delta) {
					return
				}

			case "response.function_call_arguments.done":
				if builder := toolArgs[ev.OutputIndex]; builder == nil || builder.Len() == 0 {
					if !sendToolArgs(ev.OutputIndex, ev.Arguments) {
						return
					}
				}
				if !sendToolDone(ev.OutputIndex) {
					return
				}

			case "response.output_item.done":
				switch ev.Item.Type {
				case "message":
					if sawTextDelta {
						continue
					}
					for _, content := range ev.Item.Content {
						if content.Type == "output_text" && content.Text != "" {
							if !sendOpenRouterChunk(ctx, events, sharedModel.StreamChunk{
								Type: sharedModel.ChunkEventText,
								Text: content.Text,
							}) {
								return
							}
						}
					}
				case "function_call":
					if !startedTools[ev.OutputIndex] {
						startedTools[ev.OutputIndex] = true
						if !sendOpenRouterChunk(ctx, events, sharedModel.StreamChunk{
							Type:        sharedModel.ChunkEventToolCallStart,
							ToolCallID:  openRouterCallID(ev.Item),
							ToolName:    ev.Item.Name,
							OutputIndex: ev.OutputIndex,
						}) {
							return
						}
					}
					if builder := toolArgs[ev.OutputIndex]; builder == nil || builder.Len() == 0 {
						if !sendToolArgs(ev.OutputIndex, ev.Item.Arguments) {
							return
						}
					}
					if !sendToolDone(ev.OutputIndex) {
						return
					}
				}

			case "response.done", "response.completed":
				usage = toOpenRouterUsage(ev.Response.Usage)
				sendOpenRouterChunk(ctx, events, sharedModel.StreamChunk{
					Type:  sharedModel.ChunkEventCompleted,
					Usage: usage,
				})
				return

			case "response.failed", "response.error", "error":
				message := "openrouter stream error"
				if ev.Error != nil && ev.Error.Message != "" {
					message = ev.Error.Message
				} else if ev.Response.Error != nil && ev.Response.Error.Message != "" {
					message = ev.Response.Error.Message
				}
				sendOpenRouterChunk(ctx, events, sharedModel.StreamChunk{
					Type: sharedModel.ChunkEventError,
					Text: message,
				})
				return
			}
		}
	}()

	return events
}

func sendOpenRouterChunk(
	ctx context.Context,
	events chan<- sharedModel.StreamChunk,
	chunk sharedModel.StreamChunk,
) bool {
	select {
	case events <- chunk:
		return true
	case <-ctx.Done():
		return false
	}
}
