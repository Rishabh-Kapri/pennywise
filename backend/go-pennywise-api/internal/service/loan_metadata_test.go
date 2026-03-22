package service

import (
	"context"
	"testing"
	"time"

	"pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock repository
type mockLoanMetadataRepo struct {
	mock.Mock
}

func (m *mockLoanMetadataRepo) GetAllByBudgetId(ctx context.Context, budgetId uuid.UUID) ([]model.LoanMetadata, error) {
	args := m.Called(ctx, budgetId)
	if obj := args.Get(0); obj != nil {
		return obj.([]model.LoanMetadata), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockLoanMetadataRepo) GetByAccountId(ctx context.Context, accountId uuid.UUID) (*model.LoanMetadata, error) {
	args := m.Called(ctx, accountId)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.LoanMetadata), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockLoanMetadataRepo) Create(ctx context.Context, loan model.LoanMetadata) (*model.LoanMetadata, error) {
	args := m.Called(ctx, loan)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.LoanMetadata), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockLoanMetadataRepo) Update(ctx context.Context, accountId uuid.UUID, loan model.LoanMetadata) (*model.LoanMetadata, error) {
	args := m.Called(ctx, accountId, loan)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.LoanMetadata), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockLoanMetadataRepo) Delete(ctx context.Context, accountId uuid.UUID) error {
	args := m.Called(ctx, accountId)
	return args.Error(0)
}

// Test helpers
func createTestLoanMetadata(accountId uuid.UUID, categoryId *uuid.UUID) model.LoanMetadata {
	return model.LoanMetadata{
		ID:              uuid.New(),
		AccountID:       accountId,
		InterestRate:    5.25,
		OriginalBalance: 25000.00,
		MonthlyPayment:  450.00,
		LoanStartDate:   "2024-01-15",
		CategoryID:      categoryId,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

func TestLoanMetadataService_GetAll(t *testing.T) {
	budgetId := uuid.New()
	accountId1 := uuid.New()
	accountId2 := uuid.New()
	catId := uuid.New()

	tests := []struct {
		name        string
		setupMocks  func(*mockLoanMetadataRepo)
		setupCtx    func() context.Context
		expectError bool
		expectCount int
	}{
		{
			name: "returns_all_loans_for_budget",
			setupCtx: func() context.Context {
				return context.WithValue(context.Background(), "budgetId", budgetId)
			},
			setupMocks: func(m *mockLoanMetadataRepo) {
				loans := []model.LoanMetadata{
					createTestLoanMetadata(accountId1, &catId),
					createTestLoanMetadata(accountId2, nil),
				}
				m.On("GetAllByBudgetId", mock.Anything, budgetId).Return(loans, nil)
			},
			expectError: false,
			expectCount: 2,
		},
		{
			name: "returns_empty_slice_when_no_loans",
			setupCtx: func() context.Context {
				return context.WithValue(context.Background(), "budgetId", budgetId)
			},
			setupMocks: func(m *mockLoanMetadataRepo) {
				m.On("GetAllByBudgetId", mock.Anything, budgetId).Return([]model.LoanMetadata{}, nil)
			},
			expectError: false,
			expectCount: 0,
		},
		{
			name: "repo_error_propagates",
			setupCtx: func() context.Context {
				return context.WithValue(context.Background(), "budgetId", budgetId)
			},
			setupMocks: func(m *mockLoanMetadataRepo) {
				m.On("GetAllByBudgetId", mock.Anything, budgetId).Return(nil, assert.AnError)
			},
			expectError: true,
			expectCount: 0,
		},
		{
			name: "missing_budget_id_in_context_uses_zero_uuid",
			setupCtx: func() context.Context {
				return context.Background()
			},
			setupMocks: func(m *mockLoanMetadataRepo) {
				m.On("GetAllByBudgetId", mock.Anything, uuid.UUID{}).Return([]model.LoanMetadata{}, nil)
			},
			expectError: false,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockRepo := &mockLoanMetadataRepo{}
			service := NewLoanMetadataService(mockRepo)
			ctx := tt.setupCtx()
			tt.setupMocks(mockRepo)

			// Act
			loans, err := service.GetAll(ctx)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, loans, tt.expectCount)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestLoanMetadataService_GetByAccountId(t *testing.T) {
	accountId := uuid.New()
	catId := uuid.New()

	tests := []struct {
		name        string
		setupMocks  func(*mockLoanMetadataRepo)
		expectError bool
		expectNil   bool
	}{
		{
			name: "returns_loan_for_account",
			setupMocks: func(m *mockLoanMetadataRepo) {
				loan := createTestLoanMetadata(accountId, &catId)
				m.On("GetByAccountId", mock.Anything, accountId).Return(&loan, nil)
			},
			expectError: false,
			expectNil:   false,
		},
		{
			name: "repo_error_propagates",
			setupMocks: func(m *mockLoanMetadataRepo) {
				m.On("GetByAccountId", mock.Anything, accountId).Return(nil, assert.AnError)
			},
			expectError: true,
			expectNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockLoanMetadataRepo{}
			service := NewLoanMetadataService(mockRepo)
			ctx := context.Background()
			tt.setupMocks(mockRepo)

			loan, err := service.GetByAccountId(ctx, accountId)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.expectNil {
				assert.Nil(t, loan)
			} else {
				assert.NotNil(t, loan)
				assert.Equal(t, accountId, loan.AccountID)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestLoanMetadataService_Create(t *testing.T) {
	accountId := uuid.New()
	catId := uuid.New()

	tests := []struct {
		name        string
		setupMocks  func(*mockLoanMetadataRepo)
		input       model.LoanMetadata
		expectError bool
	}{
		{
			name: "creates_loan_successfully",
			input: model.LoanMetadata{
				AccountID:       accountId,
				InterestRate:    6.5,
				OriginalBalance: 30000.00,
				MonthlyPayment:  500.00,
				LoanStartDate:   "2024-03-01",
				CategoryID:      &catId,
			},
			setupMocks: func(m *mockLoanMetadataRepo) {
				m.On("Create", mock.Anything, mock.MatchedBy(func(l model.LoanMetadata) bool {
					return l.AccountID == accountId && l.InterestRate == 6.5
				})).Return(&model.LoanMetadata{
					ID:              uuid.New(),
					AccountID:       accountId,
					InterestRate:    6.5,
					OriginalBalance: 30000.00,
					MonthlyPayment:  500.00,
					LoanStartDate:   "2024-03-01",
					CategoryID:      &catId,
					CreatedAt:       time.Now(),
					UpdatedAt:       time.Now(),
				}, nil)
			},
			expectError: false,
		},
		{
			name: "repo_error_propagates",
			input: model.LoanMetadata{
				AccountID: accountId,
			},
			setupMocks: func(m *mockLoanMetadataRepo) {
				m.On("Create", mock.Anything, mock.Anything).Return(nil, assert.AnError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockLoanMetadataRepo{}
			service := NewLoanMetadataService(mockRepo)
			ctx := context.Background()
			tt.setupMocks(mockRepo)

			result, err := service.Create(ctx, tt.input)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, accountId, result.AccountID)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestLoanMetadataService_Update(t *testing.T) {
	accountId := uuid.New()

	tests := []struct {
		name        string
		setupMocks  func(*mockLoanMetadataRepo)
		input       model.LoanMetadata
		expectError bool
	}{
		{
			name: "updates_loan_successfully",
			input: model.LoanMetadata{
				InterestRate:    4.0,
				OriginalBalance: 20000.00,
				MonthlyPayment:  400.00,
				LoanStartDate:   "2024-06-01",
			},
			setupMocks: func(m *mockLoanMetadataRepo) {
				m.On("Update", mock.Anything, accountId, mock.MatchedBy(func(l model.LoanMetadata) bool {
					return l.InterestRate == 4.0 && l.MonthlyPayment == 400.00
				})).Return(&model.LoanMetadata{
					ID:              uuid.New(),
					AccountID:       accountId,
					InterestRate:    4.0,
					OriginalBalance: 20000.00,
					MonthlyPayment:  400.00,
					LoanStartDate:   "2024-06-01",
					CreatedAt:       time.Now(),
					UpdatedAt:       time.Now(),
				}, nil)
			},
			expectError: false,
		},
		{
			name: "repo_error_propagates",
			input: model.LoanMetadata{
				InterestRate: 4.0,
			},
			setupMocks: func(m *mockLoanMetadataRepo) {
				m.On("Update", mock.Anything, accountId, mock.Anything).Return(nil, assert.AnError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockLoanMetadataRepo{}
			service := NewLoanMetadataService(mockRepo)
			ctx := context.Background()
			tt.setupMocks(mockRepo)

			result, err := service.Update(ctx, accountId, tt.input)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, accountId, result.AccountID)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestLoanMetadataService_Delete(t *testing.T) {
	accountId := uuid.New()

	tests := []struct {
		name        string
		setupMocks  func(*mockLoanMetadataRepo)
		expectError bool
	}{
		{
			name: "deletes_loan_successfully",
			setupMocks: func(m *mockLoanMetadataRepo) {
				m.On("Delete", mock.Anything, accountId).Return(nil)
			},
			expectError: false,
		},
		{
			name: "repo_error_propagates",
			setupMocks: func(m *mockLoanMetadataRepo) {
				m.On("Delete", mock.Anything, accountId).Return(assert.AnError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockLoanMetadataRepo{}
			service := NewLoanMetadataService(mockRepo)
			ctx := context.Background()
			tt.setupMocks(mockRepo)

			err := service.Delete(ctx, accountId)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}
