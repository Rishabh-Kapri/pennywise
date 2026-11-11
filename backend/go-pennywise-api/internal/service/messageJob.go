package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
)

// MessageJobService writes what cipher decided about a message: the
// message pipeline's last steps, run by the job worker. The budget is in ctx.
type MessageJobService interface {
	// CreateTransaction creates the transaction and its cipher prediction for
	// a classified message and tells the budget's clients. Its result says what
	// to learn from it.
	//
	// Runs at least once, so it must be safe to repeat. Returns
	// CodeAccountLookupFailed or CodeCategoryLookupFailed for a message that
	// doesn't fit this budget, CodeTransactionCreateFailed or
	// CodePayeeCreateFailed otherwise.
	CreateTransaction(ctx context.Context, p model.TransactionCreatePayload) (model.TransactionCreateResult, error)

	// LearnTransaction teaches the lookup step who a transaction was. Safe to
	// repeat. Returns CodeTransactionLearnFailed.
	LearnTransaction(ctx context.Context, p model.TransactionLearnPayload) error
}

// MessageJobDeps are what the methods need. CreateTransactionActivity and
// PipelineStatusActivity use the same ones for the Temporal pipeline.
type MessageJobDeps struct {
	Transactions TransactionService
	Payees       PayeeService
	Predictions  PredictionService
	Websocket    WebsocketService
	Push         PushNotificationService
	DBTxn        repository.DBTxn

	TransactionRepo     repository.TransactionRepository
	Accounts            repository.AccountRepository
	Categories          repository.CategoryRepository
	CounterpartyAliases repository.CounterpartyAliasRepository
	TransactionSources  repository.TransactionSourceRepository
	CipherPredictions   repository.CipherPredictionRepository
}

type messageJobService struct {
	deps MessageJobDeps
}

func NewMessageJobService(deps MessageJobDeps) MessageJobService {
	return &messageJobService{deps: deps}
}

// createTransactionTimeout limits the database work of creating a transaction,
// the same as transactionService.Create.
const createTransactionTimeout = 30 * time.Second

// CreateTransaction implements [MessageJobService]. The transaction and its
// cipher prediction are saved together. A retried job finds the transaction by
// its dedupe hash and adds nothing.
func (s *messageJobService) CreateTransaction(
	ctx context.Context,
	p model.TransactionCreatePayload,
) (model.TransactionCreateResult, error) {
	var result model.TransactionCreateResult
	budgetID := utils.MustBudgetID(ctx)

	suffix := utils.CleanAccountString(p.Entities.Account)
	account, err := s.deps.Accounts.GetBySuffix(ctx, budgetID, suffix)
	if err != nil {
		return result, errs.Wrap(errs.CodeAccountLookupFailed, "find account", err)
	}
	if account == nil {
		return result, errs.New(errs.CodeAccountLookupFailed, "no account ending in %q", suffix)
	}
	categoryID, err := s.categoryID(ctx, budgetID, p.Prediction.CategoryName)
	if err != nil {
		return result, err
	}

	text := p.Message.Body
	if p.Source != nil {
		text = p.Source.Text
	}
	amount := signedAmount(p)
	date := p.Entities.Date.Format("2006-01-02")

	// The whole text is in the older hash, so the same payment's email and SMS
	// differ. A bank reference is the same in both.
	textHash := utils.Hash(account.ID.String() + date + fmt.Sprintf("%.2f", amount) + text)
	hash := textHash
	if ref := strings.ToUpper(strings.TrimSpace(p.Entities.Ref)); ref != "" {
		hash = utils.Hash(account.ID.String() + "|ref|" + ref)
	}

	var (
		txn      *model.Transaction
		created  bool
		matched  bool // another channel's message already created it
		possible *uuid.UUID
	)
	txCtx, cancel := context.WithTimeout(ctx, createTransactionTimeout)
	defer cancel()
	err = s.deps.DBTxn.Do(txCtx, func(ctx context.Context) error {
		txn, created, matched, possible = nil, false, false, nil // the transaction may run again
		// The email and SMS of one payment can arrive together, so only one
		// of them looks for the other's transaction at a time.
		if err := s.deps.TransactionRepo.LockForDedupe(ctx, budgetID, account.ID, amount); err != nil {
			return err
		}
		if hash != textHash {
			// A transaction saved before refs were read has only the text hash,
			// and importing that email again must still find it.
			existing, err := s.deps.TransactionRepo.FindByDedupeHash(ctx, budgetID, textHash)
			if err != nil {
				return err
			}
			if existing != nil {
				txn = existing
				return s.recordSource(ctx, budgetID, txn.ID, p)
			}
		}
		if !p.AllowDuplicate {
			match, maybe, err := s.otherChannelMatch(ctx, budgetID, account.ID, amount, p)
			if err != nil {
				return err
			}
			if match != nil {
				txn, matched = match, true
				return s.recordSource(ctx, budgetID, txn.ID, p)
			}
			if maybe != nil {
				possible = maybe
				return nil
			}
		}
		payeeID, err := s.payeeID(ctx, p.Prediction.PayeeName)
		if err != nil {
			return err
		}
		txn, created, err = s.deps.Transactions.CreateDeduped(ctx, model.Transaction{
			BudgetID:    budgetID,
			AccountID:   &account.ID,
			PayeeID:     &payeeID,
			CategoryID:  &categoryID,
			Amount:      amount,
			Date:        model.Date(date),
			Status:      model.TransactionStatusUnapproved,
			DedupeHash:  &hash,
			RawBankText: &text,
		})
		if err != nil {
			return err
		}
		if created {
			if _, err := s.deps.Predictions.CreateCipherPrediction(ctx, cipherPrediction(budgetID, txn, p, text, amount)); err != nil {
				return err
			}
		}
		// Saved with the transaction, so the other channel's message finds it.
		return s.recordSource(ctx, budgetID, txn.ID, p)
	})
	if err != nil {
		return result, err
	}
	if possible != nil {
		result.PossibleDuplicateOf = possible
		logger.Logger(ctx).Info("message may be a payment already added from another channel",
			"transactionId", *possible, "messageId", p.Message.ExternalID)
		return result, nil
	}
	if txn == nil {
		return result, errs.New(errs.CodeTransactionNotCreated, "no transaction was created")
	}
	result.TransactionIDs = []uuid.UUID{txn.ID}
	result.Created = created
	result.Duplicate = matched
	if !created {
		result.ExistingSince = txn.CreatedAt
		logger.Logger(ctx).Info("transaction already exists, nothing added",
			"transactionId", txn.ID, "existingSince", txn.CreatedAt, "messageId", p.Message.ExternalID,
			"otherChannel", matched)
	}
	// Also set when the row was already there: a retry after the transaction
	// was saved still has to queue this. LearnTransaction makes repeating it
	// safe. A message matched to another channel's transaction teaches nothing
	// that one didn't.
	if p.Source != nil && txn.PayeeID != nil && !matched && learnable(p.Prediction) {
		result.Learn = &model.TransactionLearnPayload{
			TransactionID: txn.ID,
			PayeeID:       *txn.PayeeID,
			CategoryID:    txn.CategoryID,
			Source:        p.Source,
		}
	}
	if created {
		s.notifyCreated(ctx, budgetID, *txn)
	}
	return result, nil
}

// otherChannelWindow is how far apart an email and an SMS can say one payment
// happened. Banks send both within seconds, with the time to the second.
const otherChannelWindow = 2 * time.Minute

// minLearnConfidence is how sure a prediction must be to become an alias
// before a person has looked at it.
const minLearnConfidence = 0.8

// learnable reports whether a prediction is good enough to learn from on its
// own. Anything else is learned once the user approves or edits it.
func learnable(prediction model.MessagePrediction) bool {
	if model.IsUncategorized(prediction.CategoryName) {
		return false
	}
	return prediction.Source == model.MessagePredictionMemory || prediction.Confidence >= minLearnConfidence
}

// otherChannelMatch finds the transaction another channel's message already
// created for this payment. Card alerts have no reference, so it goes by
// account, amount and the time each message gives. When a time is missing it
// can't be sure, and returns the transaction as maybe instead.
func (s *messageJobService) otherChannelMatch(
	ctx context.Context,
	budgetID, accountID uuid.UUID,
	amount float64,
	p model.TransactionCreatePayload,
) (match *model.Transaction, maybe *uuid.UUID, err error) {
	src := p.Source
	if src == nil || (src.Kind != "EMAIL" && src.Kind != "SMS") {
		return nil, nil, nil
	}
	on := p.Entities.Date
	if src.OccurredOn != nil {
		on = *src.OccurredOn
	}
	if on.IsZero() {
		return nil, nil, nil
	}
	candidates, err := s.deps.TransactionSources.FindCrossChannel(ctx, model.SourceMatchQuery{
		BudgetID: budgetID, AccountID: accountID, Amount: amount, Kind: src.Kind, On: on, Text: src.Text,
	})
	if err != nil {
		return nil, nil, err
	}
	var (
		best    *model.SourceMatchCandidate
		bestGap time.Duration
	)
	for i := range candidates {
		c := &candidates[i]
		if src.OccurredAt == nil || c.OccurredAt == nil {
			if maybe == nil {
				maybe = &c.TransactionID
			}
			continue
		}
		gap := src.OccurredAt.Sub(*c.OccurredAt)
		if gap < 0 {
			gap = -gap
		}
		if gap <= otherChannelWindow && (best == nil || gap < bestGap) {
			best, bestGap = c, gap
		}
	}
	if best != nil {
		return &model.Transaction{
			ID: best.TransactionID, BudgetID: budgetID, PayeeID: best.PayeeID, CategoryID: best.CategoryID,
			CreatedAt: best.CreatedAt,
		}, nil, nil
	}
	return nil, maybe, nil
}

// recordSource saves the message the transaction came from. A transaction
// keeps one message per kind; saving the same one again changes nothing.
func (s *messageJobService) recordSource(
	ctx context.Context,
	budgetID, transactionID uuid.UUID,
	p model.TransactionCreatePayload,
) error {
	if p.Source == nil || p.Source.Kind == "" {
		return nil
	}
	_, err := s.deps.TransactionSources.RecordFromPipeline(ctx, budgetID, transactionID, *p.Source)
	return err
}

// LearnTransaction implements [MessageJobService].
func (s *messageJobService) LearnTransaction(ctx context.Context, p model.TransactionLearnPayload) error {
	budgetID := utils.MustBudgetID(ctx)
	var err error
	if p.Confirmed {
		err = s.confirmAlias(ctx, budgetID, p)
	} else if p.Source != nil {
		err = s.deps.DBTxn.Do(ctx, func(ctx context.Context) error {
			return s.learnFromSource(ctx, budgetID, p)
		})
	}
	if err != nil {
		return errs.Wrap(errs.CodeTransactionLearnFailed, "learn from transaction", err)
	}
	return nil
}

// confirmAlias applies what the user decided to the transaction's merchant.
func (s *messageJobService) confirmAlias(
	ctx context.Context,
	budgetID uuid.UUID,
	p model.TransactionLearnPayload,
) error {
	key, err := s.deps.TransactionSources.LookupKey(ctx, budgetID, p.TransactionID)
	if err != nil || key == nil {
		return err
	}
	if err := s.deps.CounterpartyAliases.Confirm(ctx, model.CounterpartyAliasConfirm{
		BudgetID:   budgetID,
		Channel:    key.Channel,
		Merchant:   key.Merchant,
		PayeeID:    p.PayeeID,
		CategoryID: p.CategoryID,
	}); err != nil {
		return err
	}
	if err := s.deps.CipherPredictions.MarkLearned(ctx, budgetID, p.TransactionID); err != nil &&
		!errors.Is(err, repository.ErrNotFound) {
		return err
	}
	logger.Logger(ctx).Info("alias confirmed", "transactionId", p.TransactionID, "channel", key.Channel, "merchant", key.Merchant)
	return nil
}

// learnFromSource teaches the lookup step who the transaction's merchant was.
// The prediction's learned_at is claimed first, so a retry doesn't count the
// merchant twice.
func (s *messageJobService) learnFromSource(
	ctx context.Context,
	budgetID uuid.UUID,
	p model.TransactionLearnPayload,
) error {
	// Creating the transaction saves its source now. Jobs queued before that
	// still carry it, and saving it twice changes nothing.
	if p.Source.Kind != "" {
		if _, err := s.deps.TransactionSources.RecordFromPipeline(ctx, budgetID, p.TransactionID, *p.Source); err != nil {
			return err
		}
	}
	if p.Source.Channel == "" || p.Source.Merchant == "" {
		return nil
	}
	first, err := s.deps.CipherPredictions.ClaimLearning(ctx, budgetID, p.TransactionID)
	if err != nil || !first {
		return err
	}
	return s.deps.CounterpartyAliases.Learn(ctx, model.CounterpartyAliasLearn{
		BudgetID:   budgetID,
		Channel:    p.Source.Channel,
		Merchant:   p.Source.Merchant,
		PayeeID:    p.PayeeID,
		CategoryID: p.CategoryID,
	})
}

func (s *messageJobService) categoryID(ctx context.Context, budgetID uuid.UUID, name string) (uuid.UUID, error) {
	categories, err := s.deps.Categories.GetAllSimplified(ctx, budgetID)
	if err != nil {
		return uuid.Nil, errs.Wrap(errs.CodeCategoryLookupFailed, "list categories", err)
	}
	for _, category := range categories {
		if strings.EqualFold(category.Name, name) {
			return category.ID, nil
		}
	}
	return uuid.Nil, errs.New(errs.CodeCategoryLookupFailed, "no category named %q", name)
}

// payeeID finds the payee by name, or creates it.
func (s *messageJobService) payeeID(ctx context.Context, name string) (uuid.UUID, error) {
	if name == "" {
		name = "Unknown Payee"
	}
	found, err := s.deps.Payees.Search(ctx, name)
	if err != nil {
		return uuid.Nil, errs.Wrap(errs.CodePayeeLookupFailed, "search payees", err)
	}
	for _, payee := range found {
		if strings.EqualFold(payee.Name, name) {
			return payee.ID, nil
		}
	}
	created, err := s.deps.Payees.Create(ctx, model.Payee{Name: name})
	if err != nil {
		return uuid.Nil, errs.Wrap(errs.CodePayeeCreateFailed, "create payee", err)
	}
	return created.ID, nil
}

// signedAmount uses the check step's debit or credit for the sign, in case the
// extraction got it wrong. Outflows are negative.
func signedAmount(p model.TransactionCreatePayload) float64 {
	switch p.Verdict.Type {
	case "debit":
		return -math.Abs(p.Entities.Amount)
	case "credit":
		return math.Abs(p.Entities.Amount)
	}
	return p.Entities.Amount
}

func cipherPrediction(
	budgetID uuid.UUID,
	txn *model.Transaction,
	p model.TransactionCreatePayload,
	text string,
	amount float64,
) model.CipherPredictionRecord {
	source := model.PredictionSourceLLM
	if p.Prediction.Source == model.MessagePredictionMemory {
		source = model.PredictionSourceRule
	}
	categoryConfidence := p.Prediction.Confidence * 100
	payeeConfidence := categoryConfidence
	if c := p.Prediction.PayeeConfidence; c != nil {
		payeeConfidence = *c * 100
	}
	// Verify checks the account against the message with the other details.
	var accountConfidence *float64
	if c := p.Entities.VerifiedConfidence; c != nil {
		accountConfidence = new(*c * 100)
	}
	merchant := p.Entities.Merchant
	var reasoning *string
	if r := strings.TrimSpace(p.Prediction.Reasoning); r != "" {
		reasoning = &r
	}
	return model.CipherPredictionRecord{
		BudgetID:            budgetID,
		TransactionID:       txn.ID,
		EmailText:           &text,
		LLMReasoning:        reasoning,
		Metadata:            predictionTrail(p),
		ExtractedAccount:    &p.Entities.Account,
		ExtractedPayee:      &merchant,
		PredictedPayeeID:    txn.PayeeID,
		PredictedCategoryID: txn.CategoryID,
		AccountConfidence:   accountConfidence,
		PayeeConfidence:     &payeeConfidence,
		CategoryConfidence:  &categoryConfidence,
		Amount:              &amount,
		Source:              source,
	}
}

// predictionTrail is what each step decided and which model decided it, kept in
// the prediction's metadata so a person can see later why it came out as it did.
func predictionTrail(p model.TransactionCreatePayload) json.RawMessage {
	type check struct {
		Type       string   `json:"type,omitempty"`
		Confidence *float64 `json:"confidence,omitempty"`
		Model      string   `json:"model,omitempty"`
	}
	type verify struct {
		Confidence *float64 `json:"confidence,omitempty"`
		Model      string   `json:"model,omitempty"`
		Channel    string   `json:"channel,omitempty"`
		VPA        string   `json:"vpa,omitempty"`
		Ref        string   `json:"ref,omitempty"`
	}
	type predict struct {
		Source          model.MessagePredictionSource `json:"source"`
		Confidence      float64                       `json:"confidence"`
		PayeeConfidence *float64                      `json:"payeeConfidence,omitempty"`
		Model           string                        `json:"model,omitempty"`
	}
	trail, err := json.Marshal(struct {
		MessageSource string  `json:"messageSource,omitempty"`
		MessageID     string  `json:"messageId,omitempty"`
		Check         check   `json:"check"`
		Verify        verify  `json:"verify"`
		Predict       predict `json:"predict"`
	}{
		MessageSource: p.Message.Source,
		MessageID:     p.Message.ExternalID,
		Check:         check{Type: p.Verdict.Type, Confidence: p.Verdict.Confidence, Model: p.Verdict.Model},
		Verify: verify{
			Confidence: p.Entities.VerifiedConfidence, Model: p.Entities.VerifiedBy,
			Channel: p.Entities.Channel, VPA: p.Entities.VPA, Ref: p.Entities.Ref,
		},
		Predict: predict{
			Source: p.Prediction.Source, Confidence: p.Prediction.Confidence,
			PayeeConfidence: p.Prediction.PayeeConfidence, Model: p.Prediction.Model,
		},
	})
	if err != nil {
		return nil
	}
	return trail
}

// notifyCreated runs after the commit. Failures are logged, not returned: the
// transaction exists, and a retry would notify twice.
func (s *messageJobService) notifyCreated(ctx context.Context, budgetID uuid.UUID, txn model.Transaction) {
	created := []model.Transaction{txn}
	if err := s.deps.Websocket.SendNotification(ctx, budgetID, model.EventTransactionCreated, created); err != nil {
		logger.Logger(ctx).Warn("failed to send transaction created notification", "transactionId", txn.ID, "error", err)
	}
	if s.deps.Push != nil {
		s.deps.Push.NotifyTransactionsCreated(ctx, budgetID, created)
	}
}
