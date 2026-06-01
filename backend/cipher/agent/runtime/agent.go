package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	agentContext "github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/context"
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

type AgentConfig struct {
	titleModel     string
	telemetry      *otelSDK.Telemetry
	redis          *redis.Client
	toolRegistry   *tools.ToolRegistry
	contextBuilder agentContext.ContextBuilder
	maxTurns       int
	maxToolCalls   int
	pennywiseAPI   *transport.Client
	memoryEnabled  bool
	memory         memory.Memory
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

func WithContextBuilder(cb agentContext.ContextBuilder) AgentOption {
	return func(ac *AgentConfig) {
		ac.contextBuilder = cb
	}
}

func WithMaxTurns(maxTurns int) AgentOption {
	return func(ac *AgentConfig) {
		ac.maxTurns = maxTurns
	}
}

func WithMaxToolCalls(maxToolCalls int) AgentOption {
	return func(ac *AgentConfig) {
		ac.maxToolCalls = maxToolCalls
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
	llmResolver    llm.LLMResolver
	classifyModel  string
	TitleModel     string  // model to use for title generation
	localLLM       llm.LLM // always local (Ollama) — used for narrating raw SQL results
	localModel     string
	telemetry      *otelSDK.Telemetry
	redisClient    *redis.Client
	toolRegistry   *tools.ToolRegistry
	contextBuilder agentContext.ContextBuilder
	maxTurns       int
	maxToolCalls   int
	pennywiseAPI   *transport.Client
	memoryEnabled  bool
	memory         memory.Memory
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
		llmResolver: llmResolver,
		TitleModel:  "openai/gpt-5.4",
		// localLLM:       ollamaObserved,
		localModel:     "gemma4",
		telemetry:      cfg.telemetry,
		redisClient:    cfg.redis,
		toolRegistry:   toolRegistry,
		contextBuilder: cfg.contextBuilder,
		maxTurns:       10,
		maxToolCalls:   10,
		pennywiseAPI:   cfg.pennywiseAPI,
		memoryEnabled:  cfg.memoryEnabled,
		memory:         cfg.memory,
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

// narrateWithOllama sends raw execute_sql rows to local Ollama and replaces the
// tool result content with a concise natural-language summary. Raw row data
// never reaches the cloud LLM (Claude).
func (a *Agent) narrateWithOllama(ctx context.Context, result *sharedModel.ToolResult) *sharedModel.ToolResult {
	if a.localLLM == nil || result == nil {
		return result
	}
	log := logger.Logger(ctx)

	rawContent := ""
	if result.Content != nil {
		rawContent = fmt.Sprintf("%v", result.Content)
	}

	narrateReq := sharedModel.ChatRequest{
		Model: a.localModel,
		Messages: []sharedModel.AgentMessage{
			{
				Role: sharedModel.RoleSystem,
				Content: []sharedModel.ContentBlock{{Type: "text", Text: `You are a financial data summariser. 
You will receive raw SQL query results as JSON rows. 
Summarise them in 1-3 plain English sentences that directly answer the user's question.
Do NOT reveal individual transaction amounts or bank-specific payee strings.
Use aggregated totals, counts, or category names only.
Be concise.`}},
			},
			{
				Role:    sharedModel.RoleUser,
				Content: []sharedModel.ContentBlock{{Type: "text", Text: "SQL results:\n" + rawContent}},
			},
		},
		MaxTokens: 256,
		Stream:    false,
	}

	res, err := a.localLLM.Chat(ctx, narrateReq)
	if err != nil {
		log.Warn("narrateWithOllama: local LLM failed, returning raw result", "error", err)
		return result
	}

	narration := messageContentText(res.Message.Content)
	if narration == "" {
		return result
	}

	// Replace the raw content with the narration string so Claude only sees it.
	narrated := *result
	narrated.Content = []sharedModel.ContentBlock{{Type: "text", Text: narration}}
	return &narrated
}

// Add system prompts to the LLM request.
func injectSystemContext(
	systemPrompt string,
	reqMessages []sharedModel.AgentMessage,
	budgetId uuid.UUID,
) []sharedModel.AgentMessage {
	// contextJSON, err := json.Marshal(budgetContext)
	// if err != nil {
	// 	contextJSON = []byte(fmt.Sprintf("%+v", budgetContext))
	// }

	contextMessage := sharedModel.AgentMessage{
		Role: sharedModel.RoleSystem,
		Content: []sharedModel.ContentBlock{
			{
				Type: "text",
				Text: fmt.Sprintf(
					`%s.
					Current scoped budget context.

The conversation is already scoped to this budget:
budget_id: %s

Use this budget_id internally when calling tools that require a budgetID. Do not ask the user which budget to use. Use category IDs internally when calling tools, but refer to categories by name in user-facing responses. Do not reveal internal IDs to the user at any cost.
`,
					systemPrompt, budgetId.String(),
				),
			},
		},
	}

	messages := make([]sharedModel.AgentMessage, 0, len(reqMessages)+1)
	insertAt := 0
	for insertAt < len(reqMessages) && reqMessages[insertAt].Role == sharedModel.RoleSystem {
		insertAt++
	}

	messages = append(messages, reqMessages[:insertAt]...) // copy any existing system prompt
	messages = append(messages, contextMessage)            // copy context message
	messages = append(messages, reqMessages[insertAt:]...) // copy rest of the messages

	return messages
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
	normalizedResult, err := tool.Normalize(toolCall, resultJSON)
	if err != nil {
		return nil
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
	return normalizedResult
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
) (*sharedModel.ChatResponse, error) {
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

	var err error
	var lastStepResult sharedModel.StepResult
	hasStepResult := false

	messages := make([]sharedModel.AgentMessage, len(req.Messages), len(req.Messages)+1)
	copy(messages, req.Messages)
	assistantSequence := nextAgentMessageSequence(messages)

	// Enrich with tools
	if runOpts.enableTools && len(a.toolRegistry.GetAllTools()) > 0 {
		log.Info("enriching with tools", "tools", a.toolRegistry.GetAllTools())

		req.Tools = make([]sharedModel.ToolDefiniton, 0)

		for _, tool := range a.toolRegistry.GetAllTools() {
			req.Tools = append(req.Tools, tool.Definition())
			enabledTools = append(enabledTools, tool.Definition().Name)
		}
	}
	log.Info("context builder", "builder", a.contextBuilder)

	// Enrich with budget context, for now we only put budgetID
	// if a.contextBuilder != nil && runOpts.requiresContext {
	// 	// For now budgetId is hardcoded, take this from req later.
	// 	budgetID := utils.MustBudgetID(ctx)
	// 	messages = injectSystemContext(a.contextBuilder.GetSystemPrompt(), messages, budgetID)
	// }
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

		lastStepResult = stepResult
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

			if totalToolCalls+len(stepResult.ToolCalls) > a.maxToolCalls {
				err := errs.New(errs.CodeInternalError, "agent exceeded max tool calls")
				setSpanError(span, err)
				return stepResultToChatResponse(req.Model, stepResult), err
			}

			recordToolCallsRequested(span, turnCount, len(stepResult.ToolCalls))

			toolResults := make([]sharedModel.ToolResult, 0)
			for _, toolCall := range stepResult.ToolCalls {
				tool, toolResult, err := a.executeTool(ctx, toolCall)
				if err != nil {
					log.Error(err.Error())
					continue
				}
				// Narrate execute_sql raw rows via local Ollama so Claude never
				// sees individual transaction amounts or bank-format strings.
				// if toolCall.Name == "execute_sql" {
				// 	toolResult = a.narrateWithOllama(ctx, toolResult)
				// }
				toolResults = append(toolResults, *toolResult)

				var toolArgs map[string]any
				err = json.Unmarshal(toolCall.Arguments, &toolArgs)
				if err != nil {
					log.Error(err.Error())
				}

				resultJSON, err := json.Marshal(toolResult)
				if err != nil {
					log.Error(err.Error())
					continue
				}
				normalizedResult := appendMessageToolCallPart(
					*tool,
					toolCall,
					&messageParts,
					&agentMetaToolCalls,
					toolArgs,
					json.RawMessage(resultJSON),
				)
				if req.Stream && normalizedResult != nil {
					toolMessage := map[string]any{
						"id":          toolCall.ID,
						"displayName": normalizedResult.DisplayName,
						"summary":     normalizedResult.Summary,
						"result":      string(normalizedResult.Result),
					}
					a.publishChatStreamEvent(
						ctx,
						utils.MustBudgetID(ctx),
						utils.MustUserID(ctx),
						conversationID,
						messageID,
						"tool_call",
						toolMessage,
					)
				}
			}
			if len(toolResults) == 0 {
				err := errs.New(errs.CodeToolExecuteFail, "no tool results produced")
				setSpanError(span, err)
				return stepResultToChatResponse(req.Model, stepResult), err
			}
			totalToolCalls += len(toolResults)
			recordTotalToolCalls(span, totalToolCalls)

			// Preserve the assistant message that requested tools before appending
			// provider-neutral tool result messages.
			messages = append(messages, sharedModel.AgentMessage{
				Sequence:  assistantSequence,
				Role:      sharedModel.RoleAssistant,
				Content:   contentBlocksFromText(stepResult.Text),
				ToolCalls: stepResult.ToolCalls,
			})
			for i := range toolResults {
				messages = append(messages, sharedModel.AgentMessage{
					Sequence:   assistantSequence,
					Role:       sharedModel.RoleTool,
					ToolResult: &toolResults[i],
				})
			}
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
			err := errs.New(errs.CodeInternalError, "llm max tokens reached")
			setSpanError(span, err)
			return stepResultToChatResponse(req.Model, stepResult), err

		case sharedModel.StopReasonError:
			err := errs.New(errs.CodeInternalError, "llm responded with error")
			log.Error("llm responded with error", "error", err)
			setSpanError(span, err)
			return stepResultToChatResponse(req.Model, stepResult), err

		default:
			err := errs.New(errs.CodeInternalError, "unsupported stop reason: %s", stepResult.StopReason)
			setSpanError(span, err)
			return stepResultToChatResponse(req.Model, stepResult), err
		}
	}

	err = errs.New(errs.CodeInternalError, "agent exceeded max turns")
	setSpanError(span, err)
	if hasStepResult {
		return stepResultToChatResponse(req.Model, lastStepResult), err
	}
	return nil, err
}
