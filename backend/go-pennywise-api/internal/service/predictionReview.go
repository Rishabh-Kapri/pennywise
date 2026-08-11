package service

import (
	"context"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

type PredictionReviewService interface {
	GetQueue(ctx context.Context, includeReviewed bool, limit int) ([]model.PredictionReviewItem, error)
	Review(ctx context.Context, id uuid.UUID, req model.PredictionReviewRequest) (*model.PredictionReviewItem, error)
}

type predictionReviewService struct {
	repo               repository.PredictionReviewRepository
	transactionRepo    repository.TransactionRepository
	transactionService TransactionService
}

func NewPredictionReviewService(
	repo repository.PredictionReviewRepository,
	transactionRepo repository.TransactionRepository,
	transactionService TransactionService,
) PredictionReviewService {
	return &predictionReviewService{
		repo:               repo,
		transactionRepo:    transactionRepo,
		transactionService: transactionService,
	}
}

func (s *predictionReviewService) GetQueue(
	ctx context.Context,
	includeReviewed bool,
	limit int,
) ([]model.PredictionReviewItem, error) {
	budgetID := utils.MustBudgetID(ctx)
	return s.repo.GetQueue(ctx, budgetID, includeReviewed, limit)
}

func (s *predictionReviewService) Review(
	ctx context.Context,
	id uuid.UUID,
	req model.PredictionReviewRequest,
) (*model.PredictionReviewItem, error) {
	budgetID := utils.MustBudgetID(ctx)

	item, err := s.repo.GetByID(ctx, budgetID, id)
	if err != nil {
		return nil, err
	}

	switch req.Action {
	case model.PredictionReviewAccept:
		// the transaction already carries what cipher predicted; record the
		// confirmed labels so accepted rows are usable training signal too
		if err := s.repo.MarkReviewed(ctx, budgetID, id, item.CurrentPayeeID, item.CurrentCategoryID); err != nil {
			return nil, err
		}

	case model.PredictionReviewCorrect:
		if req.PayeeID == nil && req.CategoryID == nil {
			return nil, errs.New(errs.CodeInvalidArgument, "payeeId or categoryId is required to correct a prediction")
		}

		txn, err := s.transactionRepo.GetById(ctx, budgetID, item.TransactionID)
		if err != nil {
			return nil, errs.Wrap(errs.CodeTransactionLookupFailed, "error getting transaction", err)
		}
		if txn == nil {
			return nil, errs.New(errs.CodeTransactionLookupFailed, "transaction not found for prediction")
		}

		if req.PayeeID != nil {
			txn.PayeeID = req.PayeeID
		}
		if req.CategoryID != nil {
			txn.CategoryID = req.CategoryID
		}

		// Update owns the correction side effects: it marks the cipher
		// prediction user-corrected and queues the payee-rule learning
		if err := s.transactionService.Update(ctx, item.TransactionID, *txn); err != nil {
			return nil, err
		}
		if err := s.repo.MarkReviewed(ctx, budgetID, id, nil, nil); err != nil {
			return nil, err
		}

	default:
		return nil, errs.New(errs.CodeInvalidArgument, "action must be either accept or correct")
	}

	return s.repo.GetByID(ctx, budgetID, id)
}
