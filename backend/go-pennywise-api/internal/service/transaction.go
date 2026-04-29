package service

import (
	"context"
	"strings"
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
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.TransactionStatus) error
	Create(ctx context.Context, txn model.Transaction) ([]model.Transaction, error)
	CreateWithTx(ctx context.Context, tx pgx.Tx, txn model.Transaction) ([]model.Transaction, error)
	DeleteById(ctx context.Context, id uuid.UUID) error
}

type transactionService struct {
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
) (budget *model.Budget, account *model.Account, payee *model.Payee, err error) {
	budget, err = s.budgetRepo.GetById(ctx, tx, budgetId)
	if err != nil {
		return nil, nil, nil, errs.Wrap(errs.CodeBudgetLookupFailed, "error fetching budget", err)
	}

	account, err = s.accountRepo.GetById(ctx, tx, budgetId, *txn.AccountID)
	if err != nil {
		return nil, nil, nil, errs.Wrap(errs.CodeAccountLookupFailed, "error getting account", err)
	}

	payee, err = s.payeeRepo.GetByIdTx(ctx, tx, budgetId, *txn.PayeeID)
	if err != nil {
		return nil, nil, nil, errs.Wrap(errs.CodePayeeLookupFailed, "error getting payee", err)
	}

	return budget, account, payee, nil
}

// validate the category of the transaction
// for budget transfers, the category should be nil
func (s *transactionService) validateCategory(
	categoryID *uuid.UUID,
	inflowCategoryID uuid.UUID,
	account model.Account,
	payee model.Payee,
	amount float64,
) error {
	// budget -> budget transfers don't have a category
	if payee.TransferAccountID != nil {
		if account.Type == "savings" || account.Type == "checking" || account.Type == "creditCard" {
			if categoryID != nil {
				return errs.New(errs.CodeInvalidArgument, "category is not allowed for budget transfers")
			}
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
		TransferAccountID:     txn.AccountID,
		TransferTransactionID: &parentId,
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
	if s.cipherClient == nil || s.payeeRuleRepo == nil || s.txnEmbeddingRepo == nil {
		logger.Logger(ctx).
			Warn("skipping transaction learning because dependencies are not configured", "txnId", txn.ID)
		return
	}
	if txn.PayeeID == nil || txn.CategoryID == nil {
		logger.Logger(ctx).Warn("skipping transaction learning because payee or category is missing", "txnId", txn.ID)
		return
	}
	if txn.RawBankText == nil || strings.TrimSpace(*txn.RawBankText) == "" {
		logger.Logger(ctx).Warn("skipping transaction learning because raw bank text is missing", "txnId", txn.ID)
		return
	}

	go func() {
		bgCtx := utils.WithRequestMetadata(context.Background(), utils.RequestMetadataFromContext(ctx))
		bgCtx = utils.WithInternalAuthToken(bgCtx, utils.InternalAuthTokenFromContext(ctx))
		bgCtx, cancel := context.WithTimeout(bgCtx, 2*time.Minute)
		defer cancel()

		generatedEmbedding, err := s.cipherClient.GenerateTransactionEmbedding(bgCtx, TransactionEmbeddingRequest{
			RawBankText: *txn.RawBankText,
			Amount:      txn.Amount,
		})
		if err != nil {
			err := errs.Wrap(errs.CodeInternalError, "error generating transaction embedding", err)
			logger.Logger(bgCtx).Error("error generating transaction embedding", "error", err)
			return
		}
		if generatedEmbedding == nil || strings.TrimSpace(generatedEmbedding.Embedding) == "" {
			err := errs.New(errs.CodeInternalError, "cipher returned empty transaction embedding")
			logger.Logger(bgCtx).Error("cipher returned empty transaction embedding", "error", err)
			return
		}

		payeeRule := model.PayeeRule{
			BudgetID:    budgetId,
			PayeeID:     *txn.PayeeID,
			CategoryID:  *txn.CategoryID,
			MatchString: generatedEmbedding.MatchString,
		}
		err = withTx(bgCtx, s.repo.GetDB(), func(tx pgx.Tx) error {
			if err = s.payeeRuleRepo.CreatePayeeRule(bgCtx, tx, payeeRule); err != nil {
				err := errs.Wrap(errs.CodeInternalError, "error creating payee rule", err)
				logger.Logger(bgCtx).Error("error creating payee rule", "error", err)
				return err
			}

			embedding := model.TransactionEmbedding{
				BudgetID:      budgetId,
				PayeeID:       *txn.PayeeID,
				CategoryID:    *txn.CategoryID,
				Amount:        txn.Amount,
				Source:        "AUTO_LEARNED",
				EmbeddingText: generatedEmbedding.EmbeddingText,
			}
			return s.txnEmbeddingRepo.Upsert(bgCtx, tx, embedding, generatedEmbedding.Embedding)
		})
		if err != nil {
			logger.Logger(bgCtx).Error("error upserting transaction embedding", "error", err)
		}
	}()
}

// applySideEffects applies all side effects (carryovers, transfers, predictions)
// for a transaction create, update, or delete in a single unified method.
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
	switch {
	case isUpdate && transactionMappingChanged(input.oldTxn, input.newTxn):
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
		if cipherPrediction == nil {
			return nil
		}

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

		if input.queueLearning != nil {
			learningTxn := *input.newTxn
			learningTxn.RawBankText = input.oldTxn.RawBankText
			input.queueLearning(learningTxn)
		}

		logger.Logger(ctx).Info("updated cipher prediction for transaction mapping change", "txnId", input.oldTxn.ID)
	}

	return nil
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
			TransferAccountID:     newTxn.AccountID,
			TransferTransactionID: &foundTxn.ID,
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
	if tx == nil {
		return s.Create(ctx, txn)
	}

	budgetID := utils.MustBudgetID(ctx)
	txn.BudgetID = budgetID

	if err := s.validateTransactionPayload(txn, budgetID); err != nil {
		return nil, err
	}

	var createdTxn []model.Transaction
	budget, account, payee, err := s.loadDependencies(ctx, tx, budgetID, txn)
	if err != nil {
		return nil, err
	}

	if err = s.validateCategory(
		txn.CategoryID,
		budget.Metadata.InflowCategoryID,
		*account,
		*payee,
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
		budget, account, payee, err := s.loadDependencies(txCtx, tx, budgetId, toUpdate)
		if err != nil {
			return err
		}

		err = s.validateCategory(
			toUpdate.CategoryID,
			budget.Metadata.InflowCategoryID,
			*account,
			*payee,
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
