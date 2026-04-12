package service

import (
	"context"
	"errors"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/repository"
	utils "github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/pkg"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type MonthlyBudgetService interface {
	UpsertCarryover(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, categoryId uuid.UUID, monthKey string, delta float64) error
	ApplyCarryoverOps(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, txnDiff *txnDiff, carryoverCase carryoverCase) error
	UpdateCarryovers(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, oldTxn *model.Transaction, newTxn model.Transaction) error
}

type monthlyBudgetService struct {
	repo repository.MonthlyBudgetRepository
}

func NewMonthlyBudgetService(r repository.MonthlyBudgetRepository) MonthlyBudgetService {
	return &monthlyBudgetService{repo: r}
}

type txnDiff struct {
	oldCatId    *uuid.UUID
	newCatId    *uuid.UUID
	oldMonthKey string
	newMonthKey string
	oldAmount   float64
	newAmount   float64
}

// carryoverCase is a helper struct for carryover logic
type carryoverCase struct {
	sameCategory bool
	sameMonth    bool
}

type carryoverOp struct {
	categoryId  uuid.UUID
	monthKey    string
	amountDelta float64
}

// returns a list of carryover operations to be performed
func (s *monthlyBudgetService) getCarryoverOps(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, txnDiff *txnDiff, carryoverCase carryoverCase) []carryoverOp {
	var carryoverOps []carryoverOp
	diff := txnDiff.newAmount - txnDiff.oldAmount

	switch {
	// same category and month
	case carryoverCase.sameCategory && carryoverCase.sameMonth:
		{
			// check if amount has changed
			if diff == 0 {
				return carryoverOps
			}
			// amount has changed
			carryoverOps = append(carryoverOps, carryoverOp{
				categoryId:  *txnDiff.newCatId,   // categoryId is the same as oldCatId
				monthKey:    txnDiff.newMonthKey, // monthKey is the same as oldMonthKey
				amountDelta: diff,
			})
			return carryoverOps
		}
	// same category and different month
	// case carryoverCase.sameCategory && !carryoverCase.sameMonth:
	// case !carryoverCase.sameCategory && carryoverCase.sameMonth:
	// case !carryoverCase.sameCategory && !carryoverCase.sameMonth:
	default:
		// handles the above cases
		{
			if txnDiff.oldCatId != nil {
				carryoverOps = append(carryoverOps, carryoverOp{
					categoryId:  *txnDiff.oldCatId,
					monthKey:    txnDiff.oldMonthKey,
					amountDelta: -txnDiff.oldAmount,
				})
			}
			if txnDiff.newCatId != nil {
				carryoverOps = append(carryoverOps, carryoverOp{
					categoryId:  *txnDiff.newCatId,
					monthKey:    txnDiff.newMonthKey,
					amountDelta: txnDiff.newAmount,
				})
			}
			return carryoverOps
		}
	}
}

// upsertCarryover adjusts the carryover balance for a category/month, creating the monthly budget if it doesn't exist.
func (s *monthlyBudgetService) UpsertCarryover(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, categoryId uuid.UUID, monthKey string, delta float64) error {
	_, err := s.repo.GetByCatIdAndMonth(ctx, tx, budgetId, categoryId, monthKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			monthlyBudget := model.MonthlyBudget{
				Month:            monthKey,
				BudgetID:         budgetId,
				Budgeted:         0,
				CarryoverBalance: delta,
				CategoryID:       categoryId,
			}
			if err = s.repo.Create(ctx, tx, budgetId, monthlyBudget); err != nil {
				return errs.Wrap(errs.CodeMonthlyBudgetCreateFailed, "error while creating monthly budget", err)
			}
			return nil
		}
		return errs.Wrap(errs.CodeMonthlyBudgetLookupFailed, "error while fetching monthly budget", err)
	}
	return s.repo.UpdateCarryoverByCatIdAndMonth(ctx, tx, budgetId, categoryId, monthKey, delta)
}

func (s *monthlyBudgetService) ApplyCarryoverOps(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, txnDiff *txnDiff, carryoverCase carryoverCase) error {
	carryoverOps := s.getCarryoverOps(ctx, tx, budgetId, txnDiff, carryoverCase)
	for _, op := range carryoverOps {
		if err := s.UpsertCarryover(ctx, tx, budgetId, op.categoryId, op.monthKey, op.amountDelta); err != nil {
			return err
		}
	}
	return nil
}

// updateCarryovers computes and applies carryover adjustments when a transaction changes
func (s *monthlyBudgetService) UpdateCarryovers(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, oldTxn *model.Transaction, newTxn model.Transaction) error {
	diff := &txnDiff{
		oldCatId:    oldTxn.CategoryID,
		newCatId:    newTxn.CategoryID,
		oldMonthKey: utils.GetMonthKey(oldTxn.Date.String()),
		newMonthKey: utils.GetMonthKey(newTxn.Date.String()),
		oldAmount:   oldTxn.Amount,
		newAmount:   newTxn.Amount,
	}
	cc := carryoverCase{
		sameCategory: oldTxn.CategoryID != nil && newTxn.CategoryID != nil && *oldTxn.CategoryID == *newTxn.CategoryID,
		sameMonth:    utils.GetMonthKey(oldTxn.Date.String()) == utils.GetMonthKey(newTxn.Date.String()),
	}
	return s.ApplyCarryoverOps(ctx, tx, budgetId, diff, cc)
}
