package service

import (
	"context"
	"encoding/json"

	// "encoding/json"
	stderrors "errors"
	"fmt"
	"strings"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	defaultAgentKey    = "chat"
	defaultTemperature = 0.2
	defaultMaxTokens   = 1024
)

type AgentClient interface {
	CreateRun(ctx context.Context, req sharedModel.AgentRunCreateRequest) (*sharedModel.AgentRun, error)
	GetRun(ctx context.Context, id uuid.UUID) (*sharedModel.AgentRun, error)
	CancelRun(ctx context.Context, id uuid.UUID) (*sharedModel.AgentRun, error)
}

type cipherAgentClient struct {
	client *transport.Client
}

func NewCipherAgentClient(client *transport.Client) AgentClient {
	return &cipherAgentClient{client: client}
}

func (c *cipherAgentClient) CreateRun(
	ctx context.Context,
	req sharedModel.AgentRunCreateRequest,
) (*sharedModel.AgentRun, error) {
	return transport.Post[*sharedModel.AgentRun](ctx, c.client, "/api/agent/runs", nil, req)
}

func (c *cipherAgentClient) GetRun(ctx context.Context, id uuid.UUID) (*sharedModel.AgentRun, error) {
	return transport.Get[*sharedModel.AgentRun](ctx, c.client, fmt.Sprintf("/api/agent/runs/%s", id.String()))
}

func (c *cipherAgentClient) CancelRun(ctx context.Context, id uuid.UUID) (*sharedModel.AgentRun, error) {
	return transport.Post[*sharedModel.AgentRun](
		ctx,
		c.client,
		fmt.Sprintf("/api/agent/runs/%s/cancel", id.String()),
		nil,
		nil,
	)
}

type AgentService interface {
	GetConversations(ctx context.Context) ([]sharedModel.AgentConversation, error)
	GetConversationMessages(ctx context.Context, id uuid.UUID) ([]sharedModel.ConversationMessage, error)
	CreateRun(ctx context.Context, req sharedModel.AgentRunCreateRequest) (*sharedModel.AgentRun, error)
	GetRun(ctx context.Context, id uuid.UUID) (*sharedModel.AgentRun, error)
	CancelRun(ctx context.Context, id uuid.UUID) (*sharedModel.AgentRun, error)
	UpdateConversation(ctx context.Context, conversationID uuid.UUID, data sharedModel.AgentConversation) error
	DeleteConversation(ctx context.Context, conversationID uuid.UUID) error
	UpdateConversationMessageContent(ctx context.Context, messageID uuid.UUID, message []sharedModel.MessagePart) error
	UpdateEntityMetadata(ctx context.Context, entity string, id uuid.UUID, data map[string]any) error
}

type agentService struct {
	client AgentClient
	repo   repository.AgentRepository
}

func NewAgentService(client AgentClient, repo repository.AgentRepository) AgentService {
	return &agentService{client: client, repo: repo}
}

func (s *agentService) GetConversations(ctx context.Context) ([]sharedModel.AgentConversation, error) {
	budgetID := utils.MustBudgetID(ctx)
	userID := utils.MustUserID(ctx)

	return s.repo.GetAllConversations(ctx, userID, budgetID)
}

func (s *agentService) GetConversationMessages(
	ctx context.Context,
	id uuid.UUID,
) ([]sharedModel.ConversationMessage, error) {
	_ = utils.MustBudgetID(ctx)
	_ = utils.MustUserID(ctx)

	return s.repo.ListConversationMessages(ctx, id, nil)
}

func (s *agentService) continueRun(
	ctx context.Context,
	req sharedModel.AgentRunCreateRequest,
	userID uuid.UUID,
	budgetID uuid.UUID,
	conversation *sharedModel.AgentConversation,
	agentKey string,
) {
	log := logger.Logger(ctx)

	var run *sharedModel.AgentRun
	var userMessage *sharedModel.ConversationMessage
	var assistantMsg *sharedModel.ConversationMessage
	var prevMessages []sharedModel.ConversationMessage
	var prevAgentRuns []sharedModel.AgentRun

	err := utils.WithTx(ctx, s.repo.GetDB(), func(tx pgx.Tx) error {
		var err error

		prevMessages, err = s.repo.ListConversationMessages(ctx, *req.ConversationID, nil)
		if err != nil {
			return wrapAgentRepoError(
				errs.CodeAgentMessageLookupFailed,
				"failed to get previous conversation messages",
				err,
			)
		}

		prevAgentRuns, err = s.repo.GetAllConversationRuns(ctx, tx, *req.ConversationID)
		if err != nil {
			return wrapAgentRepoError(
				errs.CodeAgentRunLookupFailed,
				"failed to get previous conversation runs",
				err,
			)
		}

		runMetadata := req.Metadata

		temperature := defaultTemperature
		maxTokens := defaultMaxTokens
		if req.Temperature == nil {
			req.Temperature = &temperature
		}
		if req.MaxTokens == nil {
			req.MaxTokens = &maxTokens
		}

		if runMetadata == nil {
			maxTokens := defaultMaxTokens
			if req.MaxTokens != nil {
				maxTokens = *req.MaxTokens
			}
			temperature := defaultTemperature
			if req.Temperature != nil {
				temperature = *req.Temperature
			}
			runMetadata = map[string]any{}
			runMetadata["traceId"] = uuid.NewString()
			runMetadata["model"] = *req.ModelProvider + "/" + *req.ModelName
			runMetadata["maxTokens"] = maxTokens
			runMetadata["temperature"] = temperature
		}
		req.Metadata = runMetadata

		run, err = s.repo.CreateRun(ctx, tx, repository.CreateAgentRunParams{
			UserID:         userID,
			BudgetID:       budgetID,
			AgentKey:       agentKey,
			ConversationID: conversation.ID,
			ModelProvider:  req.ModelProvider,
			ModelName:      req.ModelName,
			Temperature:    req.Temperature,
			MaxTokens:      req.MaxTokens,
			Metadata:       runMetadata,
		})
		if err != nil {
			return wrapAgentRepoError(errs.CodeAgentRunCreateFailed, "failed to create agent run", err)
		}

		// create user message
		userMessageContent := []sharedModel.MessagePart{
			{Type: sharedModel.MessageTypeText, Content: &req.Message},
		}
		userMessageJSON, err := json.Marshal(userMessageContent)
		if err != nil {
			return errs.Wrap(errs.CodeInternalError, "failed to marshal user message", err)
		}

		userMessage, err = s.repo.CreateConversationMessage(ctx, tx, repository.CreateConversationMessageParams{
			ConversationID: conversation.ID,
			RunID:          &run.ID,
			Role:           sharedModel.RoleUser,
			Content:        json.RawMessage(userMessageJSON),
		})
		if err != nil {
			return wrapAgentRepoError(
				errs.CodeAgentMessageCreateFailed,
				"failed to create user conversation message",
				err,
			)
		}

		// create initial assistant message
		assistantMsgContent := []sharedModel.MessagePart{}
		assitantMsgJSON, err := json.Marshal(assistantMsgContent)
		if err != nil {
			return errs.Wrap(errs.CodeInternalError, "failed to marshal assistant message", err)
		}

		assistantMsg, err = s.repo.CreateConversationMessage(ctx, tx, repository.CreateConversationMessageParams{
			ConversationID: conversation.ID,
			RunID:          &run.ID,
			Role:           sharedModel.RoleAssistant,
			Content:        json.RawMessage(assitantMsgJSON),
		})
		if err != nil {
			return wrapAgentRepoError(
				errs.CodeAgentMessageCreateFailed,
				"failed to create assistant conversation message",
				err,
			)
		}

		return nil
	})
	if err != nil {
		log.Error("failed to create assistant conversation message", err)
		return
	}

	run.Conversation = conversation
	run.ConversationID = &conversation.ID
	run.Messages = []sharedModel.ConversationMessage{*userMessage}
	run.UserMessage = req.Message

	dispatchReq := req
	dispatchReq.RunID = &run.ID
	dispatchReq.AgentKey = &agentKey
	dispatchReq.ConversationID = &conversation.ID
	dispatchReq.MessageID = &assistantMsg.ID
	dispatchReq.PrevMessages = prevMessages
	dispatchReq.PrevRuns = prevAgentRuns

	dispatchedRun, err := s.client.CreateRun(ctx, dispatchReq)
	if err != nil {
		failedRun, updateErr := s.setRunStatus(
			ctx,
			run.ID,
			budgetID,
			sharedModel.AgentRunStatusFailed,
			ptrString(err.Error()),
		)
		if updateErr == nil {
			log.Error("failed to update run status", "failedRun", failedRun, "error", updateErr)
			// return failedRun, errs.Wrap(errs.CodeAgentDispatchFailed, "failed to dispatch agent run", err)
		}

		log.Error("failed to dispatch agent run", "error", err)
		// return run, errs.Wrap(errs.CodeAgentDispatchFailed, "failed to dispatch agent run", err)
	}

	status := sharedModel.AgentRunStatusRunning
	if dispatchedRun != nil && validAgentRunStatus(dispatchedRun.Status) {
		status = dispatchedRun.Status
	}
	updatedRun, err := s.setRunStatus(ctx, run.ID, budgetID, status, nil)
	if err != nil {
		log.Error("failed to update run status", "error", err)
		return
	}

	log.Info("dispatched agent run", "run", updatedRun)
	return
}

func (s *agentService) CreateRun(
	ctx context.Context,
	req sharedModel.AgentRunCreateRequest,
) (*sharedModel.AgentRun, error) {
	if s.client == nil {
		return nil, errs.New(errs.CodeInternalError, "agent client is not configured")
	}
	if s.repo == nil {
		return nil, errs.New(errs.CodeInternalError, "agent repository is not configured")
	}

	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		return nil, errs.New(errs.CodeInvalidArgument, "message is required")
	}

	userID, budgetID, err := agentContext(ctx)
	if err != nil {
		return nil, err
	}

	agentKey, err := normalizeAgentKey(req.AgentKey)
	if err != nil {
		return nil, err
	}
	if err := normalizeAgentModel(&req); err != nil {
		return nil, err
	}

	var conversation *sharedModel.AgentConversation

	if req.ConversationID != nil {
		conversation, err = s.repo.GetConversationForUpdate(
			ctx,
			nil,
			*req.ConversationID,
			userID,
			budgetID,
			agentKey,
		)
		if err != nil {
			return nil, wrapAgentRepoError(
				errs.CodeAgentConversationLookupFailed,
				"failed to get agent conversation",
				err,
			)
		}

		req.ConversationMetadata = conversation.Metadata
	} else {
		conversationMetadata := req.ConversationMetadata

		if conversationMetadata == nil {
			conversationMetadata = map[string]any{}
			conversationMetadata["defaultModel"] = *req.ModelProvider + "/" + *req.ModelName
		}
		if req.Title == nil {
			conversationMetadata["titleSource"] = "auto"
		}

		conversation, err = s.repo.CreateConversation(ctx, nil, repository.CreateAgentConversationParams{
			UserID:   userID,
			BudgetID: budgetID,
			AgentKey: agentKey,
			Title:    trimOptionalString(req.Title),
			Metadata: conversationMetadata,
		})
		if err != nil {
			return nil, wrapAgentRepoError(
				errs.CodeAgentConversationCreateFailed,
				"failed to create agent conversation",
				err,
			)
		}

		req.ConversationID = &conversation.ID
		req.ConversationMetadata = conversationMetadata
	}

	// pass a detached context to not close the conntext after the request is done
	backgroundCtx := utils.DetachedRequestContext(ctx)
	go s.continueRun(backgroundCtx, req, userID, budgetID, conversation, agentKey)

	agentRun := sharedModel.AgentRun{
		UserID:         &userID,
		BudgetID:       &budgetID,
		ConversationID: &conversation.ID,
		Status:         sharedModel.AgentRunStatusQueued,
	}

	return &agentRun, nil
}

func (s *agentService) GetRun(ctx context.Context, id uuid.UUID) (*sharedModel.AgentRun, error) {
	if s.repo == nil {
		return nil, errs.New(errs.CodeInternalError, "agent repository is not configured")
	}
	userID, budgetID, err := agentContext(ctx)
	if err != nil {
		return nil, err
	}
	return s.getRun(ctx, userID, budgetID, id)
}

func (s *agentService) CancelRun(ctx context.Context, id uuid.UUID) (*sharedModel.AgentRun, error) {
	if s.client == nil {
		return nil, errs.New(errs.CodeInternalError, "agent client is not configured")
	}
	if s.repo == nil {
		return nil, errs.New(errs.CodeInternalError, "agent repository is not configured")
	}
	userID, budgetID, err := agentContext(ctx)
	if err != nil {
		return nil, err
	}

	run, err := s.getRun(ctx, userID, budgetID, id)
	if err != nil {
		return nil, err
	}
	if terminalAgentRunStatus(run.Status) {
		return run, nil
	}

	if _, err := s.client.CancelRun(ctx, id); err != nil {
		return run, errs.Wrap(errs.CodeAgentCancelFailed, "failed to cancel agent run", err)
	}

	return s.setRunStatus(ctx, id, budgetID, sharedModel.AgentRunStatusCancelled, nil)
}

func (s *agentService) UpdateConversation(
	ctx context.Context,
	conversationID uuid.UUID,
	data sharedModel.AgentConversation,
) error {
	return s.repo.UpdateConversation(ctx, nil, conversationID, data)
}

func (s *agentService) DeleteConversation(ctx context.Context, conversationID uuid.UUID) error {
	if s.repo == nil {
		return errs.New(errs.CodeInternalError, "agent repository is not configured")
	}

	userID, budgetID, err := agentContext(ctx)
	if err != nil {
		return err
	}

	return utils.WithTx(ctx, s.repo.GetDB(), func(tx pgx.Tx) error {
		return s.repo.DeleteConversation(ctx, tx, conversationID, userID, budgetID)
	})
}

func (s *agentService) UpdateConversationMessageContent(
	ctx context.Context,
	messageID uuid.UUID,
	content []sharedModel.MessagePart,
) error {
	return s.repo.UpdateConversationMessageContent(ctx, nil, messageID, content)
}

func (s *agentService) UpdateEntityMetadata(
	ctx context.Context,
	entity string,
	id uuid.UUID,
	data map[string]any,
) error {
	return s.repo.UpdateEntityMetadata(ctx, nil, entity, id, data)
}

func (s *agentService) getRun(
	ctx context.Context,
	userID uuid.UUID,
	budgetID uuid.UUID,
	id uuid.UUID,
) (*sharedModel.AgentRun, error) {
	run, err := s.repo.GetRun(ctx, userID, budgetID, id)
	if err != nil {
		return nil, wrapAgentRepoError(errs.CodeAgentRunLookupFailed, "failed to get agent run", err)
	}

	if run.ConversationID != nil {
		conversation, err := s.repo.GetConversation(ctx, *run.ConversationID, userID, budgetID)
		if err != nil && !hasAgentErrorCode(err, errs.CodeAgentConversationNotFound) {
			return nil, wrapAgentRepoError(
				errs.CodeAgentConversationLookupFailed,
				"failed to get agent conversation",
				err,
			)
		}
		run.Conversation = conversation

		// for this run and conversation find all the messages
		messages, err := s.repo.ListConversationMessages(ctx, *run.ConversationID, &run.ID)
		if err != nil {
			return nil, wrapAgentRepoError(
				errs.CodeAgentMessageLookupFailed,
				"failed to list conversation messages",
				err,
			)
		}
		run.Messages = messages
		// for _, message := range messages {
		// 	switch message.Role {
		// 	case sharedModel.RoleUser:
		// 		if run.UserMessage == "" {
		// 			run.UserMessage = message.Text
		// 		}
		// 	case sharedModel.RoleAssistant:
		// 		text := message.Text
		// 		run.FinalMessage = &text
		// 	}
		// }
	}

	return run, nil
}

func (s *agentService) setRunStatus(
	ctx context.Context,
	id uuid.UUID,
	budgetID uuid.UUID,
	status sharedModel.AgentRunStatus,
	errorMessage *string,
) (*sharedModel.AgentRun, error) {
	run, err := s.repo.UpdateRunStatus(ctx, id, budgetID, status, errorMessage)
	if err != nil {
		return nil, wrapAgentRepoError(errs.CodeAgentRunUpdateFailed, "failed to update agent run status", err)
	}
	if run.UserID == nil {
		return run, nil
	}
	return s.getRun(ctx, *run.UserID, budgetID, id)
}

func agentContext(ctx context.Context) (uuid.UUID, uuid.UUID, error) {
	userID, err := utils.UserIDFromContext(ctx)
	if err != nil {
		return uuid.Nil, uuid.Nil, errs.Wrap(errs.CodeInvalidArgument, "missing user context", err)
	}
	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		return uuid.Nil, uuid.Nil, errs.Wrap(errs.CodeInvalidArgument, "missing budget context", err)
	}
	return userID, budgetID, nil
}

func normalizeAgentKey(agentKey *string) (string, error) {
	value := defaultAgentKey
	if agentKey != nil {
		value = strings.TrimSpace(*agentKey)
	}
	if value == "" {
		value = defaultAgentKey
	}
	if value != defaultAgentKey {
		return "", errs.New(errs.CodeInvalidArgument, "unsupported agent key")
	}
	return value, nil
}

func normalizeAgentModel(req *sharedModel.AgentRunCreateRequest) error {
	req.ModelProvider = trimOptionalString(req.ModelProvider)
	req.ModelName = trimOptionalString(req.ModelName)
	if (req.ModelProvider == nil) != (req.ModelName == nil) {
		return errs.New(errs.CodeInvalidArgument, "modelProvider and modelName must be provided together")
	}
	if req.Temperature != nil && *req.Temperature < 0 {
		return errs.New(errs.CodeInvalidArgument, "temperature must be non-negative")
	}
	if req.MaxTokens != nil && *req.MaxTokens <= 0 {
		return errs.New(errs.CodeInvalidArgument, "maxTokens must be greater than zero")
	}
	return nil
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func wrapAgentRepoError(code errs.Code, message string, err error) error {
	if err == nil {
		return nil
	}
	if hasAgentErrorCode(err, errs.CodeAgentRunNotFound) || hasAgentErrorCode(err, errs.CodeAgentConversationNotFound) {
		return err
	}
	return errs.Wrap(code, message, err)
}

func hasAgentErrorCode(err error, code errs.Code) bool {
	var apiErr *errs.Error
	return stderrors.As(err, &apiErr) && apiErr.Code == code
}

func validAgentRunStatus(status sharedModel.AgentRunStatus) bool {
	switch status {
	case sharedModel.AgentRunStatusQueued,
		sharedModel.AgentRunStatusRunning,
		sharedModel.AgentRunStatusCompleted,
		sharedModel.AgentRunStatusFailed,
		sharedModel.AgentRunStatusCancelled:
		return true
	default:
		return false
	}
}

func terminalAgentRunStatus(status sharedModel.AgentRunStatus) bool {
	switch status {
	case sharedModel.AgentRunStatusCompleted, sharedModel.AgentRunStatusFailed, sharedModel.AgentRunStatusCancelled:
		return true
	default:
		return false
	}
}

func ptrString(value string) *string {
	return &value
}
