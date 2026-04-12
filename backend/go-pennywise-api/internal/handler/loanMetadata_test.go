package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock service
type mockLoanMetadataService struct {
	mock.Mock
}

func (m *mockLoanMetadataService) GetAll(ctx context.Context) ([]model.LoanMetadata, error) {
	args := m.Called(ctx)
	if obj := args.Get(0); obj != nil {
		return obj.([]model.LoanMetadata), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockLoanMetadataService) GetByAccountId(ctx context.Context, accountId uuid.UUID) (*model.LoanMetadata, error) {
	args := m.Called(ctx, accountId)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.LoanMetadata), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockLoanMetadataService) Create(ctx context.Context, loan model.LoanMetadata) (*model.LoanMetadata, error) {
	args := m.Called(ctx, loan)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.LoanMetadata), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockLoanMetadataService) Update(ctx context.Context, accountId uuid.UUID, loan model.LoanMetadata) (*model.LoanMetadata, error) {
	args := m.Called(ctx, accountId, loan)
	if obj := args.Get(0); obj != nil {
		return obj.(*model.LoanMetadata), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockLoanMetadataService) Delete(ctx context.Context, accountId uuid.UUID) error {
	args := m.Called(ctx, accountId)
	return args.Error(0)
}

// Test helpers
const budgetIdHeader = "X-Budget-ID"

func setupGinTestContext(method, path string, body interface{}) (*httptest.ResponseRecorder, *gin.Context) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	var req *http.Request
	if body != nil {
		jsonBytes, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	c.Request = req
	return w, c
}

func createTestLoanMetadata(accountId uuid.UUID, categoryId *uuid.UUID) *model.LoanMetadata {
	return &model.LoanMetadata{
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

// --- List handler tests ---

func TestLoanMetadataHandler_List(t *testing.T) {
	budgetId := uuid.New()
	accountId := uuid.New()
	catId := uuid.New()

	tests := []struct {
		name           string
		budgetIdHeader string
		setupMocks     func(*mockLoanMetadataService)
		expectedStatus int
	}{
		{
			name:           "returns_loans_successfully",
			budgetIdHeader: budgetId.String(),
			setupMocks: func(m *mockLoanMetadataService) {
				loans := []model.LoanMetadata{
					*createTestLoanMetadata(accountId, &catId),
				}
				m.On("GetAll", mock.Anything).Return(loans, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "returns_empty_list",
			budgetIdHeader: budgetId.String(),
			setupMocks: func(m *mockLoanMetadataService) {
				m.On("GetAll", mock.Anything).Return([]model.LoanMetadata{}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing_budget_id_returns_400",
			budgetIdHeader: "",
			setupMocks:     func(m *mockLoanMetadataService) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid_budget_id_returns_400",
			budgetIdHeader: "not-a-uuid",
			setupMocks:     func(m *mockLoanMetadataService) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "service_error_returns_500",
			budgetIdHeader: budgetId.String(),
			setupMocks: func(m *mockLoanMetadataService) {
				m.On("GetAll", mock.Anything).Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockLoanMetadataService{}
			handler := NewLoanMetadataHandler(mockService)

			w, c := setupGinTestContext("GET", "/api/loan-metadata", nil)
			if tt.budgetIdHeader != "" {
				c.Request.Header.Set(budgetIdHeader, tt.budgetIdHeader)
			}

			tt.setupMocks(mockService)

			handler.List(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockService.AssertExpectations(t)
		})
	}
}

// --- GetByAccountId handler tests ---

func TestLoanMetadataHandler_GetByAccountId(t *testing.T) {
	budgetId := uuid.New()
	accountId := uuid.New()
	catId := uuid.New()

	tests := []struct {
		name           string
		budgetIdHeader string
		accountIdParam string
		setupMocks     func(*mockLoanMetadataService)
		expectedStatus int
	}{
		{
			name:           "returns_loan_successfully",
			budgetIdHeader: budgetId.String(),
			accountIdParam: accountId.String(),
			setupMocks: func(m *mockLoanMetadataService) {
				loan := createTestLoanMetadata(accountId, &catId)
				m.On("GetByAccountId", mock.Anything, accountId).Return(loan, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing_budget_id_returns_400",
			budgetIdHeader: "",
			accountIdParam: accountId.String(),
			setupMocks:     func(m *mockLoanMetadataService) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid_account_id_returns_400",
			budgetIdHeader: budgetId.String(),
			accountIdParam: "not-a-uuid",
			setupMocks:     func(m *mockLoanMetadataService) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "not_found_returns_404",
			budgetIdHeader: budgetId.String(),
			accountIdParam: accountId.String(),
			setupMocks: func(m *mockLoanMetadataService) {
				m.On("GetByAccountId", mock.Anything, accountId).Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockLoanMetadataService{}
			handler := NewLoanMetadataHandler(mockService)

			w, c := setupGinTestContext("GET", "/api/loan-metadata/"+tt.accountIdParam, nil)
			if tt.budgetIdHeader != "" {
				c.Request.Header.Set(budgetIdHeader, tt.budgetIdHeader)
			}
			c.Params = gin.Params{{Key: "accountId", Value: tt.accountIdParam}}

			tt.setupMocks(mockService)

			handler.GetByAccountId(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockService.AssertExpectations(t)
		})
	}
}

// --- Create handler tests ---

func TestLoanMetadataHandler_Create(t *testing.T) {
	budgetId := uuid.New()
	accountId := uuid.New()
	catId := uuid.New()

	tests := []struct {
		name           string
		budgetIdHeader string
		body           interface{}
		setupMocks     func(*mockLoanMetadataService)
		expectedStatus int
	}{
		{
			name:           "creates_loan_successfully",
			budgetIdHeader: budgetId.String(),
			body: model.LoanMetadata{
				AccountID:       accountId,
				InterestRate:    5.25,
				OriginalBalance: 25000.00,
				MonthlyPayment:  450.00,
				LoanStartDate:   "2024-01-15",
				CategoryID:      &catId,
			},
			setupMocks: func(m *mockLoanMetadataService) {
				m.On("Create", mock.Anything, mock.MatchedBy(func(l model.LoanMetadata) bool {
					return l.AccountID == accountId && l.InterestRate == 5.25
				})).Return(createTestLoanMetadata(accountId, &catId), nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "missing_budget_id_returns_400",
			budgetIdHeader: "",
			body: model.LoanMetadata{
				AccountID: accountId,
			},
			setupMocks:     func(m *mockLoanMetadataService) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid_json_body_returns_400",
			budgetIdHeader: budgetId.String(),
			body:           "not json",
			setupMocks:     func(m *mockLoanMetadataService) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "service_error_returns_500",
			budgetIdHeader: budgetId.String(),
			body: model.LoanMetadata{
				AccountID:       accountId,
				InterestRate:    5.25,
				OriginalBalance: 25000.00,
				MonthlyPayment:  450.00,
				LoanStartDate:   "2024-01-15",
			},
			setupMocks: func(m *mockLoanMetadataService) {
				m.On("Create", mock.Anything, mock.Anything).Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockLoanMetadataService{}
			handler := NewLoanMetadataHandler(mockService)

			w, c := setupGinTestContext("POST", "/api/loan-metadata", tt.body)
			if tt.budgetIdHeader != "" {
				c.Request.Header.Set(budgetIdHeader, tt.budgetIdHeader)
			}

			tt.setupMocks(mockService)

			handler.Create(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockService.AssertExpectations(t)
		})
	}
}

// --- Update handler tests ---

func TestLoanMetadataHandler_Update(t *testing.T) {
	budgetId := uuid.New()
	accountId := uuid.New()

	tests := []struct {
		name           string
		budgetIdHeader string
		accountIdParam string
		body           interface{}
		setupMocks     func(*mockLoanMetadataService)
		expectedStatus int
	}{
		{
			name:           "updates_loan_successfully",
			budgetIdHeader: budgetId.String(),
			accountIdParam: accountId.String(),
			body: model.LoanMetadata{
				InterestRate:    4.0,
				OriginalBalance: 20000.00,
				MonthlyPayment:  400.00,
				LoanStartDate:   "2024-06-01",
			},
			setupMocks: func(m *mockLoanMetadataService) {
				m.On("Update", mock.Anything, accountId, mock.MatchedBy(func(l model.LoanMetadata) bool {
					return l.InterestRate == 4.0
				})).Return(createTestLoanMetadata(accountId, nil), nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing_budget_id_returns_400",
			budgetIdHeader: "",
			accountIdParam: accountId.String(),
			body: model.LoanMetadata{
				InterestRate: 4.0,
			},
			setupMocks:     func(m *mockLoanMetadataService) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid_account_id_returns_400",
			budgetIdHeader: budgetId.String(),
			accountIdParam: "not-a-uuid",
			body: model.LoanMetadata{
				InterestRate: 4.0,
			},
			setupMocks:     func(m *mockLoanMetadataService) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid_json_body_returns_400",
			budgetIdHeader: budgetId.String(),
			accountIdParam: accountId.String(),
			body:           "not json",
			setupMocks:     func(m *mockLoanMetadataService) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "service_error_returns_500",
			budgetIdHeader: budgetId.String(),
			accountIdParam: accountId.String(),
			body: model.LoanMetadata{
				InterestRate: 4.0,
			},
			setupMocks: func(m *mockLoanMetadataService) {
				m.On("Update", mock.Anything, accountId, mock.Anything).Return(nil, assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockLoanMetadataService{}
			handler := NewLoanMetadataHandler(mockService)

			w, c := setupGinTestContext("PATCH", "/api/loan-metadata/"+tt.accountIdParam, tt.body)
			if tt.budgetIdHeader != "" {
				c.Request.Header.Set(budgetIdHeader, tt.budgetIdHeader)
			}
			c.Params = gin.Params{{Key: "accountId", Value: tt.accountIdParam}}

			tt.setupMocks(mockService)

			handler.Update(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockService.AssertExpectations(t)
		})
	}
}

// --- Delete handler tests ---

func TestLoanMetadataHandler_Delete(t *testing.T) {
	budgetId := uuid.New()
	accountId := uuid.New()

	tests := []struct {
		name           string
		budgetIdHeader string
		accountIdParam string
		setupMocks     func(*mockLoanMetadataService)
		expectedStatus int
	}{
		{
			name:           "deletes_loan_successfully",
			budgetIdHeader: budgetId.String(),
			accountIdParam: accountId.String(),
			setupMocks: func(m *mockLoanMetadataService) {
				m.On("Delete", mock.Anything, accountId).Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing_budget_id_returns_400",
			budgetIdHeader: "",
			accountIdParam: accountId.String(),
			setupMocks:     func(m *mockLoanMetadataService) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid_account_id_returns_400",
			budgetIdHeader: budgetId.String(),
			accountIdParam: "not-a-uuid",
			setupMocks:     func(m *mockLoanMetadataService) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "service_error_returns_500",
			budgetIdHeader: budgetId.String(),
			accountIdParam: accountId.String(),
			setupMocks: func(m *mockLoanMetadataService) {
				m.On("Delete", mock.Anything, accountId).Return(assert.AnError)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockLoanMetadataService{}
			handler := NewLoanMetadataHandler(mockService)

			w, c := setupGinTestContext("DELETE", "/api/loan-metadata/"+tt.accountIdParam, nil)
			if tt.budgetIdHeader != "" {
				c.Request.Header.Set(budgetIdHeader, tt.budgetIdHeader)
			}
			c.Params = gin.Params{{Key: "accountId", Value: tt.accountIdParam}}

			tt.setupMocks(mockService)

			handler.Delete(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockService.AssertExpectations(t)
		})
	}
}

// --- Response body verification tests ---

func TestLoanMetadataHandler_List_ResponseBody(t *testing.T) {
	budgetId := uuid.New()
	accountId := uuid.New()
	catId := uuid.New()

	mockService := &mockLoanMetadataService{}
	handler := NewLoanMetadataHandler(mockService)

	loan := createTestLoanMetadata(accountId, &catId)
	loans := []model.LoanMetadata{*loan}
	mockService.On("GetAll", mock.Anything).Return(loans, nil)

	w, c := setupGinTestContext("GET", "/api/loan-metadata", nil)
	c.Request.Header.Set(budgetIdHeader, budgetId.String())

	handler.List(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []model.LoanMetadata
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response, 1)
	assert.Equal(t, loan.AccountID, response[0].AccountID)
	assert.Equal(t, loan.InterestRate, response[0].InterestRate)
	assert.Equal(t, loan.OriginalBalance, response[0].OriginalBalance)
	assert.Equal(t, loan.MonthlyPayment, response[0].MonthlyPayment)
	assert.Equal(t, loan.LoanStartDate, response[0].LoanStartDate)

	mockService.AssertExpectations(t)
}

func TestLoanMetadataHandler_Create_ResponseBody(t *testing.T) {
	budgetId := uuid.New()
	accountId := uuid.New()
	catId := uuid.New()

	mockService := &mockLoanMetadataService{}
	handler := NewLoanMetadataHandler(mockService)

	created := createTestLoanMetadata(accountId, &catId)
	mockService.On("Create", mock.Anything, mock.Anything).Return(created, nil)

	body := model.LoanMetadata{
		AccountID:       accountId,
		InterestRate:    5.25,
		OriginalBalance: 25000.00,
		MonthlyPayment:  450.00,
		LoanStartDate:   "2024-01-15",
		CategoryID:      &catId,
	}

	w, c := setupGinTestContext("POST", "/api/loan-metadata", body)
	c.Request.Header.Set(budgetIdHeader, budgetId.String())

	handler.Create(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response model.LoanMetadata
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, created.AccountID, response.AccountID)
	assert.Equal(t, created.InterestRate, response.InterestRate)
	assert.NotEqual(t, uuid.Nil, response.ID)

	mockService.AssertExpectations(t)
}

func TestLoanMetadataHandler_Delete_ResponseBody(t *testing.T) {
	budgetId := uuid.New()
	accountId := uuid.New()

	mockService := &mockLoanMetadataService{}
	handler := NewLoanMetadataHandler(mockService)

	mockService.On("Delete", mock.Anything, accountId).Return(nil)

	w, c := setupGinTestContext("DELETE", "/api/loan-metadata/"+accountId.String(), nil)
	c.Request.Header.Set(budgetIdHeader, budgetId.String())
	c.Params = gin.Params{{Key: "accountId", Value: accountId.String()}}

	handler.Delete(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "loan metadata deleted", response["message"])

	mockService.AssertExpectations(t)
}
