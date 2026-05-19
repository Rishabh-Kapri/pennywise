package llm

import (
	"context"
	"encoding/json"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

type anthropicClient struct {
	name       string
	httpClient *transport.Client
}

// content mirrors Anthropic content blocks. Different block types use different
// subsets of these fields: text uses Text, tool_use uses ID/Name/Input, and
// tool_result uses ToolUseID/Content/IsError.
type content struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   []content       `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

// message is Anthropic's chat message shape. System prompts are intentionally
// excluded because Anthropic expects them on the top-level request field.
type message struct {
	Role    sharedModel.Role `json:"role"`
	Content []content        `json:"content"`
}

type anthropicReq struct {
	Model       string               `json:"model"`
	MaxTokens   int                  `json:"max_tokens"`
	Messages    []message            `json:"messages"`
	System      string               `json:"system,omitempty"`
	Temperature float32              `json:"temperature,omitempty"`
	Tools       []anthropicTool      `json:"tools,omitempty"`
	ToolChoice  *anthropicToolChoice `json:"tool_choice,omitempty"`
	Stream      bool                 `json:"stream,omitempty"`
}

type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema sharedModel.ToolSchema `json:"input_schema"`
}

type anthropicToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// anthropicRes and its nested structs are provider wire types. Keep them
// private so the rest of the agent only depends on sharedModel.ChatResponse.
type anthropicRes struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         sharedModel.Role        `json:"role"`
	Model        string                  `json:"model"`
	Content      []anthropicContentBlock `json:"content"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence string                  `json:"stop_sequence"`
	Usage        anthropicUsage          `json:"usage"`
}

type anthropicContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicStreamEvent struct {
	Type         string                     `json:"type"`
	Message      anthropicStreamMessage     `json:"message"`
	Index        int                        `json:"index"`
	ContentBlock anthropicContentBlock      `json:"content_block"`
	Delta        anthropicStreamDelta       `json:"delta"`
	Usage        anthropicUsage             `json:"usage"`
	Error        *anthropicStreamErrorEvent `json:"error,omitempty"`
}

type anthropicStreamMessage struct {
	ID    string         `json:"id"`
	Model string         `json:"model"`
	Usage anthropicUsage `json:"usage"`
}

type anthropicStreamDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

type anthropicStreamErrorEvent struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func NewAnthropicClient(name string) (LLM, error) {
	config := config.Load()
	if config.AnthropicAPIKey == "" {
		return nil, errs.New(errs.CodeInternalError, "no api key found")
	}
	headers := map[string][]string{
		"content-type":      {"application/json"},
		"x-api-key":         {config.AnthropicAPIKey},
		"anthropic-version": {"2023-06-01"},
	}

	headerOpt := transport.WithDefaultHeaders(headers)

	httpTransport := httpclient.NewHttpTransport(
		"https://api.anthropic.com",
	)
	logger.Logger(context.Background()).Info("anthropic client created", "headers", headers)

	return &anthropicClient{
		name: name,
		httpClient: transport.NewClient(
			"anthropic",
			httpTransport,
			headerOpt,
			transport.WithPropagateInternalHeaders(false),
		),
	}, nil
}

func toAnthropicContent(blocks []sharedModel.ContentBlock) []content {
	out := make([]content, 0, len(blocks))

	for _, block := range blocks {
		out = append(out, content{
			Type: block.Type,
			Text: block.Text,
		})
	}
	return out
}

func toAnthropicToolResult(result *sharedModel.ToolResult) content {
	if result == nil {
		return content{}
	}

	return content{
		Type:      "tool_result",
		ToolUseID: result.ToolCallId,
		Content:   toAnthropicContent(result.Content),
		IsError:   result.IsError,
	}
}

// toAnthropicToolUse replays a prior assistant tool call when sending the next
// turn back to Anthropic after local tool execution.
func toAnthropicToolUse(call sharedModel.ToolCall) content {
	return content{
		Type:  "tool_use",
		ID:    call.ID,
		Name:  call.Name,
		Input: call.Arguments,
	}
}

// toAnthropicTools converts the framework's provider-neutral tool schema into
// Anthropic's tool format. Anthropic uses input_schema where OpenAI uses parameters.
func toAnthropicTools(tools []sharedModel.ToolDefiniton) []anthropicTool {
	out := make([]anthropicTool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, anthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}
	return out
}

// toAnthropicToolChoice maps the framework's generic choice names to
// Anthropic-specific values. "required" becomes "any" in Anthropic's API.
func toAnthropicToolChoice(choices []sharedModel.ToolChoice) *anthropicToolChoice {
	if len(choices) == 0 {
		return nil
	}

	choice := choices[0]
	switch choice.Type {
	case sharedModel.ToolChoiceAuto:
		return &anthropicToolChoice{Type: "auto"}
	case sharedModel.ToolChoiceNone:
		return &anthropicToolChoice{Type: "none"}
	case sharedModel.ToolChoiceRequired:
		return &anthropicToolChoice{Type: "any"}
	case sharedModel.ToolChoiceSpecific:
		return &anthropicToolChoice{Type: "tool", Name: choice.Name}
	default:
		return nil
	}
}

// toAnthropicReq converts the framework request to Anthropic's wire format.
// The main provider-specific difference is that Anthropic takes system prompts
// as a top-level string instead of regular messages.
func (c *anthropicClient) toAnthropicReq(req sharedModel.ChatRequest) anthropicReq {
	messages := make([]message, 0, len(req.Messages))
	var system string
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1024
	}

	for _, msg := range req.Messages {
		// Anthropic rejects system messages inside messages, so collect all text
		// system blocks into the top-level System field.
		if msg.Role == sharedModel.RoleSystem {
			for _, block := range msg.Content {
				if block.Type != "text" {
					continue
				}
				if system != "" {
					system += "\n\n"
				}
				system += block.Text
			}
			continue
		}

		// Anthropic represents tool results as user messages containing
		// tool_result blocks, not as role=tool messages.
		if msg.ToolResult != nil {
			messages = append(messages, message{
				Role:    sharedModel.RoleUser,
				Content: []content{toAnthropicToolResult(msg.ToolResult)},
			})
			continue
		}

		msgContent := toAnthropicContent(msg.Content)
		for _, toolCall := range msg.ToolCalls {
			msgContent = append(msgContent, toAnthropicToolUse(toolCall))
		}

		messages = append(messages, message{
			Role:    msg.Role,
			Content: msgContent,
		})
	}
	return anthropicReq{
		MaxTokens:   maxTokens,
		Model:       req.Model,
		Messages:    messages,
		System:      system,
		Temperature: req.Temperature,
		Tools:       toAnthropicTools(req.Tools),
		ToolChoice:  toAnthropicToolChoice(req.ToolChoice),
		Stream:      req.Stream,
	}
}

// fromAnthropicRes normalizes Anthropic's response into the framework sharedModel.
// Text blocks become assistant content and tool_use blocks become ToolCalls for
// the runtime to execute.
func (c *anthropicClient) fromAnthropicRes(res anthropicRes) sharedModel.ChatResponse {
	content := make([]sharedModel.ContentBlock, 0, len(res.Content))
	toolCalls := make([]sharedModel.ToolCall, 0)

	for _, block := range res.Content {
		switch block.Type {
		case "text":
			content = append(content, sharedModel.ContentBlock{
				Type: block.Type,
				Text: block.Text,
			})
		case "tool_use":
			toolCalls = append(toolCalls, sharedModel.ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: block.Input,
			})
		}
	}

	return sharedModel.ChatResponse{
		ID:    res.ID,
		Model: res.Model,
		Message: sharedModel.AgentMessage{
			Role:      res.Role,
			Content:   content,
			ToolCalls: toolCalls,
		},
		Usage: sharedModel.Usage{
			InputTokens:  res.Usage.InputTokens,
			OutputTokens: res.Usage.OutputTokens,
			TotalTokens:  res.Usage.InputTokens + res.Usage.OutputTokens,
		},
		StopReason:  toModelStopReason(res.StopReason),
		RawProvider: res,
	}
}

// toModelStopReason collapses provider stop reasons into the small set the
// agent runtime needs for loop control.
func toModelStopReason(stopReason string) sharedModel.StopReason {
	switch stopReason {
	case "end_turn", "stop_sequence":
		return sharedModel.StopReasonEndTurn
	case "tool_use":
		return sharedModel.StopReasonToolUse
	case "max_tokens":
		return sharedModel.StopReasonMaxTokens
	default:
		return sharedModel.StopReasonError
	}
}

func (c *anthropicClient) Chat(ctx context.Context, req sharedModel.ChatRequest) (*sharedModel.ChatResponse, error) {
	log := logger.Logger(ctx)
	log.Info("Chat", "client", c)
	anthropicReq := c.toAnthropicReq(req)
	log.Info("anthropic req", "req", anthropicReq)
	res, err := transport.Post[anthropicRes](ctx, c.httpClient, "/v1/messages", nil, anthropicReq)
	if err != nil {
		log.Error("error while sending /v1/messages", "error", err)
		return nil, err
	}
	b, _ := utils.Marshal(res, 0)
	log.Info("raw res", "body", string(b))

	chatRes := c.fromAnthropicRes(res)
	b, _ = utils.Marshal(chatRes, 0)
	log.Info("transformed", "body", b)
	log.Info("/v1/messages", "res", chatRes)
	return &chatRes, nil
}

func (c *anthropicClient) Stream(ctx context.Context, req sharedModel.ChatRequest) <-chan sharedModel.StreamChunk {
	log := logger.Logger(ctx)
	anthropicReq := c.toAnthropicReq(req)
	anthropicReq.Stream = true
	events := make(chan sharedModel.StreamChunk, 1)

	headers := map[string][]string{
		"Accept": {"text/event-stream"},
	}
	res, err := transport.StreamPost(ctx, c.httpClient, "/v1/messages", headers, anthropicReq)
	if err != nil {
		log.Error("error while sending anthropic streaming /v1/messages", "error", err)
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

		for event := range res.Events {
			var ev anthropicStreamEvent
			if err := json.Unmarshal(event.Data, &ev); err != nil {
				log.Error("error while unmarshalling anthropic stream event", "event", event.Event, "error", err)
				sendAnthropicChunk(ctx, events, sharedModel.StreamChunk{
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
			case "message_start":
				usage.InputTokens = ev.Message.Usage.InputTokens
				if ev.Message.Usage.OutputTokens > 0 {
					usage.OutputTokens = ev.Message.Usage.OutputTokens
				}
				if !sendAnthropicChunk(ctx, events, sharedModel.StreamChunk{
					Type: sharedModel.ChunkEventStarted,
				}) {
					return
				}

			case "content_block_start":
				if ev.ContentBlock.Type == "tool_use" {
					if !sendAnthropicChunk(ctx, events, sharedModel.StreamChunk{
						Type:        sharedModel.ChunkEventToolCallStart,
						ToolCallID:  ev.ContentBlock.ID,
						ToolName:    ev.ContentBlock.Name,
						OutputIndex: ev.Index,
					}) {
						return
					}
				}

			case "content_block_delta":
				switch ev.Delta.Type {
				case "text_delta":
					if ev.Delta.Text == "" {
						continue
					}
					if !sendAnthropicChunk(ctx, events, sharedModel.StreamChunk{
						Type: sharedModel.ChunkEventText,
						Text: ev.Delta.Text,
					}) {
						return
					}
				case "input_json_delta":
					if ev.Delta.PartialJSON == "" {
						continue
					}
					if !sendAnthropicChunk(ctx, events, sharedModel.StreamChunk{
						Type:          sharedModel.ChunkEventToolCallDelta,
						ToolArgsDelta: ev.Delta.PartialJSON,
						OutputIndex:   ev.Index,
					}) {
						return
					}
				}

			case "content_block_stop":
				if !sendAnthropicChunk(ctx, events, sharedModel.StreamChunk{
					Type:        sharedModel.ChunkEventToolCall,
					OutputIndex: ev.Index,
				}) {
					return
				}

			case "message_delta":
				if ev.Usage.OutputTokens > 0 {
					usage.OutputTokens = ev.Usage.OutputTokens
				}

			case "message_stop":
				usage.TotalTokens = usage.InputTokens + usage.OutputTokens
				sendAnthropicChunk(ctx, events, sharedModel.StreamChunk{
					Type:  sharedModel.ChunkEventCompleted,
					Usage: usage,
				})
				return

			case "error":
				message := "anthropic stream error"
				if ev.Error != nil && ev.Error.Message != "" {
					message = ev.Error.Message
				}
				sendAnthropicChunk(ctx, events, sharedModel.StreamChunk{
					Type: sharedModel.ChunkEventError,
					Text: message,
				})
				return

			case "ping":
				continue
			default:
				continue
			}
		}
	}()

	return events
}

func sendAnthropicChunk(
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
