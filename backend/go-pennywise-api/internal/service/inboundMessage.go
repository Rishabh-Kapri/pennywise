package service

import (
	"context"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

const (
	defaultMessageListLimit = 50
	maxMessageListLimit     = 200
)

// MessageList is GET /api/messages: the budget's messages in one status, and
// how many it has in each.
type MessageList struct {
	Status model.InboundMessageStatus         `json:"status"`
	Items  []model.InboundMessage             `json:"items"`
	Counts map[model.InboundMessageStatus]int `json:"counts"`
}

// InboundMessageService answers where the budget's bank messages stand. The
// budget comes from ctx.
type InboundMessageService interface {
	// List returns up to limit messages in status, newest change first.
	// CodeInvalidArgument for an unknown status.
	List(ctx context.Context, status model.InboundMessageStatus, limit int) (MessageList, error)
}

type inboundMessageService struct {
	messages repository.InboundMessageRepository
}

func NewInboundMessageService(messages repository.InboundMessageRepository) InboundMessageService {
	return &inboundMessageService{messages: messages}
}

func (s *inboundMessageService) List(
	ctx context.Context,
	status model.InboundMessageStatus,
	limit int,
) (MessageList, error) {
	if !status.Valid() {
		return MessageList{}, errs.New(errs.CodeInvalidArgument, "unknown message status %q", status)
	}
	if limit <= 0 {
		limit = defaultMessageListLimit
	}
	limit = min(limit, maxMessageListLimit)
	budgetID := utils.MustBudgetID(ctx)

	items, err := s.messages.List(ctx, budgetID, status, limit)
	if err != nil {
		return MessageList{}, errs.Wrap(errs.CodeInternalError, "list messages", err)
	}
	counts, err := s.messages.CountByStatus(ctx, budgetID)
	if err != nil {
		return MessageList{}, errs.Wrap(errs.CodeInternalError, "count messages", err)
	}
	return MessageList{Status: status, Items: items, Counts: counts}, nil
}
