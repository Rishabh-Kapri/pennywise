package service

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var withTx = utils.WithTx

type TransactionService interface {
	GetAll(ctx context.Context) ([]model.Transaction, error)
	GetAllNormalized(
		ctx context.Context,
		filter *model.TransactionFilter,
	) (model.PaginatedResponse[model.Transaction], error)
	// GetById(ctx context.Context, id uuid.UUID) (*model.Transaction, error)
	Update(ctx context.Context, id uuid.UUID, txn model.Transaction) error
	UpdateLocation(ctx context.Context, id uuid.UUID, req model.TransactionLocationReq) (*model.Transaction, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.TransactionStatus) error
	Create(ctx context.Context, txn model.Transaction) ([]model.Transaction, error)
	CreateWithTx(ctx context.Context, tx pgx.Tx, txn model.Transaction) ([]model.Transaction, error)
	CreateWithTxDeduped(ctx context.Context, tx pgx.Tx, txn model.Transaction) (*model.Transaction, bool, error)
	DeleteById(ctx context.Context, id uuid.UUID) error
	// LearnFromTransaction queues best-effort learning for a confirmed
	// transaction. Fire and forget: failures are recorded on the prediction,
	// never returned to the caller.
	LearnFromTransaction(ctx context.Context, txn model.Transaction)
	BackfillLearning(ctx context.Context, req model.LearningBackfillRequest) (*model.LearningBackfillResult, error)
	LearningStatus(ctx context.Context) (model.LearningStats, error)
}

type transactionService struct {
	// learningBackfillRunning single-flights the backfill: a second call while
	// one is in flight would re-read the same pending page and pay for the same
	// LLM round-trips twice.
	learningBackfillRunning atomic.Bool

	repo                 repository.TransactionRepository
	budgetRepo           repository.BudgetRepository
	txnEmbeddingRepo     repository.TransactionEmbeddingRepository
	cipherClient         CipherClient
	predictionRepo       repository.PredictionRepository
	cipherPredictionRepo repository.CipherPredictionRepository
	payeeRuleRepo        repository.PayeeRuleRepository
	accountRepo          repository.AccountRepository
	payeeRepo            repository.PayeesRepository
	categoryRepo         repository.CategoryRepository
	mbService            MonthlyBudgetService
	geocodeService       GeocodeService
}

func NewTransactionService(
	r repository.TransactionRepository,
	budgetRepo repository.BudgetRepository,
	txnEmbeddingRepo repository.TransactionEmbeddingRepository,
	cipherClient CipherClient,
	predictionRepo repository.PredictionRepository,
	cipherPredictionRepo repository.CipherPredictionRepository,
	payeeRuleRepo repository.PayeeRuleRepository,
	accountRepo repository.AccountRepository,
	payeeRepo repository.PayeesRepository,
	catRepo repository.CategoryRepository,
	mbService MonthlyBudgetService,
	geocodeService GeocodeService,
) TransactionService {
	return &transactionService{
		repo:                 r,
		budgetRepo:           budgetRepo,
		txnEmbeddingRepo:     txnEmbeddingRepo,
		cipherClient:         cipherClient,
		predictionRepo:       predictionRepo,
		cipherPredictionRepo: cipherPredictionRepo,
		payeeRuleRepo:        payeeRuleRepo,
		accountRepo:          accountRepo,
		payeeRepo:            payeeRepo,
		categoryRepo:         catRepo,
		mbService:            mbService,
		geocodeService:       geocodeService,
	}
}

// Deprecated: legacy MLP prediction corrections are no longer updated from transaction edits.
func (s *transactionService) updatePrediction(
	ctx context.Context,
	tx pgx.Tx,
	budgetId uuid.UUID,
	txnId uuid.UUID,
	txn model.Transaction,
	account model.Account,
	payee model.Payee,
) error {
	prediction, err := s.predictionRepo.GetByTxnIdTx(ctx, tx, budgetId, txnId)
	if err != nil {
		return errs.Wrap(errs.CodePredictionLookupFailed, "error getting prediction", err)
	}
	if prediction == nil {
		logger.Logger(ctx).Info("prediction not found for txn", "txnId", txnId)
		return nil
	}
	logger.Logger(ctx).Debug("prediction", "prediction", prediction.String())

	logger.Logger(ctx).Debug("transaction", "txn", txn.String())

	var category *model.Category
	if txn.CategoryID != nil {
		category, err = s.categoryRepo.GetByIdSimplifiedTx(ctx, tx, budgetId, *txn.CategoryID)
		if err != nil {
			return errs.Wrap(errs.CodeCategoryLookupFailed, "error getting category", err)
		}
	}

	// if the prediction is not the same as the existing one, update it
	trueVal := true
	falseVal := false
	needsUpdate := false

	accountName := account.Name
	payeeName := payee.Name

	if prediction.Account != nil && *prediction.Account != accountName {
		prediction.HasUserCorrected = &trueVal
		corrected := account.Name
		prediction.UserCorrectedAccount = &corrected
		needsUpdate = true
	}
	if prediction.Payee != nil && *prediction.Payee != payeeName {
		prediction.HasUserCorrected = &trueVal
		corrected := payee.Name
		prediction.UserCorrectedPayee = &corrected
		needsUpdate = true
	}
	if prediction.Category != nil && category != nil && *prediction.Category != category.Name {
		prediction.HasUserCorrected = &trueVal
		corrected := category.Name
		prediction.UserCorrectedCategory = &corrected
		needsUpdate = true
	}

	// If no mismatch found but prediction was previously marked as corrected, clear it
	if !needsUpdate && prediction.HasUserCorrected != nil && *prediction.HasUserCorrected {
		prediction.HasUserCorrected = &falseVal
		needsUpdate = true
	}

	if needsUpdate {
		logger.Logger(ctx).Info("updating prediction", "txnId", txnId)
		err = s.predictionRepo.Update(ctx, tx, budgetId, prediction.ID, *prediction)
		if err != nil {
			return errs.Wrap(errs.CodePredictionUpdateFailed, "failed to update prediction", err)
		}
	}
	return nil
}

// validateTransactionPayload validates the payload of a transaction
func (s *transactionService) validateTransactionPayload(txn model.Transaction, budgetID uuid.UUID) error {
	if txn.BudgetID != budgetID {
		return errs.New(errs.CodeInvalidArgument, "transaction is not for this budget")
	}
	if txn.AccountID == nil {
		return errs.New(errs.CodeInvalidArgument, "account_id is required")
	}
	if txn.PayeeID == nil {
		return errs.New(errs.CodeInvalidArgument, "payee_id is required")
	}
	if err := txn.Date.Valid(); err != nil {
		return err
	}

	return nil
}

// loadDependencies loads the budget, account, and payee for the transaction
func (s *transactionService) loadDependencies(
	ctx context.Context,
	tx pgx.Tx,
	budgetId uuid.UUID,
	txn model.Transaction,
) (budget *model.Budget, account *model.Account, payee *model.Payee, transferAccount *model.Account, err error) {
	budget, err = s.budgetRepo.GetById(ctx, tx, budgetId)
	if err != nil {
		return nil, nil, nil, nil, errs.Wrap(errs.CodeBudgetLookupFailed, "error fetching budget", err)
	}

	account, err = s.accountRepo.GetById(ctx, tx, budgetId, *txn.AccountID)
	if err != nil {
		return nil, nil, nil, nil, errs.Wrap(errs.CodeAccountLookupFailed, "error getting account", err)
	}

	payee, err = s.payeeRepo.GetByIdTx(ctx, tx, budgetId, *txn.PayeeID)
	if err != nil {
		return nil, nil, nil, nil, errs.Wrap(errs.CodePayeeLookupFailed, "error getting payee", err)
	}

	if payee.TransferAccountID != nil {
		transferAccount, err = s.accountRepo.GetById(ctx, tx, budgetId, *payee.TransferAccountID)
		if err != nil {
			return nil, nil, nil, nil, errs.Wrap(errs.CodeAccountLookupFailed, "error getting transfer account", err)
		}
	}

	return budget, account, payee, transferAccount, nil
}

// validate the category of the transaction
// for budget transfers, the category should be nil
func (s *transactionService) validateCategory(
	categoryID *uuid.UUID,
	inflowCategoryID uuid.UUID,
	account model.Account,
	payee model.Payee,
	transferAccount *model.Account,
	amount float64,
) error {
	// budget -> budget transfers don't have a category
	isBudgetAcount := account.Type == "savings" || account.Type == "checking" || account.Type == "creditCard"
	isTransferBudget := false
	if transferAccount != nil && isBudgetAcount {
		isTransferBudget = transferAccount.Type == "savings" || transferAccount.Type == "checking" ||
			transferAccount.Type == "creditCard"
	}
	if isBudgetAcount && isTransferBudget {
		if categoryID != nil {
			return errs.New(errs.CodeInvalidArgument, "category is not allowed for budget transfers")
		}
	}
	if categoryID != nil && *categoryID == inflowCategoryID && amount < 0 {
		return errs.New(errs.CodeInvalidArgument, "negative inflow category amounts are not allowed")
	}
	return nil
}

// createCounterpartTxn creates the counterpart transaction for a transfer
func (s *transactionService) createCounterpartTxn(
	ctx context.Context,
	tx pgx.Tx,
	budgetId uuid.UUID,
	parentId uuid.UUID,
	txn model.Transaction,
	account model.Account,
	payee model.Payee,
) (uuid.UUID, error) {
	counterpart := model.Transaction{
		BudgetID:              budgetId,
		AccountID:             payee.TransferAccountID,
		PayeeID:               account.TransferPayeeID,
		CategoryID:            nil,
		Amount:                -txn.Amount,
		Date:                  txn.Date,
		Note:                  txn.Note,
		Status:                txn.Status,
		TransferAccountID:     txn.AccountID,
		TransferTransactionID: &parentId,
	}
	if counterpart.Status == "" {
		counterpart.Status = model.TransactionStatusManual
	}
	created, err := s.repo.Create(ctx, tx, counterpart)
	if err != nil {
		return uuid.Nil, errs.Wrap(errs.CodeTransferCreateFailed, "error creating transfer transaction", err)
	}
	if len(created) == 0 {
		return uuid.Nil, errs.New(errs.CodeTransferNotCreated, "no transfer transaction was created")
	}
	return created[0].ID, nil
}

// sideEffectInput holds the context for applying side effects to a transaction.
// oldTxn == nil means create, newTxn == nil means delete, both non-nil means update.
type sideEffectInput struct {
	budgetId      uuid.UUID
	oldTxn        *model.Transaction // nil for create
	newTxn        *model.Transaction // nil for delete
	budget        *model.Budget      // required for create (inflow category check)
	account       *model.Account     // nil for delete
	payee         *model.Payee       // nil for delete
	queueLearning func(model.Transaction)
}

func transactionMappingChanged(oldTxn, newTxn *model.Transaction) bool {
	if oldTxn == nil || newTxn == nil {
		return false
	}

	payeeChanged := (oldTxn.PayeeID == nil) != (newTxn.PayeeID == nil) ||
		(oldTxn.PayeeID != nil && newTxn.PayeeID != nil && *oldTxn.PayeeID != *newTxn.PayeeID)
	categoryChanged := (oldTxn.CategoryID == nil) != (newTxn.CategoryID == nil) ||
		(oldTxn.CategoryID != nil && newTxn.CategoryID != nil && *oldTxn.CategoryID != *newTxn.CategoryID)

	return payeeChanged || categoryChanged
}

func (s *transactionService) learnTransactionMappingAsync(
	ctx context.Context,
	budgetId uuid.UUID,
	txn model.Transaction,
) {
	if !s.canLearnFrom(ctx, txn) {
		return
	}

	go func() {
		bgCtx := utils.WithRequestMetadata(context.Background(), utils.RequestMetadataFromContext(ctx))
		bgCtx = utils.WithInternalAuthToken(bgCtx, utils.InternalAuthTokenFromContext(ctx))
		bgCtx, cancel := context.WithTimeout(bgCtx, 2*time.Minute)
		defer cancel()

		if err := s.learnTransactionMapping(bgCtx, budgetId, txn); err != nil {
			logger.Logger(bgCtx).Error("transaction learning failed", "txnId", txn.ID, "error", err)
		}
	}()
}

// LearnFromTransaction queues learning for a transaction the user just
// confirmed. Exported so the review queue's accept path can teach the pipeline
// without going through a full transaction update.
func (s *transactionService) LearnFromTransaction(ctx context.Context, txn model.Transaction) {
	s.learnTransactionMappingAsync(ctx, utils.MustBudgetID(ctx), txn)
}

// canLearnFrom reports whether a transaction carries what the learning step
// needs. A manually entered row has no raw bank text, so there is nothing for a
// future email to match against.
func (s *transactionService) canLearnFrom(ctx context.Context, txn model.Transaction) bool {
	if s.cipherClient == nil || s.payeeRuleRepo == nil || s.txnEmbeddingRepo == nil {
		logger.Logger(ctx).
			Warn("skipping transaction learning because dependencies are not configured", "txnId", txn.ID)
		return false
	}
	if txn.PayeeID == nil || txn.CategoryID == nil {
		logger.Logger(ctx).Warn("skipping transaction learning because payee or category is missing", "txnId", txn.ID)
		return false
	}
	if txn.RawBankText == nil || strings.TrimSpace(*txn.RawBankText) == "" {
		logger.Logger(ctx).Warn("skipping transaction learning because raw bank text is missing", "txnId", txn.ID)
		return false
	}
	return true
}

// learnTransactionMapping turns one confirmed transaction into a payee rule and
// an AUTO_LEARNED embedding, then stamps the prediction so a backfill can tell
// what has already been learned from. Runs synchronously; the async wrapper and
// the backfill share it.
//
// The outcome is recorded on the cipher prediction either way -- learning is
// best-effort and nothing upstream surfaces its failures, so learn_error is the
// only place a broken learning step becomes visible.
func (s *transactionService) learnTransactionMapping(
	ctx context.Context,
	budgetId uuid.UUID,
	txn model.Transaction,
) error {
	err := s.doLearnTransactionMapping(ctx, budgetId, txn)
	if err == nil {
		return nil
	}
	if s.cipherPredictionRepo != nil {
		if markErr := s.cipherPredictionRepo.MarkLearnFailed(ctx, budgetId, txn.ID, err.Error()); markErr != nil {
			logger.Logger(ctx).Warn("failed to record learning error", "txnId", txn.ID, "error", markErr)
		}
	}
	return err
}

func (s *transactionService) doLearnTransactionMapping(
	ctx context.Context,
	budgetId uuid.UUID,
	txn model.Transaction,
) error {
	generatedEmbedding, err := s.cipherClient.GenerateTransactionEmbedding(ctx, TransactionEmbeddingRequest{
		RawBankText: *txn.RawBankText,
		Amount:      txn.Amount,
	})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, "error generating transaction embedding", err)
	}
	if generatedEmbedding == nil || strings.TrimSpace(generatedEmbedding.Embedding) == "" {
		return errs.New(errs.CodeInternalError, "cipher returned empty transaction embedding")
	}
	// An empty match string would be stored as a payee rule matching nothing,
	// and every such rule collides on (budget_id, match_string).
	if strings.TrimSpace(generatedEmbedding.MatchString) == "" {
		return errs.New(errs.CodeInternalError, "cipher returned empty payee match string")
	}

	payeeRule := model.PayeeRule{
		BudgetID:    budgetId,
		PayeeID:     *txn.PayeeID,
		CategoryID:  txn.CategoryID,
		MatchString: generatedEmbedding.MatchString,
	}
	if err := withTx(ctx, s.repo.GetDB(), func(tx pgx.Tx) error {
		if err := s.payeeRuleRepo.CreatePayeeRule(ctx, tx, payeeRule); err != nil {
			return errs.Wrap(errs.CodeInternalError, "error creating payee rule", err)
		}

		embedding := model.TransactionEmbedding{
			BudgetID:      budgetId,
			PayeeID:       *txn.PayeeID,
			CategoryID:    *txn.CategoryID,
			Amount:        txn.Amount,
			Source:        "AUTO_LEARNED",
			EmbeddingText: generatedEmbedding.EmbeddingText,
		}
		if err := s.txnEmbeddingRepo.Upsert(ctx, tx, embedding, generatedEmbedding.Embedding); err != nil {
			return errs.Wrap(errs.CodeInternalError, "error upserting transaction embedding", err)
		}

		if s.cipherPredictionRepo != nil {
			// Absent for a transaction the pipeline did not create; the rule and
			// embedding above are still worth keeping.
			if err := s.cipherPredictionRepo.MarkLearned(ctx, tx, budgetId, txn.ID); err != nil && err != pgx.ErrNoRows {
				return errs.Wrap(errs.CodeInternalError, "error marking prediction learned", err)
			}
		}
		return nil
	}); err != nil {
		return err
	}

	logger.Logger(ctx).Info("learned transaction mapping",
		"txnId", txn.ID, "matchString", generatedEmbedding.MatchString)
	return nil
}

// Learning backfill bounds. Each item costs an extraction and an embedding
// round-trip, so a run is bounded per call; callers walk history by calling it
// repeatedly.
const (
	defaultLearningBackfillLimit = 25
	maxLearningBackfillLimit     = 100

	// A whole run is capped so a wedged backend cannot hold the single-flight
	// guard forever, and each item is capped separately so one hung call cannot
	// eat the entire run's budget.
	learningBackfillRunTimeout  = time.Hour
	learningBackfillItemTimeout = 5 * time.Minute
)

// LearningStatus reports how much of the budget's prediction history has taught
// the pipeline, and whether a backfill is currently running.
func (s *transactionService) LearningStatus(ctx context.Context) (model.LearningStats, error) {
	budgetId := utils.MustBudgetID(ctx)
	if s.cipherPredictionRepo == nil {
		return model.LearningStats{}, errs.New(errs.CodeInternalError, "cipher prediction repository is not configured")
	}
	stats, err := s.cipherPredictionRepo.LearningStats(ctx, budgetId)
	if err != nil {
		return model.LearningStats{}, err
	}
	stats.Running = s.learningBackfillRunning.Load()
	return stats, nil
}

// BackfillLearning starts learning over past predictions that never taught the
// pipeline -- transactions approved before learning worked, or whose learning
// step failed at the time.
//
// The work runs in the background and the call returns as soon as it is queued:
// each item costs two LLM round-trips against a local model, so even a small
// page runs far past the proxy's request timeout. Progress is durable rather
// than streamed -- every item stamps learned_at or learn_error as it finishes --
// so callers poll GET /api/predictions/learning to follow along.
func (s *transactionService) BackfillLearning(
	ctx context.Context,
	req model.LearningBackfillRequest,
) (*model.LearningBackfillResult, error) {
	budgetId := utils.MustBudgetID(ctx)
	log := logger.Logger(ctx)

	if s.cipherPredictionRepo == nil {
		return nil, errs.New(errs.CodeInternalError, "cipher prediction repository is not configured")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultLearningBackfillLimit
	}
	if limit > maxLearningBackfillLimit {
		limit = maxLearningBackfillLimit
	}

	candidates, err := s.cipherPredictionRepo.ListPendingLearning(ctx, budgetId, limit, req.RetryFailed)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "error listing predictions pending learning", err)
	}

	result := &model.LearningBackfillResult{Queued: len(candidates)}
	if stats, err := s.cipherPredictionRepo.LearningStats(ctx, budgetId); err != nil {
		log.Warn("could not read learning stats", "error", err)
	} else {
		result.Remaining = stats.Pending
	}

	if len(candidates) == 0 {
		result.Running = s.learningBackfillRunning.Load()
		return result, nil
	}
	if !s.learningBackfillRunning.CompareAndSwap(false, true) {
		log.Info("learning backfill already running, not starting another")
		result.Queued = 0
		result.Running = true
		return result, nil
	}

	// Detach from the request: this outlives the response by design.
	bgCtx := utils.WithRequestMetadata(context.Background(), utils.RequestMetadataFromContext(ctx))
	bgCtx = utils.WithInternalAuthToken(bgCtx, utils.InternalAuthTokenFromContext(ctx))
	bgCtx = utils.WithBudgetID(bgCtx, budgetId)

	go func() {
		defer s.learningBackfillRunning.Store(false)
		runCtx, cancel := context.WithTimeout(bgCtx, learningBackfillRunTimeout)
		defer cancel()
		s.runLearningBackfill(runCtx, budgetId, candidates)
	}()

	result.Started = true
	result.Running = true
	return result, nil
}

// runLearningBackfill works the page sequentially: each item calls out to cipher
// for an extraction and an embedding, and a local model serves those one at a
// time anyway, so parallel requests would only queue behind each other. A failed
// item is recorded on its prediction and the run carries on.
func (s *transactionService) runLearningBackfill(
	ctx context.Context,
	budgetId uuid.UUID,
	candidates []model.LearningCandidate,
) {
	log := logger.Logger(ctx)
	log.Info("learning backfill started", "count", len(candidates))

	var learned, failed int
	for _, candidate := range candidates {
		if ctxErr := ctx.Err(); ctxErr != nil {
			log.Warn("learning backfill stopped early",
				"learned", learned, "failed", failed, "error", ctxErr)
			break
		}

		txn := model.Transaction{
			ID:          candidate.TransactionID,
			BudgetID:    budgetId,
			PayeeID:     candidate.PayeeID,
			CategoryID:  candidate.CategoryID,
			Amount:      candidate.Amount,
			RawBankText: &candidate.RawBankText,
		}

		itemCtx, cancel := context.WithTimeout(ctx, learningBackfillItemTimeout)
		err := s.learnTransactionMapping(itemCtx, budgetId, txn)
		cancel()
		if err != nil {
			log.Warn("learning backfill item failed", "txnId", candidate.TransactionID, "error", err)
			failed++
			continue
		}
		learned++
	}

	log.Info("learning backfill finished", "learned", learned, "failed", failed)
}

func (s *transactionService) applySideEffects(ctx context.Context, tx pgx.Tx, input sideEffectInput) error {
	isCreate := input.oldTxn == nil && input.newTxn != nil
	isUpdate := input.oldTxn != nil && input.newTxn != nil
	isDelete := input.oldTxn != nil && input.newTxn == nil

	// --- Carryovers ---
	switch {
	case isCreate:
		if input.newTxn.CategoryID != nil && *input.newTxn.CategoryID != input.budget.Metadata.InflowCategoryID {
			monthKey := utils.GetMonthKey(input.newTxn.Date.String())
			if err := s.mbService.UpsertCarryover(
				ctx,
				tx,
				input.budgetId,
				*input.newTxn.CategoryID,
				monthKey,
				input.newTxn.Amount,
			); err != nil {
				return err
			}
		}
	case isUpdate:
		if err := s.mbService.UpdateCarryovers(
			ctx,
			tx,
			input.budgetId,
			input.oldTxn,
			input.newTxn,
			input.budget.Metadata.InflowCategoryID,
		); err != nil {
			return err
		}
	case isDelete:
		if input.oldTxn.CategoryID != nil &&
			(input.budget == nil || *input.oldTxn.CategoryID != input.budget.Metadata.InflowCategoryID) {
			monthKey := utils.GetMonthKey(input.oldTxn.Date.String())
			if err := s.mbService.UpsertCarryover(
				ctx,
				tx,
				input.budgetId,
				*input.oldTxn.CategoryID,
				monthKey,
				-input.oldTxn.Amount,
			); err != nil {
				return err
			}
		}
	}

	// --- Transfers ---
	switch {
	case isCreate:
		if input.payee != nil && input.payee.TransferAccountID != nil {
			createdId, err := s.createCounterpartTxn(
				ctx,
				tx,
				input.budgetId,
				input.newTxn.ID,
				*input.newTxn,
				*input.account,
				*input.payee,
			)
			if err != nil {
				return err
			}
			input.newTxn.TransferAccountID = input.payee.TransferAccountID
			input.newTxn.TransferTransactionID = &createdId
			if err = s.repo.Update(ctx, tx, input.budgetId, input.newTxn.ID, *input.newTxn); err != nil {
				return errs.Wrap(errs.CodeTransactionUpdateFailed, "error linking transfer transaction", err)
			}
		}
	case isUpdate:
		if err := s.reconcileTransfer(
			ctx,
			tx,
			input.budgetId,
			*input.oldTxn,
			input.newTxn,
			*input.account,
			*input.payee,
		); err != nil {
			return err
		}
	case isDelete:
		if input.oldTxn.TransferTransactionID != nil {
			if err := s.repo.DeleteById(ctx, tx, input.budgetId, *input.oldTxn.TransferTransactionID); err != nil {
				return errs.Wrap(errs.CodeTransactionDeleteFailed, "error deleting transfer transaction", err)
			}
		}
	}

	// --- Cipher Predictions ---
	// Two things teach the pipeline, and both end in the same learning call: a
	// correction (the user changed payee or category) and a confirmation (the
	// user approved the prediction as it stood). Learning only from corrections
	// meant the common case -- "yes, that is right" -- taught nothing, so the
	// same merchant went through the whole LLM fallback on every future email.
	if isUpdate {
		mappingChanged := transactionMappingChanged(input.oldTxn, input.newTxn)
		confirmed := predictionConfirmed(input.oldTxn, input.newTxn)

		if mappingChanged || confirmed {
			if s.cipherPredictionRepo == nil {
				return nil
			}

			cipherPrediction, err := s.cipherPredictionRepo.GetByTransactionID(ctx, input.budgetId, input.oldTxn.ID)
			if err != nil {
				if err == pgx.ErrNoRows {
					return nil
				}
				return errs.Wrap(errs.CodeTransactionLookupFailed, "error getting cipher prediction", err)
			}
			// No prediction means the transaction was not created by the
			// pipeline, so there is nothing to correct and nothing worth
			// learning from -- a manually entered row has no raw bank text to
			// match future emails against.
			if cipherPrediction == nil {
				return nil
			}

			// Only a real change is a correction. Approving an untouched
			// prediction confirms it, and marking that as user-corrected would
			// poison the review queue's accuracy signal.
			if mappingChanged {
				if err := s.cipherPredictionRepo.MarkUserCorrected(
					ctx,
					tx,
					input.budgetId,
					input.oldTxn.ID,
					input.newTxn.PayeeID,
					input.newTxn.CategoryID,
				); err != nil {
					return errs.Wrap(errs.CodeTransactionUpdateFailed, "error updating cipher prediction correction", err)
				}
			}

			if input.queueLearning != nil {
				learningTxn := *input.newTxn
				// The update payload carries no raw bank text; the stored row
				// does, and it is what the learned rule matches on.
				learningTxn.RawBankText = input.oldTxn.RawBankText
				input.queueLearning(learningTxn)
			}

			logger.Logger(ctx).Info("learning from cipher prediction",
				"txnId", input.oldTxn.ID, "corrected", mappingChanged, "confirmed", confirmed)
		}
	}

	return nil
}

// predictionConfirmed reports whether this update is the user approving a
// pending prediction. Approval is the only status transition that carries a
// judgement -- re-saving an already approved transaction says nothing new, so
// it must not re-trigger learning.
func predictionConfirmed(oldTxn, newTxn *model.Transaction) bool {
	if oldTxn == nil || newTxn == nil {
		return false
	}
	return oldTxn.Status == model.TransactionStatusUnapproved &&
		newTxn.Status == model.TransactionStatusApproved
}

func (s *transactionService) reconcileTransfer(
	ctx context.Context,
	tx pgx.Tx,
	budgetId uuid.UUID,
	foundTxn model.Transaction,
	newTxn *model.Transaction,
	account model.Account,
	payee model.Payee,
) error {
	// reconcile transfer transactions
	wasTransfer := foundTxn.TransferTransactionID != nil
	isTransfer := payee.TransferAccountID != nil
	samePayee := foundTxn.PayeeID != nil && newTxn.PayeeID != nil && *foundTxn.PayeeID == *newTxn.PayeeID

	switch {
	case wasTransfer && !isTransfer:
		// transfer → regular: delete counterpart, clear fields
		logger.Logger(ctx).
			Info("converting transfer to regular, deleting counterpart", "transferTxnId", *foundTxn.TransferTransactionID)
		if err := s.repo.DeleteById(ctx, tx, budgetId, *foundTxn.TransferTransactionID); err != nil {
			return errs.Wrap(errs.CodeTransactionDeleteFailed, "error deleting transfer transaction", err)
		}
		newTxn.TransferAccountID = nil
		newTxn.TransferTransactionID = nil

	case !wasTransfer && isTransfer:
		// regular → transfer: create counterpart
		logger.Logger(ctx).Info("converting regular to transfer, creating counterpart")
		createdId, err := s.createCounterpartTxn(ctx, tx, budgetId, foundTxn.ID, *newTxn, account, payee)
		if err != nil {
			return err
		}
		newTxn.TransferAccountID = payee.TransferAccountID
		newTxn.TransferTransactionID = &createdId

	case wasTransfer && isTransfer && !samePayee:
		// transfer → different transfer: delete old counterpart, create new
		logger.Logger(ctx).Info("changing transfer destination, recreating counterpart")
		if err := s.repo.DeleteById(ctx, tx, budgetId, *foundTxn.TransferTransactionID); err != nil {
			return errs.Wrap(errs.CodeTransactionDeleteFailed, "error deleting old transfer transaction", err)
		}
		createdId, err := s.createCounterpartTxn(ctx, tx, budgetId, foundTxn.ID, *newTxn, account, payee)
		if err != nil {
			return err
		}
		newTxn.TransferAccountID = payee.TransferAccountID
		newTxn.TransferTransactionID = &createdId

	case wasTransfer && isTransfer && samePayee:
		// same transfer: update counterpart with new amount/date/note
		logger.Logger(ctx).
			Info("updating existing transfer counterpart", "transferTxnId", *foundTxn.TransferTransactionID)
		counterpart := model.Transaction{
			BudgetID:              budgetId,
			AccountID:             payee.TransferAccountID,
			PayeeID:               account.TransferPayeeID,
			CategoryID:            nil,
			Amount:                -newTxn.Amount,
			Date:                  newTxn.Date,
			Note:                  newTxn.Note,
			Status:                newTxn.Status,
			TransferAccountID:     newTxn.AccountID,
			TransferTransactionID: &foundTxn.ID,
		}
		if counterpart.Status == "" {
			counterpart.Status = model.TransactionStatusManual
		}
		if err := s.repo.Update(ctx, tx, budgetId, *foundTxn.TransferTransactionID, counterpart); err != nil {
			return errs.Wrap(errs.CodeTransactionUpdateFailed, "error updating transfer counterpart", err)
		}
		newTxn.TransferAccountID = foundTxn.TransferAccountID
		newTxn.TransferTransactionID = foundTxn.TransferTransactionID
	}
	return nil
}

func (s *transactionService) GetAll(ctx context.Context) ([]model.Transaction, error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.GetAll(ctx, budgetId, nil)
}

func (s *transactionService) GetAllNormalized(
	ctx context.Context,
	filter *model.TransactionFilter,
) (model.PaginatedResponse[model.Transaction], error) {
	budgetId := utils.MustBudgetID(ctx)
	return s.repo.GetAllNormalized(ctx, budgetId, filter)
}

func (s *transactionService) Create(ctx context.Context, txn model.Transaction) ([]model.Transaction, error) {
	txCtx, txCancel := context.WithTimeout(ctx, 30*time.Second)
	defer txCancel()

	budgetID := utils.MustBudgetID(txCtx)
	txn.BudgetID = budgetID
	if err := s.validateTransactionPayload(txn, budgetID); err != nil {
		return nil, err
	}

	s.enrichLocation(txCtx, &txn)

	var createdTxn []model.Transaction
	err := withTx(txCtx, s.repo.GetDB(), func(tx pgx.Tx) error {
		var err error
		createdTxn, err = s.CreateWithTx(txCtx, tx, txn)
		return err
	})
	if err != nil {
		return nil, err
	}
	return createdTxn, nil
}

func (s *transactionService) CreateWithTx(
	ctx context.Context,
	tx pgx.Tx,
	txn model.Transaction,
) ([]model.Transaction, error) {
	budgetID := utils.MustBudgetID(ctx)
	txn.BudgetID = budgetID
	if txn.Status == "" {
		txn.Status = model.TransactionStatusManual
	}

	if err := s.validateTransactionPayload(txn, budgetID); err != nil {
		return nil, err
	}

	var createdTxn []model.Transaction
	budget, account, payee, transferAccount, err := s.loadDependencies(ctx, tx, budgetID, txn)
	if err != nil {
		return nil, err
	}

	if err = s.validateCategory(
		txn.CategoryID,
		budget.Metadata.InflowCategoryID,
		*account,
		*payee,
		transferAccount,
		txn.Amount,
	); err != nil {
		return nil, err
	}

	// clear transfer fields in case they are set
	txn.TransferAccountID = nil
	txn.TransferTransactionID = nil

	createdTxn, err = s.repo.Create(ctx, tx, txn)
	if err != nil {
		return nil, errs.Wrap(errs.CodeTransactionCreateFailed, "failed to create transaction", err)
	}
	if len(createdTxn) == 0 {
		return nil, errs.New(errs.CodeTransactionNotCreated, "no transaction was created")
	}

	txn.ID = createdTxn[0].ID

	if err = s.applySideEffects(ctx, tx, sideEffectInput{
		budgetId: budgetID,
		oldTxn:   nil,
		newTxn:   &txn,
		budget:   budget,
		account:  account,
		payee:    payee,
	}); err != nil {
		return nil, err
	}

	// Reload to pick up any mutations from side effects (e.g., transfer linking)
	final, err := s.repo.GetByIdTx(ctx, tx, budgetID, txn.ID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeTransactionLookupFailed, "error reloading created transaction", err)
	}
	createdTxn[0] = *final

	return createdTxn, nil
}

// CreateWithTxDeduped behaves like CreateWithTx, but when a transaction with
// the same (budget_id, dedupe_hash) already exists it returns the existing row
// with created=false instead of failing — and skips budget side effects, which
// already ran when the row was first created. Used by the email pipeline so
// activity retries and duplicate Gmail pushes are idempotent.
func (s *transactionService) CreateWithTxDeduped(
	ctx context.Context,
	tx pgx.Tx,
	txn model.Transaction,
) (*model.Transaction, bool, error) {
	budgetID := utils.MustBudgetID(ctx)
	txn.BudgetID = budgetID
	if txn.Status == "" {
		txn.Status = model.TransactionStatusManual
	}

	if err := s.validateTransactionPayload(txn, budgetID); err != nil {
		return nil, false, err
	}

	budget, account, payee, transferAccount, err := s.loadDependencies(ctx, tx, budgetID, txn)
	if err != nil {
		return nil, false, err
	}

	if err = s.validateCategory(
		txn.CategoryID,
		budget.Metadata.InflowCategoryID,
		*account,
		*payee,
		transferAccount,
		txn.Amount,
	); err != nil {
		return nil, false, err
	}

	// clear transfer fields in case they are set
	txn.TransferAccountID = nil
	txn.TransferTransactionID = nil

	createdTxn, created, err := s.repo.CreateDeduped(ctx, tx, txn)
	if err != nil {
		return nil, false, errs.Wrap(errs.CodeTransactionCreateFailed, "failed to create transaction", err)
	}
	if !created {
		return createdTxn, false, nil
	}

	txn.ID = createdTxn.ID

	if err = s.applySideEffects(ctx, tx, sideEffectInput{
		budgetId: budgetID,
		oldTxn:   nil,
		newTxn:   &txn,
		budget:   budget,
		account:  account,
		payee:    payee,
	}); err != nil {
		return nil, false, err
	}

	// Reload to pick up any mutations from side effects (e.g., transfer linking)
	final, err := s.repo.GetByIdTx(ctx, tx, budgetID, txn.ID)
	if err != nil {
		return nil, false, errs.Wrap(errs.CodeTransactionLookupFailed, "error reloading created transaction", err)
	}

	return final, true, nil
}

func (s *transactionService) Update(ctx context.Context, id uuid.UUID, txn model.Transaction) error {
	// Create a shorter context for each individual transaction attempt
	txCtx, txCancel := context.WithTimeout(ctx, 30*time.Second)
	defer txCancel()

	budgetId := utils.MustBudgetID(ctx)
	logger.Logger(ctx).Info("updating transaction", "id", id)

	// Create a copy of the transaction to update
	toUpdate := txn

	if err := s.validateTransactionPayload(toUpdate, budgetId); err != nil {
		return err
	}

	s.enrichLocation(txCtx, &toUpdate)

	var learningTxn *model.Transaction
	err := withTx(txCtx, s.repo.GetDB(), func(tx pgx.Tx) error {
		foundTxn, err := s.repo.GetByIdTx(txCtx, tx, budgetId, id)
		if err != nil {
			return errs.Wrap(errs.CodeTransactionLookupFailed, "error getting transaction", err)
		}
		if foundTxn == nil {
			return errs.New(errs.CodeTransactionLookupFailed, "transaction not found for id %v", id)
		}
		if foundTxn.ID != id {
			return errs.New(errs.CodeTransactionLookupFailed, "transaction id mismatch")
		}

		toUpdate.ID = id
		toUpdate.Status = foundTxn.Status
		if foundTxn.Status == model.TransactionStatusUnapproved {
			toUpdate.Status = model.TransactionStatusApproved
		}
		same := foundTxn.Compare(&toUpdate)
		if same {
			logger.Logger(txCtx).Info("transaction is the same as the existing transaction, skipping update")
			return nil
		}

		// fetch updated txn account and payee
		budget, account, payee, transferAccount, err := s.loadDependencies(txCtx, tx, budgetId, toUpdate)
		if err != nil {
			return err
		}

		err = s.validateCategory(
			toUpdate.CategoryID,
			budget.Metadata.InflowCategoryID,
			*account,
			*payee,
			transferAccount,
			toUpdate.Amount,
		)
		if err != nil {
			return err
		}

		if err = s.applySideEffects(txCtx, tx, sideEffectInput{
			budgetId: budgetId,
			oldTxn:   foundTxn,
			newTxn:   &toUpdate,
			budget:   budget,
			account:  account,
			payee:    payee,
			queueLearning: func(txn model.Transaction) {
				learningTxn = &txn
			},
		}); err != nil {
			return err
		}

		if err = s.repo.Update(txCtx, tx, budgetId, id, toUpdate); err != nil {
			return errs.Wrap(errs.CodeTransactionUpdateFailed, "error updating transaction", err)
		}

		return nil
	})
	if err != nil {
		return err
	}
	if learningTxn != nil {
		s.learnTransactionMappingAsync(ctx, budgetId, *learningTxn)
	}

	return nil
}

// enrichLocation fills in a missing place name via reverse geocoding and
// defaults the source to manual. Best-effort: geocoding failures leave the
// coordinates as-is and never fail the write.
func (s *transactionService) enrichLocation(ctx context.Context, txn *model.Transaction) {
	if txn.LocationLat == nil || txn.LocationLng == nil {
		return
	}
	if txn.LocationSource == nil || !txn.LocationSource.Valid() {
		src := model.LocationSourceManual
		txn.LocationSource = &src
	}
	if txn.LocationName != nil && *txn.LocationName != "" {
		return
	}
	if s.geocodeService == nil {
		return
	}
	geoCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	name, err := s.geocodeService.Reverse(geoCtx, *txn.LocationLat, *txn.LocationLng)
	if err != nil {
		logger.Logger(ctx).Warn("reverse geocoding failed, keeping coordinates only", "err", err)
		return
	}
	txn.LocationName = &name
}

// UpdateLocation sets or clears just the location of a transaction. Used by the
// mobile app's push-triggered auto-tagging and by the manual pin editors.
func (s *transactionService) UpdateLocation(
	ctx context.Context,
	id uuid.UUID,
	req model.TransactionLocationReq,
) (*model.Transaction, error) {
	txCtx, txCancel := context.WithTimeout(ctx, 30*time.Second)
	defer txCancel()
	budgetId := utils.MustBudgetID(txCtx)

	clearing := req.Lat == nil && req.Lng == nil
	if !clearing && (req.Lat == nil || req.Lng == nil) {
		return nil, errs.New(errs.CodeInvalidArgument, "lat and lng must both be set or both be null")
	}
	if !clearing && (*req.Lat < -90 || *req.Lat > 90 || *req.Lng < -180 || *req.Lng > 180) {
		return nil, errs.New(errs.CodeInvalidArgument, "lat/lng out of range")
	}

	var lat, lng *float64
	var name *string
	var source *model.LocationSource
	if !clearing {
		lat, lng, name = req.Lat, req.Lng, req.Name
		src := req.Source
		if !src.Valid() {
			src = model.LocationSourceManual
		}
		source = &src

		if (name == nil || *name == "") && s.geocodeService != nil {
			geoCtx, cancel := context.WithTimeout(txCtx, 8*time.Second)
			resolved, err := s.geocodeService.Reverse(geoCtx, *lat, *lng)
			cancel()
			if err != nil {
				logger.Logger(ctx).Warn("reverse geocoding failed, keeping coordinates only", "err", err)
			} else {
				name = &resolved
			}
		}
	}

	err := withTx(txCtx, s.repo.GetDB(), func(tx pgx.Tx) error {
		return s.repo.UpdateLocation(txCtx, tx, budgetId, id, lat, lng, name, source)
	})
	if err != nil {
		return nil, errs.Wrap(errs.CodeTransactionUpdateFailed, "error updating transaction location", err)
	}

	updated, err := s.repo.GetById(txCtx, budgetId, id)
	if err != nil {
		return nil, errs.Wrap(errs.CodeTransactionLookupFailed, "error reloading transaction", err)
	}
	return updated, nil
}

func (s *transactionService) UpdateStatus(ctx context.Context, id uuid.UUID, status model.TransactionStatus) error {
	txCtx, txCancel := context.WithTimeout(ctx, 30*time.Second)
	defer txCancel()
	budgetId := utils.MustBudgetID(ctx)
	logger.Logger(txCtx).Info("updating transaction status", "id", id, "status", status)

	if status != model.TransactionStatusApproved {
		return withTx(txCtx, s.repo.GetDB(), func(tx pgx.Tx) error {
			return s.repo.UpdateStatus(txCtx, tx, budgetId, id, status)
		})
	}

	foundTxn, err := s.repo.GetById(txCtx, budgetId, id)
	if err != nil {
		return errs.Wrap(errs.CodeTransactionLookupFailed, "error getting transaction", err)
	}

	if foundTxn == nil {
		return errs.New(errs.CodeTransactionLookupFailed, "transaction not found for id %v", id)
	}
	if s.cipherPredictionRepo == nil {
		return errs.New(errs.CodeInternalError, "cipher prediction repository is not configured")
	}

	cipherPrediction, err := s.cipherPredictionRepo.GetByTransactionID(txCtx, budgetId, foundTxn.ID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return withTx(txCtx, s.repo.GetDB(), func(tx pgx.Tx) error {
				return s.repo.UpdateStatus(txCtx, tx, budgetId, id, status)
			})
		}
		return errs.Wrap(errs.CodeTransactionLookupFailed, "error getting cipher prediction", err)
	}
	if cipherPrediction == nil {
		return withTx(txCtx, s.repo.GetDB(), func(tx pgx.Tx) error {
			return s.repo.UpdateStatus(txCtx, tx, budgetId, id, status)
		})
	}

	if foundTxn.PayeeID == nil {
		return errs.New(errs.CodeInvalidArgument, "payee is required to approve transaction")
	}
	if foundTxn.CategoryID == nil {
		return errs.New(errs.CodeInvalidArgument, "category is required to approve transaction")
	}
	if foundTxn.RawBankText == nil || strings.TrimSpace(*foundTxn.RawBankText) == "" {
		return errs.New(errs.CodeInvalidArgument, "raw bank text is required to approve transaction")
	}

	if err := withTx(txCtx, s.repo.GetDB(), func(tx pgx.Tx) error {
		return s.repo.UpdateStatus(txCtx, tx, budgetId, id, status)
	}); err != nil {
		return errs.Wrap(errs.CodeTransactionUpdateFailed, "error updating transaction status", err)
	}

	s.learnTransactionMappingAsync(ctx, budgetId, *foundTxn)

	return nil
}

func (s *transactionService) DeleteById(ctx context.Context, id uuid.UUID) error {
	txCtx, txCancel := context.WithTimeout(ctx, 30*time.Second)
	defer txCancel()

	budgetId := utils.MustBudgetID(ctx)
	logger.Logger(ctx).Info("deleting transaction", "id", id)

	return withTx(txCtx, s.repo.GetDB(), func(tx pgx.Tx) error {
		foundTxn, err := s.repo.GetByIdTx(txCtx, tx, budgetId, id)
		if err != nil {
			return errs.Wrap(errs.CodeTransactionLookupFailed, "error getting transaction", err)
		}
		if foundTxn == nil {
			return errs.New(errs.CodeTransactionLookupFailed, "transaction not found for id %v", id)
		}
		logger.Logger(txCtx).Debug("found transaction for delete", "txn", foundTxn.String())

		budget, err := s.budgetRepo.GetById(txCtx, tx, budgetId)
		if err != nil {
			return errs.Wrap(errs.CodeBudgetLookupFailed, "error fetching budget", err)
		}

		if err = s.applySideEffects(txCtx, tx, sideEffectInput{
			budgetId: budgetId,
			oldTxn:   foundTxn,
			newTxn:   nil,
			budget:   budget,
		}); err != nil {
			return err
		}

		if err = s.repo.DeleteById(txCtx, tx, budgetId, id); err != nil {
			return errs.Wrap(errs.CodeTransactionDeleteFailed, "error deleting transaction", err)
		}

		return nil
	})
}
