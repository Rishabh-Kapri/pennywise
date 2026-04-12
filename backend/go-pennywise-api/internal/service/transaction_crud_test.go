package service

import (
	"context"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGetAll(t *testing.T) {
	mockRepo := &mockTransactionRepo{}
	service := newTestTransactionService(mockRepo, nil, nil, nil, nil, nil, nil)
	budgetId := uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budgetId)

	mockRepo.On("GetAll", ctx, budgetId, mock.Anything).Return([]model.Transaction{}, nil).Once()
	_, err := service.GetAll(ctx)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestGetAllNormalized(t *testing.T) {
	mockRepo := &mockTransactionRepo{}
	service := newTestTransactionService(mockRepo, nil, nil, nil, nil, nil, nil)
	budgetId := uuid.New()
	accountId := uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budgetId)

	mockRepo.On("GetAllNormalized", ctx, budgetId, &accountId).Return([]model.Transaction{}, nil).Once()
	_, err := service.GetAllNormalized(ctx, &accountId)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestLoadDependencies(t *testing.T) {
	ctx := context.Background()
	var mockTx pgx.Tx
	budgetId := uuid.New()

	accountId := uuid.New()
	payeeId := uuid.New()
	txn := model.Transaction{
		AccountID: &accountId,
		PayeeID:   &payeeId,
	}

	t.Run("success", func(t *testing.T) {
		mockBudget := &mockBudgetRepo{}
		mockAccount := &mockAccountRepo{}
		mockPayee := &mockPayeesRepo{}
		service := newTestTransactionService(nil, mockBudget, nil, mockAccount, mockPayee, nil, nil)

		mockBudget.On("GetById", ctx, mockTx, budgetId).Return(&model.Budget{}, nil).Once()
		mockAccount.On("GetById", ctx, mockTx, budgetId, accountId).Return(&model.Account{}, nil).Once()
		mockPayee.On("GetByIdTx", ctx, mockTx, budgetId, payeeId).Return(&model.Payee{}, nil).Once()

		b, a, p, err := service.loadDependencies(ctx, mockTx, budgetId, txn)
		assert.NoError(t, err)
		assert.NotNil(t, b)
		assert.NotNil(t, a)
		assert.NotNil(t, p)
	})

	t.Run("budget_error", func(t *testing.T) {
		mockBudget := &mockBudgetRepo{}
		service := newTestTransactionService(nil, mockBudget, nil, nil, nil, nil, nil)
		mockBudget.On("GetById", ctx, mockTx, budgetId).Return(nil, assert.AnError).Once()

		_, _, _, err := service.loadDependencies(ctx, mockTx, budgetId, txn)
		assert.Error(t, err)
	})

	t.Run("account_error", func(t *testing.T) {
		mockBudget := &mockBudgetRepo{}
		mockAccount := &mockAccountRepo{}
		service := newTestTransactionService(nil, mockBudget, nil, mockAccount, nil, nil, nil)
		mockBudget.On("GetById", ctx, mockTx, budgetId).Return(&model.Budget{}, nil).Once()
		mockAccount.On("GetById", ctx, mockTx, budgetId, accountId).Return(nil, assert.AnError).Once()

		_, _, _, err := service.loadDependencies(ctx, mockTx, budgetId, txn)
		assert.Error(t, err)
	})

	t.Run("payee_error", func(t *testing.T) {
		mockBudget := &mockBudgetRepo{}
		mockAccount := &mockAccountRepo{}
		mockPayee := &mockPayeesRepo{}
		service := newTestTransactionService(nil, mockBudget, nil, mockAccount, mockPayee, nil, nil)
		mockBudget.On("GetById", ctx, mockTx, budgetId).Return(&model.Budget{}, nil).Once()
		mockAccount.On("GetById", ctx, mockTx, budgetId, accountId).Return(&model.Account{}, nil).Once()
		mockPayee.On("GetByIdTx", ctx, mockTx, budgetId, payeeId).Return(nil, assert.AnError).Once()

		_, _, _, err := service.loadDependencies(ctx, mockTx, budgetId, txn)
		assert.Error(t, err)
	})
}

// Override withTx for tests
func mockWithTxSuccess(mockTx pgx.Tx) {
	withTx = func(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
		return fn(mockTx)
	}
}

func mockWithTxError() {
	withTx = func(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
		return assert.AnError
	}
}

func TestCreate(t *testing.T) {
	var mockTx pgx.Tx
	mockWithTxSuccess(mockTx)
	defer func() { withTx = utils.WithTx }()

	budgetId := uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budgetId)

	accountId := uuid.New()
	payeeId := uuid.New()
	txnId := uuid.New()
	validTxn := model.Transaction{
		AccountID: &accountId,
		PayeeID:   &payeeId,
		Amount:    10.0,
		Date:      "2023-01-01",
	}

	t.Run("validation_error", func(t *testing.T) {
		service := newTestTransactionService(nil, nil, nil, nil, nil, nil, nil)
		invalidTxn := model.Transaction{} // no account or payee
		_, err := service.Create(ctx, invalidTxn)
		assert.Error(t, err)
	})

	t.Run("load_dependencies_error", func(t *testing.T) {
		mockBudget := &mockBudgetRepo{}
		service := newTestTransactionService(nil, mockBudget, nil, nil, nil, nil, nil)
		mockBudget.On("GetById", mock.Anything, mockTx, budgetId).Return(nil, assert.AnError).Once()

		_, err := service.Create(ctx, validTxn)
		assert.Error(t, err)
	})

	t.Run("create_repo_error", func(t *testing.T) {
		mockBudget := &mockBudgetRepo{}
		mockAccount := &mockAccountRepo{}
		mockPayee := &mockPayeesRepo{}
		mockRepo := &mockTransactionRepo{}
		service := newTestTransactionService(mockRepo, mockBudget, nil, mockAccount, mockPayee, nil, nil)

		mockBudget.On("GetById", mock.Anything, mockTx, budgetId).Return(&model.Budget{}, nil).Once()
		mockAccount.On("GetById", mock.Anything, mockTx, budgetId, accountId).Return(&model.Account{Type: "checking"}, nil).Once()
		mockPayee.On("GetByIdTx", mock.Anything, mockTx, budgetId, payeeId).Return(&model.Payee{}, nil).Once()
		mockRepo.On("Create", mock.Anything, mockTx, mock.Anything).Return(nil, assert.AnError).Once()

		_, err := service.Create(ctx, validTxn)
		assert.Error(t, err)
	})

	t.Run("create_repo_empty_return", func(t *testing.T) {
		mockBudget := &mockBudgetRepo{}
		mockAccount := &mockAccountRepo{}
		mockPayee := &mockPayeesRepo{}
		mockRepo := &mockTransactionRepo{}
		service := newTestTransactionService(mockRepo, mockBudget, nil, mockAccount, mockPayee, nil, nil)

		mockBudget.On("GetById", mock.Anything, mockTx, budgetId).Return(&model.Budget{}, nil).Once()
		mockAccount.On("GetById", mock.Anything, mockTx, budgetId, accountId).Return(&model.Account{Type: "checking"}, nil).Once()
		mockPayee.On("GetByIdTx", mock.Anything, mockTx, budgetId, payeeId).Return(&model.Payee{}, nil).Once()
		mockRepo.On("Create", mock.Anything, mockTx, mock.Anything).Return([]model.Transaction{}, nil).Once()

		_, err := service.Create(ctx, validTxn)
		assert.Error(t, err)
		var appErr *errs.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, errs.CodeTransactionNotCreated, appErr.Code)
	})

	t.Run("success_no_transfer", func(t *testing.T) {
		mockBudget := &mockBudgetRepo{}
		mockAccount := &mockAccountRepo{}
		mockPayee := &mockPayeesRepo{}
		mockRepo := &mockTransactionRepo{}
		service := newTestTransactionService(mockRepo, mockBudget, nil, mockAccount, mockPayee, nil, nil)

		mockBudget.On("GetById", mock.Anything, mockTx, budgetId).Return(&model.Budget{}, nil).Once()
		mockAccount.On("GetById", mock.Anything, mockTx, budgetId, accountId).Return(&model.Account{Type: "checking"}, nil).Once()
		mockPayee.On("GetByIdTx", mock.Anything, mockTx, budgetId, payeeId).Return(&model.Payee{}, nil).Once()
		mockRepo.On("Create", mock.Anything, mockTx, mock.Anything).Return([]model.Transaction{{ID: txnId}}, nil).Once()
		mockRepo.On("GetByIdTx", mock.Anything, mockTx, budgetId, txnId).Return(&model.Transaction{ID: txnId}, nil).Once()

		res, err := service.Create(ctx, validTxn)
		assert.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, txnId, res[0].ID)
	})

	t.Run("success_with_transfer", func(t *testing.T) {
		mockBudget := &mockBudgetRepo{}
		mockAccount := &mockAccountRepo{}
		mockPayee := &mockPayeesRepo{}
		mockRepo := &mockTransactionRepo{}
		service := newTestTransactionService(mockRepo, mockBudget, nil, mockAccount, mockPayee, nil, nil)

		transferAccountId := uuid.New()

		mockBudget.On("GetById", mock.Anything, mockTx, budgetId).Return(&model.Budget{}, nil).Once()
		mockAccount.On("GetById", mock.Anything, mockTx, budgetId, accountId).Return(&model.Account{Type: "checking"}, nil).Once()
		mockPayee.On("GetByIdTx", mock.Anything, mockTx, budgetId, payeeId).Return(&model.Payee{TransferAccountID: &transferAccountId}, nil).Once()

		counterpartId := uuid.New()
		// Main transaction insertion
		mockRepo.On("Create", mock.Anything, mockTx, mock.MatchedBy(func(tx model.Transaction) bool { return tx.Amount == 10.0 })).Return([]model.Transaction{{ID: txnId}}, nil).Once()
		// Counterpart transaction insertion
		mockRepo.On("Create", mock.Anything, mockTx, mock.MatchedBy(func(tx model.Transaction) bool { return tx.Amount == -10.0 })).Return([]model.Transaction{{ID: counterpartId}}, nil).Once()

		// Update original with counterpart link
		mockRepo.On("Update", mock.Anything, mockTx, budgetId, txnId, mock.Anything).Return(nil).Once()
		mockRepo.On("GetByIdTx", mock.Anything, mockTx, budgetId, txnId).Return(&model.Transaction{ID: txnId}, nil).Once()

		res, err := service.Create(ctx, validTxn)
		assert.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, txnId, res[0].ID)
	})

	t.Run("withtx_returns_error", func(t *testing.T) {
		mockWithTxError()
		service := newTestTransactionService(nil, nil, nil, nil, nil, nil, nil)
		_, err := service.Create(ctx, validTxn)
		assert.Error(t, err)
		mockWithTxSuccess(mockTx) // reset
	})
}

func TestUpdate(t *testing.T) {
	var mockTx pgx.Tx
	mockWithTxSuccess(mockTx)
	defer func() { withTx = utils.WithTx }()

	budgetId := uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budgetId)

	accountId := uuid.New()
	payeeId := uuid.New()
	txnId := uuid.New()
	validTxn := model.Transaction{
		AccountID: &accountId,
		PayeeID:   &payeeId,
		Amount:    10.0,
		Date:      "2023-01-01",
	}

	t.Run("validation_error", func(t *testing.T) {
		service := newTestTransactionService(nil, nil, nil, nil, nil, nil, nil)
		invalidTxn := model.Transaction{}
		err := service.Update(ctx, txnId, invalidTxn)
		assert.Error(t, err)
	})

	t.Run("get_by_id_error", func(t *testing.T) {
		mockRepo := &mockTransactionRepo{}
		service := newTestTransactionService(mockRepo, nil, nil, nil, nil, nil, nil)
		mockRepo.On("GetByIdTx", mock.Anything, mockTx, budgetId, txnId).Return(nil, assert.AnError).Once()

		err := service.Update(ctx, txnId, validTxn)
		assert.Error(t, err)
	})

	t.Run("get_by_id_not_found", func(t *testing.T) {
		mockRepo := &mockTransactionRepo{}
		service := newTestTransactionService(mockRepo, nil, nil, nil, nil, nil, nil)
		mockRepo.On("GetByIdTx", mock.Anything, mockTx, budgetId, txnId).Return(nil, nil).Once()

		err := service.Update(ctx, txnId, validTxn)
		assert.Error(t, err)
	})

	t.Run("same_transaction", func(t *testing.T) {
		mockRepo := &mockTransactionRepo{}
		service := newTestTransactionService(mockRepo, nil, nil, nil, nil, nil, nil)

		existingTxn := validTxn
		existingTxn.ID = txnId

		mockRepo.On("GetByIdTx", mock.Anything, mockTx, budgetId, txnId).Return(&existingTxn, nil).Once()

		err := service.Update(ctx, txnId, validTxn)
		assert.NoError(t, err)
	})

	t.Run("reconcile_transfer_error_propagates", func(t *testing.T) {
		// As previously discovered, reconcileTransfer error is no longer ignored!
		mockRepo := &mockTransactionRepo{}
		mockBudget := &mockBudgetRepo{}
		mockAccount := &mockAccountRepo{}
		mockPayee := &mockPayeesRepo{}
		mockMB := &mockMonthlyBudgetRepo{}
		service := newTestTransactionService(mockRepo, mockBudget, nil, mockAccount, mockPayee, nil, mockMB)

		existingTxn := validTxn
		existingTxn.ID = txnId
		existingTxn.Amount = 5.0 // Different amount to avoid same check

		mockRepo.On("GetByIdTx", mock.Anything, mockTx, budgetId, txnId).Return(&existingTxn, nil).Once()

		mockBudget.On("GetById", mock.Anything, mockTx, budgetId).Return(&model.Budget{}, nil).Once()
		mockAccount.On("GetById", mock.Anything, mockTx, budgetId, accountId).Return(&model.Account{Type: "checking"}, nil).Once()

		transferAccountId := uuid.New()
		mockPayee.On("GetByIdTx", mock.Anything, mockTx, budgetId, payeeId).Return(&model.Payee{TransferAccountID: &transferAccountId}, nil).Once()

		// MB Service dependencies ignored because categoryID is nil

		// To cause reconcileTransfer to fail, make createCounterpartTxn fail
		mockRepo.On("Create", mock.Anything, mockTx, mock.Anything).Return(nil, assert.AnError).Once()

		err := service.Update(ctx, txnId, validTxn)
		assert.Error(t, err) // It fails and propagates!
	})

	t.Run("prediction_update_fails", func(t *testing.T) {
		mockRepo := &mockTransactionRepo{}
		mockBudget := &mockBudgetRepo{}
		mockAccount := &mockAccountRepo{}
		mockPayee := &mockPayeesRepo{}
		mockPrediction := &mockPredictionRepo{}
		service := newTestTransactionService(mockRepo, mockBudget, mockPrediction, mockAccount, mockPayee, nil, nil)

		existingTxn := validTxn
		existingTxn.ID = txnId
		existingTxn.Amount = 5.0
		existingTxn.Source = "MLP" // Trigger prediction update

		mockRepo.On("GetByIdTx", mock.Anything, mockTx, budgetId, txnId).Return(&existingTxn, nil).Once()
		mockBudget.On("GetById", mock.Anything, mockTx, budgetId).Return(&model.Budget{}, nil).Once()
		mockAccount.On("GetById", mock.Anything, mockTx, budgetId, accountId).Return(&model.Account{Type: "checking"}, nil).Once()
		mockPayee.On("GetByIdTx", mock.Anything, mockTx, budgetId, payeeId).Return(&model.Payee{}, nil).Once()

		// Prediction mock returns error
		mockPrediction.On("GetByTxnIdTx", mock.Anything, mockTx, budgetId, txnId).Return(nil, assert.AnError).Once()

		err := service.Update(ctx, txnId, validTxn)
		assert.Error(t, err)
	})

	t.Run("repo_update_fails", func(t *testing.T) {
		mockRepo := &mockTransactionRepo{}
		mockBudget := &mockBudgetRepo{}
		mockAccount := &mockAccountRepo{}
		mockPayee := &mockPayeesRepo{}
		service := newTestTransactionService(mockRepo, mockBudget, nil, mockAccount, mockPayee, nil, nil)

		existingTxn := validTxn
		existingTxn.ID = txnId
		existingTxn.Amount = 5.0

		mockRepo.On("GetByIdTx", mock.Anything, mockTx, budgetId, txnId).Return(&existingTxn, nil).Once()
		mockBudget.On("GetById", mock.Anything, mockTx, budgetId).Return(&model.Budget{}, nil).Once()
		mockAccount.On("GetById", mock.Anything, mockTx, budgetId, accountId).Return(&model.Account{Type: "checking"}, nil).Once()
		mockPayee.On("GetByIdTx", mock.Anything, mockTx, budgetId, payeeId).Return(&model.Payee{}, nil).Once()

		mockRepo.On("Update", mock.Anything, mockTx, budgetId, txnId, mock.Anything).Return(assert.AnError).Once()

		err := service.Update(ctx, txnId, validTxn)
		assert.Error(t, err)
	})
}

func TestDeleteById(t *testing.T) {
	var mockTx pgx.Tx
	mockWithTxSuccess(mockTx)
	defer func() { withTx = utils.WithTx }()

	budgetId := uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budgetId)
	txnId := uuid.New()

	t.Run("get_by_id_error", func(t *testing.T) {
		mockRepo := &mockTransactionRepo{}
		service := newTestTransactionService(mockRepo, nil, nil, nil, nil, nil, nil)
		mockRepo.On("GetByIdTx", mock.Anything, mockTx, budgetId, txnId).Return(nil, assert.AnError).Once()

		err := service.DeleteById(ctx, txnId)
		assert.Error(t, err)
	})

	t.Run("get_by_id_not_found", func(t *testing.T) {
		mockRepo := &mockTransactionRepo{}
		service := newTestTransactionService(mockRepo, nil, nil, nil, nil, nil, nil)
		mockRepo.On("GetByIdTx", mock.Anything, mockTx, budgetId, txnId).Return(nil, nil).Once()

		err := service.DeleteById(ctx, txnId)
		assert.Error(t, err)
	})

	t.Run("success_with_transfer_and_prediction", func(t *testing.T) {
		mockRepo := &mockTransactionRepo{}
		mockPrediction := &mockPredictionRepo{}
		mockBudget := &mockBudgetRepo{}
		service := newTestTransactionService(mockRepo, mockBudget, mockPrediction, nil, nil, nil, nil)

		mockBudget.On("GetById", mock.Anything, mockTx, budgetId).Return(&model.Budget{}, nil).Once()

		transferTxnId := uuid.New()
		foundTxn := model.Transaction{
			ID:                    txnId,
			TransferTransactionID: &transferTxnId,
			Source:                "MLP",
		}

		mockRepo.On("GetByIdTx", mock.Anything, mockTx, budgetId, txnId).Return(&foundTxn, nil).Once()
		mockRepo.On("DeleteById", mock.Anything, mockTx, budgetId, transferTxnId).Return(nil).Once()

		prediction := model.Prediction{ID: uuid.New()}
		mockPrediction.On("GetByTxnIdTx", mock.Anything, mockTx, budgetId, txnId).Return(&prediction, nil).Once()
		mockPrediction.On("DeleteByTxnId", mock.Anything, mockTx, budgetId, txnId).Return(nil).Once()

		// Delete main transaction
		mockRepo.On("DeleteById", mock.Anything, mockTx, budgetId, txnId).Return(nil).Once()

		err := service.DeleteById(ctx, txnId)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
		mockPrediction.AssertExpectations(t)
	})

	t.Run("prediction_deletion_fails", func(t *testing.T) {
		mockRepo := &mockTransactionRepo{}
		mockPrediction := &mockPredictionRepo{}
		mockBudget := &mockBudgetRepo{}
		service := newTestTransactionService(mockRepo, mockBudget, mockPrediction, nil, nil, nil, nil)

		mockBudget.On("GetById", mock.Anything, mockTx, budgetId).Return(&model.Budget{}, nil).Once()

		foundTxn := model.Transaction{ID: txnId, Source: "MLP"}

		mockRepo.On("GetByIdTx", mock.Anything, mockTx, budgetId, txnId).Return(&foundTxn, nil).Once()

		prediction := model.Prediction{ID: uuid.New()}
		mockPrediction.On("GetByTxnIdTx", mock.Anything, mockTx, budgetId, txnId).Return(&prediction, nil).Once()
		mockPrediction.On("DeleteByTxnId", mock.Anything, mockTx, budgetId, txnId).Return(assert.AnError).Once()

		err := service.DeleteById(ctx, txnId)
		assert.Error(t, err)
	})

	t.Run("repo_deletion_fails", func(t *testing.T) {
		mockRepo := &mockTransactionRepo{}
		mockPrediction := &mockPredictionRepo{}
		mockBudget := &mockBudgetRepo{}
		service := newTestTransactionService(mockRepo, mockBudget, mockPrediction, nil, nil, nil, nil)

		mockBudget.On("GetById", mock.Anything, mockTx, budgetId).Return(&model.Budget{}, nil).Once()

		foundTxn := model.Transaction{ID: txnId, Source: "MLP"}

		mockRepo.On("GetByIdTx", mock.Anything, mockTx, budgetId, txnId).Return(&foundTxn, nil).Once()

		mockPrediction.On("GetByTxnIdTx", mock.Anything, mockTx, budgetId, txnId).Return(nil, nil).Once()

		mockRepo.On("DeleteById", mock.Anything, mockTx, budgetId, txnId).Return(assert.AnError).Once()

		err := service.DeleteById(ctx, txnId)
		assert.Error(t, err)
	})
}
