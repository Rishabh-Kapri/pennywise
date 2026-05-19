package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	agentPrompts "github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/context"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/memory"
	agent "github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/runtime"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
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
	agent         *agent.Agent
	pennywiseAPI  *transport.Client
	memoryService memory.Memory
}

func NewAgentService(a *agent.Agent, pennywiseClient *transport.Client, memoryService memory.Memory) AgentService {
	return &agentService{agent: a, pennywiseAPI: pennywiseClient, memoryService: memoryService}
}

// title model is in format "provider/model"
func titleChatRequest(model string, message string, metadata map[string]string) sharedModel.ChatRequest {
	values := strings.Split(model, "/")

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
		Messages:    []sharedModel.AgentMessage{userMessage},
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

	messages := make([]sharedModel.AgentMessage, 0, len(req.PrevMessages)+1)

	runsByID := make(map[uuid.UUID]sharedModel.AgentRun, len(req.PrevRuns))
	for _, run := range req.PrevRuns {
		runsByID[run.ID] = run
	}

	if len(req.PrevMessages) > 0 {
		for _, msg := range req.PrevMessages {
			parts := conversationMessageParts(msg.Content)
			switch msg.Role {
			case sharedModel.RoleSystem:
			case sharedModel.RoleUser:
				content := contentBlocksFromMessageParts(parts...)
				if len(content) == 0 {
					continue
				}
				messages = append(messages, sharedModel.AgentMessage{
					Role:    msg.Role,
					Content: content,
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
								Role:    msg.Role,
								Content: contentBlocksFromMessageParts(part),
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
						Role:      sharedModel.RoleAssistant,
						ToolCalls: toolCalls,
					})
					for i := range toolResults {
						messages = append(messages, sharedModel.AgentMessage{
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
		Role: sharedModel.RoleUser,
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

func (s *agentService) CreateRun(
	ctx context.Context,
	req sharedModel.AgentRunCreateRequest,
) (*sharedModel.AgentRun, error) {
	// Step 1 — Router: classify intent, resolve payees, load scoped context.
	// If the date range is unresolved the router returns NeedsClarify; we stop
	// here and return a typed error so the handler can surface the question to
	// the user without starting an agent run.
	// routerResult, err := s.router.Route(ctx, budgetID, req.Message)
	// if err != nil {
	// 	return nil, fmt.Errorf("agent router: %w", err)
	// }
	// if routerResult.NeedsClarification {
	// 	return nil, &ClarifyError{Prompt: routerResult.ClarifyPrompt}
	// }
	if req.RunID == nil {
		return nil, &ClarifyError{Prompt: "runId is required"}
	}

	budgetID := utils.MustBudgetID(ctx)
	userID, _ := utils.UserIDFromContext(ctx)

	// Step 2 — Build chat request and inject router-enriched context.
	chatReq := agentRunToChatRequest(req)
	// chatReq = injectRouterContext(chatReq, routerResult)
	logger.Logger(ctx).Info(s.memoryService.GetWorkingMemory(ctx, budgetID))


	// Step 3 — Run the agent loop.
	systemPrompt := agent.SystemPrompt{
		Message: agentPrompts.SystemPrompt,
		Args: []any{
			time.Now().Format(time.DateOnly),
			s.memoryService.GetWorkingMemory(ctx, budgetID),
			budgetID.String(),
		}, // current date, working memory, budgetID
	}
	res, err := s.agent.Run(ctx, chatReq, agent.WithSystemPrompt(systemPrompt))
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
			titleRes, err := s.agent.Run(
				ctxBackground,
				titleChatRequest(s.agent.TitleModel, req.Message, nil),
				agent.WithToolsEnabled(false),
				agent.WithUpdateMetadata(false),
				agent.WithRequiresContext(false),
				agent.WithSystemPrompt(agent.SystemPrompt{Message: agentPrompts.TitleGenerationPrompt}),
			)
			if err != nil {
				logger.Logger(ctxBackground).Error("error while generating title", "error", err)
			}
			logger.Logger(ctxBackground).Info("title res", "res", titleRes)
			if titleRes.Message.Content != nil {
				title := strings.TrimSpace(string(titleRes.Message.Content[0].Text))
				url := fmt.Sprintf("/api/agent/conversations/%s", req.ConversationID.String())
				data := map[string]any{
					"title": title,
				}
				if title != "" {
					patchRes, err := transport.Patch[any](ctxBackground, s.pennywiseAPI, url, nil, data)
					logger.Logger(ctxBackground).Info("title patch", "res", patchRes, "error", err)
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

// injectRouterContext prepends a system message that summarises the router
// result (intent, date range, scoped categories/payees/spend totals) so the
// chat agent has structured context without touching raw DB rows.
// func injectRouterContext(req sharedModel.ChatRequest, r *agentRouter.RouterResult) sharedModel.ChatRequest {
// 	if r == nil {
// 		return req
// 	}
//
// 	routerCtx := fmt.Sprintf(
// 		"[Router context]\nintent: %s\ndate_range: from=%s to=%s\ncategory_groups: %v",
// 		r.Intent,
// 		func() string {
// 			if r.DateRange != nil {
// 				return r.DateRange.From
// 			}
// 			return ""
// 		}(),
// 		func() string {
// 			if r.DateRange != nil {
// 				return r.DateRange.To
// 			}
// 			return ""
// 		}(),
// 		r.CategoryGroups,
// 	)
//
// 	if r.ScopedContext != nil {
// 		routerCtx += fmt.Sprintf(
// 			"\n\nscoped_categories_spend (name → total):\n",
// 		)
// 		for _, cs := range r.ScopedContext.Categories {
// 			routerCtx += fmt.Sprintf("  %s: %.2f\n", cs.Name, cs.TotalSpend)
// 		}
// 		if len(r.ScopedContext.PayeeNames) > 0 {
// 			routerCtx += fmt.Sprintf("active_payees: %v\n", r.ScopedContext.PayeeNames)
// 		}
// 	}
//
// 	if len(r.ResolvedPayees) > 0 {
// 		routerCtx += "\nresolved_payee_ids:\n"
// 		for _, rp := range r.ResolvedPayees {
// 			routerCtx += fmt.Sprintf("  term=%q id=%s score=%.3f\n", rp.Term, rp.ID, rp.Score)
// 		}
// 	}
//
// 	contextMsg := sharedModel.AgentMessage{
// 		Role: sharedModel.RoleSystem,
// 		Content: []sharedModel.ContentBlock{
// 			{Type: "text", Text: routerCtx},
// 		},
// 	}
//
// 	// Insert after any existing system messages but before user turn.
// 	messages := make([]sharedModel.AgentMessage, 0, len(req.Messages)+1)
// 	insertAt := 0
// 	for insertAt < len(req.Messages) && req.Messages[insertAt].Role == sharedModel.RoleSystem {
// 		insertAt++
// 	}
// 	messages = append(messages, req.Messages[:insertAt]...)
// 	messages = append(messages, contextMsg)
// 	messages = append(messages, req.Messages[insertAt:]...)
//
// 	req.Messages = messages
// 	return req
// }

func (s *agentService) GetRun(ctx context.Context, id uuid.UUID) (*sharedModel.AgentRun, error) {
	return nil, nil
}

func (s *agentService) CancelRun(ctx context.Context, id uuid.UUID) (*sharedModel.AgentRun, error) {
	return nil, nil
}
