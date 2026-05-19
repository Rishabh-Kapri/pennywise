package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
)

const ollamaChatPath = "/api/chat"

type ollamaClient struct {
	httpClient *transport.Client
}

type ollamaChatReq struct {
	Model     string          `json:"model"`
	Messages  []ollamaMessage `json:"messages"`
	Tools     []ollamaTool    `json:"tools,omitempty"`
	Stream    bool            `json:"stream"`
	Options   map[string]any  `json:"options,omitempty"`
	KeepAlive string          `json:"keep_alive,omitempty"`
	Format    any             `json:"format,omitempty"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaTool struct {
	Type     string         `json:"type"`
	Function ollamaFunction `json:"function"`
}

type ollamaFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ollamaToolCall struct {
	Function ollamaToolCallFunction `json:"function"`
}

type ollamaToolCallFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ollamaChatRes struct {
	Model           string        `json:"model"`
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	DoneReason      string        `json:"done_reason"`
	PromptEvalCount int           `json:"prompt_eval_count"`
	EvalCount       int           `json:"eval_count"`
}

func NewOllamaClient() (LLM, error) {
	cfg := config.Load()
	if cfg.OllamaURL == "" {
		return nil, errs.New(errs.CodeInternalError, "no ollama url found")
	}

	httpTransport := httpclient.NewHttpTransport(cfg.OllamaURL)
	return &ollamaClient{
		httpClient: transport.NewClient(
			"ollama",
			httpTransport,
			transport.WithPropagateInternalHeaders(false),
		),
	}, nil
}

func (c *ollamaClient) toOllamaReq(req sharedModel.ChatRequest) ollamaChatReq {
	options := make(map[string]any)
	if req.Temperature != 0 {
		options["temperature"] = req.Temperature
	}
	if req.MaxTokens != 0 {
		options["num_predict"] = req.MaxTokens
	}
	if len(options) == 0 {
		options = nil
	}

	var format any
	if req.Format == "json" {
		format = "json"
	}

	return ollamaChatReq{
		Model:     req.Model,
		Messages:  toOllamaMessages(req.Messages),
		Tools:     toOllamaTools(req.Tools),
		Stream:    false,
		Options:   options,
		KeepAlive: "1h",
		Format:    format,
	}
}

func toOllamaMessages(messages []sharedModel.AgentMessage) []ollamaMessage {
	out := make([]ollamaMessage, 0, len(messages))

	for _, msg := range messages {
		if msg.ToolResult != nil {
			out = append(out, ollamaMessage{
				Role:    string(sharedModel.RoleTool),
				Content: contentBlocksText(msg.ToolResult.Content),
			})
			continue
		}

		ollamaMsg := ollamaMessage{
			Role:    string(msg.Role),
			Content: contentBlocksText(msg.Content),
		}
		for _, call := range msg.ToolCalls {
			ollamaMsg.ToolCalls = append(ollamaMsg.ToolCalls, toOllamaToolCall(call))
		}

		out = append(out, ollamaMsg)
	}

	return out
}

func toOllamaToolCall(call sharedModel.ToolCall) ollamaToolCall {
	return ollamaToolCall{
		Function: ollamaToolCallFunction{
			Name:      call.Name,
			Arguments: call.Arguments,
		},
	}
}

func toOllamaTools(tools []sharedModel.ToolDefiniton) []ollamaTool {
	out := make([]ollamaTool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, ollamaTool{
			Type: "function",
			Function: ollamaFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  toOpenAISchema(tool.InputSchema),
			},
		})
	}
	return out
}

func (c *ollamaClient) fromOllamaRes(res ollamaChatRes) sharedModel.ChatResponse {
	toolCalls := make([]sharedModel.ToolCall, 0, len(res.Message.ToolCalls))
	for i, call := range res.Message.ToolCalls {
		toolCalls = append(toolCalls, sharedModel.ToolCall{
			ID:        fmt.Sprintf("ollama_tool_%d", i),
			Name:      call.Function.Name,
			Arguments: call.Function.Arguments,
		})
	}

	content := make([]sharedModel.ContentBlock, 0)
	if res.Message.Content != "" {
		content = append(content, sharedModel.ContentBlock{Type: "text", Text: res.Message.Content})
	}

	return sharedModel.ChatResponse{
		Model: res.Model,
		Message: sharedModel.AgentMessage{
			Role:      sharedModel.RoleAssistant,
			Content:   content,
			ToolCalls: toolCalls,
		},
		Usage: sharedModel.Usage{
			InputTokens:  res.PromptEvalCount,
			OutputTokens: res.EvalCount,
			TotalTokens:  res.PromptEvalCount + res.EvalCount,
		},
		StopReason:  toOllamaStopReason(res),
		RawProvider: res,
	}
}

func toOllamaStopReason(res ollamaChatRes) sharedModel.StopReason {
	if len(res.Message.ToolCalls) > 0 {
		return sharedModel.StopReasonToolUse
	}

	switch res.DoneReason {
	case "", "stop", "unload":
		if res.Done {
			return sharedModel.StopReasonEndTurn
		}
	case "length":
		return sharedModel.StopReasonMaxTokens
	}

	return sharedModel.StopReasonError
}

func (c *ollamaClient) Chat(ctx context.Context, req sharedModel.ChatRequest) (*sharedModel.ChatResponse, error) {
	log := logger.Logger(ctx)
	log.Info("ollama", "req", c.toOllamaReq(req))
	res, err := transport.Post[ollamaChatRes](ctx, c.httpClient, ollamaChatPath, nil, c.toOllamaReq(req))
	if err != nil {
		log.Error("error while sending /api/chat", "error", err)
		return nil, err
	}

	chatRes := c.fromOllamaRes(res)
	log.Info("/api/chat", "res", chatRes)
	return &chatRes, nil
}

func (c *ollamaClient) Stream(ctx context.Context, req sharedModel.ChatRequest) <-chan sharedModel.StreamChunk {
	events := make(chan sharedModel.StreamChunk)
	close(events)
	return events
}
