package service

import (
	"context"
	"testing"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
)

// usageAgentRepo owns one conversation; the rest of the interface panics.
type usageAgentRepo struct {
	repository.AgentRepository
	owner, budget, conversation uuid.UUID
	totalled                    bool
}

func (r *usageAgentRepo) GetConversation(
	_ context.Context, id, userID, budgetID uuid.UUID,
) (*sharedModel.AgentConversation, error) {
	if id != r.conversation || userID != r.owner || budgetID != r.budget {
		return nil, errs.New(errs.CodeAgentConversationNotFound, "agent conversation not found")
	}
	return &sharedModel.AgentConversation{ID: id}, nil
}

func (r *usageAgentRepo) ConversationUsage(
	_ context.Context, _, _ uuid.UUID,
) (sharedModel.ConversationUsage, error) {
	r.totalled = true
	cost := int64(42)
	return sharedModel.ConversationUsage{Total: sharedModel.ModelUsage{Calls: 1, CostMicros: &cost}}, nil
}

func TestConversationUsageOnlyForTheOwner(t *testing.T) {
	repo := &usageAgentRepo{owner: uuid.New(), budget: uuid.New(), conversation: uuid.New()}
	svc := NewAgentService(nil, repo, nil)
	ctx := utils.WithBudgetID(context.Background(), repo.budget)

	got, err := svc.ConversationUsage(utils.WithUserID(ctx, repo.owner), repo.conversation)
	if err != nil || got.Total.Calls != 1 {
		t.Fatalf("owner: usage %+v err %v", got, err)
	}

	repo.totalled = false
	_, err = svc.ConversationUsage(utils.WithUserID(ctx, uuid.New()), repo.conversation)
	if !hasAgentErrorCode(err, errs.CodeAgentConversationNotFound) {
		t.Fatalf("someone else: err = %v, want conversation not found", err)
	}
	if repo.totalled {
		t.Fatal("usage was totalled for someone who can't see the conversation")
	}
}
