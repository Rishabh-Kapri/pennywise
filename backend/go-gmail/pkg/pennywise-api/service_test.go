package pennywise

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/parser"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/prediction"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
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

func newTestService(serverURL string) *Service {
	httpTransport := httpclient.NewHttpTransport(serverURL)
	client := transport.NewClient("pennywise-api-test", httpTransport)
	return NewService(client)
}

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
			ctx := context.Background()
			ctx = utils.WithBudgetID(ctx, uuid.MustParse("2166418d-3fa2-4acc-b92c-ab9f36c18d76"))
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
