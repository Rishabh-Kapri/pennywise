package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock repositories
type mockBaseRepo struct{}

type mockTransactionRepo struct {
	mockBaseRepo
	mock.Mock
}

// Create implements repository.TransactionRepository.
func (m *mockTransactionRepo) Create(
	ctx context.Context,
	txn model.Transaction,
) ([]model.Transaction, error) {
	args := m.Called(ctx, txn)
	if obj := args.Get(0); obj != nil {
		return obj.([]model.Transaction), args.Error(1)
	}
	return nil, args.Error(1)
}

// CreateDeduped implements repository.TransactionRepository.
func (m *mockTransactionRepo) CreateDeduped(
	ctx context.Context,
	txn model.Transaction,
) (*model.Transaction, bool, error) {
	args := m.Called(ctx, txn)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.Transaction), args.Bool(1), args.Error(2)
	}
	return nil, args.Bool(1), args.Error(2)
}

// DeleteById implements repository.TransactionRepository.
func (m *mockTransactionRepo) DeleteById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error {
	args := m.Called(ctx, budgetId, id)
	return args.Error(0)
}

// GetAll implements repository.TransactionRepository.
func (m *mockTransactionRepo) GetAll(
	ctx context.Context,
	budgetId uuid.UUID,
	filter *model.TransactionFilter,
) ([]model.Transaction, error) {
	args := m.Called(ctx, budgetId, filter)
	if obj := args.Get(0); obj != nil {
		return obj.([]model.Transaction), args.Error(1)
	}
	return nil, args.Error(1)
}

// GetAllNormalized implements repository.TransactionRepository.
func (m *mockTransactionRepo) GetAllNormalized(
	ctx context.Context,
	budgetId uuid.UUID,
	filter *model.TransactionFilter,
) (model.PaginatedResponse[model.Transaction], error) {
	args := m.Called(ctx, budgetId, filter)
	if obj := args.Get(0); obj != nil {
		return obj.(model.PaginatedResponse[model.Transaction]), args.Error(1)
	}
	return model.PaginatedResponse[model.Transaction]{}, args.Error(1)
}

// GetById implements repository.TransactionRepository.
func (m *mockTransactionRepo) GetById(
	ctx context.Context,
	budgetId uuid.UUID,
	id uuid.UUID,
) (*model.Transaction, error) {
	args := m.Called(ctx, budgetId, id)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.Transaction), args.Error(1)
	}
	return nil, args.Error(1)
}

// Update implements repository.TransactionRepository.
func (m *mockTransactionRepo) Update(
	ctx context.Context,
	budgetId uuid.UUID,
	id uuid.UUID,
	txn model.Transaction,
) error {
	args := m.Called(ctx, budgetId, id, txn)
	return args.Error(0)
}

// UpdateLocation implements repository.TransactionRepository.
func (m *mockTransactionRepo) UpdateLocation(
	ctx context.Context,
	budgetId uuid.UUID,
	id uuid.UUID,
	lat *float64,
	lng *float64,
	name *string,
	source *model.LocationSource,
) error {
	args := m.Called(ctx, budgetId, id, lat, lng, name, source)
	return args.Error(0)
}

func (m *mockTransactionRepo) UpdateStatus(
	ctx context.Context,
	budgetId uuid.UUID,
	id uuid.UUID,
	status model.TransactionStatus,
) error {
	args := m.Called(ctx, budgetId, id, status)
	return args.Error(0)
}

type mockBudgetRepo struct {
	mockBaseRepo
	mock.Mock
}

func (m *mockBudgetRepo) GetAll(ctx context.Context, userID uuid.UUID) ([]model.Budget, error) {
	panic("unimplemented")
}

func (m *mockBudgetRepo) GetById(ctx context.Context, id uuid.UUID) (*model.Budget, error) {
	args := m.Called(ctx, id)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.Budget), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockBudgetRepo) Create(ctx context.Context, name string, userID uuid.UUID) (*model.Budget, error) {
	panic("unimplemented")
}

func (m *mockBudgetRepo) UpdateById(ctx context.Context, id uuid.UUID, budget model.Budget) error {
	panic("unimplemented")
}

func (m *mockBudgetRepo) IsOwnedByUser(ctx context.Context, budgetID uuid.UUID, userID uuid.UUID) (bool, error) {
	panic("unimplemented")
}

func (m *mockBudgetRepo) UpdateOnboarding(ctx context.Context, id uuid.UUID, onboarding model.BudgetOnboarding) error {
	panic("unimplemented")
}

type mockPredictionRepo struct {
	mockBaseRepo
	mock.Mock
}

type mockCipherPredictionRepo struct {
	mockBaseRepo
	mock.Mock
}

func (m *mockCipherPredictionRepo) Create(
	ctx context.Context,
	p model.CipherPredictionRecord,
) (*model.CipherPredictionRecord, error) {
	args := m.Called(ctx, p)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.CipherPredictionRecord), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockCipherPredictionRepo) GetAll(
	ctx context.Context,
	budgetID uuid.UUID,
) ([]model.CipherPredictionRecord, error) {
	args := m.Called(ctx, budgetID)
	if obj := args.Get(0); obj != nil {
		return obj.([]model.CipherPredictionRecord), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockCipherPredictionRepo) GetByTransactionID(
	ctx context.Context,
	budgetID uuid.UUID,
	txnID uuid.UUID,
) (*model.CipherPredictionRecord, error) {
	args := m.Called(ctx, budgetID, txnID)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.CipherPredictionRecord), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockCipherPredictionRepo) MarkLearned(
	ctx context.Context,
	budgetID uuid.UUID,
	txnID uuid.UUID,
) error {
	return m.Called(ctx, budgetID, txnID).Error(0)
}

func (m *mockCipherPredictionRepo) MarkLearnFailed(
	ctx context.Context,
	budgetID uuid.UUID,
	txnID uuid.UUID,
	reason string,
) error {
	return m.Called(ctx, budgetID, txnID, reason).Error(0)
}

func (m *mockCipherPredictionRepo) ListPendingLearning(
	ctx context.Context,
	budgetID uuid.UUID,
	limit int,
	retryFailed bool,
) ([]model.LearningCandidate, error) {
	args := m.Called(ctx, budgetID, limit, retryFailed)
	if v := args.Get(0); v != nil {
		return v.([]model.LearningCandidate), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockCipherPredictionRepo) LearningStats(
	ctx context.Context,
	budgetID uuid.UUID,
) (model.LearningStats, error) {
	args := m.Called(ctx, budgetID)
	return args.Get(0).(model.LearningStats), args.Error(1)
}

func (m *mockCipherPredictionRepo) MarkUserCorrected(
	ctx context.Context,
	budgetID uuid.UUID,
	txnID uuid.UUID,
	actualPayeeID *uuid.UUID,
	actualCategoryID *uuid.UUID,
) error {
	args := m.Called(ctx, budgetID, txnID, actualPayeeID, actualCategoryID)
	return args.Error(0)
}

type mockTransactionEmbeddingRepo struct {
	mockBaseRepo
	mock.Mock
}

func (m *mockTransactionEmbeddingRepo) SearchSimilar(
	ctx context.Context,
	budgetID uuid.UUID,
	amount float64,
	embedding []float64,
	limit int,
) ([]model.TransactionEmbedding, error) {
	args := m.Called(ctx, budgetID, amount, embedding, limit)
	if obj := args.Get(0); obj != nil {
		return obj.([]model.TransactionEmbedding), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockTransactionEmbeddingRepo) Upsert(
	ctx context.Context,
	data model.TransactionEmbedding,
	embedding []float64,
) error {
	args := m.Called(ctx, data, embedding)
	return args.Error(0)
}

type mockPayeeRuleRepo struct {
	mockBaseRepo
	mock.Mock
}

func (m *mockPayeeRuleRepo) CreatePayeeRule(ctx context.Context, payeeRule model.PayeeRule) error {
	args := m.Called(ctx, payeeRule)
	return args.Error(0)
}

func (m *mockPayeeRuleRepo) FindByMatchString(
	ctx context.Context,
	budgetId uuid.UUID,
	matchString string,
) (*model.PayeeRule, error) {
	args := m.Called(ctx, budgetId, matchString)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.PayeeRule), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPayeeRuleRepo) FindByPayeeID(
	ctx context.Context,
	budgetId uuid.UUID,
	payeeId uuid.UUID,
) ([]model.PayeeRuleDetails, error) {
	args := m.Called(ctx, budgetId, payeeId)
	if obj := args.Get(0); obj != nil {
		return obj.([]model.PayeeRuleDetails), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPayeeRuleRepo) Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, payeeRule model.PayeeRule) error {
	args := m.Called(ctx, budgetId, id, payeeRule)
	return args.Error(0)
}

func (m *mockPayeeRuleRepo) DeleteByID(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) error {
	args := m.Called(ctx, budgetId, id)
	return args.Error(0)
}

func (m *mockPayeeRuleRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.PayeeRuleDetails, error) {
	args := m.Called(ctx, budgetId)
	if obj := args.Get(0); obj != nil {
		return obj.([]model.PayeeRuleDetails), args.Error(1)
	}
	return nil, args.Error(1)
}

type mockCipherClient struct {
	mock.Mock
}

func (m *mockCipherClient) SuggestCategoryDescription(
	ctx context.Context,
	req model.CategoryDescriptionContext,
) (*CategoryDescriptionResponse, error) {
	args := m.Called(ctx, req)
	if v := args.Get(0); v != nil {
		return v.(*CategoryDescriptionResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockCipherClient) ExtractStatement(
	ctx context.Context,
	req model.StatementExtractRequest,
) (*model.StatementExtractResponse, error) {
	args := m.Called(ctx, req)
	if v := args.Get(0); v != nil {
		return v.(*model.StatementExtractResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockCipherClient) GenerateTransactionEmbedding(
	ctx context.Context,
	req TransactionEmbeddingRequest,
) (*TransactionEmbeddingResponse, error) {
	args := m.Called(ctx, req)
	if obj := args.Get(0); obj != nil {
		return obj.(*TransactionEmbeddingResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

// Create implements repository.PredictionRepository.
func (m *mockPredictionRepo) Create(ctx context.Context, prediction model.Prediction) ([]model.Prediction, error) {
	panic("unimplemented")
}

// DeleteByTxnId implements repository.PredictionRepository.
func (m *mockPredictionRepo) DeleteByTxnId(ctx context.Context, budgetId uuid.UUID, txnId uuid.UUID) error {
	args := m.Called(ctx, budgetId, txnId)
	return args.Error(0)
}

// GetAll implements repository.PredictionRepository.
func (m *mockPredictionRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Prediction, error) {
	panic("unimplemented")
}

// GetByTxnId implements repository.PredictionRepository.
func (m *mockPredictionRepo) GetByTxnId(
	ctx context.Context,
	budgetId, txnId uuid.UUID,
) (*model.Prediction, error) {
	args := m.Called(ctx, budgetId, txnId)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.Prediction), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPredictionRepo) Update(
	ctx context.Context,
	budgetId, predictionId uuid.UUID,
	prediction model.Prediction,
) error {
	args := m.Called(ctx, budgetId, predictionId, prediction)
	return args.Error(0)
}

type mockAccountRepo struct {
	mockBaseRepo
	mock.Mock
}

// Create implements repository.AccountRepository.
func (m *mockAccountRepo) Create(ctx context.Context, account model.Account) (*model.Account, error) {
	panic("unimplemented")
}

// Update implements repository.AccountRepository.
func (m *mockAccountRepo) Update(ctx context.Context, budgetId, id uuid.UUID, account model.Account) (*model.Account, error) {
	panic("unimplemented")
}

// GetAll implements repository.AccountRepository.
func (m *mockAccountRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Account, error) {
	panic("unimplemented")
}

func (m *mockAccountRepo) GetAllSimplified(ctx context.Context, budgetId uuid.UUID) ([]model.AccountSimplified, error) {
	panic("unimplemented")
}

func (m *mockAccountRepo) GetBySuffix(ctx context.Context, budgetId uuid.UUID, suffix string) (*model.Account, error) {
	panic("unimplemented")
}

// GetById implements repository.AccountRepository.
func (m *mockAccountRepo) GetById(
	ctx context.Context,
	budgetId, accountId uuid.UUID,
) (*model.Account, error) {
	args := m.Called(ctx, budgetId, accountId)
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
func (m *mockAccountRepo) UpdateTransferPayee(
	ctx context.Context,
	accountId uuid.UUID,
	payeeId uuid.UUID,
) error {
	panic("unimplemented")
}

type mockPayeesRepo struct {
	mockBaseRepo
	mock.Mock
}

// Create implements repository.PayeesRepository.
func (m *mockPayeesRepo) Create(ctx context.Context, payee model.Payee) (*model.Payee, error) {
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
func (m *mockPayeesRepo) GetById(ctx context.Context, budgetId, payeeId uuid.UUID) (*model.Payee, error) {
	args := m.Called(ctx, budgetId, payeeId)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.Payee), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockPayeesRepo) GetStats(ctx context.Context, budgetId uuid.UUID) ([]model.PayeeStats, error) {
	panic("unimplemented")
}

func (m *mockPayeesRepo) Merge(
	ctx context.Context,
	budgetId, targetId uuid.UUID,
	sourceIds []uuid.UUID,
) (*model.PayeeMergeResult, error) {
	panic("unimplemented")
}

// Search implements repository.PayeesRepository.
func (m *mockPayeesRepo) Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Payee, error) {
	panic("unimplemented")
}

// Update implements repository.PayeesRepository.
func (m *mockPayeesRepo) Update(ctx context.Context, budgetId uuid.UUID, id uuid.UUID, payee model.Payee) error {
	panic("unimplemented")
}

func (m *mockCategoryRepo) UpdateDetails(ctx context.Context, budgetId, id uuid.UUID, note, description *string) error {
	return m.Called(ctx, budgetId, id, note, description).Error(0)
}

func (m *mockCategoryRepo) GetDescriptionContext(
	ctx context.Context,
	budgetId, categoryId uuid.UUID,
	payeeLimit int,
) (*model.CategoryDescriptionContext, error) {
	args := m.Called(ctx, budgetId, categoryId, payeeLimit)
	if v := args.Get(0); v != nil {
		return v.(*model.CategoryDescriptionContext), args.Error(1)
	}
	return nil, args.Error(1)
}

type mockCategoryRepo struct {
	mockBaseRepo
	mock.Mock
}

// Create implements repository.CategoryRepository.
func (m *mockCategoryRepo) Create(ctx context.Context, category model.Category) (*model.Category, error) {
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

// GetAllSimplified implements repository.CategoryRepository.
func (m *mockCategoryRepo) GetAllSimplified(
	ctx context.Context,
	budgetId uuid.UUID,
) ([]model.CategorySimplified, error) {
	panic("unimplemented")
}

// FindClosestSimplified implements repository.CategoryRepository.
func (m *mockCategoryRepo) FindClosestSimplified(
	ctx context.Context,
	budgetId uuid.UUID,
	name string,
	limit int,
) ([]model.CategoryNameMatch, error) {
	panic("unimplemented")
}

// GetInflowBalance implements repository.CategoryRepository.
func (m *mockCategoryRepo) GetInflowBalance(ctx context.Context, budgetId uuid.UUID) (float64, error) {
	panic("unimplemented")
}

// GetByFilter implements repository.CategoryRepository.
func (m *mockCategoryRepo) GetByFilter(
	ctx context.Context,
	budgetId uuid.UUID,
	filter model.CategoryFilter,
) ([]model.Category, error) {
	panic("unimplemented")
}

// GetById implements repository.CategoryRepository.
func (m *mockCategoryRepo) GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Category, error) {
	panic("unimplemented")
}

// GetByIdSimplified implements repository.CategoryRepository.
func (m *mockCategoryRepo) GetByIdSimplified(
	ctx context.Context,
	budgetId, categoryId uuid.UUID,
) (*model.Category, error) {
	args := m.Called(ctx, budgetId, categoryId)
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
func (m *mockCategoryRepo) Update(
	ctx context.Context,
	budgetId uuid.UUID,
	id uuid.UUID,
	category model.Category,
) error {
	panic("unimplemented")
}

type mockMonthlyBudgetRepo struct {
	mockBaseRepo
	mock.Mock
}

// Create implements repository.MonthlyBudgetRepository.
func (m *mockMonthlyBudgetRepo) Create(
	ctx context.Context,
	budgetId uuid.UUID,
	monthlyBudget model.MonthlyBudget,
) error {
	args := m.Called(ctx, budgetId, monthlyBudget)
	return args.Error(0)
}

// GetByCatIdAndMonth implements repository.MonthlyBudgetRepository.
func (m *mockMonthlyBudgetRepo) GetByCatIdAndMonth(
	ctx context.Context,
	budgetId uuid.UUID,
	categoryId uuid.UUID,
	month string,
) (*model.MonthlyBudget, error) {
	args := m.Called(ctx, budgetId, categoryId, month)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.MonthlyBudget), args.Error(1)
	}
	return nil, args.Error(1)
}

// UpdateBudgetedByCatIdAndMonth implements repository.MonthlyBudgetRepository.
func (m *mockMonthlyBudgetRepo) UpdateBudgetedByCatIdAndMonth(
	ctx context.Context,
	budgetId uuid.UUID,
	categoryId uuid.UUID,
	month string,
	newBudgeted float64,
) error {
	panic("unimplemented")
}

// UpdateCarryoverByCatIdAndMonth implements repository.MonthlyBudgetRepository.
func (m *mockMonthlyBudgetRepo) UpdateCarryoverByCatIdAndMonth(
	ctx context.Context,
	budgetId uuid.UUID,
	categoryId uuid.UUID,
	month string,
	amount float64,
) error {
	args := m.Called(ctx, budgetId, categoryId, month, amount)
	return args.Error(0)
}

// Mock transaction interface to expose private methods for testing
type testableTransactionService struct {
	service transactionService
}

// Test helpers
func createTestUUIDs() (budgetId, txnId, accountId, payeeId, categoryId, predictionId, inflowCategoryID uuid.UUID) {
	budgetId = uuid.New()
	txnId = uuid.New()
	accountId = uuid.New()
	payeeId = uuid.New()
	categoryId = uuid.New()
	predictionId = uuid.New()
	inflowCategoryID = uuid.New()
	return budgetId, txnId, accountId, payeeId, categoryId, predictionId, inflowCategoryID
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

func setupTransactionTestableService(
	mockPrediction *mockPredictionRepo,
	mockAccount *mockAccountRepo,
	mockPayees *mockPayeesRepo,
	mockCategory *mockCategoryRepo,
	mockMonthlyBudget *mockMonthlyBudgetRepo,
) *transactionService {
	return newTestTransactionService(
		&mockTransactionRepo{},
		&mockBudgetRepo{},
		mockPrediction,
		mockAccount,
		mockPayees,
		mockCategory,
		mockMonthlyBudget,
	)
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
		nil,
		nil,
		mockPrediction,
		nil,
		nil,
		mockAccount,
		mockPayees,
		mockCategory,
		NewMonthlyBudgetService(mockMonthlyBudget),
		nil,
		fakeDBTxn{},
		nil,
	)

	return service.(*transactionService)
}

func TestUpdateStatusRejectedOnlyUpdatesStatus(t *testing.T) {
	budgetId, txnId, _, _, _, _, _ := createTestUUIDs()
	ctx := utils.WithBudgetID(context.Background(), budgetId)
	txnRepo := &mockTransactionRepo{}
	svc := &transactionService{dbTxn: fakeDBTxn{}, repo: txnRepo}

	txnRepo.On("UpdateStatus", mock.Anything, budgetId, txnId, model.TransactionStatusRejected).Return(nil).Once()

	err := svc.UpdateStatus(ctx, txnId, model.TransactionStatusRejected)

	require.NoError(t, err)
	txnRepo.AssertExpectations(t)
}

func TestUpdateStatusApprovedWithoutCipherPredictionOnlyUpdatesStatus(t *testing.T) {
	budgetId, txnId, _, payeeId, categoryId, _, _ := createTestUUIDs()
	ctx := utils.WithBudgetID(context.Background(), budgetId)
	rawBankText := "paid to coffee shop"
	txn := &model.Transaction{
		ID:          txnId,
		BudgetID:    budgetId,
		PayeeID:     &payeeId,
		CategoryID:  &categoryId,
		Amount:      -125,
		RawBankText: &rawBankText,
	}
	txnRepo := &mockTransactionRepo{}
	cipherPredictionRepo := &mockCipherPredictionRepo{}
	svc := &transactionService{dbTxn: fakeDBTxn{}, repo: txnRepo, cipherPredictionRepo: cipherPredictionRepo}

	txnRepo.On("GetById", mock.Anything, budgetId, txnId).Return(txn, nil).Once()
	cipherPredictionRepo.On("GetByTransactionID", mock.Anything, budgetId, txnId).Return(nil, repository.ErrNotFound).Once()
	txnRepo.On("UpdateStatus", mock.Anything, budgetId, txnId, model.TransactionStatusApproved).Return(nil).Once()

	err := svc.UpdateStatus(ctx, txnId, model.TransactionStatusApproved)

	require.NoError(t, err)
	txnRepo.AssertExpectations(t)
	cipherPredictionRepo.AssertExpectations(t)
}

func TestUpdateStatusApprovedWithNonLLMCipherPredictionDoesNotEmbed(t *testing.T) {
	budgetId, txnId, _, payeeId, categoryId, _, _ := createTestUUIDs()
	ctx := utils.WithBudgetID(context.Background(), budgetId)
	rawBankText := "paid to coffee shop"
	txn := &model.Transaction{
		ID:          txnId,
		BudgetID:    budgetId,
		PayeeID:     &payeeId,
		CategoryID:  &categoryId,
		Amount:      -125,
		RawBankText: &rawBankText,
	}
	txnRepo := &mockTransactionRepo{}
	cipherPredictionRepo := &mockCipherPredictionRepo{}
	cipherClient := &mockCipherClient{}
	payeeRuleRepo := &mockPayeeRuleRepo{}
	txnEmbeddingRepo := &mockTransactionEmbeddingRepo{}
	svc := &transactionService{dbTxn: fakeDBTxn{},
		repo:                 txnRepo,
		cipherPredictionRepo: cipherPredictionRepo,
		cipherClient:         cipherClient,
		payeeRuleRepo:        payeeRuleRepo,
		txnEmbeddingRepo:     txnEmbeddingRepo,
	}

	txnRepo.On("GetById", mock.Anything, budgetId, txnId).Return(txn, nil).Once()
	cipherPredictionRepo.On("GetByTransactionID", mock.Anything, budgetId, txnId).Return(&model.CipherPredictionRecord{
		BudgetID:      budgetId,
		TransactionID: txnId,
		Source:        model.PredictionSourceVector,
	}, nil).Once()
	txnRepo.On("UpdateStatus", mock.Anything, budgetId, txnId, model.TransactionStatusApproved).Return(nil).Once()

	err := svc.UpdateStatus(ctx, txnId, model.TransactionStatusApproved)

	require.NoError(t, err)
	txnRepo.AssertExpectations(t)
	cipherPredictionRepo.AssertExpectations(t)
	cipherClient.AssertNotCalled(t, "GenerateTransactionEmbedding", mock.Anything, mock.Anything)
}

func TestUpdateStatusApprovedWithLLMCipherPredictionDoesNotEmbed(t *testing.T) {
	budgetId, txnId, _, payeeId, categoryId, _, _ := createTestUUIDs()
	ctx := utils.WithBudgetID(context.Background(), budgetId)
	rawBankText := "paid to amazon marketplace"
	extractedMerchant := "amazon marketplace"
	txn := &model.Transaction{
		ID:          txnId,
		BudgetID:    budgetId,
		PayeeID:     &payeeId,
		CategoryID:  &categoryId,
		Amount:      -999.50,
		RawBankText: &rawBankText,
	}
	txnRepo := &mockTransactionRepo{}
	cipherPredictionRepo := &mockCipherPredictionRepo{}
	cipherClient := &mockCipherClient{}
	payeeRuleRepo := &mockPayeeRuleRepo{}
	txnEmbeddingRepo := &mockTransactionEmbeddingRepo{}
	svc := &transactionService{dbTxn: fakeDBTxn{},
		repo:                 txnRepo,
		cipherPredictionRepo: cipherPredictionRepo,
		cipherClient:         cipherClient,
		payeeRuleRepo:        payeeRuleRepo,
		txnEmbeddingRepo:     txnEmbeddingRepo,
	}

	txnRepo.On("GetById", mock.Anything, budgetId, txnId).Return(txn, nil).Once()
	cipherPredictionRepo.On("GetByTransactionID", mock.Anything, budgetId, txnId).Return(&model.CipherPredictionRecord{
		BudgetID:       budgetId,
		TransactionID:  txnId,
		Source:         model.PredictionSourceLLM,
		ExtractedPayee: &extractedMerchant,
	}, nil).Once()
	txnRepo.On("UpdateStatus", mock.Anything, budgetId, txnId, model.TransactionStatusApproved).Return(nil).Once()

	err := svc.UpdateStatus(ctx, txnId, model.TransactionStatusApproved)

	require.NoError(t, err)
	txnRepo.AssertExpectations(t)
	cipherPredictionRepo.AssertExpectations(t)
	cipherClient.AssertNotCalled(t, "GenerateTransactionEmbedding", mock.Anything, mock.Anything)
}

func TestUpdateWithCipherPredictionMappingChangeMarksCorrection(t *testing.T) {
	budgetId, txnId, accountId, oldPayeeId, categoryId, _, inflowCategoryID := createTestUUIDs()
	newPayeeId := uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budgetId)
	rawBankText := "paid to grocery store"

	foundTxn := model.Transaction{
		ID:          txnId,
		BudgetID:    budgetId,
		AccountID:   &accountId,
		PayeeID:     &oldPayeeId,
		CategoryID:  &categoryId,
		Amount:      -450,
		Date:        "2023-01-01",
		Status:      model.TransactionStatusRejected,
		RawBankText: &rawBankText,
	}
	toUpdate := model.Transaction{
		BudgetID:   budgetId,
		AccountID:  &accountId,
		PayeeID:    &newPayeeId,
		CategoryID: &categoryId,
		Amount:     -450,
		Date:       "2023-01-01",
	}

	txnRepo := &mockTransactionRepo{}
	budgetRepo := &mockBudgetRepo{}
	accountRepo := &mockAccountRepo{}
	payeeRepo := &mockPayeesRepo{}
	monthlyBudgetRepo := &mockMonthlyBudgetRepo{}
	cipherPredictionRepo := &mockCipherPredictionRepo{}
	cipherClient := &mockCipherClient{}
	payeeRuleRepo := &mockPayeeRuleRepo{}
	txnEmbeddingRepo := &mockTransactionEmbeddingRepo{}
	svc := &transactionService{dbTxn: fakeDBTxn{},
		repo:                 txnRepo,
		budgetRepo:           budgetRepo,
		txnEmbeddingRepo:     txnEmbeddingRepo,
		cipherClient:         cipherClient,
		cipherPredictionRepo: cipherPredictionRepo,
		payeeRuleRepo:        payeeRuleRepo,
		accountRepo:          accountRepo,
		payeeRepo:            payeeRepo,
		mbService:            NewMonthlyBudgetService(monthlyBudgetRepo),
	}

	txnRepo.On("GetById", mock.Anything, budgetId, txnId).Return(&foundTxn, nil).Once()
	budgetRepo.On("GetById", mock.Anything, budgetId).Return(&model.Budget{
		ID: budgetId,
		Metadata: model.BudgetMetadata{
			InflowCategoryID: inflowCategoryID,
		},
	}, nil).Once()
	accountRepo.On("GetById", mock.Anything, budgetId, accountId).
		Return(&model.Account{ID: accountId, BudgetID: budgetId, Type: "checking"}, nil).
		Once()
	payeeRepo.On("GetById", mock.Anything, budgetId, newPayeeId).
		Return(&model.Payee{ID: newPayeeId, BudgetID: budgetId}, nil).
		Once()
	cipherPredictionRepo.On("GetByTransactionID", mock.Anything, budgetId, txnId).Return(&model.CipherPredictionRecord{
		BudgetID:      budgetId,
		TransactionID: txnId,
		Source:        model.PredictionSourceRule,
	}, nil).Once()
	cipherPredictionRepo.On(
		"MarkUserCorrected",
		mock.Anything,
		budgetId,
		txnId,
		mock.MatchedBy(func(id *uuid.UUID) bool { return id != nil && *id == newPayeeId }),
		mock.MatchedBy(func(id *uuid.UUID) bool { return id != nil && *id == categoryId }),
	).Return(nil).Once()
	txnRepo.On("Update", mock.Anything, budgetId, txnId, mock.MatchedBy(func(txn model.Transaction) bool {
		return txn.ID == txnId &&
			txn.PayeeID != nil && *txn.PayeeID == newPayeeId &&
			txn.Status == model.TransactionStatusRejected
	})).Return(nil).Once()

	err := svc.Update(ctx, txnId, toUpdate)

	require.NoError(t, err)
	txnRepo.AssertExpectations(t)
	budgetRepo.AssertExpectations(t)
	accountRepo.AssertExpectations(t)
	payeeRepo.AssertExpectations(t)
	cipherPredictionRepo.AssertExpectations(t)
	cipherClient.AssertNotCalled(t, "GenerateTransactionEmbedding", mock.Anything, mock.Anything)
}

func TestUpdatePrediction(t *testing.T) {
	budgetId, txnId, accountId, payeeId, categoryId, predictionId, _ := createTestUUIDs()
	tests := []struct {
		name              string
		account           model.Account
		payee             model.Payee
		setupMocks        func(*mockPredictionRepo, *mockCategoryRepo)
		expectError       bool
		expectUpdate      bool
		predictionExists  bool
		userCorrectedData bool
	}{
		{
			name:             "prediction_not_found_returns_nil",
			account:          *createTestAccount(accountId, budgetId, "Test Account"),
			payee:            *createTestPayee(payeeId, budgetId, "Test Payee"),
			predictionExists: false,
			expectError:      false,
			expectUpdate:     false,
			setupMocks: func(mp *mockPredictionRepo, mc *mockCategoryRepo) {
				mp.On("GetByTxnId", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
			},
		},
		{
			name:             "prediction_found_no_changes_needed",
			account:          *createTestAccount(accountId, budgetId, "Test Account"),
			payee:            *createTestPayee(payeeId, budgetId, "Test Payee"),
			predictionExists: true,
			expectError:      false,
			expectUpdate:     false,
			setupMocks: func(mp *mockPredictionRepo, mc *mockCategoryRepo) {
				prediction := createTestPrediction(predictionId, budgetId, txnId)
				category := createTestCategory(categoryId, budgetId, "Test Category")

				mp.On("GetByTxnId", mock.Anything, budgetId, txnId).Return(prediction, nil)
				mc.On("GetByIdSimplified", mock.Anything, budgetId, categoryId).Return(category, nil)
			},
		},
		{
			name:              "prediction_found_user_correction_needed",
			account:           *createTestAccount(accountId, budgetId, "Different Account"),
			payee:             *createTestPayee(payeeId, budgetId, "Different Payee"),
			predictionExists:  true,
			expectError:       false,
			expectUpdate:      true,
			userCorrectedData: true,
			setupMocks: func(mp *mockPredictionRepo, mc *mockCategoryRepo) {
				prediction := createTestPrediction(predictionId, budgetId, txnId)
				category := createTestCategory(categoryId, budgetId, "Different Category")

				mp.On("GetByTxnId", mock.Anything, budgetId, txnId).Return(prediction, nil)
				mc.On("GetByIdSimplified", mock.Anything, budgetId, categoryId).Return(category, nil)
				mp.On("Update", mock.Anything, budgetId, predictionId, mock.MatchedBy(func(p model.Prediction) bool {
					return p.HasUserCorrected != nil && *p.HasUserCorrected &&
						p.UserCorrectedAccount != nil && *p.UserCorrectedAccount == "Different Account" &&
						p.UserCorrectedPayee != nil && *p.UserCorrectedPayee == "Different Payee" &&
						p.UserCorrectedCategory != nil && *p.UserCorrectedCategory == "Different Category"
				})).
					Return(nil)
			},
		},
		{
			name:             "prediction_repo_error",
			account:          *createTestAccount(accountId, budgetId, "Test Account"),
			payee:            *createTestPayee(payeeId, budgetId, "Test Payee"),
			predictionExists: false,
			expectError:      true,
			expectUpdate:     false,
			setupMocks: func(mp *mockPredictionRepo, mc *mockCategoryRepo) {
				mp.On("GetByTxnId", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, assert.AnError)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockPrediction := &mockPredictionRepo{}
			mockCategory := &mockCategoryRepo{}
			mockMonthlyBudget := &mockMonthlyBudgetRepo{}

			service := setupTransactionTestableService(mockPrediction, nil, nil, mockCategory, mockMonthlyBudget)

			txn := model.Transaction{
				AccountID:  &accountId,
				PayeeID:    &payeeId,
				CategoryID: &categoryId,
			}

			ctx := context.Background()

			tt.setupMocks(mockPrediction, mockCategory)

			// Act
			err := service.updatePrediction(ctx, budgetId, txnId, txn, tt.account, tt.payee)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify all expectations were met
			mockPrediction.AssertExpectations(t)
			mockCategory.AssertExpectations(t)
		})
	}
}

func TestUpdateCarryovers(t *testing.T) {
	budgetId, txnId, accountId, payeeId, categoryId, _, inflowCategoryID := createTestUUIDs()
	_, _, _, _, newCategoryId, _, _ := createTestUUIDs()

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
					budgetId,
					categoryId,
					"2025-01",
				).Return(&model.MonthlyBudget{}, nil).Once()
				mb.On(
					"GetByCatIdAndMonth",
					mock.Anything,
					budgetId,
					newCategoryId,
					"2025-02",
				).Return(&model.MonthlyBudget{}, nil).Once()
				mb.On(
					"UpdateCarryoverByCatIdAndMonth",
					mock.Anything,
					budgetId,
					categoryId,
					"2025-01",
					1000.00,
				).Return(nil).Once()
				mb.On(
					"UpdateCarryoverByCatIdAndMonth",
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
					budgetId,
					categoryId,
					"2025-01",
				).Return(&model.MonthlyBudget{}, nil).Once()
				mb.On(
					"UpdateCarryoverByCatIdAndMonth",
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
		{
			name:       "same_inflow_category_ignored",
			setupMocks: func(mb *mockMonthlyBudgetRepo) {},
			existingTxn: &model.Transaction{
				ID:         txnId,
				AccountID:  &accountId,
				PayeeID:    &payeeId,
				CategoryID: &inflowCategoryID,
				Amount:     1000.00,
				Date:       "2025-01-01",
			},
			newTxn: model.Transaction{
				ID:         txnId,
				AccountID:  &accountId,
				PayeeID:    &payeeId,
				CategoryID: &inflowCategoryID,
				Amount:     1500.00,
				Date:       "2025-01-01",
			},
			expectError:  false,
			expectUpdate: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPrediction := &mockPredictionRepo{}
			mockAccount := &mockAccountRepo{}
			mockPayees := &mockPayeesRepo{}
			mockCategory := &mockCategoryRepo{}
			mockMonthlyBudget := &mockMonthlyBudgetRepo{}

			service := setupTransactionTestableService(
				mockPrediction,
				mockAccount,
				mockPayees,
				mockCategory,
				mockMonthlyBudget,
			)

			ctx := context.Background()

			tt.setupMocks(mockMonthlyBudget)

			err := service.mbService.UpdateCarryovers(
				ctx,
				budgetId,
				tt.existingTxn,
				&tt.newTxn,
				inflowCategoryID,
			)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCreateCounterpartTxn(t *testing.T) {
	ctx := context.Background()

	budgetId := uuid.New()
	parentId := uuid.New()
	accountId := uuid.New()
	transferAccountID := uuid.New()
	transferPayeeID := uuid.New()
	transferTxnID := uuid.New()

	txn := model.Transaction{
		AccountID: &accountId,
		Amount:    -42.50,
		Date:      "2025-01-15",
	}

	payee := model.Payee{TransferAccountID: &transferAccountID}

	t.Run("wraps transfer create failures", func(t *testing.T) {
		mockTransaction := &mockTransactionRepo{}
		service := newTestTransactionService(mockTransaction, nil, nil, nil, nil, nil, nil)

		account := model.Account{TransferPayeeID: &transferPayeeID}

		mockTransaction.On(
			"Create",
			mock.Anything,
			mock.MatchedBy(func(txn model.Transaction) bool {
				return txn.PayeeID != nil && *txn.PayeeID == transferPayeeID
			}),
		).Return(nil, assert.AnError).Once()

		createdId, err := service.createCounterpartTxn(ctx, budgetId, parentId, txn, account, payee)

		assert.Equal(t, uuid.Nil, createdId)
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

		mockTransaction.On("Create", mock.Anything, mock.Anything).
			Return([]model.Transaction{}, nil).
			Once()

		createdId, err := service.createCounterpartTxn(ctx, budgetId, parentId, txn, account, payee)

		assert.Equal(t, uuid.Nil, createdId)
		require.Error(t, err)

		var appErr *errs.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, errs.CodeTransferNotCreated, appErr.Code)
		mockTransaction.AssertExpectations(t)
	})

	t.Run("creates counterpart with correct fields", func(t *testing.T) {
		mockTransaction := &mockTransactionRepo{}
		service := newTestTransactionService(mockTransaction, nil, nil, nil, nil, nil, nil)

		account := model.Account{TransferPayeeID: &transferPayeeID}

		mockTransaction.On(
			"Create",
			mock.Anything,
			mock.MatchedBy(func(created model.Transaction) bool {
				return created.AccountID != nil && *created.AccountID == transferAccountID &&
					created.PayeeID != nil && *created.PayeeID == transferPayeeID &&
					created.TransferTransactionID != nil && *created.TransferTransactionID == parentId &&
					created.Amount == 42.50
			}),
		).Return([]model.Transaction{{ID: transferTxnID}}, nil).Once()

		createdId, err := service.createCounterpartTxn(ctx, budgetId, parentId, txn, account, payee)

		require.NoError(t, err)
		assert.Equal(t, transferTxnID, createdId)
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
		newTxn := model.Transaction{
			Date:   "2025-01-01",
			Amount: 100,
		}
		err := service.applySideEffects(ctx, sideEffectInput{
			budgetId: budgetId,
			newTxn:   &newTxn,
			budget:   &budget,
		})

		require.NoError(t, err)
	})

	t.Run("inflow category skips carryover work", func(t *testing.T) {
		newTxn := model.Transaction{
			Date:       "2025-01-01",
			Amount:     100,
			CategoryID: &inflowCategoryID,
		}
		err := service.applySideEffects(ctx, sideEffectInput{
			budgetId: budgetId,
			newTxn:   &newTxn,
			budget:   &budget,
		})

		require.NoError(t, err)
	})

	monthlyBudgetRepo.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: UpdateStatus — missing branches
// ─────────────────────────────────────────────────────────────────────────────

func TestUpdateStatus_GetByIdError_ReturnsError(t *testing.T) {
	budgetId, txnId := uuid.New(), uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budgetId)

	txnRepo := &mockTransactionRepo{}
	svc := &transactionService{dbTxn: fakeDBTxn{}, repo: txnRepo}

	txnRepo.On("GetById", mock.Anything, budgetId, txnId).Return(nil, assert.AnError).Once()

	err := svc.UpdateStatus(ctx, txnId, model.TransactionStatusApproved)
	require.Error(t, err)
	txnRepo.AssertExpectations(t)
}

func TestUpdateStatus_NotFoundNilTxn_ReturnsError(t *testing.T) {
	budgetId, txnId := uuid.New(), uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budgetId)

	txnRepo := &mockTransactionRepo{}
	svc := &transactionService{dbTxn: fakeDBTxn{}, repo: txnRepo}

	txnRepo.On("GetById", mock.Anything, budgetId, txnId).Return((*model.Transaction)(nil), nil).Once()

	err := svc.UpdateStatus(ctx, txnId, model.TransactionStatusApproved)
	require.Error(t, err)
	var appErr *errs.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeTransactionLookupFailed, appErr.Code)
	txnRepo.AssertExpectations(t)
}

func TestUpdateStatus_NilCipherPredictionRepo_ReturnsError(t *testing.T) {
	budgetId, txnId := uuid.New(), uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budgetId)
	payeeId := uuid.New()
	categoryId := uuid.New()
	rawText := "some text"

	txnRepo := &mockTransactionRepo{}
	svc := &transactionService{dbTxn: fakeDBTxn{},
		repo:                 txnRepo,
		cipherPredictionRepo: nil, // explicitly nil
	}

	txn := &model.Transaction{
		ID:          txnId,
		BudgetID:    budgetId,
		PayeeID:     &payeeId,
		CategoryID:  &categoryId,
		RawBankText: &rawText,
	}
	txnRepo.On("GetById", mock.Anything, budgetId, txnId).Return(txn, nil).Once()

	err := svc.UpdateStatus(ctx, txnId, model.TransactionStatusApproved)
	require.Error(t, err)
	var appErr *errs.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInternalError, appErr.Code)
	txnRepo.AssertExpectations(t)
}

func TestUpdateStatus_CipherPredictionGetError_ReturnsError(t *testing.T) {
	budgetId, txnId := uuid.New(), uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budgetId)
	payeeId := uuid.New()
	categoryId := uuid.New()
	rawText := "some text"

	txnRepo := &mockTransactionRepo{}
	cipherPredictionRepo := &mockCipherPredictionRepo{}
	svc := &transactionService{dbTxn: fakeDBTxn{}, repo: txnRepo, cipherPredictionRepo: cipherPredictionRepo}

	txn := &model.Transaction{
		ID:          txnId,
		BudgetID:    budgetId,
		PayeeID:     &payeeId,
		CategoryID:  &categoryId,
		RawBankText: &rawText,
	}
	txnRepo.On("GetById", mock.Anything, budgetId, txnId).Return(txn, nil).Once()
	cipherPredictionRepo.On("GetByTransactionID", mock.Anything, budgetId, txnId).Return(nil, assert.AnError).Once()

	err := svc.UpdateStatus(ctx, txnId, model.TransactionStatusApproved)
	require.Error(t, err)
	txnRepo.AssertExpectations(t)
	cipherPredictionRepo.AssertExpectations(t)
}

func TestUpdateStatus_NilPayeeID_ReturnsError(t *testing.T) {
	budgetId, txnId := uuid.New(), uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budgetId)
	categoryId := uuid.New()
	rawText := "some text"

	txnRepo := &mockTransactionRepo{}
	cipherPredictionRepo := &mockCipherPredictionRepo{}
	svc := &transactionService{dbTxn: fakeDBTxn{}, repo: txnRepo, cipherPredictionRepo: cipherPredictionRepo}

	txn := &model.Transaction{
		ID:          txnId,
		BudgetID:    budgetId,
		PayeeID:     nil, // nil payee
		CategoryID:  &categoryId,
		RawBankText: &rawText,
	}
	txnRepo.On("GetById", mock.Anything, budgetId, txnId).Return(txn, nil).Once()
	cipherPredictionRepo.On("GetByTransactionID", mock.Anything, budgetId, txnId).Return(&model.CipherPredictionRecord{}, nil).Once()

	err := svc.UpdateStatus(ctx, txnId, model.TransactionStatusApproved)
	require.Error(t, err)
	var appErr *errs.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidArgument, appErr.Code)
}

func TestUpdateStatus_NilCategoryID_ReturnsError(t *testing.T) {
	budgetId, txnId := uuid.New(), uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budgetId)
	payeeId := uuid.New()
	rawText := "some text"

	txnRepo := &mockTransactionRepo{}
	cipherPredictionRepo := &mockCipherPredictionRepo{}
	svc := &transactionService{dbTxn: fakeDBTxn{}, repo: txnRepo, cipherPredictionRepo: cipherPredictionRepo}

	txn := &model.Transaction{
		ID:          txnId,
		BudgetID:    budgetId,
		PayeeID:     &payeeId,
		CategoryID:  nil, // nil category
		RawBankText: &rawText,
	}
	txnRepo.On("GetById", mock.Anything, budgetId, txnId).Return(txn, nil).Once()
	cipherPredictionRepo.On("GetByTransactionID", mock.Anything, budgetId, txnId).Return(&model.CipherPredictionRecord{}, nil).Once()

	err := svc.UpdateStatus(ctx, txnId, model.TransactionStatusApproved)
	require.Error(t, err)
	var appErr *errs.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidArgument, appErr.Code)
}

func TestUpdateStatus_NilRawBankText_ReturnsError(t *testing.T) {
	budgetId, txnId := uuid.New(), uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budgetId)
	payeeId := uuid.New()
	categoryId := uuid.New()

	txnRepo := &mockTransactionRepo{}
	cipherPredictionRepo := &mockCipherPredictionRepo{}
	svc := &transactionService{dbTxn: fakeDBTxn{}, repo: txnRepo, cipherPredictionRepo: cipherPredictionRepo}

	txn := &model.Transaction{
		ID:          txnId,
		BudgetID:    budgetId,
		PayeeID:     &payeeId,
		CategoryID:  &categoryId,
		RawBankText: nil, // nil raw bank text
	}
	txnRepo.On("GetById", mock.Anything, budgetId, txnId).Return(txn, nil).Once()
	cipherPredictionRepo.On("GetByTransactionID", mock.Anything, budgetId, txnId).Return(&model.CipherPredictionRecord{}, nil).Once()

	err := svc.UpdateStatus(ctx, txnId, model.TransactionStatusApproved)
	require.Error(t, err)
	var appErr *errs.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidArgument, appErr.Code)
}

func TestUpdateStatus_BlankRawBankText_ReturnsError(t *testing.T) {
	budgetId, txnId := uuid.New(), uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budgetId)
	payeeId := uuid.New()
	categoryId := uuid.New()
	blank := "  "

	txnRepo := &mockTransactionRepo{}
	cipherPredictionRepo := &mockCipherPredictionRepo{}
	svc := &transactionService{dbTxn: fakeDBTxn{}, repo: txnRepo, cipherPredictionRepo: cipherPredictionRepo}

	txn := &model.Transaction{
		ID:          txnId,
		BudgetID:    budgetId,
		PayeeID:     &payeeId,
		CategoryID:  &categoryId,
		RawBankText: &blank,
	}
	txnRepo.On("GetById", mock.Anything, budgetId, txnId).Return(txn, nil).Once()
	cipherPredictionRepo.On("GetByTransactionID", mock.Anything, budgetId, txnId).Return(&model.CipherPredictionRecord{}, nil).Once()

	err := svc.UpdateStatus(ctx, txnId, model.TransactionStatusApproved)
	require.Error(t, err)
	var appErr *errs.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, errs.CodeInvalidArgument, appErr.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: applySideEffects — isDelete branches
// ─────────────────────────────────────────────────────────────────────────────

func TestApplySideEffects_Delete_NilCategory_Skips(t *testing.T) {
	ctx := context.Background()
	budgetId := uuid.New()
	inflowCategoryID := uuid.New()
	monthlyBudgetRepo := &mockMonthlyBudgetRepo{}
	service := newTestTransactionService(nil, nil, nil, nil, nil, nil, monthlyBudgetRepo)

	budget := model.Budget{
		Metadata: model.BudgetMetadata{InflowCategoryID: inflowCategoryID},
	}

	oldTxn := model.Transaction{
		Date:       "2025-01-01",
		Amount:     100,
		CategoryID: nil, // nil → skip UpsertCarryover
	}
	err := service.applySideEffects(ctx, sideEffectInput{
		budgetId: budgetId,
		oldTxn:   &oldTxn,
		budget:   &budget,
	})
	require.NoError(t, err)
	monthlyBudgetRepo.AssertExpectations(t)
}

func TestApplySideEffects_Delete_InflowCategory_Skips(t *testing.T) {
	ctx := context.Background()
	budgetId := uuid.New()
	inflowCategoryID := uuid.New()
	monthlyBudgetRepo := &mockMonthlyBudgetRepo{}
	service := newTestTransactionService(nil, nil, nil, nil, nil, nil, monthlyBudgetRepo)

	budget := model.Budget{
		Metadata: model.BudgetMetadata{InflowCategoryID: inflowCategoryID},
	}

	oldTxn := model.Transaction{
		Date:       "2025-01-01",
		Amount:     100,
		CategoryID: &inflowCategoryID, // matches inflow → skip
	}
	err := service.applySideEffects(ctx, sideEffectInput{
		budgetId: budgetId,
		oldTxn:   &oldTxn,
		budget:   &budget,
	})
	require.NoError(t, err)
	monthlyBudgetRepo.AssertExpectations(t)
}

// Approving a prediction that was already right is a confirmation, not a
// correction: it must still teach the pipeline (payee rule + embedding) but
// must not mark the prediction user-corrected.
func TestUpdateApprovingUnchangedPredictionDoesNotMarkCorrected(t *testing.T) {
	budgetId, txnId, accountId, payeeId, categoryId, _, inflowCategoryID := createTestUUIDs()
	ctx := utils.WithBudgetID(context.Background(), budgetId)
	rawBankText := "paid to less n more the pasta shop"
	note := "dinner"

	foundTxn := model.Transaction{
		ID:          txnId,
		BudgetID:    budgetId,
		AccountID:   &accountId,
		PayeeID:     &payeeId,
		CategoryID:  &categoryId,
		Amount:      -450,
		Date:        "2023-01-01",
		Status:      model.TransactionStatusUnapproved,
		RawBankText: &rawBankText,
	}
	// Same payee and category -- the only thing the user changed is that they
	// accepted it (plus an unrelated note, so the update is not a no-op).
	toUpdate := model.Transaction{
		BudgetID:   budgetId,
		AccountID:  &accountId,
		PayeeID:    &payeeId,
		CategoryID: &categoryId,
		Amount:     -450,
		Date:       "2023-01-01",
		Note:       note,
	}

	txnRepo := &mockTransactionRepo{}
	budgetRepo := &mockBudgetRepo{}
	accountRepo := &mockAccountRepo{}
	payeeRepo := &mockPayeesRepo{}
	monthlyBudgetRepo := &mockMonthlyBudgetRepo{}
	cipherPredictionRepo := &mockCipherPredictionRepo{}
	cipherClient := &mockCipherClient{}
	payeeRuleRepo := &mockPayeeRuleRepo{}
	txnEmbeddingRepo := &mockTransactionEmbeddingRepo{}
	svc := &transactionService{dbTxn: fakeDBTxn{},
		repo:                 txnRepo,
		budgetRepo:           budgetRepo,
		txnEmbeddingRepo:     txnEmbeddingRepo,
		cipherClient:         cipherClient,
		cipherPredictionRepo: cipherPredictionRepo,
		payeeRuleRepo:        payeeRuleRepo,
		accountRepo:          accountRepo,
		payeeRepo:            payeeRepo,
		mbService:            NewMonthlyBudgetService(monthlyBudgetRepo),
	}

	txnRepo.On("GetById", mock.Anything, budgetId, txnId).Return(&foundTxn, nil).Once()
	budgetRepo.On("GetById", mock.Anything, budgetId).Return(&model.Budget{
		ID:       budgetId,
		Metadata: model.BudgetMetadata{InflowCategoryID: inflowCategoryID},
	}, nil).Once()
	accountRepo.On("GetById", mock.Anything, budgetId, accountId).
		Return(&model.Account{ID: accountId, BudgetID: budgetId, Type: "checking"}, nil).
		Once()
	payeeRepo.On("GetById", mock.Anything, budgetId, payeeId).
		Return(&model.Payee{ID: payeeId, BudgetID: budgetId}, nil).
		Once()
	cipherPredictionRepo.On("GetByTransactionID", mock.Anything, budgetId, txnId).Return(&model.CipherPredictionRecord{
		BudgetID:      budgetId,
		TransactionID: txnId,
		Source:        model.PredictionSourceLLM,
	}, nil).Once()
	txnRepo.On("Update", mock.Anything, budgetId, txnId, mock.MatchedBy(func(txn model.Transaction) bool {
		return txn.ID == txnId && txn.Status == model.TransactionStatusApproved
	})).Return(nil).Once()

	err := svc.Update(ctx, txnId, toUpdate)

	require.NoError(t, err)
	cipherPredictionRepo.AssertNotCalled(t, "MarkUserCorrected",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	txnRepo.AssertExpectations(t)
	cipherPredictionRepo.AssertExpectations(t)
	cipherClient.AssertNotCalled(t, "GenerateTransactionEmbedding", mock.Anything, mock.Anything)
}

func TestBackfillLearningRunsInBackgroundAndRecordsFailures(t *testing.T) {
	budgetId, txnId, _, payeeId, categoryId, _, _ := createTestUUIDs()
	failingTxnId := uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budgetId)

	txnRepo := &mockTransactionRepo{}
	cipherPredictionRepo := &mockCipherPredictionRepo{}
	cipherClient := &mockCipherClient{}
	payeeRuleRepo := &mockPayeeRuleRepo{}
	txnEmbeddingRepo := &mockTransactionEmbeddingRepo{}
	svc := &transactionService{dbTxn: fakeDBTxn{},
		repo:                 txnRepo,
		txnEmbeddingRepo:     txnEmbeddingRepo,
		cipherClient:         cipherClient,
		cipherPredictionRepo: cipherPredictionRepo,
		payeeRuleRepo:        payeeRuleRepo,
	}

	// Default limit applies when the request leaves it unset.
	cipherPredictionRepo.On("ListPendingLearning", mock.Anything, budgetId, defaultLearningBackfillLimit, false).
		Return([]model.LearningCandidate{
			{
				TransactionID: txnId,
				PayeeID:       &payeeId,
				CategoryID:    &categoryId,
				Amount:        -450,
				RawBankText:   "paid to grocery store",
			},
			{
				TransactionID: failingTxnId,
				PayeeID:       &payeeId,
				CategoryID:    &categoryId,
				Amount:        -100,
				RawBankText:   "paid to broken merchant",
			},
		}, nil).Once()
	cipherPredictionRepo.On("LearningStats", mock.Anything, budgetId).
		Return(model.LearningStats{Total: 10, Learned: 8, Pending: 2, Failed: 1}, nil).Once()

	cipherClient.On("GenerateTransactionEmbedding", mock.Anything, TransactionEmbeddingRequest{
		RawBankText: "paid to grocery store",
		Amount:      -450,
	}).Return(&TransactionEmbeddingResponse{
		MatchString:   "grocery store",
		EmbeddingText: "debit grocery store",
		Embedding:     "[0.1,0.2]",
	}, nil).Once()
	payeeRuleRepo.On("CreatePayeeRule", mock.Anything, mock.Anything).Return(nil).Once()
	txnEmbeddingRepo.On("Upsert", mock.Anything, mock.Anything, []float64{0.1, 0.2}).Return(nil).Once()
	cipherPredictionRepo.On("MarkLearned", mock.Anything, budgetId, txnId).Return(nil).Once()

	// The second one fails upstream: the run must continue and record why.
	done := make(chan struct{})
	cipherClient.On("GenerateTransactionEmbedding", mock.Anything, TransactionEmbeddingRequest{
		RawBankText: "paid to broken merchant",
		Amount:      -100,
	}).Return(nil, errors.New("ollama unreachable")).Once()
	cipherPredictionRepo.On("MarkLearnFailed", mock.Anything, budgetId, failingTxnId, mock.MatchedBy(func(reason string) bool {
		return strings.Contains(reason, "ollama unreachable")
	})).Return(nil).Run(func(args mock.Arguments) {
		close(done)
	}).Once()

	result, err := svc.BackfillLearning(ctx, model.LearningBackfillRequest{})

	// The call returns immediately; the work is still going.
	require.NoError(t, err)
	require.True(t, result.Started)
	require.True(t, result.Running)
	require.Equal(t, 2, result.Queued)
	require.Equal(t, 2, result.Remaining)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the backfill to finish")
	}

	cipherPredictionRepo.AssertExpectations(t)
	cipherClient.AssertExpectations(t)
	payeeRuleRepo.AssertExpectations(t)
	txnEmbeddingRepo.AssertExpectations(t)
}

// A second request while a run is in flight must not queue the same page twice:
// the items are the same and each one costs two LLM round-trips.
func TestBackfillLearningSingleFlights(t *testing.T) {
	budgetId, txnId, _, payeeId, categoryId, _, _ := createTestUUIDs()
	ctx := utils.WithBudgetID(context.Background(), budgetId)

	cipherPredictionRepo := &mockCipherPredictionRepo{}
	svc := &transactionService{dbTxn: fakeDBTxn{}, cipherPredictionRepo: cipherPredictionRepo}
	svc.learningBackfillRunning.Store(true)

	cipherPredictionRepo.On("ListPendingLearning", mock.Anything, budgetId, defaultLearningBackfillLimit, false).
		Return([]model.LearningCandidate{{
			TransactionID: txnId,
			PayeeID:       &payeeId,
			CategoryID:    &categoryId,
			RawBankText:   "paid to grocery store",
		}}, nil).Once()
	cipherPredictionRepo.On("LearningStats", mock.Anything, budgetId).
		Return(model.LearningStats{Pending: 1}, nil).Once()

	result, err := svc.BackfillLearning(ctx, model.LearningBackfillRequest{})

	require.NoError(t, err)
	require.False(t, result.Started)
	require.True(t, result.Running)
	require.Equal(t, 0, result.Queued)
	cipherPredictionRepo.AssertExpectations(t)
}

func TestBackfillLearningClampsLimit(t *testing.T) {
	budgetId, _, _, _, _, _, _ := createTestUUIDs()
	ctx := utils.WithBudgetID(context.Background(), budgetId)

	cipherPredictionRepo := &mockCipherPredictionRepo{}
	svc := &transactionService{dbTxn: fakeDBTxn{}, cipherPredictionRepo: cipherPredictionRepo}

	cipherPredictionRepo.On("ListPendingLearning", mock.Anything, budgetId, maxLearningBackfillLimit, true).
		Return([]model.LearningCandidate{}, nil).Once()
	cipherPredictionRepo.On("LearningStats", mock.Anything, budgetId).
		Return(model.LearningStats{}, nil).Once()

	// Nothing pending: no run starts, and the caller is told so.
	result, err := svc.BackfillLearning(ctx, model.LearningBackfillRequest{Limit: 5000, RetryFailed: true})

	require.NoError(t, err)
	require.False(t, result.Started)
	require.Equal(t, 0, result.Queued)
	cipherPredictionRepo.AssertExpectations(t)
}

func TestLearningStatusReportsRunning(t *testing.T) {
	budgetId, _, _, _, _, _, _ := createTestUUIDs()
	ctx := utils.WithBudgetID(context.Background(), budgetId)

	cipherPredictionRepo := &mockCipherPredictionRepo{}
	svc := &transactionService{dbTxn: fakeDBTxn{}, cipherPredictionRepo: cipherPredictionRepo}
	svc.learningBackfillRunning.Store(true)

	cipherPredictionRepo.On("LearningStats", mock.Anything, budgetId).
		Return(model.LearningStats{Total: 3, Learned: 1, Pending: 2}, nil).Once()

	stats, err := svc.LearningStatus(ctx)

	require.NoError(t, err)
	require.True(t, stats.Running)
	require.Equal(t, 2, stats.Pending)
}
