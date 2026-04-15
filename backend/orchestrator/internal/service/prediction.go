package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/Rishabh-Kapri/pennywise/backend/orchestrator/internal/client"
	"github.com/Rishabh-Kapri/pennywise/backend/orchestrator/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/orchestrator/internal/repository"

	"github.com/google/uuid"
)

const (
	SimilarityThreshold = 0.70
	MLPConfThreshold    = 0.70
	EmbeddingModel      = "bge-m3"

	SourcePgvector      = "pgvector"
	SourceMLP           = "mlp"
	SourceFallback      = "fallback"
	SourcePrediction    = "prediction"
	SourceUserCorrected = "user_corrected"
)

type PredictRequest struct {
	EmailText string  `json:"emailText"`
	Amount    float64 `json:"amount"`
	Account   string  `json:"account"` // fallback account from email headers
}

type PredictResponse struct {
	Payee      string  `json:"payee"`
	Category   string  `json:"category"`
	Account    string  `json:"account"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"` // pgvector | mlp | fallback
}

type CorrectionRequest struct {
	BudgetID      uuid.UUID  `json:"budgetId"`
	EmailText     string     `json:"emailText"`
	Amount        float64    `json:"amount"`
	TransactionID *uuid.UUID `json:"transactionId,omitempty"`
	Payee         string     `json:"payee"`
	Category      string     `json:"category"`
	Account       string     `json:"account"`
}

type PredictionService interface {
	Predict(ctx context.Context, budgetID uuid.UUID, req PredictRequest) (*PredictResponse, error)
	HandleCorrection(ctx context.Context, req CorrectionRequest) error
}

type predictionService struct {
	ollama        *client.OllamaClient
	mlp           *client.MLPClient
	embeddingRepo repository.TransactionEmbeddingRepository
}

func NewPredictionService(
	ollama *client.OllamaClient,
	mlp *client.MLPClient,
	embeddingRepo repository.TransactionEmbeddingRepository,
) PredictionService {
	return &predictionService{
		ollama:        ollama,
		mlp:           mlp,
		embeddingRepo: embeddingRepo,
	}
}

func (s *predictionService) Predict(ctx context.Context, budgetID uuid.UUID, req PredictRequest) (*PredictResponse, error) {
	log := slog.Default().With("budgetId", budgetID.String())

	// Step 1: Generate embedding via Ollama
	cleanedEmailText := utils.CleanEmailText(req.EmailText, "debit")
	log.Info("cleaned email text", "text", cleanedEmailText)
	embedding, err := s.ollama.Embed(ctx, EmbeddingModel, cleanedEmailText)
	if err != nil {
		log.Warn("ollama embed failed, falling back to MLP", "error", err)
		// return s.mlpFallback(ctx, req, log)
		return nil, nil
	}

	embeddingStr := db.VectorToString(embedding)

	// Step 2: pgvector similarity search
	matches, err := s.embeddingRepo.SearchSimilar(ctx, budgetID, embeddingStr, 3)
	log.Info("pgvector search", "matches", matches)
	if err != nil {
		log.Warn("pgvector search failed, falling back to MLP", "error", err)
		// return s.mlpFallback(ctx, req, log)
		return nil, nil
	}

	if result := s.resolveMatches(matches); result != nil {
		log.Info("pgvector match found", "payee", result.Payee, "similarity", result.Confidence)

		// Store prediction embedding for future lookups
		// s.storeEmbedding(ctx, budgetID, req, result, SourcePrediction, embeddingStr)

		return result, nil
	}

	// Step 3: MLP fallback
	// mlpResult, err := s.mlpFallback(ctx, req, log)
	// if err != nil {
	// 	return s.defaultFallback(req), nil
	// }
	//
	// // Store MLP result embedding if confident
	// if mlpResult.Source == SourceMLP {
	// 	s.storeEmbedding(ctx, budgetID, req, mlpResult, SourcePrediction, embeddingStr)
	// }

	// return mlpResult, nil
	return nil, nil
}

func (s *predictionService) HandleCorrection(ctx context.Context, req CorrectionRequest) error {
	log := slog.Default().With("budgetId", req.BudgetID.String())

	// Generate embedding for the corrected transaction
	embedding, err := s.ollama.Embed(ctx, EmbeddingModel, req.EmailText)
	if err != nil {
		return fmt.Errorf("embed correction: %w", err)
	}

	embeddingStr := db.VectorToString(embedding)

	data := model.TransactionEmbedding{
		BudgetID:      req.BudgetID,
		EmbeddingText: req.EmailText,
		Payee:         req.Payee,
		Category:      req.Category,
		Account:       req.Account,
		Amount:        req.Amount,
		TransactionID: req.TransactionID,
		Source:        SourceUserCorrected,
	}

	if err := s.embeddingRepo.Upsert(ctx, nil, data, embeddingStr); err != nil {
		return fmt.Errorf("upsert correction embedding: %w", err)
	}

	log.Info("correction embedding stored",
		"payee", req.Payee,
		"category", req.Category,
		"transactionId", req.TransactionID,
	)

	return nil
}

func (s *predictionService) resolveMatches(matches []model.TransactionEmbedding) *PredictResponse {
	if len(matches) == 0 {
		return nil
	}

	best := matches[0]
	if best.Similarity == nil || *best.Similarity < SimilarityThreshold {
		return nil
	}

	return &PredictResponse{
		Payee:      best.Payee,
		Category:   best.Category,
		Account:    best.Account,
		Confidence: *best.Similarity,
		Source:     SourcePgvector,
	}
}

func (s *predictionService) mlpFallback(ctx context.Context, req PredictRequest, log *slog.Logger) (*PredictResponse, error) {
	accountResult, payeeResult, categoryResult, err := s.mlp.PredictAll(ctx, req.EmailText, req.Amount)
	if err != nil {
		log.Error("MLP predict failed", "error", err)
		return nil, err
	}

	// Use confidence gating like go-gmail does
	result := s.defaultFallback(req)

	if accountResult.Confidence >= MLPConfThreshold {
		result.Account = accountResult.Label
	}
	if payeeResult != nil && payeeResult.Confidence >= MLPConfThreshold {
		result.Payee = payeeResult.Label
	}
	if categoryResult != nil && categoryResult.Confidence >= MLPConfThreshold {
		result.Category = categoryResult.Label
	}

	// Only mark as MLP source if at least account passed threshold
	if accountResult.Confidence >= MLPConfThreshold {
		result.Source = SourceMLP
		result.Confidence = accountResult.Confidence
	}

	log.Info("MLP prediction", "payee", result.Payee, "category", result.Category, "account", result.Account, "source", result.Source)

	return result, nil
}

func (s *predictionService) defaultFallback(req PredictRequest) *PredictResponse {
	account := req.Account
	if account == "" {
		account = "Unknown"
	}
	return &PredictResponse{
		Payee:      "Unexpected",
		Category:   "❗ Unexpected expenses",
		Account:    account,
		Confidence: 0,
		Source:     SourceFallback,
	}
}

func (s *predictionService) storeEmbedding(ctx context.Context, budgetID uuid.UUID, req PredictRequest, result *PredictResponse, source string, embeddingStr string) {
	data := model.TransactionEmbedding{
		BudgetID:      budgetID,
		EmbeddingText: req.EmailText,
		Payee:         result.Payee,
		Category:      result.Category,
		Account:       result.Account,
		Amount:        req.Amount,
		Source:        source,
	}

	if err := s.embeddingRepo.Upsert(ctx, nil, data, embeddingStr); err != nil {
		slog.Error("failed to store prediction embedding", "error", err)
	}
}
