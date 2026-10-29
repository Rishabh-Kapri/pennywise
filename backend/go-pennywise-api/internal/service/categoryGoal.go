package service

import (
	"context"
	"errors"
	"time"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CategoryGoalService interface {
	// Upsert validates and stores the category's target, returning it as
	// stored (fields its type does not use are cleared).
	Upsert(ctx context.Context, categoryId uuid.UUID, goal model.CategoryGoal) (*model.CategoryGoal, error)
	Delete(ctx context.Context, categoryId uuid.UUID) error
}

// ErrCategoryGoalNotFound is returned when removing a target the category does
// not have.
var ErrCategoryGoalNotFound = errors.New("category has no target")

type categoryGoalService struct {
	repo repository.CategoryGoalRepository
	now  func() time.Time
}

func NewCategoryGoalService(repo repository.CategoryGoalRepository) CategoryGoalService {
	return &categoryGoalService{repo: repo, now: time.Now}
}

func (s *categoryGoalService) Upsert(
	ctx context.Context,
	categoryId uuid.UUID,
	goal model.CategoryGoal,
) (*model.CategoryGoal, error) {
	if err := goal.Normalize(s.now()); err != nil {
		return nil, err
	}
	err := s.repo.Upsert(ctx, utils.MustBudgetID(ctx), categoryId, goal)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCategoryNotFound
	}
	if err != nil {
		return nil, err
	}
	return &goal, nil
}

func (s *categoryGoalService) Delete(ctx context.Context, categoryId uuid.UUID) error {
	err := s.repo.Delete(ctx, utils.MustBudgetID(ctx), categoryId)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCategoryGoalNotFound
	}
	return err
}
