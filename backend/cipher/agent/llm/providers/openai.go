package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
)

const openAIResponsesPath = "/v1/responses"

type openAIClient struct {
	httpClient *transport.Client
}

type openAIReq struct {
	Model           string        `json:"model"`
	Input           []openAIInput `json:"input"`
	Tools           []openAITool  `json:"tools,omitempty"`
	ToolChoice      any           `json:"tool_choice,omitempty"`
	Temperature     float32       `json:"temperature,omitempty"`
	MaxOutputTokens int           `json:"max_output_tokens,omitempty"`
	Stream          bool          `json:"stream,omitempty"`
}

type openAIInput struct {
	Type      string `json:"type,omitempty"`
	Role      string `json:"role,omitempty"`
	Content   string `json:"content,omitempty"`
	Output    string `json:"output,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type openAITool struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type openAIRes struct {
	ID                string                   `json:"id"`
	Model             string                   `json:"model"`
	Output            []openAIOutput           `json:"output"`
	Usage             openAIUsage              `json:"usage"`
	Status            string                   `json:"status"`
	IncompleteDetails *openAIIncompleteDetails `json:"incomplete_details,omitempty"`
}

type openAIOutput struct {
	ID        string                `json:"id"`
	Type      string                `json:"type"`
	Role      string                `json:"role"`
	Content   []openAIOutputContent `json:"content"`
	CallID    string                `json:"call_id"`
	Name      string                `json:"name"`
	Arguments string                `json:"arguments"`
}

type openAIOutputContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type openAIIncompleteDetails struct {
	Reason string `json:"reason"`
}

type openAIUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

func NewOpenAIClient() (llm.LLM, error) {
	cfg := config.Load()
	if cfg.OpenAIAPIKey == "" {
		return nil, errs.New(errs.CodeInternalError, "no openai api key found")
	}

	headers := map[string][]string{
		"content-type":  {"application/json"},
		"Authorization": {fmt.Sprintf("Bearer %s", cfg.OpenAIAPIKey)},
	}

	httpTransport := httpclient.NewHttpTransport("https://api.openai.com")

	return &openAIClient{
		httpClient: transport.NewClient(
			"openai",
			httpTransport,
			transport.WithDefaultHeaders(headers),
			transport.WithPropagateInternalHeaders(false),
		),
	}, nil
}

func (c *openAIClient) toOpenAIReq(req sharedModel.ChatRequest) openAIReq {
	return openAIReq{
		Model:           req.Model,
		Input:           toOpenAIInput(req.Messages),
		Tools:           toOpenAITools(req.Tools),
		ToolChoice:      toOpenAIToolChoice(req.ToolChoice),
		Temperature:     req.Temperature,
		MaxOutputTokens: req.MaxTokens,
		Stream:          req.Stream,
	}
}

func toOpenAIInput(messages []sharedModel.AgentMessage) []openAIInput {
	out := make([]openAIInput, 0, len(messages))

	for _, msg := range messages {
		if msg.ToolResult != nil {
			out = append(out, openAIInput{
				Type:   "function_call_output",
				CallID: msg.ToolResult.ToolCallId,
				Output: contentBlocksText(msg.ToolResult.Content),
			})
			continue
		}

		if len(msg.Content) > 0 || len(msg.ToolCalls) == 0 {
			out = append(out, openAIInput{
				Role:    string(msg.Role),
				Content: contentBlocksText(msg.Content),
			})
		}

		for _, call := range msg.ToolCalls {
			out = append(out, toOpenAIFunctionCall(call))
		}
	}

	return out
}

func toOpenAIFunctionCall(call sharedModel.ToolCall) openAIInput {
	return openAIInput{
		Type:      "function_call",
		CallID:    call.ID,
		Name:      call.Name,
		Arguments: string(call.Arguments),
	}
}

func toOpenAITools(tools []sharedModel.ToolDefiniton) []openAITool {
	out := make([]openAITool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, openAITool{
			Type:        "function",
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  toOpenAISchema(tool.InputSchema),
		})
	}
	return out
}

func toOpenAISchema(schema sharedModel.ToolSchema) map[string]any {
	out := map[string]any{
		"type": schema.Type,
	}

	if schema.Description != "" {
		out["description"] = schema.Description
	}

	if schema.Enum != nil {
		out["enum"] = *schema.Enum
	}

	if schema.Type == "object" {
		properties := make(map[string]any, len(schema.Properties))
		for name, property := range schema.Properties {
			properties[name] = toOpenAISchema(property)
		}
		// OpenAI requires object schemas to include properties, even when empty.
		out["properties"] = properties
		if schema.AdditionalProperties {
			out["additionalProperties"] = true
		}
	}

	if schema.Type == "array" && schema.Items != nil {
		out["items"] = toOpenAISchema(*schema.Items)
	}

	if len(schema.Required) > 0 {
		out["required"] = schema.Required
	}

	return out
}

func toOpenAIToolChoice(choices []sharedModel.ToolChoice) any {
	if len(choices) == 0 {
		return nil
	}

	choice := choices[0]
	switch choice.Type {
	case sharedModel.ToolChoiceAuto:
		return "auto"
	case sharedModel.ToolChoiceNone:
		return "none"
	case sharedModel.ToolChoiceRequired:
		return "required"
	case sharedModel.ToolChoiceSpecific:
		return map[string]any{
			"type": "function",
			"name": choice.Name,
		}
	default:
		return nil
	}
}

func (c *openAIClient) fromOpenAIRes(res openAIRes) (sharedModel.ChatResponse, error) {
	if len(res.Output) == 0 {
		return sharedModel.ChatResponse{}, errs.New(errs.CodeInternalError, "openai: no output returned")
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
				ID:        output.CallID,
				Name:      output.Name,
				Arguments: json.RawMessage(output.Arguments),
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
		Usage: sharedModel.Usage{
			InputTokens:  res.Usage.InputTokens,
			OutputTokens: res.Usage.OutputTokens,
			TotalTokens:  res.Usage.TotalTokens,
		},
		StopReason:  toOpenAIStopReason(res),
		RawProvider: res,
	}, nil
}

func toOpenAIStopReason(res openAIRes) sharedModel.StopReason {
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

func contentBlocksText(blocks []sharedModel.ContentBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Type == "text" && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func (c *openAIClient) Chat(ctx context.Context, req sharedModel.ChatRequest) (*sharedModel.ChatResponse, error) {
	log := logger.Logger(ctx)
	openAIReq := c.toOpenAIReq(req)
	log.Info("open ai req", "req", openAIReq)
	res, err := transport.Post[openAIRes](ctx, c.httpClient, openAIResponsesPath, nil, openAIReq)
	if err != nil {
		log.Error("error while sending /v1/responses", "error", err)
		return nil, err
	}

	chatRes, err := c.fromOpenAIRes(res)
	if err != nil {
		return nil, err
	}

	log.Info("/v1/responses", "res", chatRes)
	return &chatRes, nil
}

func (c *openAIClient) Stream(ctx context.Context, req sharedModel.ChatRequest) <-chan sharedModel.StreamChunk {
	log := logger.Logger(ctx)
	openAIReq := c.toOpenAIReq(req)
	log.Info("sending stream req")
	events := make(chan sharedModel.StreamChunk)

	headers := map[string][]string{
		"Accept": {"text/event-stream"},
	}
	res, err := transport.StreamPost(ctx, c.httpClient, openAIResponsesPath, headers, openAIReq)
	if err != nil {
		log.Error("error while sending openai streaming /v1/messages", "error", err)
		close(events)
		return events
	}

	log.Info("stream req done", "res", res, "error", err)

	go func() {
		defer func() {
			close(events)
		}()

		log.Info("stream call", "res", res, "error", err)
		for event := range res.Events {
			// parsed, _ := utils.UnmarshalResponse[any](event.Data)
			// log.Info("event loop", "event", event.Event, "data", parsed)
			// log.Info("event loop", "event", event.Event)

			switch event.Event {
			case "response.created":
				events <- sharedModel.StreamChunk{
					Type: sharedModel.ChunkEventStarted,
				}
			case "response.output_item.added":
				var ev struct {
					OutputIndex int `json:"output_index"`
					Item        struct {
						ID   string `json:"id"`
						Type string `json:"type"` // function_call, message
						Name string `json:"name"` // function name
						Role string `json:"role"` // assistant
					} `json:"item"`
				}
				if json.Unmarshal(event.Data, &ev) == nil {
					switch ev.Item.Type {
					case "message":
						// simple message call, skip
					case "reasoning":
						// @TODO: support later
						return
					case "function_call":
						// function call started, send a tool call event
						events <- sharedModel.StreamChunk{
							Type:        sharedModel.ChunkEventToolCallStart,
							ToolCallID:  ev.Item.ID,
							ToolName:    ev.Item.Name,
							OutputIndex: ev.OutputIndex,
						}
					}
				} else {
					return
				}
			case "response.output_text.delta":
				var ev struct {
					Delta string `json:"delta"`
				}
				if json.Unmarshal(event.Data, &ev) == nil {
					events <- sharedModel.StreamChunk{
						Type: sharedModel.ChunkEventText,
						Text: ev.Delta,
					}
				} else {
					return
				}
			case "response.function_call_arguments.delta":
				var ev struct {
					OutputIndex int    `json:"output_index"`
					Delta       string `json:"delta"`
				}
				if json.Unmarshal(event.Data, &ev) == nil {
					events <- sharedModel.StreamChunk{
						Type:          sharedModel.ChunkEventToolCallDelta,
						ToolArgsDelta: ev.Delta,
						OutputIndex:   ev.OutputIndex,
					}
				} else {
					return
				}
			case "response.function_call_arguments.done":
				var ev struct {
					OutputIndex int `json:"output_index"`
				}
				if json.Unmarshal(event.Data, &ev) == nil {
					events <- sharedModel.StreamChunk{
						Type:        sharedModel.ChunkEventToolCall, // this event should merge all the tool related chunks
						OutputIndex: ev.OutputIndex,
					}
				} else {
					return
				}
			// @TODO: handle incomplete here too
			case "response.completed":
				var ev struct {
					Response struct {
						ID                string `json:"id"`
						Model             string `json:"model"`
						Status            string `json:"status"`
						IncompleteDetails *struct {
							Reason string `json:"reason"`
						} `json:"incomplete_details"`
						Usage struct {
							InputTokens         int `json:"input_tokens"`
							OutputTokens        int `json:"output_tokens"`
							OutputTokensDetails *struct {
								ReasoningTokens int `json:"reasoning_tokens"`
							} `json:"output_tokens_details"`
							InputTokensDetails *struct {
								CachedTokens int `json:"cached_tokens"`
							} `json:"input_tokens_details"`
						} `json:"usage"`
					} `json:"response"`
				}
				if json.Unmarshal(event.Data, &ev) == nil {
					response := openAIRes{
						Status: ev.Response.Status,
						IncompleteDetails: func() *openAIIncompleteDetails {
							if ev.Response.IncompleteDetails == nil {
								return nil
							}
							return &openAIIncompleteDetails{Reason: ev.Response.IncompleteDetails.Reason}
						}(),
					}
					usage := sharedModel.Usage{
						InputTokens:  ev.Response.Usage.InputTokens,
						OutputTokens: ev.Response.Usage.OutputTokens,
						TotalTokens:  ev.Response.Usage.InputTokens + ev.Response.Usage.OutputTokens,
					}
					events <- sharedModel.StreamChunk{
						Type:       sharedModel.ChunkEventCompleted,
						Usage:      usage,
						StopReason: toOpenAIStopReason(response),
					}
				} else {
					return
				}
			case "response.failed", "response.error", "response.incomplete":
			}
			if err != nil {
				log.Error("error while event unmarshal", "error", err)
			}
		}
	}()

	return events
}
