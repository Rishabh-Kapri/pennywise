package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/categorymatch"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/client"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/emailtext"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/llmusage"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/progress"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/userllm"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/google/uuid"
)

// Semantic search accepts its best match above similarityThreshold, or above
// exactAmountThreshold when the amounts agree exactly.
const (
	similarityThreshold  = 0.80
	exactAmountThreshold = 0.70
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

type PredictionService interface {
	// Normalizes the email text using LLM to remove any formatting or other irrelevant information
	SummarizeEmailText(ctx context.Context, text string) (string, error)
	ExtractEmailData(ctx context.Context, req ExtractEmailDataRequest) (*sharedModel.ExtractedEmailResponse, error)
	Predict(ctx context.Context, req PredictRequest) (*PredictResponse, error)
	GenerateTransactionEmbedding(
		ctx context.Context,
		req TransactionEmbeddingRequest,
	) (*TransactionEmbeddingResponse, error)
	// SuggestCategoryDescription drafts a one- or two-sentence description of
	// what belongs in a budget category.
	SuggestCategoryDescription(ctx context.Context, req sharedModel.CategoryDescriptionContext) (string, error)
	// ExtractStatement pulls transaction rows out of one chunk of statement
	// text for reconciliation.
	ExtractStatement(
		ctx context.Context,
		req sharedModel.StatementExtractRequest,
	) (*sharedModel.StatementExtractResponse, error)
}

type predictionService struct {
	llmResolver   llm.LLMResolver
	embeddingRepo repository.TransactionEmbeddingRepository
	accountRepo   repository.AccountRepository
	payeeRepo     repository.PayeesRepository
	payeeRuleRepo repository.PayeeRuleRepository
	categoryRepo  repository.CategoryRepository
	llmModels     repository.UserLLMModelRepository
	embedders     EmbedderResolver
	tracer        oteltrace.Tracer
	// llmCallTimeout bounds each model and embedding call.
	llmCallTimeout time.Duration
}

// EmbedderResolver gets a client for one of the user's embedding models.
type EmbedderResolver interface {
	ResolveEmbedder(m sharedModel.UserLLMModel) (client.Embedder, error)
}

// PredictionDeps is what the prediction service runs with.
type PredictionDeps struct {
	LLMResolver   llm.LLMResolver
	EmbeddingRepo repository.TransactionEmbeddingRepository
	AccountRepo   repository.AccountRepository
	PayeeRepo     repository.PayeesRepository
	PayeeRuleRepo repository.PayeeRuleRepository
	CategoryRepo  repository.CategoryRepository
	// LLMModels and Embedders find and call the budget owner's embedding model.
	LLMModels repository.UserLLMModelRepository
	Embedders EmbedderResolver
	Tracer    oteltrace.Tracer
	// LLMCallTimeout bounds each model and embedding call (CIPHER_LLM_CALL_TIMEOUT).
	LLMCallTimeout time.Duration
}

// NewPredictionService wires the prediction pipeline.
func NewPredictionService(d PredictionDeps) PredictionService {
	return &predictionService{
		llmResolver:    d.LLMResolver,
		embeddingRepo:  d.EmbeddingRepo,
		accountRepo:    d.AccountRepo,
		payeeRepo:      d.PayeeRepo,
		payeeRuleRepo:  d.PayeeRuleRepo,
		categoryRepo:   d.CategoryRepo,
		llmModels:      d.LLMModels,
		embedders:      d.Embedders,
		tracer:         d.Tracer,
		llmCallTimeout: d.LLMCallTimeout,
	}
}

// embed generates an embedding with the budget owner's default embedding model.
// There is no fallback to their other models: stored vectors are only comparable
// to ones from the same model. It returns the model so callers can record it.
func (s *predictionService) embed(ctx context.Context, text string) ([]float64, sharedModel.UserLLMModel, error) {
	var none sharedModel.UserLLMModel
	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		return nil, none, err
	}
	models, err := s.llmModels.ListUsableForBudget(ctx, budgetID, sharedModel.LLMModelEmbedding)
	if err != nil {
		return nil, none, errs.Wrap(errs.CodeInternalError, "loading embedding models", err)
	}
	if len(models) == 0 || !models[0].IsDefault {
		return nil, none, errs.New(errs.CodeAIProviderNotConfigured, "no embedding model set up; add one in Settings → AI")
	}
	m := models[0]

	embedder, err := s.embedders.ResolveEmbedder(m)
	if err != nil {
		return nil, none, err
	}

	embedCtx, cancel := s.withLLMTimeout(ctx)
	defer cancel()
	startedAt := time.Now()
	embedding, err := embedder.Embed(embedCtx, m.Slug, text)
	call := sharedModel.LLMCall{
		Step:       "predict:embed",
		Provider:   string(m.Provider),
		Model:      m.Slug,
		DurationMs: time.Since(startedAt).Milliseconds(),
	}
	if err != nil {
		call.Failed, call.Error = true, err.Error()
		llmusage.Record(ctx, call)
		return nil, none, err
	}
	// Embedding backends do not report token counts, so this records the call
	// and its cost in time only.
	llmusage.Record(ctx, call)
	return embedding, m, nil
}

// cloudMaxOutputTokens bounds the output budget requested from hosted providers.
// Ollama maps the cap to num_predict, a soft generation ceiling where an
// oversized value costs nothing, so local targets keep whatever the caller asked
// for. Hosted APIs instead validate it (max_output_tokens / max_tokens) against
// the model's own limit and reject anything above it — Claude Haiku 4.5 tops out
// near 64k, GPT-4o at 16k — and some reserve quota against the requested figure.
// The pipeline's largest real response is a small JSON object, so this ceiling is
// still ample.
const cloudMaxOutputTokens = 8192

// localLLMProviders serve models on infrastructure we run, where the token cap
// is advisory. Everything else is treated as a hosted API.
var localLLMProviders = map[string]bool{"ollama": true}

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

	// The chain is the budget owner's own providers, in their order.
	targets, err := s.llmResolver.Targets(ctx)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, userllm.ErrNotConfigured
	}

	var lastErr error
	for _, target := range targets {
		lc, model, err := s.llmResolver.Resolve(ctx, target.Provider, target.Model)
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
		if !localLLMProviders[target.Provider] && req.MaxTokens > cloudMaxOutputTokens {
			req.MaxTokens = cloudMaxOutputTokens
		}

		progress.Report(ctx, step)
		chatCtx, chatCancel := s.withLLMTimeout(ctx)
		startedAt := time.Now()
		res, err := lc.Chat(chatCtx, req, nil)
		elapsed := time.Since(startedAt)
		chatCancel()
		if err != nil {
			log.Warn("pipeline provider call failed, trying next",
				"step", step, "provider", target.Provider, "model", model, "error", err)
			// Record the failed attempt too: it cost wall-clock time, and on a
			// hosted provider often tokens, so leaving it out understates the run.
			llmusage.Record(ctx, sharedModel.LLMCall{
				Step:       step,
				Provider:   target.Provider,
				Model:      model,
				DurationMs: elapsed.Milliseconds(),
				Failed:     true,
				Error:      err.Error(),
			})
			lastErr = err
			continue
		}

		usage := sharedModel.LLMCall{
			Step:       step,
			Provider:   target.Provider,
			Model:      model,
			DurationMs: elapsed.Milliseconds(),
		}
		if res != nil {
			// Prefer the model name the provider echoed back — with an aliased
			// or routed model that is the one that actually served the call.
			if res.Model != "" {
				usage.Model = res.Model
			}
			usage.InputTokens = res.Usage.InputTokens
			usage.OutputTokens = res.Usage.OutputTokens
		}
		llmusage.Record(ctx, usage)

		log.Info("pipeline llm call succeeded",
			"step", step, "provider", target.Provider, "model", model,
			"inputTokens", usage.InputTokens, "outputTokens", usage.OutputTokens)
		return res, nil
	}

	return nil, errs.Wrap(errs.CodeInternalError,
		"all email pipeline providers failed for step "+step, lastErr)
}

// withLLMTimeout bounds a single model or embedding call.
func (s *predictionService) withLLMTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, s.llmCallTimeout)
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
	result.Source = sharedModel.PredictionSourceRule
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
	embedding, embedModel, err := s.embed(ctx, embeddingText)
	if err != nil {
		// Semantic search is optional: without a vector we simply fall through
		// to the payee rules / LLM fallback rather than failing the prediction.
		log.Warn("embedding failed, skipping semantic search", "error", err)
		return nil, nil
	}

	// Step 2: pgvector similarity search
	matches, err := s.embeddingRepo.SearchSimilar(ctx, budgetId, req.Amount, embedding, 3)
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
		result.Metadata["embedding_model"] = embedModel.Slug
		result.Metadata["embedding_provider"] = string(embedModel.Provider)
		return result, nil
	}
	return nil, nil
}

func (s *predictionService) handleLLM(
	ctx context.Context,
	budgetId uuid.UUID,
	req PredictRequest,
) (*PredictResponse, error) {
	parsed, categoryID, metadata, err := s.llmFallback(ctx, budgetId, req)
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
	result.Source = sharedModel.PredictionSourceLLM
	result.Payee = parsed.MerchantName
	result.Category = parsed.SuggestedTag
	result.Reasoning = parsed.Reasoning
	result.Confidence = fmt.Sprintf("%d", parsed.Confidence)
	result.Metadata = metadata

	return &result, nil
}

var htmlTags = regexp.MustCompile(`<[^>]*>`)

type summaryResponse struct {
	Summary string `json:"summary"`
}

func (s *predictionService) SummarizeEmailText(ctx context.Context, text string) (string, error) {
	log := logger.Logger(ctx)
	log.Info("SummarizeEmailText started", "text", text)

	text = htmlTags.ReplaceAllString(text, "")

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
							Text: emailSummarizationPrompt,
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
			ChatType:    sharedModel.EmailPredictChatType,
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

	text, err := emailtext.Plain(req.EmailHtml)
	if err != nil {
		return nil, err
	}

	chatRes, err := s.chatWithFallback(ctx, "parse:extract", func(model string) sharedModel.ChatRequest {
		return sharedModel.ChatRequest{
			Model: model,
			Messages: []sharedModel.AgentMessage{
				{
					Role: sharedModel.RoleSystem,
					Content: []sharedModel.ContentBlock{
						{
							Type: "text",
							Text: EmailExtractionPrompt,
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
			Format:      emailExtractionSchema,
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

	keys := MatchKeysFor(extractedEmail.Merchant, req.Amount)
	log.Info("cleaned email text", "text", keys.EmbeddingText)

	// Step 2: Search for payee specific rules
	predictResponse, err := s.handlePayeeRules(ctx, budgetId, keys.MatchString)
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
	predictResponse, err = s.handleSemanticSearch(ctx, budgetId, keys.EmbeddingText, req)
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

	// Step 4: LLM fallback. This is the last step, so its failures are the
	// prediction's failures: returning nil here would have the activity record
	// the email as a skip and drop it silently. Surfacing the error instead
	// parks the email on the retry signal, where it can be reprocessed once the
	// cause (a missing category, an LLM outage) is dealt with.
	predictResponse, err = s.handleLLM(ctx, budgetId, req)
	if err != nil {
		log.Error("LLM prediction failed", "error", err)
		return nil, err
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

	// Go through the provider chain rather than straight to ollama, like every
	// other extraction in the pipeline: this runs when the user approves or
	// corrects a transaction, and a local ollama outage used to silently cost
	// them the learned payee rule while ingestion itself carried on fine.
	extracted, err := s.ExtractEmailData(ctx, ExtractEmailDataRequest{EmailHtml: req.RawBankText})
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "extract transaction embedding text", err)
	}
	if extracted == nil {
		return nil, errs.New(errs.CodeInternalError, "transaction embedding text extraction failed")
	}

	keys := MatchKeysFor(extracted.Merchant, req.Amount)
	if keys.MerchantName == "" {
		return nil, errs.New(errs.CodeInvalidArgument, "merchant name could not be extracted")
	}

	embedding, _, err := s.embed(ctx, keys.EmbeddingText)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "generate transaction embedding", err)
	}

	return &TransactionEmbeddingResponse{
		MatchString:   keys.MatchString,
		EmbeddingText: keys.EmbeddingText,
		Embedding:     sharedModel.FormatVector(embedding),
	}, nil
}

// MatchKeys are what a merchant is found by: payee rules by MatchString (the
// UPI handle, else the name), stored embeddings by EmbeddingText.
type MatchKeys struct {
	MerchantName  string
	MatchString   string
	EmbeddingText string
}

// MatchKeysFor builds the keys the same way for a lookup (Predict) and for
// what is stored (GenerateTransactionEmbedding, cmd/backfill), so they meet.
func MatchKeysFor(merchant string, amount float64) MatchKeys {
	upiText, merchantName := utils.CleanUPIText(merchant)
	merchantName = utils.CleanMerchantString(merchantName)
	transactionType := "debit"
	if amount > 0 {
		transactionType = "credit"
	}
	matchString := upiText
	if matchString == "" {
		matchString = merchantName
	}
	return MatchKeys{
		MerchantName:  merchantName,
		MatchString:   matchString,
		EmbeddingText: transactionType + " " + merchantName,
	}
}

func (s *predictionService) resolveMatches(matches []sharedModel.TransactionEmbedding) *PredictResponse {
	if len(matches) == 0 {
		return nil
	}
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
		Source:     sharedModel.PredictionSourceVector,
		Metadata: map[string]any{
			"strategy":        "semantic_search",
			"vector_distance": best.VectorDistance,
			"amount_penalty":  best.AmountPenalty,
		},
	}

	if similarity >= similarityThreshold {
		return res
	}

	if amountPenalty == 0.0 && similarity >= exactAmountThreshold {
		return res
	}

	return nil
}

// llmFallback asks the owner's chat model for a payee and category, and
// resolves the category against the budget's (categorymatch).
func (s *predictionService) llmFallback(
	ctx context.Context,
	budgetId uuid.UUID,
	req PredictRequest,
) (*model.LLMPrediction, uuid.UUID, map[string]any, error) {
	log := logger.Logger(ctx)

	userCategories, err := s.categoryRepo.GetAllSimplified(ctx, budgetId)
	if err != nil {
		return nil, uuid.Nil, nil, err
	}
	userCategoriesText := ""
	for _, c := range userCategories {
		userCategoriesText += c.Name + ", "
	}
	prompt := strings.ReplaceAll(predictionPrompt, "{categories}", userCategoriesText)

	chatRes, err := s.chatWithFallback(ctx, "predict:llm_fallback", func(model string) sharedModel.ChatRequest {
		return sharedModel.ChatRequest{
			Model: model,
			Messages: []sharedModel.AgentMessage{
				{
					Role:    sharedModel.RoleSystem,
					Content: []sharedModel.ContentBlock{{Type: "text", Text: prompt}},
				},
				{
					Role:    sharedModel.RoleUser,
					Content: []sharedModel.ContentBlock{{Type: "text", Text: req.EmailText}},
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
	if len(chatRes.Message.Content) == 0 {
		return nil, uuid.Nil, nil, errs.New(errs.CodeInternalError, "LLM fallback: no content returned")
	}

	parsed, err := utils.UnmarshalResponse[model.LLMPrediction]([]byte(chatRes.Message.Content[0].Text))
	if err != nil {
		return nil, uuid.Nil, nil, err
	}

	suggested := parsed.SuggestedTag
	match, ok, err := categorymatch.Match(ctx, s.categoryRepo, budgetId, userCategories, suggested)
	if err != nil {
		return nil, uuid.Nil, nil, err
	}
	if !ok {
		// Deterministic: the activity parks the email instead of retrying.
		return nil, uuid.Nil, nil, errs.New(errs.CodeCategoryLookupFailed, "category not found for %q", suggested)
	}
	if match.Kind == categorymatch.Fuzzy {
		log.Info("category resolved by fuzzy match", "suggested", suggested, "matched", match.Name, "score", match.Score)
	}
	// Callers surface SuggestedTag as the prediction's category, so it must
	// agree with the ID.
	parsed.SuggestedTag = match.Name

	metadata := map[string]any{
		"strategy":             "llm_fallback",
		"model":                chatRes.Model,
		"prompt":               prompt,
		"input_text":           req.EmailText,
		"input_amount":         req.Amount,
		"response":             chatRes.Message.Content[0].Text,
		"categories_count":     len(userCategories),
		"prompt_template":      "promptV2",
		"response_category":    suggested,
		"category_match":       string(match.Kind),
		"category_matched":     match.Name,
		"category_match_score": match.Score,
	}

	return &parsed, match.ID, metadata, nil
}
