package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/google/uuid"
)

func newTestCipherClient(serverURL string) *CipherClient {
	httpTransport := httpclient.NewHttpTransport(serverURL)
	client := transport.NewClient("cipher-test", httpTransport)
	return NewCipherClient(client)
}

func TestCipherPredict(t *testing.T) {
	tests := []struct {
		name        string
		req         PredictRequest
		handler     http.HandlerFunc
		expectError bool
		wantPayee   string
	}{
		{
			name: "Success",
			req: PredictRequest{
				EmailText: "Dear Customer...",
				Amount:    500.00,
				Account:   "HDFC Savings",
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/predict" || r.Method != http.MethodPost {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.WriteHeader(http.StatusOK)
				resp := PredictResponse{
					PayeeID:    uuid.MustParse("11111111-1111-1111-1111-111111111111"),
					CategoryID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
					Payee:      "Google Cloud",
					Category:   "Subscriptions",
					Amount:     500.00,
					Confidence: "high",
					Source:     "pgvector",
				}
				json.NewEncoder(w).Encode(resp)
			},
			expectError: false,
			wantPayee:   "Google Cloud",
		},
		{
			name: "Server error",
			req:  PredictRequest{EmailText: "test", Amount: 100},
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

			svc := newTestCipherClient(server.URL)
			resp, err := svc.Predict(context.Background(), tt.req)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if resp == nil {
				t.Error("expected response, got nil")
				return
			}
			if tt.wantPayee != "" && resp.Payee != tt.wantPayee {
				t.Errorf("expected payee %q, got %q", tt.wantPayee, resp.Payee)
			}
		})
	}
}
