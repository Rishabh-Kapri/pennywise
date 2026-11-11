package llmAdapter

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	prompts "github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/context"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	cipherModel "github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	sharedUtils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
)

// Config is what one adapter runs with.
type Config struct {
	Client     *llm.ObservedLLM
	Provider   string
	Model      string
	MaxTokens  int // 0 leaves it to the provider
	Categories repository.CategoryRepository
}

type adapter struct {
	cfg Config
}

type LLM interface {
	ports.CheckTransaction
	ports.Extract
	ports.Verifier
	ports.Predictor
}

func NewLLMAdapter(cfg Config) LLM {
	return &adapter{cfg: cfg}
}

// ask sends one system + user message pair and returns the reply's text.
func (a *adapter) ask(
	ctx context.Context,
	failCode errs.Code,
	system, user string,
) (string, *ports.Usage, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultLLMCallTimeout)
	defer cancel()

	res, err := a.cfg.Client.Chat(ctx, sharedModel.ChatRequest{
		Model:     a.cfg.Model,
		MaxTokens: a.cfg.MaxTokens,
		Format:    "json",
		Messages: []sharedModel.AgentMessage{
			{Role: sharedModel.RoleSystem, Content: []sharedModel.ContentBlock{{Type: "text", Text: system}}},
			{Role: sharedModel.RoleUser, Content: []sharedModel.ContentBlock{{Type: "text", Text: user}}},
		},
	}, nil)
	if err != nil {
		return "", nil, err
	}
	text, err := replyText(res, a.cfg.Model, failCode)
	if err != nil {
		return "", nil, err
	}

	var usage *ports.Usage
	if res.Usage.Available {
		usage = &ports.Usage{
			Provider:         a.cfg.Provider,
			Model:            a.cfg.Model,
			InputTokens:      res.Usage.InputTokens,
			OutputTokens:     res.Usage.OutputTokens,
			CacheReadTokens:  &res.Usage.CacheReadTokens,
			CacheWriteTokens: &res.Usage.CacheWriteTokens,
			Cost:             res.Usage.Cost,
		}
	}
	return text, usage, nil
}

// replyText returns the reply's text. A cut-off reply is reported as that, not
// as broken JSON: reasoning models spend the same token budget before answering.
func replyText(res *sharedModel.ChatResponse, model string, failCode errs.Code) (string, error) {
	if res.StopReason == sharedModel.StopReasonMaxTokens {
		return "", errs.New(failCode, "%s reached its token limit before finishing", model)
	}
	for _, block := range res.Message.Content {
		if strings.TrimSpace(block.Text) != "" {
			return block.Text, nil
		}
	}
	return "", errs.New(failCode, "empty response from %s", model)
}

// Check implements [ports.CheckTransaction]
func (a *adapter) Check(ctx context.Context, msg ports.Message) (ports.TxnVerdict, error) {
	text, usage, err := a.ask(ctx, errs.CodeMessageCheckFailed, prompts.CheckTransactionPrompt, msg.Body)
	if err != nil {
		return ports.TxnVerdict{}, err
	}
	res, err := sharedUtils.UnmarshalResponse[ports.TxnVerdict]([]byte(text))
	if err != nil {
		return ports.TxnVerdict{}, err
	}
	res.Usage = usage
	return res, nil
}

// extraction is the JSON ExtractionPrompt asks for.
type extraction struct {
	Account     string   `json:"account"`
	AccountCard string   `json:"account_card"` // some models use the older name
	Merchant    string   `json:"merchant"`
	Currency    string   `json:"currency"`
	Amount      flexible `json:"amount"`
	Date        string   `json:"date"`
	Time        string   `json:"time"`
}

// flexible is a number that models sometimes send as a string ("13.04").
type flexible float64

func (f *flexible) UnmarshalJSON(data []byte) error {
	text := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if text == "" || text == "null" {
		*f = 0
		return nil
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(text, ",", ""), 64)
	if err != nil {
		return err
	}
	*f = flexible(value)
	return nil
}

var extractionDateLayouts = []string{"2006-01-02", "02-01-2006", "02-01-06", "02/01/2006", "2 Jan, 2006", "2 Jan 2006", "2-Jan-2006"}

// parseTimestamp reads the date and time the model extracted. A blank date is
// a zero time. The bank's timezone isn't known, so the clock reading is stored
// as UTC, which keeps the calendar date as written.
func parseTimestamp(date, clock string) (time.Time, error) {
	date, clock = strings.TrimSpace(date), strings.TrimSpace(clock)
	if date == "" {
		return time.Time{}, nil
	}
	for _, layout := range extractionDateLayouts {
		day, err := time.Parse(layout, date)
		if err != nil {
			continue
		}
		if at, err := time.Parse("15:04:05", clock); err == nil {
			return day.Add(time.Duration(at.Hour())*time.Hour +
				time.Duration(at.Minute())*time.Minute +
				time.Duration(at.Second())*time.Second), nil
		}
		return day, nil
	}
	return time.Time{}, fmt.Errorf("unrecognised date %q", date)
}

func toEntities(e extraction) (ports.ExtractedEntities, error) {
	timestamp, err := parseTimestamp(e.Date, e.Time)
	if err != nil {
		return ports.ExtractedEntities{}, errs.Wrap(errs.CodeMessageExtractFailed, "reading the extracted date", err)
	}
	account := e.Account
	if account == "" {
		account = e.AccountCard
	}
	return ports.ExtractedEntities{
		Account:   account,
		Merchant:  e.Merchant,
		Currency:  e.Currency,
		Amount:    float64(e.Amount),
		Timestamp: timestamp,
	}, nil
}

// Extract implements [ports.Extract].
func (a *adapter) Extract(ctx context.Context, msg ports.Message) (ports.ExtractedEntities, error) {
	text, usage, err := a.ask(ctx, errs.CodeMessageExtractFailed, prompts.ExtractionPrompt, msg.Body)
	if err != nil {
		return ports.ExtractedEntities{}, err
	}
	parsed, err := sharedUtils.UnmarshalResponse[extraction]([]byte(text))
	if err != nil {
		return ports.ExtractedEntities{}, err
	}
	entities, err := toEntities(parsed)
	if err != nil {
		return ports.ExtractedEntities{}, err
	}
	entities.Usage = usage
	return entities, nil
}

// How close a category name must be when the model got it almost right. These
// match the old prediction service; see resolveCategoryFuzzy there.
const (
	categoryFuzzyMinScore  = 0.45
	categoryFuzzyMinMargin = 0.10
)

// matchCategory returns the budget's name for suggested, or "" when nothing is
// close enough or the two best matches are too close to call.
func (a *adapter) matchCategory(
	ctx context.Context,
	budgetID uuid.UUID,
	names map[string]bool,
	suggested string,
) (string, error) {
	if suggested == "" {
		return "", nil
	}
	if names[suggested] {
		return suggested, nil
	}
	matches, err := a.cfg.Categories.FindClosestSimplified(ctx, budgetID, suggested, 2)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 || matches[0].Score < categoryFuzzyMinScore {
		return "", nil
	}
	if len(matches) > 1 && matches[0].Score-matches[1].Score < categoryFuzzyMinMargin {
		return "", nil
	}
	return matches[0].Name, nil
}

// Predict implements [ports.Predictor].
func (a *adapter) Predict(
	ctx context.Context,
	msg ports.Message,
	entities ports.ExtractedEntities,
) (ports.PredictionResult, error) {
	log := logger.Logger(ctx)
	budgetID, err := sharedUtils.BudgetIDFromContext(ctx)
	if err != nil {
		return ports.PredictionResult{}, err
	}
	categories, err := a.cfg.Categories.GetAllSimplified(ctx, budgetID)
	if err != nil {
		return ports.PredictionResult{}, err
	}
	names := make(map[string]bool, len(categories))
	list := make([]string, 0, len(categories))
	for _, category := range categories {
		names[category.Name] = true
		list = append(list, category.Name)
	}
	system := strings.ReplaceAll(prompts.PredictionPrompt, "{categories}", strings.Join(list, ", "))
	user := fmt.Sprintf("Merchant: %s\nAmount: %.2f\nAccount: %s\n\n%s",
		entities.Merchant, entities.Amount, entities.Account, msg.Body)

	text, usage, err := a.ask(ctx, errs.CodeMessagePredictFailed, system, user)
	if err != nil {
		return ports.PredictionResult{}, err
	}
	parsed, err := sharedUtils.UnmarshalResponse[cipherModel.LLMPrediction]([]byte(text))
	if err != nil {
		return ports.PredictionResult{}, err
	}

	category, err := a.matchCategory(ctx, budgetID, names, parsed.SuggestedTag)
	if err != nil {
		return ports.PredictionResult{}, err
	}
	if category == "" {
		log.Warn("no budget category matches the prediction", "suggested", parsed.SuggestedTag, "model", a.cfg.Model)
	}
	return toPrediction(parsed, category, entities.Merchant, a.cfg.Model, usage), nil
}

// toPrediction is the step's answer from the model's reply. The category is
// what it matched in the budget; the model's own merchant name wins over the
// extracted one.
func toPrediction(
	parsed cipherModel.LLMPrediction,
	category, extractedMerchant, model string,
	usage *ports.Usage,
) ports.PredictionResult {
	payee := parsed.MerchantName
	if payee == "" {
		payee = extractedMerchant
	}
	result := ports.PredictionResult{
		PayeeName:    payee,
		CategoryName: category,
		Confidence:   float64(parsed.Confidence) / 100,
		Reasoning:    strings.TrimSpace(parsed.Reasoning),
		Model:        model,
	}
	if parsed.PayeeConfidence != nil {
		result.PayeeConfidence = new(float64(*parsed.PayeeConfidence) / 100)
	}
	if usage != nil && usage.Model != "" {
		result.Model = usage.Model
	}
	return result
}

// Verify implements [ports.Verifier]. No pipeline file offers a chat model for
// verify yet.
func (a *adapter) Verify(context.Context, ports.Message, ports.ExtractedEntities) (ports.VerifyResult, error) {
	return ports.VerifyResult{}, errs.New(errs.CodeNotImplemented, "verify on a chat model isn't built yet")
}
