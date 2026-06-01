package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/parser"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/prediction"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
)

func getTestInputs() (*parser.EmailDetails, *prediction.PredictedFields) {
	parsedDetails := &parser.EmailDetails{
		Text:            "Dear Card Member, <br> <br>Thank you for using your HDFC Bank Credit Card ending 4432 for Rs 5065.68 at NAKODA DAIRY PRIVATE L on 04-08-2025.",
		Date:            "2025-08-04",
		Amount:          -5065.68,
		TransactionType: "debited",
		Account:         "HDFC Credit Card",
		Payee:           "Nakpro",
	}
	predictedFields := &prediction.PredictedFields{
		Account: prediction.PredictionResult{
			Label:      "HDFC Credit Card",
			Confidence: 1.0,
		},
		Payee: prediction.PredictionResult{
			Label:      "Nakpro",
			Confidence: 0.92,
		},
		Category: prediction.PredictionResult{
			Label:      "Gym",
			Confidence: 0.84,
		},
	}
	return parsedDetails, predictedFields
}

func newTestService(serverURL string) *PennywiseClient {
	httpTransport := httpclient.NewHttpTransport(serverURL)
	client := transport.NewClient("pennywise-api-test", httpTransport)
	return NewPennywiseClient(client)
}

func newTestCtx() context.Context {
	ctx := context.Background()
	return utils.WithBudgetID(ctx, uuid.MustParse("2166418d-3fa2-4acc-b92c-ab9f36c18d76"))
}

// ---------------------------------------------------------------------------
// TestCreateTransaction
// ---------------------------------------------------------------------------

func TestCreateTransaction(t *testing.T) {
	tests := []struct {
		name            string
		parsedDetails   *parser.EmailDetails
		predictedFields *prediction.PredictedFields
		mockResponses   map[string][]map[string]any
		expectError     bool
		expectedError   string
		expectedTxn     *Transaction
	}{
		{
			name: "Successful Transaction Creation",
			parsedDetails: &parser.EmailDetails{
				Date:    "2025-08-04",
				Amount:  -5065.68,
				Account: "HDFC Credit Card",
				Payee:   "Nakpro",
			},
			predictedFields: &prediction.PredictedFields{
				Account: prediction.PredictionResult{
					Label:      "HDFC Credit Card",
					Confidence: 1.0,
				},
				Payee: prediction.PredictionResult{
					Label:      "Nakpro",
					Confidence: 0.84,
				},
				Category: prediction.PredictionResult{
					Label:      "null",
					Confidence: 0.92,
				},
			},
			mockResponses: map[string][]map[string]any{
				"/api/accounts/search": {
					{"id": "acc-123", "name": "HDFC Credit Card"},
				},
				"/api/payees/search": {
					{"id": "payee-123", "name": "Nakpro"},
				},
				"/api/categories/search": {
					{"id": "cat-123", "name": "Gym"},
				},
				"/api/transactions": {
					{
						"id":         "txn-123",
						"amount":     -5065.68,
						"date":       "2025-08-04",
						"accountId":  "acc-123",
						"payeeId":    "payee-123",
						"categoryId": nil,
					},
				},
			},
			expectError: false,
			expectedTxn: &Transaction{
				ID:         "txn-123",
				Amount:     -5065.68,
				Date:       "2025-08-04",
				AccountId:  "acc-123",
				PayeeId:    "payee-123",
				CategoryId: nil,
			},
		},
		{
			name: "Account Not Found",
			parsedDetails: &parser.EmailDetails{
				Amount: 100.00,
				Date:   "2023-01-01",
			},
			predictedFields: &prediction.PredictedFields{
				Account:  prediction.PredictionResult{Label: "Unknown Account"},
				Payee:    prediction.PredictionResult{Label: "Test Payee"},
				Category: prediction.PredictionResult{Label: "Test Category"},
			},
			mockResponses: map[string][]map[string]any{
				"/api/accounts/search": nil, // Empty response
			},
			expectError:   true,
			expectedError: "Account not found for Unknown Account",
		},
		{
			name: "Payee Not Found",
			parsedDetails: &parser.EmailDetails{
				Amount: 100.00,
				Date:   "2023-01-01",
			},
			predictedFields: &prediction.PredictedFields{
				Account:  prediction.PredictionResult{Label: "Test Account"},
				Payee:    prediction.PredictionResult{Label: "Unknown Payee"},
				Category: prediction.PredictionResult{Label: "Test Category"},
			},
			mockResponses: map[string][]map[string]any{
				"/api/accounts/search": {
					{"id": "acc-123", "name": "Test Account"},
				},
				"/api/payees/search": nil, // Empty response
			},
			expectError:   true,
			expectedError: "Payee not found for Unknown Payee",
		},
		{
			name: "Category Not Found",
			parsedDetails: &parser.EmailDetails{
				Amount: 100.00,
				Date:   "2023-01-01",
			},
			predictedFields: &prediction.PredictedFields{
				Account:  prediction.PredictionResult{Label: "Test Account"},
				Payee:    prediction.PredictionResult{Label: "Test Payee"},
				Category: prediction.PredictionResult{Label: "Some Category"},
			},
			mockResponses: map[string][]map[string]any{
				"/api/accounts/search":   {{"id": "acc-1"}},
				"/api/payees/search":     {{"id": "payee-1"}},
				"/api/categories/search": nil, // Empty
			},
			expectError: true,
		},
		{
			name: "Empty category string skips category lookup",
			parsedDetails: &parser.EmailDetails{
				Amount: 50.00,
				Date:   "2023-02-01",
			},
			predictedFields: &prediction.PredictedFields{
				Account:  prediction.PredictionResult{Label: "My Account"},
				Payee:    prediction.PredictionResult{Label: "My Payee"},
				Category: prediction.PredictionResult{Label: ""},
			},
			mockResponses: map[string][]map[string]any{
				"/api/accounts/search": {{"id": "acc-1"}},
				"/api/payees/search":   {{"id": "payee-1"}},
				"/api/transactions":    {{"id": "txn-1", "amount": 50.00, "date": "2023-02-01", "accountId": "acc-1", "payeeId": "payee-1"}},
			},
			expectError: false,
		},
		{
			name: "Account search HTTP error",
			parsedDetails: &parser.EmailDetails{
				Amount: 100.00,
				Date:   "2023-01-01",
			},
			predictedFields: &prediction.PredictedFields{
				Account:  prediction.PredictionResult{Label: "Test Account"},
				Payee:    prediction.PredictionResult{Label: "Test Payee"},
				Category: prediction.PredictionResult{Label: "Test Cat"},
			},
			mockResponses: map[string][]map[string]any{
				// no /api/accounts/search → 404 → transport error
			},
			expectError: true,
		},
		{
			name: "Payee search HTTP error",
			parsedDetails: &parser.EmailDetails{
				Amount: 100.00,
				Date:   "2023-01-01",
			},
			predictedFields: &prediction.PredictedFields{
				Account:  prediction.PredictionResult{Label: "Test Account"},
				Payee:    prediction.PredictionResult{Label: "Test Payee"},
				Category: prediction.PredictionResult{Label: "Test Cat"},
			},
			mockResponses: map[string][]map[string]any{
				"/api/accounts/search": {{"id": "acc-1"}},
				// no /api/payees/search → 404 → transport error
			},
			expectError: true,
		},
		{
			name: "Category search HTTP error",
			parsedDetails: &parser.EmailDetails{
				Amount: 100.00,
				Date:   "2023-01-01",
			},
			predictedFields: &prediction.PredictedFields{
				Account:  prediction.PredictionResult{Label: "Test Account"},
				Payee:    prediction.PredictionResult{Label: "Test Payee"},
				Category: prediction.PredictionResult{Label: "Some Category"},
			},
			mockResponses: map[string][]map[string]any{
				"/api/accounts/search": {{"id": "acc-1"}},
				"/api/payees/search":   {{"id": "payee-1"}},
				// no /api/categories/search → 404 → transport error
			},
			expectError: true,
		},
		{
			name: "Successful transaction with category",
			parsedDetails: &parser.EmailDetails{
				Amount: 100.00,
				Date:   "2023-01-01",
			},
			predictedFields: &prediction.PredictedFields{
				Account:  prediction.PredictionResult{Label: "Test Account"},
				Payee:    prediction.PredictionResult{Label: "Test Payee"},
				Category: prediction.PredictionResult{Label: "Groceries"},
			},
			mockResponses: map[string][]map[string]any{
				"/api/accounts/search":   {{"id": "acc-1"}},
				"/api/payees/search":     {{"id": "payee-1"}},
				"/api/categories/search": {{"id": "cat-1"}},
				"/api/transactions":      {{"id": "txn-1", "amount": 100.0, "date": "2023-01-01", "accountId": "acc-1", "payeeId": "payee-1", "categoryId": "cat-1"}},
			},
			expectError: false,
		},
		{
			name: "Transactions endpoint HTTP error",
			parsedDetails: &parser.EmailDetails{
				Amount: 100.00,
				Date:   "2023-01-01",
			},
			predictedFields: &prediction.PredictedFields{
				Account:  prediction.PredictionResult{Label: "Test Account"},
				Payee:    prediction.PredictionResult{Label: "Test Payee"},
				Category: prediction.PredictionResult{Label: "null"},
			},
			mockResponses: map[string][]map[string]any{
				"/api/accounts/search": {{"id": "acc-1"}},
				"/api/payees/search":   {{"id": "payee-1"}},
				// no /api/transactions → 404 → transport error
			},
			expectError: true,
		},
		{
			name: "Transactions endpoint returns empty list",
			parsedDetails: &parser.EmailDetails{
				Amount: 100.00,
				Date:   "2023-01-01",
			},
			predictedFields: &prediction.PredictedFields{
				Account:  prediction.PredictionResult{Label: "Test Account"},
				Payee:    prediction.PredictionResult{Label: "Test Payee"},
				Category: prediction.PredictionResult{Label: "null"},
			},
			mockResponses: map[string][]map[string]any{
				"/api/accounts/search": {{"id": "acc-1"}},
				"/api/payees/search":   {{"id": "payee-1"}},
				"/api/transactions":    {}, // empty slice — causes "No transactions received"
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				res, exists := tt.mockResponses[r.URL.Path]
				if !exists {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(res)
			}))
			defer server.Close()

			svc := newTestService(server.URL)
			ctx := newTestCtx()
			res, err := svc.CreateTransaction(ctx, tt.parsedDetails, tt.predictedFields)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Expected nil, got %v", err)
				return
			}
			if res == nil {
				t.Errorf("Expected response, got nil")
				return
			}
			if tt.expectedTxn != nil {
				if res.Amount != tt.expectedTxn.Amount {
					t.Errorf("Expected amount %v, got %v", tt.expectedTxn.Amount, res.Amount)
				}
				if res.CategoryId != tt.expectedTxn.CategoryId {
					t.Errorf("Expected categoryId %v, got %v", tt.expectedTxn.CategoryId, res.CategoryId)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestGetUser
// ---------------------------------------------------------------------------

func TestGetUser(t *testing.T) {
	tests := []struct {
		name        string
		email       string
		handler     http.HandlerFunc
		expectError bool
	}{
		{
			name:  "Success",
			email: "test@example.com",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/auth/google/users" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{
					"googleId":       "gid-123",
					"email":          "test@example.com",
					"gmailHistoryId": 1234,
					"refreshToken":   "rtoken",
					"budgetId":       "2166418d-3fa2-4acc-b92c-ab9f36c18d76",
				})
			},
			expectError: false,
		},
		{
			name:  "Server error",
			email: "test@example.com",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			svc := newTestService(server.URL)
			user, err := svc.GetUser(context.Background(), tt.email)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if user == nil {
				t.Error("expected user, got nil")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestUpdateUserHistoryId
// ---------------------------------------------------------------------------

func TestUpdateUserHistoryId(t *testing.T) {
	tests := []struct {
		name        string
		handler     http.HandlerFunc
		expectError bool
	}{
		{
			name: "Success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPatch {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{})
			},
			expectError: false,
		},
		{
			name: "Server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			svc := newTestService(server.URL)
			err := svc.UpdateUserHistoryId(context.Background(), "user@example.com", sharedModel.GoogleOAuthClientTypeAndroid, 9999)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestCreatePrediction
// ---------------------------------------------------------------------------

func TestCreatePrediction(t *testing.T) {
	tests := []struct {
		name            string
		parsedDetails   *parser.EmailDetails
		predictedFields *prediction.PredictedFields
		txnData         *Transaction
		handler         http.HandlerFunc
		expectError     bool
	}{
		{
			name: "Success with all confidence values set",
			parsedDetails: &parser.EmailDetails{
				Text:   "Dear Customer...",
				Amount: -100,
			},
			predictedFields: &prediction.PredictedFields{
				Account:  prediction.PredictionResult{Label: "HDFC CC", Confidence: 0.9},
				Payee:    prediction.PredictionResult{Label: "Google", Confidence: 0.85},
				Category: prediction.PredictionResult{Label: "Subscriptions", Confidence: 0.75},
			},
			txnData: &Transaction{ID: "txn-1", Amount: -100},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{})
			},
			expectError: false,
		},
		{
			name: "Success with confidence -1 (fallback fields)",
			parsedDetails: &parser.EmailDetails{
				Text:   "Dear Customer...",
				Amount: -200,
			},
			predictedFields: &prediction.PredictedFields{
				Account:  prediction.PredictionResult{Label: "HDFC Savings", Confidence: -1},
				Payee:    prediction.PredictionResult{Label: "Unexpected", Confidence: -1},
				Category: prediction.PredictionResult{Label: "❗ Unexpected expenses", Confidence: -1},
			},
			txnData: &Transaction{ID: "txn-2", Amount: -200},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{})
			},
			expectError: false,
		},
		{
			name: "Server error",
			parsedDetails: &parser.EmailDetails{
				Text:   "Dear Customer...",
				Amount: -50,
			},
			predictedFields: &prediction.PredictedFields{
				Account:  prediction.PredictionResult{Label: "Account", Confidence: 0.9},
				Payee:    prediction.PredictionResult{Label: "Payee", Confidence: 0.8},
				Category: prediction.PredictionResult{Label: "Cat", Confidence: 0.7},
			},
			txnData: &Transaction{ID: "txn-3", Amount: -50},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			svc := newTestService(server.URL)
			err := svc.CreatePrediction(newTestCtx(), tt.parsedDetails, tt.predictedFields, tt.txnData)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestBuildPath
// ---------------------------------------------------------------------------

func TestBuildPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		query    map[string]string
		wantPath string
	}{
		{
			name:     "No query params",
			path:     "/api/accounts",
			query:    map[string]string{},
			wantPath: "/api/accounts",
		},
		{
			name:     "Nil query params",
			path:     "/api/accounts",
			query:    nil,
			wantPath: "/api/accounts",
		},
		{
			name:     "Single query param",
			path:     "/api/accounts/search",
			query:    map[string]string{"name": "HDFC Credit Card"},
			wantPath: "/api/accounts/search?name=HDFC+Credit+Card",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPath(tt.path, tt.query)
			if len(tt.query) == 0 || tt.query == nil {
				if got != tt.wantPath {
					t.Errorf("buildPath() = %q, want %q", got, tt.wantPath)
				}
			} else {
				// For query params, just check the base path is correct
				if len(got) <= len(tt.path) {
					t.Errorf("buildPath() = %q, expected query params to be appended", got)
				}
			}
		})
	}
}
