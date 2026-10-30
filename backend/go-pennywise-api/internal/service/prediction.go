package service

import (
	"context"
	"errors"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

type PredictionService interface {
	GetAll(ctx context.Context) ([]model.Prediction, error)
	GetAllCipherPredictions(ctx context.Context) ([]model.CipherPredictionRecord, error)
	GetByTransactionID(ctx context.Context, transactionID uuid.UUID) (*model.TransactionPredictionDetails, error)
	Create(ctx context.Context, prediction model.Prediction) ([]model.Prediction, error)
	Update(ctx context.Context, id uuid.UUID, prediction model.Prediction) error
	DeleteById(ctx context.Context, id uuid.UUID) error
	CreateCipherPrediction(ctx context.Context, p model.CipherPredictionRecord) (*model.CipherPredictionRecord, error)
}

type predictionService struct {
	repo       repository.PredictionRepository
	cipherRepo repository.CipherPredictionRepository
}

func NewPredictionService(r repository.PredictionRepository, cr repository.CipherPredictionRepository) PredictionService {
	return &predictionService{repo: r, cipherRepo: cr}
}

func (s *predictionService) GetAll(ctx context.Context) ([]model.Prediction, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.GetAll(ctx, budgetId)
}

func (s *predictionService) GetAllCipherPredictions(ctx context.Context) ([]model.CipherPredictionRecord, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.cipherRepo.GetAll(ctx, budgetId)
}

func (s *predictionService) GetByTransactionID(ctx context.Context, transactionID uuid.UUID) (*model.TransactionPredictionDetails, error) {
	budgetID := utils.MustBudgetID(ctx)
	details := &model.TransactionPredictionDetails{}

	cipherPrediction, err := s.cipherRepo.GetByTransactionID(ctx, budgetID, transactionID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if err == nil {
		details.CipherPrediction = cipherPrediction
	}

	prediction, err := s.repo.GetByTxnId(ctx, budgetID, transactionID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if err == nil {
		details.Prediction = prediction
	}

	return details, nil
}

func (s *predictionService) Create(ctx context.Context, prediction model.Prediction) ([]model.Prediction, error) {
	budgetId := utils.MustBudgetID(ctx)
	prediction.BudgetID = budgetId
	return s.repo.Create(ctx, prediction)
}

func (s *predictionService) Update(ctx context.Context, id uuid.UUID, prediction model.Prediction) error {
	// budgetId := utils.MustBudgetID(ctx)
	// INFO: prediction update for now will only be done through transactions update
	// return s.repo.Update(ctx, budgetId, id, prediction)
	return nil
}

func (s *predictionService) DeleteById(ctx context.Context, id uuid.UUID) error {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.DeleteByTxnId(ctx, budgetId, id)
}

func (s *predictionService) CreateCipherPrediction(ctx context.Context, p model.CipherPredictionRecord) (*model.CipherPredictionRecord, error) {
	budgetID := utils.MustBudgetID(ctx)
	p.BudgetID = budgetID
	return s.cipherRepo.Create(ctx, p)
}
