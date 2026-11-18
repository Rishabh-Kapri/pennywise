package service

import (
	"context"
	"errors"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

// ErrInvalidUsageWindow is a days or tz the usage summary can't use.
var ErrInvalidUsageWindow = errors.New("invalid usage window")

const maxUsageDays = 366

// AIUsageService sums up what the budget's model calls used and cost.
type AIUsageService interface {
	// Summary covers the last days days (today included), counted in the
	// IANA zone tz. The budget comes from ctx.
	Summary(ctx context.Context, days int, tz string) (model.LLMUsageSummary, error)
}

type aiUsageService struct {
	repo repository.LLMUsageRepository
	now  func() time.Time
}

func NewAIUsageService(repo repository.LLMUsageRepository) AIUsageService {
	return &aiUsageService{repo: repo, now: time.Now}
}

func (s *aiUsageService) Summary(ctx context.Context, days int, tz string) (model.LLMUsageSummary, error) {
	if days < 1 || days > maxUsageDays {
		return model.LLMUsageSummary{}, ErrInvalidUsageWindow
	}
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return model.LLMUsageSummary{}, ErrInvalidUsageWindow
	}
	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		return model.LLMUsageSummary{}, err
	}
	now := s.now().In(loc)
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	since := startOfToday.AddDate(0, 0, -(days - 1))
	return s.repo.Summary(ctx, budgetID, since, loc.String())
}
