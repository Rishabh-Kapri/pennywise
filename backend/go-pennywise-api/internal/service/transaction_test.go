package service

import (
	"context"
	"testing"
	"time"

	"pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock repositories
type mockTransactionRepo struct {
	mock.Mock
}

// Create implements repository.TransactionRepository.
func (m *mockTransactionRepo) Create(ctx context.Context, tx pgx.Tx, txn model.Transaction) ([]model.Transaction, error) {
	args := m.Called(ctx, tx, txn)
	return args.Get(0).([]model.Transaction), args.Error(1)
}

// DeleteById implements repository.TransactionRepository.
func (m *mockTransactionRepo) DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error {
	panic("unimplemented")
}

// GetAll implements repository.TransactionRepository.
func (m *mockTransactionRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Transaction, error) {
	panic("unimplemented")
}

// GetAllNormalized implements repository.TransactionRepository.
func (m *mockTransactionRepo) GetAllNormalized(ctx context.Context, budgetId uuid.UUID, accountId *uuid.UUID) ([]model.Transaction, error) {
	panic("unimplemented")
}

// GetById implements repository.TransactionRepository.
func (m *mockTransactionRepo) GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Transaction, error) {
	panic("unimplemented")
}

// GetByIdTx implements repository.TransactionRepository.
func (m *mockTransactionRepo) GetByIdTx(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, id uuid.UUID) (*model.Transaction, error) {
	panic("unimplemented")
}

// GetPgxTx implements repository.TransactionRepository.
func (m *mockTransactionRepo) GetPgxTx(ctx context.Context) (pgx.Tx, error) {
	panic("unimplemented")
}

// Update implements repository.TransactionRepository.
func (m *mockTransactionRepo) Update(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, id uuid.UUID, txn model.Transaction) error {
	panic("unimplemented")
}

type mockPredictionRepo struct{ mock.Mock }

// Create implements repository.PredictionRepository.
func (m *mockPredictionRepo) Create(ctx context.Context, prediction model.Prediction) ([]model.Prediction, error) {
	panic("unimplemented")
}

// DeleteById implements repository.PredictionRepository.
func (m *mockPredictionRepo) DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error {
	panic("unimplemented")
}

// GetAll implements repository.PredictionRepository.
func (m *mockPredictionRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Prediction, error) {
	panic("unimplemented")
}

// GetByTxnId implements repository.PredictionRepository.
func (m *mockPredictionRepo) GetByTxnId(ctx context.Context, budgetId uuid.UUID, txnId uuid.UUID) (*model.Prediction, error) {
	panic("unimplemented")
}

// Correct interface for PredictionRepo.GetByTxnIdTx
func (m *mockPredictionRepo) GetByTxnIdTx(ctx context.Context, tx pgx.Tx, budgetId, txnId uuid.UUID) (*model.Prediction, error) {
	args := m.Called(ctx, tx, budgetId, txnId)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.Prediction), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPredictionRepo) Update(ctx context.Context, tx pgx.Tx, budgetId, predictionId uuid.UUID, prediction model.Prediction) error {
	args := m.Called(ctx, tx, budgetId, predictionId, prediction)
	return args.Error(0)
}

type mockAccountRepo struct {
	mock.Mock
}

// Create implements repository.AccountRepository.
func (m *mockAccountRepo) Create(ctx context.Context, account model.Account) (*model.Account, error) {
	panic("unimplemented")
}

// GetAll implements repository.AccountRepository.
func (m *mockAccountRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Account, error) {
	panic("unimplemented")
}

// GetById implements repository.AccountRepository.
func (m *mockAccountRepo) GetById(ctx context.Context, budgetId uuid.UUID, accountId uuid.UUID) (*model.Account, error) {
	panic("unimplemented")
}

// AccountRepo mock methods
func (m *mockAccountRepo) GetByIdTx(ctx context.Context, tx pgx.Tx, budgetId, accountId uuid.UUID) (*model.Account, error) {
	args := m.Called(ctx, tx, budgetId, accountId)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.Account), args.Error(1)
	}
	return nil, args.Error(1)
}

// Search implements repository.AccountRepository.
func (m *mockAccountRepo) Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Account, error) {
	panic("unimplemented")
}

type mockPayeesRepo struct {
	mock.Mock
}

// Create implements repository.PayeesRepository.
func (m *mockPayeesRepo) Create(ctx context.Context, payee model.Payee) error {
	panic("unimplemented")
}

// DeleteById implements repository.PayeesRepository.
func (m *mockPayeesRepo) DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error {
	panic("unimplemented")
}

// GetAll implements repository.PayeesRepository.
func (m *mockPayeesRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Payee, error) {
	panic("unimplemented")
}

// GetById implements repository.PayeesRepository.
func (m *mockPayeesRepo) GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Payee, error) {
	panic("unimplemented")
}

// PayeesRepo mock methods
func (m *mockPayeesRepo) GetByIdTx(ctx context.Context, tx pgx.Tx, budgetId, payeeId uuid.UUID) (*model.Payee, error) {
	args := m.Called(ctx, tx, budgetId, payeeId)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.Payee), args.Error(1)
	}
	return nil, args.Error(1)
}

// Search implements repository.PayeesRepository.
func (m *mockPayeesRepo) Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Payee, error) {
	panic("unimplemented")
}

// Update implements repository.PayeesRepository.
func (m *mockPayeesRepo) Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, payee model.Payee) error {
	panic("unimplemented")
}

type mockCategoryRepo struct {
	mock.Mock
}

// Create implements repository.CategoryRepository.
func (m *mockCategoryRepo) Create(ctx context.Context, category model.Category) error {
	panic("unimplemented")
}

// DeleteById implements repository.CategoryRepository.
func (m *mockCategoryRepo) DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error {
	panic("unimplemented")
}

// GetAll implements repository.CategoryRepository.
func (m *mockCategoryRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Category, error) {
	panic("unimplemented")
}

// GetById implements repository.CategoryRepository.
func (m *mockCategoryRepo) GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Category, error) {
	panic("unimplemented")
}

// GetByIdSimplified implements repository.CategoryRepository.
func (m *mockCategoryRepo) GetByIdSimplified(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Category, error) {
	panic("unimplemented")
}

// CategoryRepo mock methods
func (m *mockCategoryRepo) GetByIdSimplifiedTx(ctx context.Context, tx pgx.Tx, budgetId, categoryId uuid.UUID) (*model.Category, error) {
	args := m.Called(ctx, tx, budgetId, categoryId)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.Category), args.Error(1)
	}
	return nil, args.Error(1)
}

// Search implements repository.CategoryRepository.
func (m *mockCategoryRepo) Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Category, error) {
	panic("unimplemented")
}

// Update implements repository.CategoryRepository.
func (m *mockCategoryRepo) Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, category model.Category) error {
	panic("unimplemented")
}

type mockMonthlyBudgetRepo struct {
	mock.Mock
}

// Create implements repository.MonthlyBudgetRepository.
func (m *mockMonthlyBudgetRepo) Create(ctx context.Context, budgetId uuid.UUID, monthlyBudget model.MonthlyBudget) error {
	panic("unimplemented")
}

// GetByCatIdAndMonth implements repository.MonthlyBudgetRepository.
func (m *mockMonthlyBudgetRepo) GetByCatIdAndMonth(ctx context.Context, budgetId uuid.UUID, categoryId uuid.UUID, month string) (*model.MonthlyBudget, error) {
	panic("unimplemented")
}

// GetPgxTx implements repository.MonthlyBudgetRepository.
func (m *mockMonthlyBudgetRepo) GetPgxTx(ctx context.Context) (pgx.Tx, error) {
	panic("unimplemented")
}

// UpdateBudgetedByCatIdAndMonth implements repository.MonthlyBudgetRepository.
func (m *mockMonthlyBudgetRepo) UpdateBudgetedByCatIdAndMonth(ctx context.Context, budgetId uuid.UUID, categoryId uuid.UUID, month string, newBudgeted float64) error {
	panic("unimplemented")
}

// UpdateCarryoverByCatIdAndMonth implements repository.MonthlyBudgetRepository.
func (m *mockMonthlyBudgetRepo) UpdateCarryoverByCatIdAndMonth(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, categoryId uuid.UUID, month string, amount float64) error {
	panic("unimplemented")
}

// Mock transaction interface to expose private methods for testing
type testableTransactionService struct {
	service transactionService
}

// Test helpers
func createTestUUIDs() (budgetId, txnId, accountId, payeeId, categoryId, predictionId uuid.UUID) {
	budgetId = uuid.New()
	txnId = uuid.New()
	accountId = uuid.New()
	payeeId = uuid.New()
	categoryId = uuid.New()
	predictionId = uuid.New()
	return
}

func createTestPrediction(id, budgetId, txnId uuid.UUID) *model.Prediction {
	account := "Test Account"
	payee := "Test Payee"
	category := "Test Category"
	hasUserCorrected := false

	return &model.Prediction{
		ID:               id,
		BudgetID:         budgetId,
		TransactionID:    txnId,
		Account:          &account,
		Payee:            &payee,
		Category:         &category,
		HasUserCorrected: &hasUserCorrected,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

func createTestAccount(id uuid.UUID, name string) *model.Account {
	return &model.Account{
		ID:        id,
		Name:      name,
		BudgetID:  uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func createTestPayee(id uuid.UUID, name string) *model.Payee {
	return &model.Payee{
		ID:        id,
		Name:      name,
		BudgetID:  uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func createTestCategory(id uuid.UUID, name string) *model.Category {
	return &model.Category{
		ID:              id,
		Name:            name,
		BudgetID:        uuid.New(),
		CategoryGroupID: uuid.New(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

func setupTransactionTestableService(mockPrediction *mockPredictionRepo, mockAccount *mockAccountRepo, mockPayees *mockPayeesRepo, mockCategory *mockCategoryRepo) *transactionService {
	mockTransaction := &mockTransactionRepo{}
	mockMonthlyBudget := &mockMonthlyBudgetRepo{}

	service := NewTransactionService(
		mockTransaction,
		mockPrediction,
		mockAccount,
		mockPayees,
		mockCategory,
		mockMonthlyBudget,
	)

	return service.(*transactionService)
}

func TestUpdatePrediction_EdgeCases(t *testing.T) {
	t.Run("nil_account_id", func(t *testing.T) {
		// Test case where AccountID is nil
		mockPrediction := &mockPredictionRepo{}
		mockAccount := &mockAccountRepo{}
		mockPayees := &mockPayeesRepo{}
		mockCategory := &mockCategoryRepo{}

		service := setupTransactionTestableService(mockPrediction, mockAccount, mockPayees, mockCategory)

		budgetId, txnId, _, _, _, predictionId := createTestUUIDs()
		prediction := createTestPrediction(predictionId, budgetId, txnId)

		txn := model.Transaction{
			AccountID: nil, // This should cause the test to fail or handle gracefully
		}

		mockPrediction.On("GetByTxnIdTx", mock.Anything, mock.Anything, budgetId, txnId).Return(prediction, nil)

		ctx := context.Background()
		var mockTx pgx.Tx

		err := service.updatePrediction(ctx, mockTx, budgetId, txnId, txn)

		// This should either handle the nil gracefully or return an error
		// Adjust the assertion based on your expected behavior
		assert.Error(t, err) // or assert.NoError(t, err) if it should handle gracefully

		mockPrediction.AssertExpectations(t)
	})

	t.Run("nil_payee_id", func(t *testing.T) {
		// Similar test for nil PayeeID
		// Implementation similar to above
	})

	t.Run("nil_category_id", func(t *testing.T) {
		// Similar test for nil CategoryID
		// Implementation similar to above
	})
}

func TestUpdatePrediction(t *testing.T) {
	tests := []struct {
		name              string
		setupMocks        func(*mockPredictionRepo, *mockAccountRepo, *mockPayeesRepo, *mockCategoryRepo)
		expectError       bool
		expectUpdate      bool
		predictionExists  bool
		userCorrectedData bool
	}{
		{
			name:             "prediction_not_found_returns_nil",
			predictionExists: false,
			expectError:      false,
			expectUpdate:     false,
			setupMocks: func(mp *mockPredictionRepo, ma *mockAccountRepo, mpy *mockPayeesRepo, mc *mockCategoryRepo) {
				mp.On("GetByTxnIdTx", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
			},
		},
		{
			name:             "prediction_found_no_changes_needed",
			predictionExists: true,
			expectError:      false,
			expectUpdate:     false,
			setupMocks: func(mp *mockPredictionRepo, ma *mockAccountRepo, mpy *mockPayeesRepo, mc *mockCategoryRepo) {
				budgetId, txnId, accountId, payeeId, categoryId, predictionId := createTestUUIDs()

				prediction := createTestPrediction(predictionId, budgetId, txnId)
				account := createTestAccount(accountId, "Test Account")
				payee := createTestPayee(payeeId, "Test Payee")
				category := createTestCategory(categoryId, "Test Category")

				mp.On("GetByTxnIdTx", mock.Anything, mock.Anything, budgetId, txnId).Return(prediction, nil)
				ma.On("GetByIdTx", mock.Anything, mock.Anything, budgetId, accountId).Return(account, nil)
				mpy.On("GetByIdTx", mock.Anything, mock.Anything, budgetId, payeeId).Return(payee, nil)
				mc.On("GetByIdSimplifiedTx", mock.Anything, mock.Anything, budgetId, categoryId).Return(category, nil)
			},
		},
		{
			name:              "prediction_found_user_correction_needed",
			predictionExists:  true,
			expectError:       false,
			expectUpdate:      true,
			userCorrectedData: true,
			setupMocks: func(mp *mockPredictionRepo, ma *mockAccountRepo, mpy *mockPayeesRepo, mc *mockCategoryRepo) {
				budgetId, txnId, accountId, payeeId, categoryId, predictionId := createTestUUIDs()

				prediction := createTestPrediction(predictionId, budgetId, txnId)
				account := createTestAccount(accountId, "Different Account")
				payee := createTestPayee(payeeId, "Different Payee")
				category := createTestCategory(categoryId, "Different Category")

				mp.On("GetByTxnIdTx", mock.Anything, mock.Anything, budgetId, txnId).Return(prediction, nil)
				ma.On("GetByIdTx", mock.Anything, mock.Anything, budgetId, accountId).Return(account, nil)
				mpy.On("GetByIdTx", mock.Anything, mock.Anything, budgetId, payeeId).Return(payee, nil)
				mc.On("GetByIdSimplifiedTx", mock.Anything, mock.Anything, budgetId, categoryId).Return(category, nil)
				mp.On("Update", mock.Anything, mock.Anything, budgetId, predictionId, mock.MatchedBy(func(p model.Prediction) bool {
					return p.HasUserCorrected != nil && *p.HasUserCorrected &&
						p.UserCorrectedAccount != nil && *p.UserCorrectedAccount == "Different Account" &&
						p.UserCorrectedPayee != nil && *p.UserCorrectedPayee == "Different Payee" &&
						p.UserCorrectedCategory != nil && *p.UserCorrectedCategory == "Different Category"
				})).Return(nil)
			},
		},
		{
			name:             "prediction_repo_error",
			predictionExists: false,
			expectError:      true,
			expectUpdate:     false,
			setupMocks: func(mp *mockPredictionRepo, ma *mockAccountRepo, mpy *mockPayeesRepo, mc *mockCategoryRepo) {
				mp.On("GetByTxnIdTx", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
			},
		},
		{
			name:             "account_repo_error",
			predictionExists: true,
			expectError:      true,
			expectUpdate:     false,
			setupMocks: func(mp *mockPredictionRepo, ma *mockAccountRepo, mpy *mockPayeesRepo, mc *mockCategoryRepo) {
				budgetId, txnId, _, _, _, predictionId := createTestUUIDs()
				prediction := createTestPrediction(predictionId, budgetId, txnId)

				mp.On("GetByTxnIdTx", mock.Anything, mock.Anything, budgetId, txnId).Return(prediction, nil)
				ma.On("GetByIdTx", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockPrediction := &mockPredictionRepo{}
			mockAccount := &mockAccountRepo{}
			mockPayees := &mockPayeesRepo{}
			mockCategory := &mockCategoryRepo{}

			service := setupTransactionTestableService(mockPrediction, mockAccount, mockPayees, mockCategory)

			budgetId, txnId, accountId, payeeId, categoryId, _ := createTestUUIDs()

			txn := model.Transaction{
				AccountID:  &accountId,
				PayeeID:    &payeeId,
				CategoryID: &categoryId,
			}

			ctx := context.Background()
			var mockTx pgx.Tx // You might need to create a mock for this as well

			tt.setupMocks(mockPrediction, mockAccount, mockPayees, mockCategory)

			// Act
			err := service.updatePrediction(ctx, mockTx, budgetId, txnId, txn)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify all expectations were met
			mockPrediction.AssertExpectations(t)
			mockAccount.AssertExpectations(t)
			mockPayees.AssertExpectations(t)
			mockCategory.AssertExpectations(t)
		})
	}
}
