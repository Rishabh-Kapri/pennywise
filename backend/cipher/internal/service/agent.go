package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	agentPrompts "github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/context"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/memory"
	agent "github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/runtime"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type AgentService interface {
	CreateRun(ctx context.Context, req sharedModel.AgentRunCreateRequest) (*sharedModel.AgentRun, error)
	GetRun(ctx context.Context, id uuid.UUID) (*sharedModel.AgentRun, error)
	CancelRun(ctx context.Context, id uuid.UUID) (*sharedModel.AgentRun, error)
}

// ClarifyError is returned by CreateRun when the router needs more information
// from the user before the agent can proceed.
type ClarifyError struct {
	Prompt string
}

func (e *ClarifyError) Error() string   { return e.Prompt }
func (e *ClarifyError) StatusCode() int { return http.StatusUnprocessableEntity }

type agentService struct {
	redis           *redis.Client
	agent           *agent.Agent
	agentMemoryRepo db.AgentMemoryRepository
	pennywiseAPI    *transport.Client
	memoryService   memory.Memory
	llmResolver     llm.LLMResolver
}

func NewAgentService(
	redis *redis.Client,
	a *agent.Agent,
	pennywiseClient *transport.Client,
	memoryService memory.Memory,
	llmResolver llm.LLMResolver,
) AgentService {
	return &agentService{
		redis:         redis,
		agent:         a,
		pennywiseAPI:  pennywiseClient,
		memoryService: memoryService,
		llmResolver:   llmResolver,
	}
}

// title model is in format "provider/model"
func titleChatRequest(model string, message string, metadata map[string]string) sharedModel.ChatRequest {
	values := strings.Split(model, "/")

	systemPrompt := sharedModel.AgentMessage{
		Role: sharedModel.RoleSystem,
		Content: []sharedModel.ContentBlock{
			{Type: "text", Text: agentPrompts.TitleGenerationPrompt},
		},
	}

	userMessage := sharedModel.AgentMessage{
		Role: sharedModel.RoleUser,
		Content: []sharedModel.ContentBlock{
			{Type: "text", Text: message},
		},
	}
	return sharedModel.ChatRequest{
		Provider:    values[0],
		Model:       values[1],
		MaxTokens:   24,
		Temperature: 0,
		Stream:      false,
		Messages:    []sharedModel.AgentMessage{systemPrompt, userMessage},
		Metadata:    metadata,
	}
}

type runToolExchange struct {
	Call   sharedModel.ToolCall
	Result sharedModel.ToolResult
}

type storedRunToolCall struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Args   json.RawMessage `json:"args"`
	Result json.RawMessage `json:"result"`
}

func toolResultFromRaw(raw json.RawMessage, call sharedModel.ToolCall) sharedModel.ToolResult {
	result := sharedModel.ToolResult{
		ToolCallId: call.ID,
		Name:       call.Name,
	}
	if len(raw) == 0 || strings.EqualFold(strings.TrimSpace(string(raw)), "null") {
		return result
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		text := string(raw)
		var decodedText string
		if err := json.Unmarshal(raw, &decodedText); err == nil {
			text = decodedText
		}
		result.Content = []sharedModel.ContentBlock{{
			Type: "text",
			Text: text,
		}}
	}
	if result.ToolCallId == "" {
		result.ToolCallId = call.ID
	}
	if result.Name == "" {
		result.Name = call.Name
	}
	return result
}

func conversationMessageParts(raw json.RawMessage) []sharedModel.MessagePart {
	if len(raw) == 0 {
		return nil
	}

	var parts []sharedModel.MessagePart
	if err := json.Unmarshal(raw, &parts); err != nil {
		var message struct {
			Parts []sharedModel.MessagePart `json:"parts"`
		}
		if err := json.Unmarshal(raw, &message); err != nil {
			return nil
		}
		parts = message.Parts
	}

	return parts
}

func contentBlocksFromMessageParts(parts ...sharedModel.MessagePart) []sharedModel.ContentBlock {
	content := make([]sharedModel.ContentBlock, 0, len(parts))
	for _, part := range parts {
		if !strings.EqualFold(string(part.Type), string(sharedModel.MessageTypeText)) || part.Content == nil {
			continue
		}
		content = append(content, sharedModel.ContentBlock{Type: "text", Text: *part.Content})
	}
	return content
}

func messageText(blocks []sharedModel.ContentBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Type == "text" && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// agentRunToChatRequest converts an AgentRunCreateRequest into a ChatRequest
// it also replays the last conversation messages for full context
// suitable for the agent runtime. The user message becomes the last user turn.
// Provider, model, temperature, max tokens, and stream flag are carried over
// directly; nil pointer fields fall back to agent-level defaults.
func agentRunToChatRequest(req sharedModel.AgentRunCreateRequest) sharedModel.ChatRequest {
	var provider, modelName string
	if req.ModelProvider != nil {
		provider = *req.ModelProvider
	}
	if req.ModelName != nil {
		modelName = *req.ModelName
	}

	var temperature float32
	if req.Temperature != nil {
		temperature = float32(*req.Temperature)
	}

	maxTokens := 10_024 // sensible default
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxTokens = *req.MaxTokens
	}

	latestSequence := 0
	messages := make([]sharedModel.AgentMessage, 0, len(req.PrevMessages)+1)

	runsByID := make(map[uuid.UUID]sharedModel.AgentRun, len(req.PrevRuns))
	for _, run := range req.PrevRuns {
		runsByID[run.ID] = run
	}

	if len(req.PrevMessages) > 0 {
		for _, msg := range req.PrevMessages {
			if msg.Sequence > latestSequence {
				latestSequence = msg.Sequence
			}
			parts := conversationMessageParts(msg.Content)
			switch msg.Role {
			case sharedModel.RoleSystem:
			case sharedModel.RoleUser:
				content := contentBlocksFromMessageParts(parts...)
				if len(content) == 0 {
					continue
				}
				messages = append(messages, sharedModel.AgentMessage{
					Sequence: msg.Sequence,
					Role:     msg.Role,
					Content:  content,
				})
			case sharedModel.RoleAssistant:
				// for assistant message loop over
				assistantMessages := make([]sharedModel.AgentMessage, 0)
				for _, part := range parts {
					if part.Type == sharedModel.MessageTypeText {
						content := contentBlocksFromMessageParts(part)
						if len(content) > 0 {
							// append to the assistant messages
							assistantMessages = append(assistantMessages, sharedModel.AgentMessage{
								Sequence: msg.Sequence,
								Role:     msg.Role,
								Content:  contentBlocksFromMessageParts(part),
							})
						}
					}
				}

				var storedRunToolCalls []storedRunToolCall
				if msg.RunID != nil {
					if run, ok := runsByID[*msg.RunID]; ok && run.Metadata != nil && run.Metadata["toolCalls"] != nil {
						rawAgentToolCalls := run.Metadata["toolCalls"]
						payload, err := json.Marshal(rawAgentToolCalls)
						if err == nil {
							_ = json.Unmarshal(payload, &storedRunToolCalls)
						}
					}
				}

				toolCalls := make([]sharedModel.ToolCall, 0, len(storedRunToolCalls))
				toolResults := make([]sharedModel.ToolResult, 0, len(storedRunToolCalls))

				for _, rawToolCall := range storedRunToolCalls {
					// for tool calls, we need to separate out the tool call and the tool result parts
					args := rawToolCall.Args
					if len(args) == 0 || strings.EqualFold(strings.TrimSpace(string(args)), "null") {
						args = json.RawMessage("{}")
					}
					toolCall := sharedModel.ToolCall{
						ID:        rawToolCall.ID,
						Name:      rawToolCall.Name,
						Arguments: args,
					}
					toolCalls = append(toolCalls, toolCall)

					toolResult := toolResultFromRaw(rawToolCall.Result, toolCall)
					toolResults = append(toolResults, toolResult)
				}

				if len(toolCalls) > 0 {
					messages = append(messages, sharedModel.AgentMessage{
						Sequence:  msg.Sequence,
						Role:      sharedModel.RoleAssistant,
						ToolCalls: toolCalls,
					})

					for i := range toolResults {
						messages = append(messages, sharedModel.AgentMessage{
							Sequence:   msg.Sequence,
							Role:       sharedModel.RoleTool,
							ToolResult: &toolResults[i],
						})
					}
				}

				messages = append(messages, assistantMessages...)
			default:
				continue
			}
		}
	}

	messages = append(messages, sharedModel.AgentMessage{
		Sequence: latestSequence + 1,
		Role:     sharedModel.RoleUser,
		Content: []sharedModel.ContentBlock{
			{Type: "text", Text: req.Message},
		},
	})

	metadata := make(map[string]string)
	if req.RunID != nil {
		metadata["runId"] = req.RunID.String()
	}
	if req.ConversationID != nil {
		metadata["conversationId"] = req.ConversationID.String()
	}
	if req.MessageID != nil {
		metadata["messageId"] = req.MessageID.String()
	}
	if traceID, ok := req.Metadata["traceId"].(string); ok && traceID != "" {
		metadata["traceId"] = traceID
	}

	return sharedModel.ChatRequest{
		Provider:    provider,
		Model:       modelName,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		Metadata:    metadata,
		Stream:      req.Stream,
	}
}

func (s *agentService) publishTitleUpdate(
	ctx context.Context,
	budgetID uuid.UUID,
	userID uuid.UUID,
	conversationID uuid.UUID,
	title string,
) {
	log := logger.Logger(ctx)

	if s.redis == nil {
		log.Warn("redis is not configured")
		return
	}

	dataJSON, err := json.Marshal(map[string]any{
		"message": title,
		"type":    "title_update",
	})
	if err != nil {
		log.Error("error while marshaling redis pubsub event data", "type", "title_update", "error", err)
		return
	}

	values := map[string]any{
		"eventName":      string(sharedModel.AgentEventChatStream),
		"budgetId":       budgetID.String(),
		"userId":         userID.String(),
		"conversationId": conversationID.String(),
		"data":           string(dataJSON),
	}

	pipe := s.redis.Pipeline()
	pipe.XAdd(ctx, &redis.XAddArgs{
		Stream: "pubsub",
		Values: values,
	})
	if _, err := pipe.Exec(ctx); err != nil {
		log.Error("error while sending redis pubsub event", "type", "title_update", "error", err)
	}
}

func (s *agentService) CreateRun(
	ctx context.Context,
	req sharedModel.AgentRunCreateRequest,
) (*sharedModel.AgentRun, error) {
	log := logger.Logger(ctx)

	if req.RunID == nil {
		return nil, &ClarifyError{Prompt: "runId is required"}
	}

	budgetID := utils.MustBudgetID(ctx)
	userID, _ := utils.UserIDFromContext(ctx)

	chatReq := agentRunToChatRequest(req)

	context, err := s.memoryService.PrepareContext(ctx, memory.MemoryContextRequest{
		Messages:       chatReq.Messages,
		BudgetID:       budgetID,
		UserID:         userID,
		ConversationID: *req.ConversationID,
	})
	if err != nil {
		log.Error("error while getting context from memory", "error", err)
		return nil, err
	}

	chatReq.Messages = context.Messages

	systemPrompt := agent.SystemPrompt{
		Message: agentPrompts.SystemPrompt,
		Args: []any{
			time.Now().Format(time.DateOnly),
			s.memoryService.GetWorkingMemory(ctx, budgetID),
			budgetID.String(),
		}, // current date, working memory, budgetID
	}

	res, err := s.agent.Run(
		ctx,
		chatReq,
		agent.WithSystemPrompt(systemPrompt),
	)
	if err != nil {
		return nil, err
	}

	// generate conversation title if not present
	if req.ConversationID != nil &&
		(req.Title == nil || (req.Title != nil && *req.Title == "")) &&
		req.ConversationMetadata != nil &&
		req.ConversationMetadata["titleSource"] == "auto" {

		ctxBackground := utils.DetachedRequestContext(ctx)

		go func() {
			log := logger.Logger(ctxBackground)

			titleReq := titleChatRequest(s.agent.TitleModel, req.Message, nil)

			client, model, err := s.llmResolver.Resolve(titleReq.Provider, titleReq.Model)
			if err != nil {
				log.Error("error while resolving llm for title request", "error", err)
				return
			}

			titleReq.Model = model
			titleRes, err := client.Chat(ctxBackground, titleReq)
			if err != nil {
				log.Error("error while generating title", "error", err)
			}

			log.Info("title res", "res", titleRes)

			if titleRes.Message.Content != nil {
				title := strings.TrimSpace(string(titleRes.Message.Content[0].Text))

				url := fmt.Sprintf("/api/agent/conversations/%s", req.ConversationID.String())

				data := map[string]any{
					"title": title,
				}

				if title != "" {
					patchRes, err := transport.Patch[any](ctxBackground, s.pennywiseAPI, url, nil, data)
					logger.Logger(ctxBackground).Info("title patch", "res", patchRes, "error", err)
					if err != nil {
						logger.Logger(ctxBackground).Error("error while patching title", "error", err)
						return
					}

					s.publishTitleUpdate(
						utils.DetachedRequestContext(ctxBackground),
						budgetID,
						userID,
						*req.ConversationID,
						title,
					)
				}
			}
		}()
	}

	agentKey := ""
	if req.AgentKey != nil {
		agentKey = *req.AgentKey
	}

	run := &sharedModel.AgentRun{
		ID:             *req.RunID,
		AgentKey:       agentKey,
		BudgetID:       &budgetID,
		ConversationID: req.ConversationID,
		Status:         sharedModel.AgentRunStatusCompleted,
		ModelProvider:  req.ModelProvider,
		ModelName:      req.ModelName,
		Temperature:    req.Temperature,
		MaxTokens:      req.MaxTokens,
	}
	if userID != uuid.Nil {
		run.UserID = &userID
	}
	if res != nil {
		finalMessage := messageText(res.Message.Content)
		if finalMessage != "" {
			run.FinalMessage = &finalMessage
		}
	}
	return run, nil
}

func (s *agentService) GetRun(ctx context.Context, id uuid.UUID) (*sharedModel.AgentRun, error) {
	return nil, nil
}

func (s *agentService) CancelRun(ctx context.Context, id uuid.UUID) (*sharedModel.AgentRun, error) {
	return nil, nil
}
