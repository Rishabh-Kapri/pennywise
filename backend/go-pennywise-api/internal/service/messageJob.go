package service

import (
	"context"
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
		txn     *model.Transaction
		created bool
	)
	txCtx, cancel := context.WithTimeout(ctx, createTransactionTimeout)
	defer cancel()
	err = s.deps.DBTxn.Do(txCtx, func(ctx context.Context) error {
		if hash != textHash {
			// A transaction saved before refs were read has only the text hash,
			// and importing that email again must still find it.
			existing, err := s.deps.TransactionRepo.FindByDedupeHash(ctx, budgetID, textHash)
			if err != nil {
				return err
			}
			if existing != nil {
				txn = existing
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
		if !created {
			return nil
		}

		_, err = s.deps.Predictions.CreateCipherPrediction(ctx, cipherPrediction(budgetID, txn, p, text, amount))
		return err
	})
	if err != nil {
		return result, err
	}
	if txn == nil {
		return result, errs.New(errs.CodeTransactionNotCreated, "no transaction was created")
	}
	result.TransactionIDs = []uuid.UUID{txn.ID}
	result.Created = created
	if !created {
		result.ExistingSince = txn.CreatedAt
		logger.Logger(ctx).Info("transaction already exists, nothing added",
			"transactionId", txn.ID, "existingSince", txn.CreatedAt, "messageId", p.Message.ExternalID)
	}
	// Also set for a duplicate: a retry after the transaction was saved still
	// has to queue this. LearnTransaction is what makes repeating it safe.
	if p.Source != nil && txn.PayeeID != nil {
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
	logger.Logger(ctx).Info("alias confirmed", "transactionId", p.TransactionID, "channel", key.Channel, "merchant", key.Merchant)
	return nil
}

// learnFromSource saves where the transaction came from and who it was. A
// source row that already exists means this ran before, so a retry doesn't
// count the merchant twice.
func (s *messageJobService) learnFromSource(
	ctx context.Context,
	budgetID uuid.UUID,
	p model.TransactionLearnPayload,
) error {
	if p.Source.Kind != "" {
		added, err := s.deps.TransactionSources.RecordFromPipeline(ctx, budgetID, p.TransactionID, *p.Source)
		if err != nil || !added {
			return err
		}
	}
	if p.Source.Channel == "" || p.Source.Merchant == "" {
		return nil
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
	confidence := p.Prediction.Confidence * 100
	accountConfidence := 100.0
	return model.CipherPredictionRecord{
		BudgetID:            budgetID,
		TransactionID:       txn.ID,
		EmailText:           &text,
		ExtractedAccount:    &p.Entities.Account,
		ExtractedPayee:      &p.Prediction.PayeeName,
		PredictedPayeeID:    txn.PayeeID,
		PredictedCategoryID: txn.CategoryID,
		AccountConfidence:   &accountConfidence,
		PayeeConfidence:     &confidence,
		CategoryConfidence:  &confidence,
		Amount:              &amount,
		Source:              source,
	}
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
