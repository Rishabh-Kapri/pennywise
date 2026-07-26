package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"jaytaylor.com/html2text"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	agent "github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/runtime"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/client"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/progress"
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

type ExtractedInputs struct {
	Merchant string `json:"merchant"`
	Account  string `json:"account"`
	Date     string `json:"date"`
}

type PredictRequest struct {
	EmailText       string           `json:"emailText"`
	Amount          float64          `json:"amount"`
	ExtractedInputs *ExtractedInputs `json:"extractedInputs"`
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
	Summary    string                       `json:"summary"`
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
	// Normalizes the email text using LLM to remove any formatting or other irrelevant information
	SummarizeEmailText(ctx context.Context, text string) (string, error)
	ExtractEmailData(ctx context.Context, req ExtractEmailDataRequest) (*sharedModel.ExtractedEmailResponse, error)
	Predict(ctx context.Context, req PredictRequest) (*PredictResponse, error)
	GenerateTransactionEmbedding(
		ctx context.Context,
		req TransactionEmbeddingRequest,
	) (*TransactionEmbeddingResponse, error)
	HandleCorrection(ctx context.Context, req CorrectionRequest) error
}

type predictionService struct {
	agent         *agent.Agent
	llmResolver   llm.LLMResolver
	ollama        *client.OllamaClient
	mlp           *client.MLPClient
	embeddingRepo repository.TransactionEmbeddingRepository
	accountRepo   repository.AccountRepository
	payeeRepo     repository.PayeesRepository
	payeeRuleRepo repository.PayeeRuleRepository
	categoryRepo  repository.CategoryRepository
	tracer        oteltrace.Tracer
	// llmCallTimeout bounds each chat-endpoint LLM call made through the
	// llmResolver (the OllamaClient bounds its own calls).
	llmCallTimeout time.Duration
	// pipelineTargets is the ordered provider chain for the email pipeline's
	// LLM steps (extract, summarize, llm fallback).
	pipelineTargets []config.LLMTarget
}

func NewPredictionService(
	agent *agent.Agent,
	llmResolver llm.LLMResolver,
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
		agent:           agent,
		llmResolver:     llmResolver,
		ollama:          ollama,
		mlp:             mlp,
		embeddingRepo:   embeddingRepo,
		accountRepo:     accountRepo,
		payeeRepo:       payeeRepo,
		payeeRuleRepo:   payeeRuleRepo,
		categoryRepo:    categoryRepo,
		tracer:          tracer,
		llmCallTimeout:  config.Load().LLMCallTimeout,
		pipelineTargets: config.Load().EmailPipelineTargets,
	}
}

// chatWithFallback runs the request against each configured pipeline provider in
// order, returning the first successful response. Any failure — the provider
// isn't configured, the host is unreachable, the call times out — moves on to
// the next target, so a local ollama outage no longer stalls the whole pipeline.
// buildReq receives the resolved model name for the target being tried.
//
// Falling through on *every* error (rather than only transport errors) is
// deliberate: the alternative is failing the activity, and one extra call to the
// next provider is cheaper than a parked workflow. When every target fails the
// last error is returned so Temporal still sees a retryable failure.
func (s *predictionService) chatWithFallback(
	ctx context.Context,
	step string,
	buildReq func(model string) sharedModel.ChatRequest,
) (*sharedModel.ChatResponse, error) {
	log := logger.Logger(ctx)

	targets := s.pipelineTargets
	if len(targets) == 0 {
		return nil, errs.New(errs.CodeLLMNotConfigured, "no email pipeline providers configured")
	}

	var lastErr error
	for _, target := range targets {
		lc, model, err := s.llmResolver.Resolve(target.Provider, target.Model)
		if err != nil {
			// Provider has no API key / isn't registered — skip to the next.
			log.Warn("pipeline provider unavailable, trying next",
				"step", step, "provider", target.Provider, "error", err)
			lastErr = err
			continue
		}

		req := buildReq(model)
		req.Provider = target.Provider
		req.Model = model

		progress.Report(ctx, step)
		chatCtx, chatCancel := s.withLLMTimeout(ctx)
		res, err := lc.Chat(chatCtx, req)
		chatCancel()
		if err != nil {
			log.Warn("pipeline provider call failed, trying next",
				"step", step, "provider", target.Provider, "model", model, "error", err)
			lastErr = err
			continue
		}

		log.Info("pipeline llm call succeeded",
			"step", step, "provider", target.Provider, "model", model)
		return res, nil
	}

	return nil, errs.Wrap(errs.CodeInternalError,
		"all email pipeline providers failed for step "+step, lastErr)
}

// withLLMTimeout bounds a single chat-endpoint LLM call.
func (s *predictionService) withLLMTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := s.llmCallTimeout
	if timeout <= 0 {
		timeout = config.DefaultLLMCallTimeout
	}
	return context.WithTimeout(ctx, timeout)
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

	progress.Report(ctx, "predict:semantic_search")
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
	log := logger.Logger(ctx)

	llmReq := LLMRequest{
		Text:   req.EmailText,
		Amount: req.Amount,
	}
	log.Info("handleLLm", "llmReq", llmReq)

	parsed, categoryID, metadata, err := s.llmFallback(ctx, budgetId, llmReq)
	log.Info("after llmFallback", "parsed", parsed, "categoryID", categoryID, "metadata", metadata, "err", err)
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

type summaryResponse struct {
	Summary string `json:"summary"`
}

func cleanEmail(text string) string {
	// Remove forwarded headers
	if idx := strings.Index(text, "Dear Customer"); idx != -1 {
		text = text[idx:]
	}
	// Remove from "Warm Regards" onwards
	for _, marker := range []string{"Warm Regards", "warm regards", "Warm regards"} {
		if idx := strings.Index(text, marker); idx != -1 {
			text = text[:idx]
		}
	}
	// Remove URLs
	urlRegex := regexp.MustCompile(`https?://\S+`)
	text = urlRegex.ReplaceAllString(text, "")
	// Remove UPI ref
	upiRegex := regexp.MustCompile(`(?i)UPI (transaction )?reference (no\.?|number)[^.]*\.`)
	text = upiRegex.ReplaceAllString(text, "")
	// Remove support sections
	for _, marker := range []string{"If you did not auth", "Important Note", "What You Can Do", "Need Help"} {
		if idx := strings.Index(text, marker); idx != -1 {
			text = text[:idx]
		}
	}
	return strings.TrimSpace(text)
}

func (s *predictionService) SummarizeEmailText(ctx context.Context, text string) (string, error) {
	log := logger.Logger(ctx)
	log.Info("SummarizeEmailText started", "text", text)

	// text = cleanEmail(text)
	htmlRegex := regexp.MustCompile(`<[^>]*>`)
	text = htmlRegex.ReplaceAllString(text, "")

	log.Info("SummarizeEmailText before prompt", "text", text)

	chatRes, err := s.chatWithFallback(ctx, "predict:summarize", func(model string) sharedModel.ChatRequest {
		return sharedModel.ChatRequest{
			Model: model,
			Messages: []sharedModel.AgentMessage{
				{
					Role: sharedModel.RoleSystem,
					Content: []sharedModel.ContentBlock{
						{
							Type: "text",
							Text: client.EmailSummarizationPrompt,
						},
					},
				},
				{
					Role: sharedModel.RoleUser,
					Content: []sharedModel.ContentBlock{
						{
							Type: "text",
							Text: text,
						},
					},
				},
			},
			Temperature: 0.0,
			Stream:      false,
			Format:      "json",
		}
	})
	if err != nil {
		return "", err
	}
	if len(chatRes.Message.Content) == 0 {
		// Non-transaction emails legitimately summarize to nothing.
		log.Warn("empty summarization response", "text", text)
		return "", nil
	}

	res, err := utils.UnmarshalResponse[summaryResponse]([]byte(chatRes.Message.Content[0].Text))
	if err != nil {
		return "", err
	}

	log.Info("SummarizeEmailText completed", "summary", res.Summary)

	return res.Summary, nil
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

	chatRes, err := s.chatWithFallback(ctx, "parse:extract", func(model string) sharedModel.ChatRequest {
		return sharedModel.ChatRequest{
			Model: model,
			Messages: []sharedModel.AgentMessage{
				{
					Role: sharedModel.RoleSystem,
					Content: []sharedModel.ContentBlock{
						{
							Type: "text",
							Text: client.ExtractionPrompt,
						},
					},
				},
				{
					Role: sharedModel.RoleUser,
					Content: []sharedModel.ContentBlock{
						{
							Type: "text",
							Text: text,
						},
					},
				},
			},
			Temperature: 0.0,
			MaxTokens:   100000,
			Stream:      false,
			Format:      client.ExtractionSchema,
		}
	})
	if err != nil {
		return nil, err
	}
	if len(chatRes.Message.Content) == 0 {
		// The model replies with nothing for some non-transaction emails despite
		// the prompt; treat it as a skip so the batch survives — retrying at
		// temperature 0 would deterministically fail again.
		log.Warn("empty extraction response, treating email as non-transaction", "emailText", text)
		return &sharedModel.ExtractedEmailResponse{
			EmailText: text,
			Skipped:   true,
			Reasoning: "extractor returned no content",
		}, nil
	}

	extracted, err := utils.UnmarshalResponse[sharedModel.ExtractedEmailResponse](
		[]byte(chatRes.Message.Content[0].Text),
	)
	if err != nil {
		return nil, err
	}

	extracted.EmailText = text

	log.Info("email extraction", "extracted", extracted)
	return &extracted, nil
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

	extractedEmail := sharedModel.ExtractedEmailResponse{}

	if req.ExtractedInputs != nil {
		extractedEmail.Merchant = req.ExtractedInputs.Merchant
		extractedEmail.AccountCard = req.ExtractedInputs.Account
		extractedEmail.Date = req.ExtractedInputs.Date
		extractedEmail.EmailText = req.EmailText
	} else {
		// Step 1: Extract email data if the caller didn't already do it. Goes
		// through the same provider chain as the parse step rather than
		// straight to ollama, so this path survives a local outage too.
		extracted, err := s.ExtractEmailData(ctx, ExtractEmailDataRequest{EmailHtml: req.EmailText})
		if err != nil {
			return nil, err
		}
		if extracted == nil {
			return nil, errs.New(errs.CodeInternalError, "email extraction failed")
		}
		extractedEmail.Merchant = extracted.Merchant
		extractedEmail.AccountCard = extracted.AccountCard
		extractedEmail.Date = extracted.Date
		extractedEmail.EmailText = extracted.EmailText
	}

	log.Info("email extraction", "extracted", extractedEmail)

	accountStr := utils.CleanAccountString(extractedEmail.AccountCard)
	account, err := s.accountRepo.GetBySuffix(ctx, budgetId, accountStr)
	if err != nil {
		return nil, err
	}
	if account == nil {
		// Distinct code so activities can classify this as a per-email skip
		// rather than a retryable infrastructure failure.
		return nil, errs.New(errs.CodeAccountLookupFailed, "account not found for suffix %q", accountStr)
	}
	if err != nil {
		logger.Logger(ctx).Warn("email extraction failed", "error", err)
		return nil, err
	}

	var predictResponse *PredictResponse = &PredictResponse{}

	upiText, merchantName := utils.CleanUPIText(extractedEmail.Merchant)
	merchantName = utils.CleanMerchantString(extractedEmail.Merchant)
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
	log := logger.Logger(ctx)

	userCategories, err := s.categoryRepo.GetAllSimplified(ctx, budgetId)
	if err != nil {
		return nil, uuid.Nil, nil, err
	}
	log.Info("categories found", "userCategories", userCategories, "err", err)

	userCategoriesMap := make(map[string]uuid.UUID, len(userCategories))
	userCategoriesText := ""
	for _, c := range userCategories {
		userCategoriesText += c.Name + ", "
		userCategoriesMap[c.Name] = c.ID
	}

	log.Info("categories map", "map", userCategoriesMap, "text", userCategoriesText, "prompt", promptV2)

	prompt := strings.ReplaceAll(promptV2, "{categories}", userCategoriesText)
	log.Info("llmFallback", "prompt", prompt)

	chatRes, err := s.chatWithFallback(ctx, "predict:llm_fallback", func(model string) sharedModel.ChatRequest {
		return sharedModel.ChatRequest{
			Model: model,
			Messages: []sharedModel.AgentMessage{
				{
					Role: sharedModel.RoleSystem,
					Content: []sharedModel.ContentBlock{
						{
							Type: "text",
							Text: prompt,
						},
					},
				},
				{
					Role: sharedModel.RoleUser,
					Content: []sharedModel.ContentBlock{
						{
							Type: "text",
							Text: req.Text,
						},
					},
				},
			},
			Temperature: 0.0,
			MaxTokens:   10000,
			Stream:      false,
			Format:      "json",
		}
	})
	if err != nil {
		return nil, uuid.Nil, nil, errs.Wrap(errs.CodeInternalError, "error in llm fallback", err)
	}
	if chatRes.Message.Content == nil {
		return nil, uuid.Nil, nil, errs.New(errs.CodeInternalError, "LLM fallback: no content returned")
	}

	parsed, err := utils.UnmarshalResponse[model.LLMPrediction]([]byte(chatRes.Message.Content[0].Text))
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
		"model":             chatRes.Model,
		"prompt":            prompt,
		"input_text":        req.Text,
		"input_amount":      req.Amount,
		"response":          chatRes.Message.Content[0].Text,
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
