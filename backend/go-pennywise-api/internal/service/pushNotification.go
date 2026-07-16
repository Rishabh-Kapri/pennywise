package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/client"
	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

// PushNotificationService manages device push tokens and sends transaction
// pushes through Expo. Sends are best-effort: they log failures and never
// return an error to callers on the pipeline path.
type PushNotificationService interface {
	RegisterToken(ctx context.Context, req model.DevicePushTokenReq) (*model.DevicePushToken, error)
	UnregisterToken(ctx context.Context, req model.DevicePushTokenReq) error
	// NotifyTransactionsCreated pushes a "transaction.created" notification for
	// each transaction to every device of the budget's owner. The mobile app
	// uses the data payload to attach the device's current location.
	NotifyTransactionsCreated(ctx context.Context, budgetId uuid.UUID, txns []model.Transaction)
}

type pushNotificationService struct {
	repo       repository.DevicePushTokenRepository
	expoClient client.ExpoPushClient
}

func NewPushNotificationService(
	repo repository.DevicePushTokenRepository,
	expoClient client.ExpoPushClient,
) PushNotificationService {
	return &pushNotificationService{repo: repo, expoClient: expoClient}
}

func (s *pushNotificationService) RegisterToken(
	ctx context.Context,
	req model.DevicePushTokenReq,
) (*model.DevicePushToken, error) {
	userId := utils.MustUserID(ctx)
	if req.ExpoPushToken == "" {
		return nil, errs.New(errs.CodeInvalidArgument, "expoPushToken is required")
	}
	platform := req.Platform
	if platform == "" {
		platform = "android"
	}
	return s.repo.Upsert(ctx, userId, req.ExpoPushToken, platform)
}

func (s *pushNotificationService) UnregisterToken(ctx context.Context, req model.DevicePushTokenReq) error {
	userId := utils.MustUserID(ctx)
	if req.ExpoPushToken == "" {
		return errs.New(errs.CodeInvalidArgument, "expoPushToken is required")
	}
	return s.repo.DeleteByToken(ctx, userId, req.ExpoPushToken)
}

func (s *pushNotificationService) NotifyTransactionsCreated(
	ctx context.Context,
	budgetId uuid.UUID,
	txns []model.Transaction,
) {
	if len(txns) == 0 {
		return
	}

	// never let push delivery hang or fail the caller
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()

	tokens, err := s.repo.GetByBudgetId(sendCtx, budgetId)
	if err != nil {
		logger.Logger(ctx).Warn("failed to load push tokens, skipping push", "err", err)
		return
	}
	if len(tokens) == 0 {
		return
	}

	messages := make([]client.ExpoPushMessage, 0, len(tokens)*len(txns))
	for _, txn := range txns {
		body := fmt.Sprintf("Amount: %.2f", txn.Amount)
		if txn.Summary != nil && *txn.Summary != "" {
			body = *txn.Summary
		}
		for _, token := range tokens {
			messages = append(messages, client.ExpoPushMessage{
				To:       token.ExpoPushToken,
				Title:    "New transaction",
				Body:     body,
				Priority: "high",
				Data: map[string]any{
					"type":          "transaction.created",
					"transactionId": txn.ID.String(),
					"budgetId":      budgetId.String(),
				},
			})
		}
	}

	if err := s.expoClient.Send(sendCtx, messages); err != nil {
		logger.Logger(ctx).Warn("failed to send expo push notifications", "err", err)
		return
	}
	logger.Logger(ctx).Info("sent transaction push notifications", "count", len(messages))
}
