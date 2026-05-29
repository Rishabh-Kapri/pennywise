package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"jaytaylor.com/html2text"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/client"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/model"
	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/google/uuid"
)

const (
	SimilarityThreshold  = 0.80 // harder threshold for pgvector
	ExactAmountThreshold = 0.70 // lower threshold for pgvector when exact amount is known
	MLPConfThreshold     = 0.70
	EmbeddingModel       = "bge-m3"
	SourcePrediction     = "prediction"
	SourceUserCorrected  = "user_corrected"
)

// Re-export shared prediction source constants for convenience within this package.
const (
	SourcePayeeRule = sharedModel.PredictionSourceRule
	SourcePgvector  = sharedModel.PredictionSourceVector
	SourceMLP       = sharedModel.PredictionSource("MLP") // MLP is not in the DB enum but used internally
	SourceLLM       = sharedModel.PredictionSourceLLM
	SourceFallback  = sharedModel.PredictionSource("FALLBACK") // FALLBACK is used internally
)

type ExtractEmailDataRequest struct {
	EmailHtml string `json:"emailHtml"`
}

type PredictRequest struct {
	EmailText string  `json:"emailText"`
	Amount    float64 `json:"amount"`
}

type TransactionEmbeddingRequest struct {
	RawBankText string  `json:"rawBankText"`
	Amount      float64 `json:"amount"`
}

type TransactionEmbeddingResponse struct {
	MatchString   string `json:"matchString"`
	EmbeddingText string `json:"embeddingText"`
	Embedding     string `json:"embedding"`
}

type LLMRequest struct {
	Text   string  `json:"text"`
	Amount float64 `json:"amount"`
}

type PredictResponse struct {
	Account    string                       `json:"account"`
	AccountID  uuid.UUID                    `json:"accountId"`
	PayeeID    uuid.UUID                    `json:"payeeId"`
	CategoryID uuid.UUID                    `json:"categoryId"`
	Payee      string                       `json:"payee,omitempty"`
	Category   string                       `json:"category,omitempty"`
	Amount     float64                      `json:"amount"`
	Confidence string                       `json:"confidence"`
	Source     sharedModel.PredictionSource `json:"source"` // pgvector | mlp | fallback
	Reasoning  string                       `json:"reasoning,omitempty"`
	Metadata   map[string]any               `json:"metadata,omitempty"`
}

type CorrectionRequest struct {
	EmailText     string     `json:"emailText"`
	Amount        float64    `json:"amount"`
	TransactionID *uuid.UUID `json:"transactionId,omitempty"`
	Payee         string     `json:"payee"`
	Category      string     `json:"category"`
	Account       string     `json:"account"`
}

type PredictionService interface {
	ExtractEmailData(ctx context.Context, req ExtractEmailDataRequest) (*sharedModel.ExtractedEmailResponse, error)
	Predict(ctx context.Context, req PredictRequest) (*PredictResponse, error)
	GenerateTransactionEmbedding(
		ctx context.Context,
		req TransactionEmbeddingRequest,
	) (*TransactionEmbeddingResponse, error)
	HandleCorrection(ctx context.Context, req CorrectionRequest) error
}

type predictionService struct {
	ollama        *client.OllamaClient
	mlp           *client.MLPClient
	embeddingRepo repository.TransactionEmbeddingRepository
	accountRepo   repository.AccountRepository
	payeeRepo     repository.PayeesRepository
	payeeRuleRepo repository.PayeeRuleRepository
	categoryRepo  repository.CategoryRepository
	tracer        oteltrace.Tracer
}

func NewPredictionService(
	ollama *client.OllamaClient,
	mlp *client.MLPClient,
	embeddingRepo repository.TransactionEmbeddingRepository,
	accountRepo repository.AccountRepository,
	payeeRepo repository.PayeesRepository,
	payeeRuleRepo repository.PayeeRuleRepository,
	categoryRepo repository.CategoryRepository,
	tracer oteltrace.Tracer,
) PredictionService {
	return &predictionService{
		ollama:        ollama,
		mlp:           mlp,
		embeddingRepo: embeddingRepo,
		accountRepo:   accountRepo,
		payeeRepo:     payeeRepo,
		payeeRuleRepo: payeeRuleRepo,
		categoryRepo:  categoryRepo,
		tracer:        tracer,
	}
}

func (s *predictionService) getPayeeAndCategory(
	ctx context.Context,
	budgetId uuid.UUID,
	payeeId uuid.UUID,
	categoryId uuid.UUID,
) (payee *sharedModel.Payee, category *sharedModel.Category, err error) {
	foundPayee, err := s.payeeRepo.GetById(ctx, budgetId, payeeId)
	if err != nil {
		return nil, nil, err
	}
	if foundPayee == nil {
		return nil, nil, errs.New(errs.CodePayeeLookupFailed, "payee not found")
	}

	foundCategory, err := s.categoryRepo.GetById(ctx, budgetId, categoryId)
	if err != nil {
		return nil, nil, err
	}
	if foundCategory == nil {
		return nil, nil, errs.New(errs.CodeCategoryLookupFailed, "category not found")
	}
	return foundPayee, foundCategory, nil
}

func (s *predictionService) handlePayeeRules(
	ctx context.Context,
	budgetId uuid.UUID,
	matchString string,
) (*PredictResponse, error) {
	var result PredictResponse
	foundPayeeRule, err := s.payeeRuleRepo.FindByMatchString(ctx, budgetId, matchString)
	if err != nil {
		return nil, err
	}
	if foundPayeeRule == nil {
		return nil, nil
	}
	if foundPayeeRule.CategoryID == nil {
		return nil, nil
	}
	result.PayeeID = foundPayeeRule.PayeeID
	result.CategoryID = *foundPayeeRule.CategoryID

	payee, category, err := s.getPayeeAndCategory(ctx, budgetId, result.PayeeID, result.CategoryID)
	if err != nil {
		return nil, err
	}
	result.Payee = payee.Name
	result.Category = category.Name
	result.Source = SourcePayeeRule
	result.Confidence = "100"
	result.Metadata = map[string]any{
		"strategy":     "payee_rule",
		"match_string": matchString,
	}

	return &result, nil
}

func (s *predictionService) handleSemanticSearch(
	ctx context.Context,
	budgetId uuid.UUID,
	embeddingText string,
	req PredictRequest,
) (*PredictResponse, error) {
	log := logger.Logger(ctx)

	embedding, err := s.ollama.Embed(ctx, EmbeddingModel, embeddingText)
	if err != nil {
		log.Warn("ollama embed failed, falling back to MLP", "error", err)
		// return s.mlpFallback(ctx, req, log)
		return nil, nil
	}

	embeddingStr := db.VectorToString(embedding)

	// Step 2: pgvector similarity search
	matches, err := s.embeddingRepo.SearchSimilar(ctx, budgetId, req.Amount, embeddingStr, 3)
	log.Info("pgvector search", "matches", matches)
	if err != nil {
		log.Warn("pgvector search failed", "error", err)
		return nil, nil
	}

	if result := s.resolveMatches(matches); result != nil {
		log.Info("pgvector match found", "payee", result.PayeeID, "similarity", result.Confidence)
		payee, category, err := s.getPayeeAndCategory(ctx, budgetId, result.PayeeID, result.CategoryID)
		if err != nil {
			return nil, err
		}
		result.Payee = payee.Name
		result.Category = category.Name
		return result, nil
	}
	return nil, nil
}

func (s *predictionService) handleLLM(
	ctx context.Context,
	budgetId uuid.UUID,
	embeddingText string,
	req PredictRequest,
) (*PredictResponse, error) {
	llmReq := LLMRequest{
		Text:   embeddingText,
		Amount: req.Amount,
	}
	parsed, categoryID, metadata, err := s.llmFallback(ctx, budgetId, llmReq)
	if err != nil {
		return nil, err
	}

	var result PredictResponse

	foundPayee, err := s.payeeRepo.Search(ctx, budgetId, parsed.MerchantName)
	if err != nil {
		return nil, err
	}
	if len(foundPayee) > 0 {
		result.PayeeID = foundPayee[0].ID
	}

	result.CategoryID = categoryID
	result.Source = SourceLLM
	result.Payee = parsed.MerchantName
	result.Category = parsed.SuggestedTag
	result.Reasoning = parsed.Reasoning
	result.Confidence = fmt.Sprintf("%d", parsed.Confidence)
	result.Metadata = metadata

	return &result, nil
}

func (s *predictionService) ExtractEmailData(
	ctx context.Context,
	req ExtractEmailDataRequest,
) (*sharedModel.ExtractedEmailResponse, error) {
	log := logger.Logger(ctx)
	log.Info("ExtractEmailData started")

	ctx, span := s.tracer.Start(ctx, "extractEmailData")
	defer span.End()

	span.SetName("ExtractEmailData")
	span.SetAttributes(
		attribute.String("emailText", req.EmailHtml),
	)

	text, err := html2text.FromString(
		req.EmailHtml,
		html2text.Options{PrettyTables: false, OmitLinks: true, TextOnly: true},
	)
	if err != nil {
		return nil, err
	}
	text = strings.ReplaceAll(text, "\n", "")
	text = strings.TrimSpace(text)

	extracted, err := s.ollama.ExtractEmailData(ctx, text)
	if err != nil {
		return nil, err
	}
	extracted.EmailText = text

	log.Info("email extraction", "extracted", extracted)
	return extracted, nil
}

func (s *predictionService) Predict(ctx context.Context, req PredictRequest) (*PredictResponse, error) {
	log := logger.Logger(ctx)
	log.Info("Predict", "request received", req)
	budgetId := utils.MustBudgetID(ctx)

	ctx, span := s.tracer.Start(ctx, "predict")
	defer span.End()

	span.SetName("Predict")
	span.SetAttributes(
		attribute.String("emailText", req.EmailText),
		attribute.Float64("amount", req.Amount),
	)

	transactionType := "debit"
	if req.Amount > 0 {
		transactionType = "credit"
	}
	// Step 1: Extract email data using gemma4
	extracted, err := s.ollama.ExtractEmailData(ctx, req.EmailText)
	if extracted == nil {
		return nil, errs.New(errs.CodeInternalError, "email extraction failed")
	}
	log.Info("email extraction", "extracted", extracted)

	accountStr := utils.CleanAccountString(extracted.AccountCard)
	account, err := s.accountRepo.GetBySuffix(ctx, budgetId, accountStr)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, errs.New(errs.CodeInternalError, "account not found")
	}

	if err != nil {
		logger.Logger(ctx).Warn("email extraction failed", "error", err)
		return nil, err
	}

	var predictResponse *PredictResponse = &PredictResponse{}

	upiText, merchantName := utils.CleanUPIText(extracted.Merchant)
	merchantName = utils.CleanMerchantString(extracted.Merchant)
	var matchString string
	if upiText != "" {
		matchString = upiText
	} else {
		matchString = merchantName
	}
	embeddingText := transactionType + " " + merchantName
	log.Info("cleaned email text", "text", embeddingText)

	// Step 2: Search for payee specific rules
	predictResponse, err = s.handlePayeeRules(ctx, budgetId, matchString)
	if err != nil {
		log.Warn("payee rule search failed, falling back to semantic search", "error", err)
	}
	if predictResponse == nil {
		log.Info("payee rule search failed, falling back to semantic search")
	} else {
		log.Info("payee rule match found", "payee", predictResponse.PayeeID, "category", predictResponse.CategoryID)
		predictResponse.Account = account.Name
		predictResponse.AccountID = account.ID

		return predictResponse, nil
	}

	// Step 3: Semantic search in transaction embeddings
	predictResponse, err = s.handleSemanticSearch(ctx, budgetId, embeddingText, req)
	if err != nil {
		log.Warn("semantic search failed, falling back to LLM", "error", err)
	}
	if predictResponse == nil {
		log.Info("semantic search failed, falling back to LLM")
	} else {
		log.Info(
			"semantic search found",
			"payee",
			predictResponse.PayeeID,
			"category",
			predictResponse.CategoryID,
		)
		predictResponse.Account = account.Name
		predictResponse.AccountID = account.ID

		return predictResponse, nil
	}

	// Step 4: LLM fallback
	predictResponse, err = s.handleLLM(ctx, budgetId, embeddingText, req)
	if err != nil {
		log.Warn("LLM prediction failed, falling back to manual", "error", err)
	}
	if predictResponse != nil {
		log.Info("LLM prediction found", "payee", predictResponse.PayeeID, "category", predictResponse.CategoryID)
		predictResponse.Account = account.Name
		predictResponse.AccountID = account.ID

		return predictResponse, nil
	}

	return nil, nil
}

func (s *predictionService) GenerateTransactionEmbedding(
	ctx context.Context,
	req TransactionEmbeddingRequest,
) (*TransactionEmbeddingResponse, error) {
	utils.MustBudgetID(ctx)

	if strings.TrimSpace(req.RawBankText) == "" {
		return nil, errs.New(errs.CodeInvalidArgument, "rawBankText is required")
	}

	transactionType := "debit"
	if req.Amount > 0 {
		transactionType = "credit"
	}

	extracted, err := s.ollama.ExtractEmailData(ctx, req.RawBankText)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "extract transaction embedding text", err)
	}
	if extracted == nil {
		return nil, errs.New(errs.CodeInternalError, "transaction embedding text extraction failed")
	}

	upiText, merchantName := utils.CleanUPIText(extracted.Merchant)
	merchantName = utils.CleanMerchantString(merchantName)
	if merchantName == "" {
		return nil, errs.New(errs.CodeInvalidArgument, "merchant name could not be extracted")
	}

	var matchString string
	if upiText != "" {
		matchString = upiText
	} else {
		matchString = merchantName
	}

	embeddingText := transactionType + " " + merchantName
	embedding, err := s.ollama.Embed(ctx, EmbeddingModel, embeddingText)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "generate transaction embedding", err)
	}

	return &TransactionEmbeddingResponse{
		MatchString:   matchString,
		EmbeddingText: embeddingText,
		Embedding:     db.VectorToString(embedding),
	}, nil
}

func (s *predictionService) HandleCorrection(ctx context.Context, req CorrectionRequest) error {
	budgetID := utils.MustBudgetID(ctx)
	logger := logger.Logger(ctx)

	// Generate embedding for the corrected transaction
	embedding, err := s.ollama.Embed(ctx, EmbeddingModel, req.EmailText)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, "embed correction", err)
	}

	embeddingStr := db.VectorToString(embedding)

	data := sharedModel.TransactionEmbedding{
		BudgetID:      budgetID,
		EmbeddingText: req.EmailText,
		// PayeeID:       req.PayeeID,
		// CategoryID:    req.CategoryID,
		Amount: req.Amount,
		Source: SourceUserCorrected,
	}

	if err := s.embeddingRepo.Upsert(ctx, nil, data, embeddingStr); err != nil {
		return fmt.Errorf("upsert correction embedding: %w", err)
	}

	logger.Info("correction embedding stored",
		"payee", req.Payee,
		"category", req.Category,
		"transactionId", req.TransactionID,
	)

	return nil
}

func (s *predictionService) resolveMatches(matches []sharedModel.TransactionEmbedding) *PredictResponse {
	if len(matches) == 0 {
		return nil
	}
	// log := logger.Logger(context.Background())
	// log.Info("resolveMatches", "match 0", *matches[0].VectorDistance, "penalty", *matches[0].AmountPenalty, "similarity", 1-*matches[0].VectorDistance)
	// log.Info("resolveMatches", "match 1", *matches[1].VectorDistance, "penalty", *matches[1].AmountPenalty, "similarity", 1-*matches[1].VectorDistance)
	// log.Info("resolveMatches", "match 2", *matches[2].VectorDistance, "penalty", *matches[2].AmountPenalty, "similarity", 1-*matches[2].VectorDistance)

	best := matches[0]
	if best.VectorDistance == nil || best.AmountPenalty == nil {
		return nil
	}
	amountPenalty := *best.AmountPenalty
	similarity := 1 - *best.VectorDistance
	res := &PredictResponse{
		PayeeID:    best.PayeeID,
		CategoryID: best.CategoryID,
		Amount:     best.Amount,
		Confidence: fmt.Sprintf("%.2f", (1-(*best.VectorDistance+amountPenalty*0.15))*100),
		Source:     SourcePgvector,
		Metadata: map[string]any{
			"strategy":        "semantic_search",
			"embedding_model": EmbeddingModel,
			"vector_distance": best.VectorDistance,
			"amount_penalty":  best.AmountPenalty,
		},
	}

	if similarity >= SimilarityThreshold {
		return res
	}

	if amountPenalty == 0.0 && similarity >= ExactAmountThreshold {
		return res
	}

	return nil
}

// func (s *predictionService) mlpFallback(ctx context.Context, req PredictRequest) (*PredictResponse, error) {
// 	log := logger.Logger(ctx)
//
// 	accountResult, payeeResult, categoryResult, err := s.mlp.PredictAll(ctx, req.EmailText, req.Amount)
// 	if err != nil {
// 		log.Error("MLP predict failed", "error", err)
// 		return nil, err
// 	}
//
// 	// Use confidence gating like go-gmail does
// 	result := s.defaultFallback(req)
//
// 	// if accountResult.Confidence >= MLPConfThreshold {
// 	// 	result.Account = accountResult.Label
// 	// }
// 	// if payeeResult != nil && payeeResult.Confidence >= MLPConfThreshold {
// 	// 	result.PayeeID = payeeResult.Label
// 	// }
// 	// if categoryResult != nil && categoryResult.Confidence >= MLPConfThreshold {
// 	// 	result.Category = categoryResult.Label
// 	// }
//
// 	// Only mark as MLP source if at least account passed threshold
// 	if accountResult.Confidence >= MLPConfThreshold {
// 		result.Source = SourceMLP
// 		result.Confidence = accountResult.Confidence
// 	}
//
// 	log.Info("MLP prediction", "payee", result.Payee, "category", result.Category, "account", result.Account, "source", result.Source)
//
// 	return result, nil
// }

// llmFallback calls the LLM with the active prompt (promptV2), parses the JSON response,
// resolves the suggested category against the budget's category list, and returns the
// parsed prediction together with the resolved category UUID.
// Category resolution is done here so callers don't need to repeat the DB lookup.
func (s *predictionService) llmFallback(
	ctx context.Context,
	budgetId uuid.UUID,
	req LLMRequest,
) (*model.LLMPrediction, uuid.UUID, map[string]any, error) {
	llmModel := "openai/gpt-5.4"

	userCategories, err := s.categoryRepo.GetAllSimplified(ctx, budgetId)
	if err != nil {
		return nil, uuid.Nil, nil, err
	}

	userCategoriesMap := make(map[string]uuid.UUID, len(userCategories))
	userCategoriesText := ""
	for _, c := range userCategories {
		userCategoriesText += c.Name + ", "
		userCategoriesMap[c.Name] = c.ID
	}

	prompt := strings.ReplaceAll(promptV2, "{categories}", userCategoriesText)

	resp, err := s.ollama.Generate(ctx, llmModel, prompt, req.Text, req.Amount)
	if err != nil {
		return nil, uuid.Nil, nil, errs.Wrap(errs.CodeInternalError, "error in llm fallback", err)
	}
	if resp == "" {
		return nil, uuid.Nil, nil, errs.New(errs.CodeInternalError, "LLM fallback: no result returned")
	}

	parsed, err := utils.UnmarshalResponse[model.LLMPrediction]([]byte(resp))
	if err != nil {
		return nil, uuid.Nil, nil, err
	}

	categoryID, ok := userCategoriesMap[parsed.SuggestedTag]
	if !ok {
		// Category should always be found by the LLM from existing categories
		return nil, uuid.Nil, nil, errs.New(errs.CodeCategoryLookupFailed, "category not found")
	}

	metadata := map[string]any{
		"strategy":          "llm_fallback",
		"model":             llmModel,
		"prompt":            prompt,
		"input_text":        req.Text,
		"input_amount":      req.Amount,
		"response":          resp,
		"categories_count":  len(userCategories),
		"prompt_template":   "promptV2",
		"response_category": parsed.SuggestedTag,
	}

	return &parsed, categoryID, metadata, nil
}

func (s *predictionService) storeEmbedding(
	ctx context.Context,
	budgetID uuid.UUID,
	req PredictRequest,
	embeddingText string,
	result *PredictResponse,
	source string,
	embeddingStr string,
) {
	data := sharedModel.TransactionEmbedding{
		BudgetID:      budgetID,
		EmbeddingText: embeddingText,
		PayeeID:       result.PayeeID,
		CategoryID:    result.CategoryID,
		Amount:        req.Amount,
		Source:        source,
	}

	if err := s.embeddingRepo.Upsert(ctx, nil, data, embeddingStr); err != nil {
		logger.Logger(ctx).Error("failed to store prediction embedding", "error", err)
	}
}
