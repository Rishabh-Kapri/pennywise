package service

import (
	"context"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PredictionService interface {
	GetAll(ctx context.Context) ([]model.Prediction, error)
	Create(ctx context.Context, prediction model.Prediction) ([]model.Prediction, error)
	Update(ctx context.Context, id uuid.UUID, prediction model.Prediction) error
	DeleteById(ctx context.Context, id uuid.UUID) error
	CreateCipherPrediction(ctx context.Context, p model.CipherPredictionRecord) (*model.CipherPredictionRecord, error)
	CreateCipherPredictionWithTx(ctx context.Context, tx pgx.Tx, p model.CipherPredictionRecord) (*model.CipherPredictionRecord, error)
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
	return s.repo.DeleteByTxnId(ctx, nil, budgetId, id)
}

func (s *predictionService) CreateCipherPrediction(ctx context.Context, p model.CipherPredictionRecord) (*model.CipherPredictionRecord, error) {
	budgetId := utils.MustBudgetID(ctx)
	p.BudgetID = budgetId
	return s.CreateCipherPredictionWithTx(ctx, nil, p)
}

func (s *predictionService) CreateCipherPredictionWithTx(ctx context.Context, tx pgx.Tx, p model.CipherPredictionRecord) (*model.CipherPredictionRecord, error) {
	budgetID := utils.MustBudgetID(ctx)
	p.BudgetID = budgetID
	return s.cipherRepo.Create(ctx, tx, p)
}
