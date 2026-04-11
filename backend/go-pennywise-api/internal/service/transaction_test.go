package service

import (
	"context"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock repositories
type mockBaseRepo struct{}

func (m *mockBaseRepo) GetPgxTx(ctx context.Context) (pgx.Tx, error) {
	return nil, nil
}

func (m *mockBaseRepo) GetDB() *pgxpool.Pool {
	return nil
}

type mockTransactionRepo struct {
	mockBaseRepo
	mock.Mock
}

// Create implements repository.TransactionRepository.
func (m *mockTransactionRepo) Create(ctx context.Context, tx pgx.Tx, txn model.Transaction) ([]model.Transaction, error) {
	args := m.Called(ctx, tx, txn)
	if obj := args.Get(0); obj != nil {
		return obj.([]model.Transaction), args.Error(1)
	}
	return nil, args.Error(1)
}

// DeleteById implements repository.TransactionRepository.
func (m *mockTransactionRepo) DeleteById(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, id uuid.UUID) error {
	panic("unimplemented")
}

// GetAll implements repository.TransactionRepository.
func (m *mockTransactionRepo) GetAll(ctx context.Context, budgetId uuid.UUID, filter *model.TransactionFilter) ([]model.Transaction, error) {
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
	args := m.Called(ctx, tx, budgetId, id, txn)
	return args.Error(0)
}

type mockBudgetRepo struct {
	mockBaseRepo
	mock.Mock
}

func (m *mockBudgetRepo) GetAll(ctx context.Context, userID uuid.UUID) ([]model.Budget, error) {
	panic("unimplemented")
}

func (m *mockBudgetRepo) GetById(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*model.Budget, error) {
	args := m.Called(ctx, tx, id)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.Budget), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockBudgetRepo) Create(ctx context.Context, tx pgx.Tx, name string, userID uuid.UUID) (*model.Budget, error) {
	panic("unimplemented")
}

func (m *mockBudgetRepo) UpdateById(ctx context.Context, tx pgx.Tx, id uuid.UUID, budget model.Budget) error {
	panic("unimplemented")
}

func (m *mockBudgetRepo) IsOwnedByUser(ctx context.Context, budgetID uuid.UUID, userID uuid.UUID) (bool, error) {
	panic("unimplemented")
}

type mockPredictionRepo struct {
	mockBaseRepo
	mock.Mock
}

// Create implements repository.PredictionRepository.
func (m *mockPredictionRepo) Create(ctx context.Context, prediction model.Prediction) ([]model.Prediction, error) {
	panic("unimplemented")
}

// DeleteByTxnId implements repository.PredictionRepository.
func (m *mockPredictionRepo) DeleteByTxnId(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, txnId uuid.UUID) error {
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
	mockBaseRepo
	mock.Mock
}

// Create implements repository.AccountRepository.
func (m *mockAccountRepo) Create(ctx context.Context, tx pgx.Tx, account model.Account) (*model.Account, error) {
	panic("unimplemented")
}

// GetAll implements repository.AccountRepository.
func (m *mockAccountRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Account, error) {
	panic("unimplemented")
}

// GetById implements repository.AccountRepository.
func (m *mockAccountRepo) GetById(ctx context.Context, tx pgx.Tx, budgetId, accountId uuid.UUID) (*model.Account, error) {
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

// UpdateTransferPayee implements repository.AccountRepository.
func (m *mockAccountRepo) UpdateTransferPayee(ctx context.Context, tx pgx.Tx, accountId uuid.UUID, payeeId uuid.UUID) error {
	panic("unimplemented")
}

type mockPayeesRepo struct {
	mockBaseRepo
	mock.Mock
}

// Create implements repository.PayeesRepository.
func (m *mockPayeesRepo) Create(ctx context.Context, tx pgx.Tx, payee model.Payee) (*model.Payee, error) {
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
	mockBaseRepo
	mock.Mock
}

// Create implements repository.CategoryRepository.
func (m *mockCategoryRepo) Create(ctx context.Context, tx pgx.Tx, category model.Category) (*model.Category, error) {
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

// GetInflowBalance implements repository.CategoryRepository.
func (m *mockCategoryRepo) GetInflowBalance(ctx context.Context, budgetId uuid.UUID) (float64, error) {
	panic("unimplemented")
}

// GetByFilter implements repository.CategoryRepository.
func (m *mockCategoryRepo) GetByFilter(ctx context.Context, budgetId uuid.UUID, filter model.CategoryFilter) ([]model.Category, error) {
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
	mockBaseRepo
	mock.Mock
}

// Create implements repository.MonthlyBudgetRepository.
func (m *mockMonthlyBudgetRepo) Create(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, monthlyBudget model.MonthlyBudget) error {
	args := m.Called(ctx, tx, budgetId, monthlyBudget)
	return args.Error(0)
}

// GetByCatIdAndMonth implements repository.MonthlyBudgetRepository.
func (m *mockMonthlyBudgetRepo) GetByCatIdAndMonth(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, categoryId uuid.UUID, month string) (*model.MonthlyBudget, error) {
	args := m.Called(ctx, tx, budgetId, categoryId, month)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.MonthlyBudget), args.Error(1)
	}
	return nil, args.Error(1)
}

// UpdateBudgetedByCatIdAndMonth implements repository.MonthlyBudgetRepository.
func (m *mockMonthlyBudgetRepo) UpdateBudgetedByCatIdAndMonth(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, categoryId uuid.UUID, month string, newBudgeted float64) error {
	panic("unimplemented")
}

// UpdateCarryoverByCatIdAndMonth implements repository.MonthlyBudgetRepository.
func (m *mockMonthlyBudgetRepo) UpdateCarryoverByCatIdAndMonth(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, categoryId uuid.UUID, month string, amount float64) error {
	args := m.Called(ctx, tx, budgetId, categoryId, month, amount)
	return args.Error(0)
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
	return budgetId, txnId, accountId, payeeId, categoryId, predictionId
}

func createTestPrediction(id, budgetId, txnId uuid.UUID) *model.Prediction {
	account := "Test Account"
	payee := "Test Payee"
	category := "Test Category"
	hasUserCorrected := false

	accountPrediction := 0.75
	payeePrediction := 0.88
	categoryPrediction := 0.2

	return &model.Prediction{
		ID:                 id,
		BudgetID:           budgetId,
		TransactionID:      txnId,
		Account:            &account,
		AccountPrediction:  &accountPrediction,
		Payee:              &payee,
		PayeePrediction:    &payeePrediction,
		Category:           &category,
		CategoryPrediction: &categoryPrediction,
		HasUserCorrected:   &hasUserCorrected,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
}

func createTestAccount(id uuid.UUID, budgetId uuid.UUID, name string) *model.Account {
	return &model.Account{
		ID:        id,
		Name:      name,
		BudgetID:  budgetId,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func createTestPayee(id uuid.UUID, budgetId uuid.UUID, name string) *model.Payee {
	return &model.Payee{
		ID:        id,
		Name:      name,
		BudgetID:  budgetId,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func createTestCategory(id uuid.UUID, budgetId uuid.UUID, name string) *model.Category {
	return &model.Category{
		ID:              id,
		Name:            name,
		BudgetID:        budgetId,
		CategoryGroupID: uuid.New(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

func setupTransactionTestableService(mockPrediction *mockPredictionRepo, mockAccount *mockAccountRepo, mockPayees *mockPayeesRepo, mockCategory *mockCategoryRepo, mockMonthlyBudget *mockMonthlyBudgetRepo) *transactionService {
	return newTestTransactionService(&mockTransactionRepo{}, &mockBudgetRepo{}, mockPrediction, mockAccount, mockPayees, mockCategory, mockMonthlyBudget)
}

func newTestTransactionService(
	mockTransaction *mockTransactionRepo,
	mockBudget *mockBudgetRepo,
	mockPrediction *mockPredictionRepo,
	mockAccount *mockAccountRepo,
	mockPayees *mockPayeesRepo,
	mockCategory *mockCategoryRepo,
	mockMonthlyBudget *mockMonthlyBudgetRepo,
) *transactionService {
	if mockTransaction == nil {
		mockTransaction = &mockTransactionRepo{}
	}
	if mockBudget == nil {
		mockBudget = &mockBudgetRepo{}
	}
	if mockPrediction == nil {
		mockPrediction = &mockPredictionRepo{}
	}
	if mockAccount == nil {
		mockAccount = &mockAccountRepo{}
	}
	if mockPayees == nil {
		mockPayees = &mockPayeesRepo{}
	}
	if mockCategory == nil {
		mockCategory = &mockCategoryRepo{}
	}
	if mockMonthlyBudget == nil {
		mockMonthlyBudget = &mockMonthlyBudgetRepo{}
	}

	service := NewTransactionService(
		mockTransaction,
		mockBudget,
		mockPrediction,
		mockAccount,
		mockPayees,
		mockCategory,
		mockMonthlyBudget,
	)

	return service.(*transactionService)
}

// func TestUpdatePrediction_EdgeCases(t *testing.T) {
// 	t.Run("nil_account_id", func(t *testing.T) {
// 		// Test case where AccountID is nil
// 		mockPrediction := &mockPredictionRepo{}
// 		mockAccount := &mockAccountRepo{}
// 		mockPayees := &mockPayeesRepo{}
// 		mockCategory := &mockCategoryRepo{}
//
// 		service := setupTransactionTestableService(mockPrediction, mockAccount, mockPayees, mockCategory)
//
// 		budgetId, txnId, _, _, _, predictionId := createTestUUIDs()
// 		prediction := createTestPrediction(predictionId, budgetId, txnId)
//
// 		txn := model.Transaction{
// 			AccountID: nil, // This should cause the test to fail or handle gracefully
// 		}
//
// 		mockPrediction.On("GetByTxnIdTx", mock.Anything, mock.Anything, budgetId, txnId).Return(prediction, nil)
//
// 		ctx := context.Background()
// 		var mockTx pgx.Tx
//
// 		err := service.updatePrediction(ctx, mockTx, budgetId, txnId, txn)
//
// 		// This should either handle the nil gracefully or return an error
// 		// Adjust the assertion based on your expected behavior
// 		assert.Error(t, err) // or assert.NoError(t, err) if it should handle gracefully
//
// 		mockPrediction.AssertExpectations(t)
// 	})

// 	t.Run("nil_payee_id", func(t *testing.T) {
// 		// Similar test for nil PayeeID
// 		// Implementation similar to above
// 	})
//
// 	t.Run("nil_category_id", func(t *testing.T) {
// 		// Similar test for nil CategoryID
// 		// Implementation similar to above
// 	})
// }

func TestUpdatePrediction(t *testing.T) {
	budgetId, txnId, accountId, payeeId, categoryId, predictionId := createTestUUIDs()
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
				prediction := createTestPrediction(predictionId, budgetId, txnId)
				account := createTestAccount(accountId, budgetId, "Test Account")
				payee := createTestPayee(payeeId, budgetId, "Test Payee")
				category := createTestCategory(categoryId, budgetId, "Test Category")

				mp.On("GetByTxnIdTx", mock.Anything, mock.Anything, budgetId, txnId).Return(prediction, nil)
				ma.On("GetById", mock.Anything, mock.Anything, budgetId, accountId).Return(account, nil)
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
				prediction := createTestPrediction(predictionId, budgetId, txnId)
				account := createTestAccount(accountId, budgetId, "Different Account")
				payee := createTestPayee(payeeId, budgetId, "Different Payee")
				category := createTestCategory(categoryId, budgetId, "Different Category")

				mp.On("GetByTxnIdTx", mock.Anything, mock.Anything, budgetId, txnId).Return(prediction, nil)
				ma.On("GetById", mock.Anything, mock.Anything, budgetId, accountId).Return(account, nil)
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
				prediction := createTestPrediction(predictionId, budgetId, txnId)

				mp.On("GetByTxnIdTx", mock.Anything, mock.Anything, budgetId, txnId).Return(prediction, nil)
				ma.On("GetById", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
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
			mockMonthlyBudget := &mockMonthlyBudgetRepo{}

			service := setupTransactionTestableService(mockPrediction, mockAccount, mockPayees, mockCategory, mockMonthlyBudget)

			txn := model.Transaction{
				AccountID:  &accountId,
				PayeeID:    &payeeId,
				CategoryID: &categoryId,
			}

			ctx := context.Background()
			var mockTx pgx.Tx

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

func TestUpdateCarryovers(t *testing.T) {
	budgetId, txnId, accountId, payeeId, categoryId, _ := createTestUUIDs()
	_, _, _, _, newCategoryId, _ := createTestUUIDs()

	tests := []struct {
		name         string
		setupMocks   func(*mockMonthlyBudgetRepo)
		existingTxn  *model.Transaction
		newTxn       model.Transaction
		expectError  bool
		expectUpdate bool
	}{
		{
			name: "different_categories",
			setupMocks: func(mb *mockMonthlyBudgetRepo) {
				mb.On(
					"GetByCatIdAndMonth",
					mock.Anything,
					mock.Anything,
					budgetId,
					newCategoryId,
					"2025-02",
				).Return(&model.MonthlyBudget{}, nil).Once()
				mb.On(
					"UpdateCarryoverByCatIdAndMonth",
					mock.Anything,
					mock.Anything,
					budgetId,
					categoryId,
					"2025-01",
					1000.00,
				).Return(nil).Once()
				mb.On(
					"UpdateCarryoverByCatIdAndMonth",
					mock.Anything,
					mock.Anything,
					budgetId,
					newCategoryId,
					"2025-02",
					-1500.00,
				).Return(nil).Once()
			},
			existingTxn: &model.Transaction{
				ID:         txnId,
				AccountID:  &accountId,
				PayeeID:    &payeeId,
				CategoryID: &categoryId,
				Amount:     -1000.00,
				Date:       "2025-01-01",
			},
			newTxn: model.Transaction{
				ID:         txnId,
				AccountID:  &accountId,
				PayeeID:    &payeeId,
				CategoryID: &newCategoryId,
				Amount:     -1500.00,
				Date:       "2025-02-01",
			},
			expectError:  false,
			expectUpdate: true,
		},
		{
			name: "same_category",
			setupMocks: func(mb *mockMonthlyBudgetRepo) {
				mb.On(
					"GetByCatIdAndMonth",
					mock.Anything,
					mock.Anything,
					budgetId,
					categoryId,
					"2025-01",
				).Return(&model.MonthlyBudget{}, nil).Once()
				mb.On(
					"UpdateCarryoverByCatIdAndMonth",
					mock.Anything,
					mock.Anything,
					budgetId,
					categoryId,
					"2025-01",
					-500.00,
				).Return(nil).Once()
			},
			existingTxn: &model.Transaction{
				ID:         txnId,
				AccountID:  &accountId,
				PayeeID:    &payeeId,
				CategoryID: &categoryId,
				Amount:     -1000.00,
				Date:       "2025-01-01",
			},
			newTxn: model.Transaction{
				ID:         txnId,
				AccountID:  &accountId,
				PayeeID:    &payeeId,
				CategoryID: &categoryId,
				Amount:     -1500.00,
				Date:       "2025-01-01",
			},
			expectError:  false,
			expectUpdate: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPrediction := &mockPredictionRepo{}
			mockAccount := &mockAccountRepo{}
			mockPayees := &mockPayeesRepo{}
			mockCategory := &mockCategoryRepo{}
			mockMonthlyBudget := &mockMonthlyBudgetRepo{}

			service := setupTransactionTestableService(mockPrediction, mockAccount, mockPayees, mockCategory, mockMonthlyBudget)

			ctx := context.Background()
			var mockTx pgx.Tx

			tt.setupMocks(mockMonthlyBudget)

			err := service.updateCarryovers(ctx, mockTx, budgetId, tt.existingTxn, tt.newTxn)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCreateTransferTxnIfNeeded(t *testing.T) {
	ctx := context.Background()
	var mockTx pgx.Tx

	budgetId := uuid.New()
	txnId := uuid.New()
	accountId := uuid.New()
	transferAccountID := uuid.New()
	transferPayeeID := uuid.New()
	transferTxnID := uuid.New()

	txnPayload := model.Transaction{
		ID:        txnId,
		AccountID: &accountId,
		Amount:    -42.50,
		Date:      "2025-01-15",
		Source:    "manual",
	}

	payee := model.Payee{TransferAccountID: &transferAccountID}

	t.Run("wraps transfer create failures", func(t *testing.T) {
		mockTransaction := &mockTransactionRepo{}
		service := newTestTransactionService(mockTransaction, nil, nil, nil, nil, nil, nil)

		account := model.Account{TransferPayeeID: &transferPayeeID}

		mockTransaction.On(
			"Create",
			mock.Anything,
			mock.Anything,
			mock.MatchedBy(func(txn model.Transaction) bool {
				return txn.PayeeID != nil && *txn.PayeeID == transferPayeeID
			}),
		).Return(nil, assert.AnError).Once()

		createdTransferID, err := service.createTransferTxnIfNeeded(ctx, mockTx, budgetId, txnPayload, account, payee)

		require.Nil(t, createdTransferID)
		require.Error(t, err)

		var appErr *errs.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, errs.CodeTransferCreateFailed, appErr.Code)
		mockTransaction.AssertExpectations(t)
	})

	t.Run("returns transfer not created when repo returns no rows", func(t *testing.T) {
		mockTransaction := &mockTransactionRepo{}
		service := newTestTransactionService(mockTransaction, nil, nil, nil, nil, nil, nil)

		account := model.Account{TransferPayeeID: &transferPayeeID}

		mockTransaction.On("Create", mock.Anything, mock.Anything, mock.Anything).Return([]model.Transaction{}, nil).Once()

		createdTransferID, err := service.createTransferTxnIfNeeded(ctx, mockTx, budgetId, txnPayload, account, payee)

		require.Nil(t, createdTransferID)
		require.Error(t, err)

		var appErr *errs.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, errs.CodeTransferNotCreated, appErr.Code)
		mockTransaction.AssertExpectations(t)
	})

	t.Run("currently allows missing transfer payee id", func(t *testing.T) {
		mockTransaction := &mockTransactionRepo{}
		service := newTestTransactionService(mockTransaction, nil, nil, nil, nil, nil, nil)

		account := model.Account{}

		mockTransaction.On(
			"Create",
			mock.Anything,
			mock.Anything,
			mock.MatchedBy(func(txn model.Transaction) bool {
				return txn.AccountID != nil && *txn.AccountID == transferAccountID && txn.PayeeID == nil
			}),
		).Return([]model.Transaction{{ID: transferTxnID}}, nil).Once()
		mockTransaction.On(
			"Update",
			mock.Anything,
			mock.Anything,
			budgetId,
			txnId,
			mock.MatchedBy(func(txn model.Transaction) bool {
				return txn.TransferTransactionID != nil && *txn.TransferTransactionID == transferTxnID
			}),
		).Return(nil).Once()

		createdTransferID, err := service.createTransferTxnIfNeeded(ctx, mockTx, budgetId, txnPayload, account, payee)

		require.NoError(t, err)
		require.NotNil(t, createdTransferID)
		assert.Equal(t, transferTxnID, *createdTransferID)
		mockTransaction.AssertExpectations(t)
	})
}

func TestHandleCarryoversSkipsForNonCarryoverTransactions(t *testing.T) {
	ctx := context.Background()
	budgetId := uuid.New()
	inflowCategoryID := uuid.New()
	monthlyBudgetRepo := &mockMonthlyBudgetRepo{}
	service := newTestTransactionService(nil, nil, nil, nil, nil, nil, monthlyBudgetRepo)

	budget := model.Budget{
		Metadata: model.BudgetMetadata{InflowCategoryID: inflowCategoryID},
	}

	t.Run("nil category skips carryover work", func(t *testing.T) {
		err := service.handleCarryovers(ctx, nil, budgetId, model.Transaction{
			Date:   "2025-01-01",
			Amount: 100,
		}, budget)

		require.NoError(t, err)
	})

	t.Run("inflow category skips carryover work", func(t *testing.T) {
		err := service.handleCarryovers(ctx, nil, budgetId, model.Transaction{
			Date:       "2025-01-01",
			Amount:     100,
			CategoryID: &inflowCategoryID,
		}, budget)

		require.NoError(t, err)
	})

	monthlyBudgetRepo.AssertExpectations(t)
}
