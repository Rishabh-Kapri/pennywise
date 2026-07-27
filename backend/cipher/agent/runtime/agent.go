package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/handler"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/memory"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/tools"

	"github.com/redis/go-redis/v9"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

const agentSpanContentLimit = 20_000

// toolErrorContentLimit bounds how much of a failed tool's error text is handed
// back to the model. Raw driver errors can be very large and would otherwise eat
// the context window.
const toolErrorContentLimit = 2_000

const (
	toolBudgetExhaustedMessage = "Tool call budget exhausted for this run. This tool was not executed."

	toolBudgetNudge = "You have used the entire tool budget for this run, so no further tool calls will run. " +
		"Answer the user's question now using only the information already gathered. " +
		"If it is incomplete, say what you were able to determine and what is still missing."

	maxTurnsNudge = "You have reached the maximum number of turns for this run. " +
		"Answer the user's question now using only the information already gathered. " +
		"If it is incomplete, say what you were able to determine and what is still missing."
)

type AgentConfig struct {
	titleModel    string
	telemetry     *otelSDK.Telemetry
	redis         *redis.Client
	toolRegistry  *tools.ToolRegistry
	maxTurns      int
	maxToolCalls  int
	pennywiseAPI  *transport.Client
	memoryEnabled bool
	memory        memory.Memory
}

type AgentOption func(*AgentConfig)

func WithRedis(r *redis.Client) AgentOption {
	return func(ac *AgentConfig) {
		ac.redis = r
	}
}

func WithTelemetry(telemetry *otelSDK.Telemetry) AgentOption {
	return func(ac *AgentConfig) {
		ac.telemetry = telemetry
	}
}

// WithTitleModel sets the "provider/model" used for conversation title
// generation. When empty the resolver's default provider and model are used.
func WithTitleModel(model string) AgentOption {
	return func(ac *AgentConfig) {
		ac.titleModel = model
	}
}

// WithMaxTurns caps the agent loop. Non-positive values are ignored so an unset
// config cannot silently reduce the budget to zero and skip the loop entirely.
func WithMaxTurns(maxTurns int) AgentOption {
	return func(ac *AgentConfig) {
		if maxTurns > 0 {
			ac.maxTurns = maxTurns
		}
	}
}

// WithMaxToolCalls caps total tool executions per run. Non-positive values are
// ignored, as for WithMaxTurns.
func WithMaxToolCalls(maxToolCalls int) AgentOption {
	return func(ac *AgentConfig) {
		if maxToolCalls > 0 {
			ac.maxToolCalls = maxToolCalls
		}
	}
}

func WithPennywiseAPI(client *transport.Client) AgentOption {
	return func(ac *AgentConfig) {
		ac.pennywiseAPI = client
	}
}

func WithMemory(ms memory.Memory) AgentOption {
	return func(ac *AgentConfig) {
		ac.memoryEnabled = true
		ac.memory = ms
	}
}

type Agent struct {
	llmResolver   llm.LLMResolver
	TitleModel    string // "provider/model" used for conversation title generation
	telemetry     *otelSDK.Telemetry
	redisClient   *redis.Client
	toolRegistry  *tools.ToolRegistry
	maxTurns      int
	maxToolCalls  int
	pennywiseAPI  *transport.Client
	memoryEnabled bool
	memory        memory.Memory
}

func NewAgent(llmResolver llm.LLMResolver, toolRegistry *tools.ToolRegistry, opts ...AgentOption) (*Agent, error) {
	cfg := &AgentConfig{
		toolRegistry: toolRegistry,
		maxTurns:     10,
		maxToolCalls: 10,
	}
	for _, o := range opts {
		o(cfg)
	}

	return &Agent{
		llmResolver:   llmResolver,
		TitleModel:    cfg.titleModel,
		telemetry:     cfg.telemetry,
		redisClient:   cfg.redis,
		toolRegistry:  toolRegistry,
		maxTurns:      cfg.maxTurns,
		maxToolCalls:  cfg.maxToolCalls,
		pennywiseAPI:  cfg.pennywiseAPI,
		memoryEnabled: cfg.memoryEnabled,
		memory:        cfg.memory,
	}, nil
}

const redisPubsubStream = "pubsub"

type SystemPrompt struct {
	Message string
	Args    []any
}
type AgentRunOptions struct {
	enableTools       bool
	updateRunMetadata bool
	requiresContext   bool
	systemPrompt      SystemPrompt
	memoryEnabled     bool
}

type AgentRunOption func(*AgentRunOptions)

func WithToolsEnabled(value bool) AgentRunOption {
	return func(opts *AgentRunOptions) {
		opts.enableTools = value
	}
}

func WithRequiresContext(value bool) AgentRunOption {
	return func(opts *AgentRunOptions) {
		opts.requiresContext = value
	}
}

func WithUpdateMetadata(value bool) AgentRunOption {
	return func(opts *AgentRunOptions) {
		opts.updateRunMetadata = value
	}
}

func WithSystemPrompt(prompt SystemPrompt) AgentRunOption {
	return func(opts *AgentRunOptions) {
		opts.systemPrompt = prompt
	}
}

func WithRunMemoryEnabled(value bool) AgentRunOption {
	return func(opts *AgentRunOptions) {
		opts.memoryEnabled = value
	}
}

func (a *Agent) executeTool(
	ctx context.Context,
	toolCall sharedModel.ToolCall,
) (*tools.Tool, *sharedModel.ToolResult, error) {
	ctx, span := a.telemetry.TraceStart(ctx, "tool.execute")
	defer span.End()

	recordToolRequest(span, toolCall)

	tool, err := a.toolRegistry.GetTool(toolCall.Name)
	if err != nil {
		setSpanError(span, err)
		return nil, nil, errs.Wrap(errs.CodeToolNotFound, "tool not found", err)
	}

	toolResult, err := tool.Execute(ctx, toolCall)
	if err != nil {
		setSpanError(span, err)
		return nil, nil, errs.Wrap(errs.CodeToolExecuteFail, "tool execution failed", err)
	}

	recordToolResult(span, toolResult)
	return &tool, toolResult, nil
}

func (a *Agent) Chat(ctx context.Context, req sharedModel.ChatRequest) (*sharedModel.ChatResponse, error) {
	req.Stream = false
	return a.Run(ctx, req)
}

func (a *Agent) Stream(ctx context.Context, req sharedModel.ChatRequest) error {
	req.Stream = true
	_, err := a.Run(ctx, req)
	return err
}

func chatResToStepResult(chatRes sharedModel.ChatResponse) sharedModel.StepResult {
	return sharedModel.StepResult{
		Text:       messageContentText(chatRes.Message.Content),
		ToolCalls:  chatRes.Message.ToolCalls,
		Usage:      chatRes.Usage,
		StopReason: chatRes.StopReason,
	}
}

func stepResultToChatResponse(modelName string, stepResult sharedModel.StepResult) *sharedModel.ChatResponse {
	return &sharedModel.ChatResponse{
		Model: modelName,
		Message: sharedModel.AgentMessage{
			Role:      sharedModel.RoleAssistant,
			Content:   contentBlocksFromText(stepResult.Text),
			ToolCalls: stepResult.ToolCalls,
		},
		Usage:      stepResult.Usage,
		StopReason: stepResult.StopReason,
	}
}

func messageContentText(blocks []sharedModel.ContentBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Type == "text" && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func contentBlocksFromText(text string) []sharedModel.ContentBlock {
	if text == "" {
		return nil
	}
	return []sharedModel.ContentBlock{
		{
			Type: "text",
			Text: text,
		},
	}
}

// truncateText clips text to limit bytes without splitting a multi-byte rune.
func truncateText(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	truncated := text[:limit]
	for len(truncated) > 0 && !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated + "... (truncated)"
}

// toolErrorResult converts a failed tool execution into a result the model can
// see and recover from. Dropping the result instead would leave the assistant's
// tool_use block unanswered, which providers reject on the following turn.
func toolErrorResult(call sharedModel.ToolCall, err error) sharedModel.ToolResult {
	return sharedModel.ToolResult{
		ToolCallId: call.ID,
		Name:       call.Name,
		IsError:    true,
		Content: []sharedModel.ContentBlock{{
			Type: "text",
			Text: truncateText(err.Error(), toolErrorContentLimit),
		}},
	}
}

// refusedToolResults answers every requested call with the same error, used when
// the run is out of tool budget but the calls still need results.
func refusedToolResults(calls []sharedModel.ToolCall, reason string) []sharedModel.ToolResult {
	results := make([]sharedModel.ToolResult, 0, len(calls))
	for _, call := range calls {
		results = append(results, sharedModel.ToolResult{
			ToolCallId: call.ID,
			Name:       call.Name,
			IsError:    true,
			Content:    []sharedModel.ContentBlock{{Type: "text", Text: reason}},
		})
	}
	return results
}

func appendAssistantToolCalls(
	messages []sharedModel.AgentMessage,
	sequence int,
	stepResult sharedModel.StepResult,
) []sharedModel.AgentMessage {
	return append(messages, sharedModel.AgentMessage{
		Sequence:  sequence,
		Role:      sharedModel.RoleAssistant,
		Content:   contentBlocksFromText(stepResult.Text),
		ToolCalls: stepResult.ToolCalls,
	})
}

func appendToolResults(
	messages []sharedModel.AgentMessage,
	sequence int,
	results []sharedModel.ToolResult,
) []sharedModel.AgentMessage {
	for i := range results {
		messages = append(messages, sharedModel.AgentMessage{
			Sequence:   sequence,
			Role:       sharedModel.RoleTool,
			ToolResult: &results[i],
		})
	}
	return messages
}

// finalAnswerTurn runs one last LLM step with tools disabled, so a run that has
// exhausted its turn or tool budget still answers the user instead of surfacing
// an error. The nudge is deliberately not written back into the caller's message
// slice — it is scaffolding, not conversation, and should not reach storage.
func (a *Agent) finalAnswerTurn(
	ctx context.Context,
	req sharedModel.ChatRequest,
	messages []sharedModel.AgentMessage,
	nudge string,
) (sharedModel.StepResult, error) {
	finalReq := req
	finalReq.Tools = nil
	finalReq.ToolChoice = nil
	finalReq.Messages = append(slices.Clone(messages), sharedModel.AgentMessage{
		Role:    sharedModel.RoleUser,
		Content: []sharedModel.ContentBlock{{Type: "text", Text: nudge}},
	})

	return a.runLLMStep(ctx, finalReq)
}

func nextAgentMessageSequence(messages []sharedModel.AgentMessage) int {
	maxSequence := 0
	for _, msg := range messages {
		if msg.Role == sharedModel.RoleSystem {
			continue
		}
		if msg.Sequence > maxSequence {
			maxSequence = msg.Sequence
		}
	}
	return maxSequence + 1
}

func appendMessageTextPart(messageParts *[]sharedModel.MessagePart, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}

	content := text
	*messageParts = append(*messageParts, sharedModel.MessagePart{
		Type:    sharedModel.MessageTypeText,
		Content: &content,
	})
}

func appendMessageToolCallPart(
	tool tools.Tool,
	toolCall sharedModel.ToolCall,
	messageParts *[]sharedModel.MessagePart,
	metaToolCalls *[]map[string]any,
	args map[string]any,
	resultJSON json.RawMessage,
) *sharedModel.ToolResultNormalized {
	// Normalizers assume a success payload, so a failed tool result makes this
	// error out. That must not skip the metadata append below — a call that ran
	// and failed still belongs in the run record.
	normalizedResult, err := tool.Normalize(toolCall, resultJSON)
	if err != nil {
		normalizedResult = nil
	}

	if normalizedResult != nil {
		part := sharedModel.MessagePart{
			Type:        sharedModel.MessageTypeToolCall,
			DisplayName: &normalizedResult.DisplayName,
			Summary:     &normalizedResult.Summary,
			Result:      normalizedResult.Result,
		}
		if toolCall.ID != "" {
			part.ID = &toolCall.ID
		}
		*messageParts = append(*messageParts, part)
	}
	appendMetaToolCall(metaToolCalls, toolCall, args, resultJSON)
	return normalizedResult
}

// appendMetaToolCall records a tool call in the run metadata. Kept separate from
// appendMessageToolCallPart so calls that never resolved to a tool are still
// recorded.
func appendMetaToolCall(
	metaToolCalls *[]map[string]any,
	toolCall sharedModel.ToolCall,
	args map[string]any,
	resultJSON json.RawMessage,
) {
	metaToolCall := make(map[string]any)
	if toolCall.Name != "" {
		metaToolCall["name"] = toolCall.Name
	}
	if toolCall.ID != "" {
		metaToolCall["id"] = toolCall.ID
	}
	metaToolCall["args"] = args
	metaToolCall["result"] = resultJSON

	*metaToolCalls = append(*metaToolCalls, metaToolCall)
}

func (a *Agent) publishChatStreamEvent(
	ctx context.Context,
	budgetId uuid.UUID,
	userId uuid.UUID,
	conversationId string,
	messageId string,
	eventType string,
	message any,
) {
	if a.redisClient == nil {
		return
	}

	log := logger.Logger(ctx)
	dataJSON, err := json.Marshal(map[string]any{
		"id":      messageId,
		"message": message,
		"type":    eventType,
	})
	if err != nil {
		log.Error("error while marshaling redis pubsub event data", "type", eventType, "error", err)
		return
	}

	values := map[string]any{
		"eventName": string(sharedModel.AgentEventChatStream),
		"budgetId":  budgetId.String(),
		"userId":    userId.String(),
		"data":      string(dataJSON),
	}
	if conversationId != "" {
		values["conversationId"] = conversationId
	}

	pipe := a.redisClient.Pipeline()
	pipe.XAdd(ctx, &redis.XAddArgs{
		Stream: redisPubsubStream,
		Values: values,
	})
	if _, err := pipe.Exec(ctx); err != nil {
		log.Error("error while sending redis pubsub event", "type", eventType, "error", err)
	}
}

func (a *Agent) runLLMStep(ctx context.Context, req sharedModel.ChatRequest) (sharedModel.StepResult, error) {
	// budgetId := utils.MustBudgetID(ctx)
	budgetId := utils.MustBudgetID(ctx)
	userId := utils.MustUserID(ctx)

	llmClient, resolvedModel, err := a.llmResolver.Resolve(req.Provider, req.Model)
	if err != nil {
		return sharedModel.StepResult{}, err
	}
	req.Model = resolvedModel
	if req.Stream {
		log := logger.Logger(ctx)

		conversationId := req.Metadata["conversationId"]

		events := llmClient.Stream(ctx, req)

		stepResult := handler.ProcessStream(ctx, &req, events, handler.StreamHandler{
			OnTextDelta: func(textDelta string) {
				log.Info("stream \"text_delta\" received", "textDelta", textDelta)
				a.publishChatStreamEvent(
					ctx,
					budgetId,
					userId,
					conversationId,
					req.Metadata["messageId"],
					"text_delta",
					textDelta,
				)
			},
			OnToolCallStart: func(ctx context.Context, toolCall sharedModel.ToolCall) {
				log.Info("stream \"tool_call_start\" received", "tool", toolCall)

				tool, err := a.toolRegistry.GetTool(toolCall.Name)
				if err != nil {
					log.Error("tool not found", "toolName", toolCall.Name, "error", err)
					return
				}

				normalizedName := tool.GetNormalizedName(false)
				if normalizedName == "" {
					// tool call is not supposed to be shown in the UI
					return
				}

				a.publishChatStreamEvent(
					ctx,
					budgetId,
					userId,
					conversationId,
					req.Metadata["messageId"],
					"tool_call_start",
					map[string]any{
						"id":          toolCall.ID,
						"displayName": normalizedName,
					},
				)
			},
			OnToolCall: func(ctx context.Context, tool sharedModel.ToolCall) {
				log.Info("stream \"tool_call\" received", "tool", tool)
			},
			OnDone: func(usage sharedModel.Usage) {
				log.Info("stream done received", "usage", usage)
			},
		})
		if stepResult.Err != nil {
			return stepResult, stepResult.Err
		}
		return stepResult, nil
	}

	chatRes, err := llmClient.Chat(ctx, req)
	if err != nil {
		return sharedModel.StepResult{}, err
	}

	stepResult := chatResToStepResult(*chatRes)
	stepResult.MaxTokens = req.MaxTokens
	return stepResult, nil
}

type runFinishedContext struct {
	runOpts        AgentRunOptions
	runID          string
	conversationID string
	messageID      string
	enabledTools   []string
	tokenUsage     map[string]int
	maxTokens      int
	traceID        string
	agentToolCalls []map[string]any
	messageParts   []sharedModel.MessagePart
	allMessages    []sharedModel.AgentMessage
}

func (a *Agent) processRunFinish(ctx context.Context, payload runFinishedContext) {
	log := logger.Logger(ctx)

	if payload.runOpts.updateRunMetadata && payload.runID != "" {
		runMetadata := map[string]any{
			"enabledTools": payload.enabledTools,
			"inputTokens":  payload.tokenUsage["input"],
			"outputTokens": payload.tokenUsage["output"],
			"totalTokens":  payload.tokenUsage["input"] + payload.tokenUsage["output"],
			"maxTokens":    payload.maxTokens,
			"traceId":      payload.traceID,
			"toolCalls":    payload.agentToolCalls,
		}
		backgroundCtx := utils.DetachedRequestContext(ctx)
		_, err := transport.Patch[any](
			backgroundCtx,
			a.pennywiseAPI,
			"/api/agent/run/"+payload.runID+"/metadata",
			nil,
			runMetadata,
		)
		if err != nil {
			// run metadata update failed, skip all further updates
			log.Error("error while updating run metadata after run finished", "error", err)
			return
		}
	}

	if payload.conversationID != "" && payload.messageID != "" {
		// dispatch message update
		url := fmt.Sprintf("/api/agent/conversations/%s/message/%s", payload.conversationID, payload.messageID)
		backgroundCtx := utils.DetachedRequestContext(ctx)

		_, err := transport.Patch[any](backgroundCtx, a.pennywiseAPI, url, nil, payload.messageParts)
		if err != nil {
			// run metadata update failed, skip all further updates
			log.Error("error while updating messages parts after run finished", "error", err)
			return
		}
	}

	if a.memoryEnabled && payload.runOpts.memoryEnabled {
		// update memory
		if payload.conversationID == "" {
			return
		}
		backgroundCtx := utils.DetachedRequestContext(ctx)
		conversationUUID, err := uuid.Parse(payload.conversationID)
		if err != nil {
			log.Error(
				"error while parsing conversation id for memory persistence",
				"conversationId",
				payload.conversationID,
				"error",
				err,
			)
			return
		}
		a.memory.OnRunPersisted(backgroundCtx, memory.AgentRunData{
			BudgetID:       utils.MustBudgetID(ctx),
			UserID:         utils.MustUserID(ctx),
			ConversationID: conversationUUID,
			Messages:       payload.allMessages,
		})
	}
}

func (a *Agent) Run(
	ctx context.Context,
	req sharedModel.ChatRequest,
	opts ...AgentRunOption,
) (res *sharedModel.ChatResponse, err error) {
	runOpts := AgentRunOptions{
		enableTools:       true,
		updateRunMetadata: true,
		requiresContext:   true,
		memoryEnabled:     true,
		systemPrompt: SystemPrompt{
			Message: "",
			Args:    []any{},
		},
	}
	for _, opt := range opts {
		opt(&runOpts)
	}

	log := logger.Logger(ctx)
	ctx, span := a.telemetry.TraceStart(ctx, "agent.run")

	log.Info("LLM run started", "req", req)

	messageID := req.Metadata["messageId"]
	runID := req.Metadata["runId"]
	conversationID := req.Metadata["conversationId"]

	// agent metadata
	enabledTools := make([]string, 0)
	tokenUsage := map[string]int{
		"input":  0,
		"output": 0,
	}
	maxTokensUsed := req.MaxTokens

	messageParts := make([]sharedModel.MessagePart, 0)
	agentMetaToolCalls := make([]map[string]any, 0)

	defer func() {
		defer span.End()
		// Surface run failures to the chat UI over the same websocket
		// stream the deltas use, so users don't have to dig through logs.
		if err != nil && req.Stream {
			budgetID, budgetErr := utils.BudgetIDFromContext(ctx)
			userID, userErr := utils.UserIDFromContext(ctx)
			if budgetErr == nil && userErr == nil {
				a.publishChatStreamEvent(
					ctx,
					budgetID,
					userID,
					conversationID,
					messageID,
					"error",
					err.Error(),
				)
			}
		}
		go a.processRunFinish(ctx, runFinishedContext{
			runOpts:        runOpts,
			runID:          runID,
			conversationID: conversationID,
			messageID:      messageID,
			enabledTools:   enabledTools,
			tokenUsage:     tokenUsage,
			maxTokens:      maxTokensUsed,
			traceID:        span.SpanContext().SpanID().String(),
			agentToolCalls: agentMetaToolCalls,
			messageParts:   messageParts,
			allMessages:    req.Messages,
		})
	}()

	hasStepResult := false

	messages := make([]sharedModel.AgentMessage, len(req.Messages), len(req.Messages)+1)
	copy(messages, req.Messages)
	assistantSequence := nextAgentMessageSequence(messages)

	// Enrich with tools
	if runOpts.enableTools && len(a.toolRegistry.GetAllTools()) > 0 {
		req.Tools = make([]sharedModel.ToolDefiniton, 0)

		for _, tool := range a.toolRegistry.GetAllTools() {
			req.Tools = append(req.Tools, tool.Definition())
			enabledTools = append(enabledTools, tool.Definition().Name)
		}
		log.Info("enriching with tools", "tools", enabledTools)
	}

	if runOpts.systemPrompt.Message != "" {
		systemMessage := sharedModel.AgentMessage{
			Role: sharedModel.RoleSystem,
			Content: []sharedModel.ContentBlock{
				{
					Type: "text",
					Text: fmt.Sprintf(runOpts.systemPrompt.Message, runOpts.systemPrompt.Args...),
				},
			},
		}
		messages = slices.Insert(messages, 0, systemMessage)
	}
	req.Messages = messages
	recordAgentRunStart(span, req, a.maxTurns, a.maxToolCalls)

	turnCount := 0
	totalToolCalls := 0

	// finishWithFinalAnswer takes one tools-disabled turn so that exhausting a
	// budget degrades into a real reply instead of an error bubble in the UI.
	finishWithFinalAnswer := func(nudge string) (*sharedModel.ChatResponse, error) {
		finalStep, stepErr := a.finalAnswerTurn(ctx, req, messages, nudge)
		if stepErr != nil {
			setSpanError(span, stepErr)
			return nil, stepErr
		}

		tokenUsage["input"] += finalStep.Usage.InputTokens
		tokenUsage["output"] += finalStep.Usage.OutputTokens

		appendMessageTextPart(&messageParts, finalStep.Text)
		messages = append(messages, sharedModel.AgentMessage{
			Sequence: assistantSequence,
			Role:     sharedModel.RoleAssistant,
			Content:  contentBlocksFromText(finalStep.Text),
		})
		req.Messages = messages

		finalRes := stepResultToChatResponse(req.Model, finalStep)
		finalRes.Message.Sequence = assistantSequence
		finalRes.StopReason = sharedModel.StopReasonEndTurn
		recordAgentSuccess(span, finalRes, turnCount, totalToolCalls)
		return finalRes, nil
	}

	for turnCount < a.maxTurns {
		turnCount++
		recordAgentTurn(span, turnCount, len(messages), totalToolCalls)

		req.Messages = messages

		stepResult, err := a.runLLMStep(ctx, req)
		if err != nil {
			setSpanError(span, err)
			return nil, err
		}

		tokenUsage["input"] += stepResult.Usage.InputTokens
		tokenUsage["output"] += stepResult.Usage.OutputTokens
		if stepResult.MaxTokens > 0 {
			maxTokensUsed = stepResult.MaxTokens
		}

		hasStepResult = true
		recordAgentStopReason(span, stepResult.StopReason)

		switch stepResult.StopReason {

		case sharedModel.StopReasonToolUse:
			appendMessageTextPart(&messageParts, stepResult.Text)

			if len(stepResult.ToolCalls) == 0 {
				err := errs.New(errs.CodeInternalError, "llm requested tool use but returned no tool calls")
				setSpanError(span, err)
				return stepResultToChatResponse(req.Model, stepResult), err
			}

			recordToolCallsRequested(span, turnCount, len(stepResult.ToolCalls))

			// Out of tool budget. Answer the requested calls with errors rather than
			// dropping them — an unanswered tool_use block is rejected by the provider
			// on the next turn — then let the model finish from what it has.
			if totalToolCalls+len(stepResult.ToolCalls) > a.maxToolCalls {
				log.Warn(
					"agent exceeded max tool calls, forcing final answer",
					"totalToolCalls", totalToolCalls,
					"requested", len(stepResult.ToolCalls),
					"maxToolCalls", a.maxToolCalls,
				)
				messages = appendAssistantToolCalls(messages, assistantSequence, stepResult)
				messages = appendToolResults(
					messages,
					assistantSequence,
					refusedToolResults(stepResult.ToolCalls, toolBudgetExhaustedMessage),
				)
				return finishWithFinalAnswer(toolBudgetNudge)
			}

			// The model may request several tools in one turn; run them concurrently
			// but keep results positional so they pair with stepResult.ToolCalls.
			type toolExecution struct {
				tool   *tools.Tool
				result sharedModel.ToolResult
			}
			executions := make([]toolExecution, len(stepResult.ToolCalls))

			var toolWg sync.WaitGroup
			for i, toolCall := range stepResult.ToolCalls {
				toolWg.Add(1)
				go func(index int, call sharedModel.ToolCall) {
					defer toolWg.Done()

					tool, toolResult, execErr := a.executeTool(ctx, call)
					if execErr != nil {
						// Hand the failure back to the model instead of dropping it, so it
						// can correct a bad query rather than losing the whole run.
						log.Error("tool execution failed", "tool", call.Name, "error", execErr)
						executions[index] = toolExecution{result: toolErrorResult(call, execErr)}
						return
					}
					executions[index] = toolExecution{tool: tool, result: *toolResult}
				}(i, toolCall)
			}
			toolWg.Wait()

			toolResults := make([]sharedModel.ToolResult, 0, len(executions))
			for i, execution := range executions {
				toolCall := stepResult.ToolCalls[i]
				toolResults = append(toolResults, execution.result)

				var toolArgs map[string]any
				if err := json.Unmarshal(toolCall.Arguments, &toolArgs); err != nil {
					log.Error("error parsing tool call arguments", "tool", toolCall.Name, "error", err)
				}

				resultJSON, err := json.Marshal(execution.result)
				if err != nil {
					log.Error("error marshaling tool result", "tool", toolCall.Name, "error", err)
					continue
				}

				// An unresolved tool has no normalizer. The error still reaches the model
				// via toolResults above; record it for the run metadata and move on.
				if execution.tool == nil {
					appendMetaToolCall(&agentMetaToolCalls, toolCall, toolArgs, json.RawMessage(resultJSON))
					continue
				}

				normalizedResult := appendMessageToolCallPart(
					*execution.tool,
					toolCall,
					&messageParts,
					&agentMetaToolCalls,
					toolArgs,
					json.RawMessage(resultJSON),
				)

				if !req.Stream {
					continue
				}

				displayName := (*execution.tool).GetNormalizedName(true)
				switch {
				case execution.result.IsError:
					// Normalizers only understand success payloads, so surface the failure
					// directly rather than leaving the UI spinner running forever.
					if displayName == "" {
						continue
					}
					a.publishChatStreamEvent(
						ctx,
						utils.MustBudgetID(ctx),
						utils.MustUserID(ctx),
						conversationID,
						messageID,
						"tool_call",
						map[string]any{
							"id":          toolCall.ID,
							"displayName": displayName,
							"summary":     "Failed",
							"isError":     true,
						},
					)
				case normalizedResult != nil:
					a.publishChatStreamEvent(
						ctx,
						utils.MustBudgetID(ctx),
						utils.MustUserID(ctx),
						conversationID,
						messageID,
						"tool_call",
						map[string]any{
							"id":          toolCall.ID,
							"displayName": normalizedResult.DisplayName,
							"summary":     normalizedResult.Summary,
							"result":      string(normalizedResult.Result),
						},
					)
				}
			}

			totalToolCalls += len(toolResults)
			recordTotalToolCalls(span, totalToolCalls)

			// Preserve the assistant message that requested tools before appending
			// provider-neutral tool result messages.
			messages = appendAssistantToolCalls(messages, assistantSequence, stepResult)
			messages = appendToolResults(messages, assistantSequence, toolResults)
			req.Messages = messages
			continue

		case sharedModel.StopReasonEndTurn:
			appendMessageTextPart(&messageParts, stepResult.Text)

			messages = append(messages, sharedModel.AgentMessage{
				Sequence: assistantSequence,
				Role:     sharedModel.RoleAssistant,
				Content:  contentBlocksFromText(stepResult.Text),
			})
			req.Messages = messages

			res := stepResultToChatResponse(req.Model, stepResult)
			res.Message.Sequence = assistantSequence
			log.Info("StopReasonEndTurn", "res", *res)
			log.Info("llm response done: run loop is closing", "res", res)
			recordAgentSuccess(span, res, turnCount, totalToolCalls)
			return res, nil

		case sharedModel.StopReasonMaxTokens:
			// The reply was cut short. Partial text is more useful to the user than an
			// error, so return it; only fail when there is nothing at all to show.
			if strings.TrimSpace(stepResult.Text) == "" {
				err := errs.New(errs.CodeInternalError, "llm max tokens reached before producing any output")
				setSpanError(span, err)
				return stepResultToChatResponse(req.Model, stepResult), err
			}

			log.Warn("llm hit max tokens, returning truncated answer", "turn", turnCount)
			appendMessageTextPart(&messageParts, stepResult.Text)
			messages = append(messages, sharedModel.AgentMessage{
				Sequence: assistantSequence,
				Role:     sharedModel.RoleAssistant,
				Content:  contentBlocksFromText(stepResult.Text),
			})
			req.Messages = messages

			truncatedRes := stepResultToChatResponse(req.Model, stepResult)
			truncatedRes.Message.Sequence = assistantSequence
			recordAgentSuccess(span, truncatedRes, turnCount, totalToolCalls)
			return truncatedRes, nil

		case sharedModel.StopReasonError:
			err := errs.New(errs.CodeInternalError, "llm responded with error")
			if stepResult.Err != nil {
				err = errs.Wrap(errs.CodeInternalError, "llm responded with error", stepResult.Err)
			}
			log.Error("llm responded with error", "error", err)
			setSpanError(span, err)
			return stepResultToChatResponse(req.Model, stepResult), err

		default:
			err := errs.New(errs.CodeInternalError, "unsupported stop reason: %s", stepResult.StopReason)
			setSpanError(span, err)
			return stepResultToChatResponse(req.Model, stepResult), err
		}
	}

	// Turn budget exhausted. Messages already end with a complete tool_use /
	// tool_result pairing here, so one tools-disabled turn can still answer.
	log.Warn("agent exceeded max turns, forcing final answer", "maxTurns", a.maxTurns)
	if hasStepResult {
		return finishWithFinalAnswer(maxTurnsNudge)
	}

	err = errs.New(errs.CodeInternalError, "agent exceeded max turns without producing a response")
	setSpanError(span, err)
	return nil, err
}
