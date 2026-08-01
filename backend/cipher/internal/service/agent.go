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

// titleMaxTokens must leave room for reasoning-capable models, which draw their
// internal tokens from the same budget. Too small a value returns empty output.
const titleMaxTokens = 128

// fallbackTitleMaxChars bounds the truncated-message title used when the title
// model is unavailable.
const fallbackTitleMaxChars = 60

// fallbackTitle derives a title from the user's own message so a provider
// failure leaves the conversation labelled rather than permanently untitled.
func fallbackTitle(message string) string {
	title := strings.TrimSpace(strings.Join(strings.Fields(message), " "))
	if title == "" {
		return ""
	}
	if len([]rune(title)) <= fallbackTitleMaxChars {
		return title
	}
	return string([]rune(title)[:fallbackTitleMaxChars]) + "..."
}

// titleChatRequest builds the title request. The configured model is
// "provider/model", but a bare "model" is accepted and resolves against the
// registry's default provider — indexing a split blindly used to panic here.
func titleChatRequest(model string, message string, metadata map[string]string) sharedModel.ChatRequest {
	provider, modelName, found := strings.Cut(strings.TrimSpace(model), "/")
	if !found {
		// No provider prefix: treat the whole value as a model name and let the
		// resolver pick the provider. An empty value resolves to both defaults.
		provider, modelName = "", provider
	}

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
		Provider:    provider,
		Model:       modelName,
		MaxTokens:   titleMaxTokens,
		Temperature: 0,
		Stream:      false,
		Messages:    []sharedModel.AgentMessage{systemPrompt, userMessage},
		Metadata:    metadata,
	}
}

// generateTitle asks the title model for a short conversation title. It returns
// an empty string with no error when the model produced nothing usable, so the
// caller can fall back without treating it as a failure.
func (s *agentService) generateTitle(ctx context.Context, message string) (string, error) {
	titleReq := titleChatRequest(s.agent.TitleModel, message, nil)

	client, model, err := s.llmResolver.Resolve(titleReq.Provider, titleReq.Model)
	if err != nil {
		return "", fmt.Errorf("resolving title model %q: %w", s.agent.TitleModel, err)
	}
	titleReq.Model = model

	titleRes, err := client.Chat(ctx, titleReq)
	if err != nil {
		return "", fmt.Errorf("title model call: %w", err)
	}
	if titleRes == nil {
		return "", fmt.Errorf("title model returned no response")
	}

	return strings.TrimSpace(messageText(titleRes.Message.Content)), nil
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
					Sequence:  msg.Sequence,
					Role:      msg.Role,
					Content:   content,
					CreatedAt: msg.CreatedAt,
				})
			case sharedModel.RoleAssistant:
				// Message parts preserve the order the model produced them. Text before
				// the first tool call belongs on the assistant message that carries the
				// calls; text after it is the post-tool answer. Replaying them in the
				// wrong order misrepresents the conversation to the model.
				// Hidden tools (get_schema, update_working_memory) render no tool-call
				// part, so their stored calls have no position in the content. Only
				// split the text when a tool-call part is actually present; otherwise
				// all of it is the post-tool answer.
				hasToolCallPart := false
				for _, part := range parts {
					if part.Type == sharedModel.MessageTypeToolCall {
						hasToolCallPart = true
						break
					}
				}

				preToolParts := make([]sharedModel.MessagePart, 0, len(parts))
				postToolParts := make([]sharedModel.MessagePart, 0, len(parts))
				seenToolCall := !hasToolCallPart
				for _, part := range parts {
					if part.Type == sharedModel.MessageTypeToolCall {
						seenToolCall = true
						continue
					}
					if part.Type != sharedModel.MessageTypeText {
						continue
					}
					if seenToolCall {
						postToolParts = append(postToolParts, part)
					} else {
						preToolParts = append(preToolParts, part)
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
						Content:   contentBlocksFromMessageParts(preToolParts...),
						CreatedAt: msg.CreatedAt,
						ToolCalls: toolCalls,
					})

					for i := range toolResults {
						messages = append(messages, sharedModel.AgentMessage{
							Sequence:   msg.Sequence,
							Role:       sharedModel.RoleTool,
							CreatedAt:  msg.CreatedAt,
							ToolResult: &toolResults[i],
						})
					}
				} else {
					// Nothing to replay, so the split is meaningless: keep all the text
					// together as a single assistant turn.
					combined := make([]sharedModel.MessagePart, 0, len(preToolParts)+len(postToolParts))
					combined = append(combined, preToolParts...)
					combined = append(combined, postToolParts...)
					postToolParts = combined
				}

				if content := contentBlocksFromMessageParts(postToolParts...); len(content) > 0 {
					messages = append(messages, sharedModel.AgentMessage{
						Sequence:  msg.Sequence,
						Role:      msg.Role,
						Content:   content,
						CreatedAt: msg.CreatedAt,
					})
				}
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
		Static:  agentPrompts.SystemPromptStatic,
		Dynamic: agentPrompts.SystemPromptDynamic,
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

			// Falling back to a truncated first message keeps the conversation
			// labelled when the title model is unavailable or misconfigured.
			title := fallbackTitle(req.Message)

			if generated, err := s.generateTitle(ctxBackground, req.Message); err != nil {
				log.Error("error while generating title, using fallback", "error", err)
			} else if generated != "" {
				title = generated
			}

			if title == "" {
				return
			}

			url := fmt.Sprintf("/api/agent/conversations/%s", req.ConversationID.String())
			if _, err := transport.Patch[any](ctxBackground, s.pennywiseAPI, url, nil, map[string]any{
				"title": title,
			}); err != nil {
				log.Error("error while patching title", "error", err)
				return
			}

			s.publishTitleUpdate(
				utils.DetachedRequestContext(ctxBackground),
				budgetID,
				userID,
				*req.ConversationID,
				title,
			)
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
