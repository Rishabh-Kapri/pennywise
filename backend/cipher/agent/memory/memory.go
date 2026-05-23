package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	agentPrompts "github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/context"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/pkoukk/tiktoken-go"
)

const (
	defaultTokenEncoding = "cl100k_base"
	tiktokenMultiplier   = 1.4
	observerMaxTokens    = 4096
)

type MemoryContextRequest struct {
	BudgetID       uuid.UUID
	ConversationID uuid.UUID
	UserID         uuid.UUID
	CurrentModel   string
	Messages       []sharedModel.AgentMessage
	PrevMessages   []sharedModel.ConversationMessage
	PrevRuns       []sharedModel.AgentRun
}

type MemoryContext struct {
	Messages           []sharedModel.AgentMessage
	ActiveObservations []sharedModel.AgentObservationalMemory
}

type tokenCountMessage struct {
	Sequence   int                        `json:"-"`
	Role       sharedModel.Role           `json:"role"`
	Content    []sharedModel.ContentBlock `json:"content,omitempty"`
	ToolCalls  []sharedModel.ToolCall     `json:"toolCalls,omitempty"`
	ToolResult *sharedModel.ToolResult    `json:"toolResult,omitempty"`
}

type AgentRunData struct {
	BudgetID       uuid.UUID
	UserID         uuid.UUID
	ConversationID uuid.UUID
	Messages       []sharedModel.AgentMessage
}

type Memory interface {
	GetWorkingMemory(ctx context.Context, budgetID uuid.UUID) string
	PrepareContext(ctx context.Context, req MemoryContextRequest) (*MemoryContext, error)
	OnRunPersisted(ctx context.Context, data AgentRunData) error
}

type memory struct {
	agentMemoryRepo  db.AgentMemoryRepository
	llmResolver      llm.LLMResolver
	bufferTokens     int
	messageTokens    int
	bufferActivation float32
}

// memory service should be singleton
func NewMemoryService(agentMemoryRepo db.AgentMemoryRepository, llmResolver llm.LLMResolver) Memory {
	service := &memory{
		agentMemoryRepo:  agentMemoryRepo,
		llmResolver:      llmResolver,
		bufferTokens:     2200,
		messageTokens:    8000,
		bufferActivation: 0.8,
	}
	return service
}

// returns messages in text format with system role messages removed
func formatMessagesForTokenCount(messages []sharedModel.AgentMessage, lastSequence *int) string {
	var tokenMessage []tokenCountMessage
	if lastSequence == nil || *lastSequence <= 0 {
		tokenMessage = make([]tokenCountMessage, 0, len(messages))
	} else {
		tokenMessage = make([]tokenCountMessage, 0, *lastSequence+1)
	}

	for _, msg := range messages {
		if lastSequence != nil && msg.Sequence <= *lastSequence {
			continue
		}
		if msg.Role == sharedModel.RoleSystem {
			continue
		}
		tokenMessage = append(tokenMessage, tokenCountMessage{
			Sequence:   msg.Sequence,
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCalls:  msg.ToolCalls,
			ToolResult: msg.ToolResult,
		})
	}

	payload, err := json.Marshal(tokenMessage)
	if err != nil {
		logger.Logger(context.Background()).Error("error while marshalling", "error", err)
		return ""
	}

	return string(payload)
}

func formatMessagesForObserver(messages []sharedModel.AgentMessage, lastSequence *int) string {
	var b strings.Builder
	hasContent := false

	b.WriteString("## New Message History To Observe\n\n")
	b.WriteString("System/developer messages are intentionally omitted. ")
	b.WriteString("Tool call arguments and internal IDs are intentionally omitted. ")
	b.WriteString("Tool results are included only as evidence for conversation continuity.\n\n")

	for _, msg := range messages {
		if lastSequence != nil && msg.Sequence <= *lastSequence {
			continue
		}
		if msg.Role == sharedModel.RoleSystem {
			continue
		}

		before := b.Len()
		if msg.ToolResult != nil {
			appendToolResultForObserver(&b, msg.ToolResult, msg.Sequence)
		} else if text := contentBlocksText(msg.Content); text != "" {
			appendObserverSection(&b, roleLabel(msg.Role), text, msg.Sequence)
		}
		if b.Len() > before {
			hasContent = true
		}

		// if b.Len() > observerTranscriptMaxChars {
		// 	b.WriteString("\n[Transcript truncated for observer input]\n")
		// 	break
		// }
	}

	if !hasContent {
		return ""
	}

	return strings.TrimSpace(b.String())
}

func appendToolResultForObserver(b *strings.Builder, result *sharedModel.ToolResult, sequence int) {
	if result == nil || result.Name == "get_schema" {
		return
	}

	text := contentBlocksText(result.Content)
	if text == "" {
		return
	}

	title := "Tool Result"
	switch result.Name {
	case "update_working_memory":
		title = "Working Memory Update Result"
	case "get_budget_info", "execute_sql":
		title = "Finance Lookup Result"
	}
	if result.IsError {
		title += " Error"
	}

	appendObserverSection(b, title, text, sequence)
}

func appendObserverSection(b *strings.Builder, title string, text string, sequence int) {
	b.WriteString(fmt.Sprintf("%d. ", sequence))
	b.WriteString(title)
	b.WriteString(":\n")
	b.WriteString(text)
	b.WriteString("\n\n")
}

func roleLabel(role sharedModel.Role) string {
	switch role {
	case sharedModel.RoleUser:
		return "User"
	case sharedModel.RoleAssistant:
		return "Assistant"
	case sharedModel.RoleTool:
		return "Tool Result"
	default:
		return fmt.Sprintf("%s", role)
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

func stripMarkdownFence(text string) string {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "```") {
		return text
	}

	// Drop the opening fence line, including optional language labels like
	// ```json. If the response is malformed and has no newline, leave it as-is.
	firstNewline := strings.IndexByte(text, '\n')
	if firstNewline == -1 {
		return text
	}
	text = strings.TrimSpace(text[firstNewline+1:])

	if endFence := strings.LastIndex(text, "```"); endFence != -1 {
		text = strings.TrimSpace(text[:endFence])
	}

	return text
}

func countTokens(messages []sharedModel.AgentMessage, lastSequence int) (int, error) {
	tkm, err := tiktoken.GetEncoding(defaultTokenEncoding)
	if err != nil {
		return 0, errs.Wrap(errs.CodeInternalError, "failed to get token encoding", err)
	}

	text := formatMessagesForTokenCount(messages, &lastSequence)

	tokens := tkm.Encode(text, nil, nil)
	tokenCount := tiktokenMultiplier * float32(len(tokens)) // tiktoken undercounts

	return int(tokenCount), nil
}

func observedSequence(messages []sharedModel.AgentMessage, lastSequence int) (seqStart, seqEnd int) {
	for _, msg := range messages {
		if msg.Role != sharedModel.RoleSystem && msg.Sequence > lastSequence {
			seqStart = msg.Sequence
			break
		}
	}

	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != sharedModel.RoleSystem && messages[i].Sequence > lastSequence {
			seqEnd = messages[i].Sequence
			break
		}
	}

	return seqStart, seqEnd
}

func observationsToMessages(b *strings.Builder, observations []sharedModel.AgentObservations) {
	priorityToText := map[sharedModel.ObservationPriority]string{
		sharedModel.ObservationPriorityHigh:      "🔴",
		sharedModel.ObservationPriorityMedium:    "🟡",
		sharedModel.ObservationPriorityLow:       "🟢",
		sharedModel.ObservationPriorityCompleted: "✅",
	}
	for _, obs := range observations {
		b.WriteString(fmt.Sprintf("-%s %s:%s %s\n", priorityToText[obs.Priority], obs.Date, obs.Time, obs.Text))
		if obs.SupportingDetails != nil {
			b.WriteString(fmt.Sprintf("%s\n\n", strings.Join(obs.SupportingDetails, "\n")))
		}
	}
}

func (m *memory) GetWorkingMemory(ctx context.Context, budgetID uuid.UUID) string {
	workingMemory, err := m.agentMemoryRepo.GetWorkingMemory(ctx, nil, budgetID)
	if err != nil {
		return ""
	}

	return string(workingMemory.Document)
}

func (m *memory) PrepareContext(ctx context.Context, req MemoryContextRequest) (*MemoryContext, error) {
	log := logger.Logger(ctx)

	tokenCount, err := countTokens(req.Messages, 0)
	if err != nil {
		log.Error("failed to count tokens", "error", err)
		return nil, err
	}
	log.Info(
		"tokens",
		"count",
		tokenCount,
		"budgetId",
		req.BudgetID,
		"userId",
		req.UserID,
		"conversationId",
		req.ConversationID,
	)

	if tokenCount < m.messageTokens {
		log.Info("skipping observation context addition: threshold not reached", "threshold", m.messageTokens)
		return &MemoryContext{req.Messages, nil}, nil
	}

	rawTailTokens := int((float32(m.messageTokens) * (1 - m.bufferActivation)))

	observations, err := m.agentMemoryRepo.GetObservationalMemory(
		ctx,
		nil,
		req.BudgetID,
		req.UserID,
		req.ConversationID,
	)
	if err != nil {
		return nil, err
	}

	messages := slices.Clone(req.Messages)
	activeObservations := []sharedModel.AgentObservationalMemory{}

	// log.Info("observational_memory", "om", observations)

	var b strings.Builder

	for _, om := range observations {
		startIdx := slices.IndexFunc(messages, func(m sharedModel.AgentMessage) bool {
			return m.Sequence == om.SequenceStart
		})

		// need special handling for end index, since we can have
		// 14 assistant tool call
		// 14 tool result
		// 14 assistant final answer
		// we want the last final answer index
		endIdx := -1
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Sequence == om.SequenceEnd {
				endIdx = i
				break
			}
		}

		if startIdx != -1 && endIdx != -1 && startIdx <= endIdx {
			// replace the sequence start and end with the observations
			observationsToMessages(&b, om.Observations)
			messages[startIdx] = sharedModel.AgentMessage{
				Role:     sharedModel.RoleSystem,
				Sequence: om.SequenceStart,
				Content:  []sharedModel.ContentBlock{{Type: "text", Text: b.String()}},
			}
			messages = slices.Delete(messages, startIdx+1, endIdx+1)
			b.Reset()
			activeObservations = append(activeObservations, om)
		}

		remainingTokens, _ := countTokens(messages, 0)

		if remainingTokens <= rawTailTokens {
			// we want to keep the raw messages intact from here on
			break
		}
	}
	pay, err := json.Marshal(messages)
	log.Info("message", "messages", string(pay))
	tokenCount, err = countTokens(messages, 0)
	log.Info("new token count", "count", tokenCount, "error", err)

	return &MemoryContext{messages, activeObservations}, nil
}

// This is called after the agent run is finished
// Create observation buffer chunks based on the thresholds
func (m *memory) OnRunPersisted(ctx context.Context, data AgentRunData) error {
	log := logger.Logger(ctx)

	// find the last observed sequence
	lastSequence, err := m.agentMemoryRepo.GetLastObservedSequence(
		ctx,
		nil,
		data.BudgetID,
		data.UserID,
		data.ConversationID,
	)
	if err != nil {
		log.Error("failed to get last observed sequence", "error", err)
		return err
	}

	tokenCount, err := countTokens(data.Messages, lastSequence)
	if err != nil {
		log.Error("failed to count tokens", "error", err)
		return err
	}

	if tokenCount < m.bufferTokens {
		log.Info(
			"skipping buffer observation generation: threshold not reached",
			"current",
			tokenCount,
			"threshold",
			m.bufferTokens,
		)
		return nil
	}

	// get transcript to
	transcript := formatMessagesForObserver(data.Messages, &lastSequence)
	if transcript == "" {
		log.Info("skipping observation generation: no observable transcript content")
		return nil
	}

	// generate observation
	systemPrompt := sharedModel.AgentMessage{
		Role: sharedModel.RoleSystem,
		Content: []sharedModel.ContentBlock{
			{Type: "text", Text: agentPrompts.ObservationalMemoryPrompt},
		},
	}
	historyMessage := sharedModel.AgentMessage{
		Role: sharedModel.RoleUser,
		Content: []sharedModel.ContentBlock{
			{Type: "text", Text: transcript},
		},
	}

	client, model, err := m.llmResolver.Resolve("openrouter", "google/gemini-2.5-flash")
	if err != nil {
		log.Error("error while resolving llm for memory persistence", "error", err)
		return nil
	}

	chatReq := sharedModel.ChatRequest{
		Provider:    "openrouter",
		Model:       model,
		Messages:    []sharedModel.AgentMessage{systemPrompt, historyMessage},
		Tools:       nil,
		ToolChoice:  []sharedModel.ToolChoice{{Type: sharedModel.ToolChoiceNone}},
		MaxTokens:   observerMaxTokens,
		Temperature: 0.2,
	}
	res, err := client.Chat(ctx, chatReq)
	if err != nil {
		log.Error("error on observation chat request", "error", err)
		return nil
	}
	if res == nil {
		log.Error("no res returned from the observation llm")
		return nil
	}
	if len(res.Message.Content) == 0 {
		log.Error("no message content returned but the observation llm", "res", *res)
		return nil
	}

	log.Info("observation", "res", res)

	msg := res.Message.Content[0]
	var observation sharedModel.AgentObservationalMemory

	observerJSON := stripMarkdownFence(msg.Text)
	if err := json.Unmarshal([]byte(observerJSON), &observation); err != nil {
		log.Error(
			"failed to unmarshal llm observation res",
			"error",
			err,
			"stopReason",
			res.StopReason,
			"outputTokens",
			res.Usage.OutputTokens,
			"outputChars",
			len(observerJSON),
		)
		return nil
	}

	if len(observation.Observations) == 0 {
		log.Error("llm generated no observations, skipping")
		return nil
	}

	sequenceStart, sequenceEnd := observedSequence(data.Messages, lastSequence)
	if sequenceStart == 0 || sequenceEnd == 0 {
		log.Warn("wrong sequences received", "sequenceStart", sequenceStart, "sequenceEnd", sequenceEnd)
		return nil
	}

	_, err = m.agentMemoryRepo.CreateObservationalMemory(ctx, nil, sharedModel.AgentObservationalMemory{
		BudgetID:          data.BudgetID,
		UserID:            data.UserID,
		ConversationID:    data.ConversationID,
		SequenceStart:     sequenceStart,
		SequenceEnd:       sequenceEnd,
		Observations:      observation.Observations,
		CurrentTask:       observation.CurrentTask,
		SuggestedResponse: observation.SuggestedResponse,
	})
	if err != nil {
		log.Error("error while creating observation", "error", err)
		return err
	}

	return nil
}

func (m *memory) GenerateReflections() {}
