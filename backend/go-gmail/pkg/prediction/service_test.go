package prediction

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/config"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/parser"
)

func newTestPredictionService(serverURL string) *Service {
	cfg := &config.Config{MLPServiceURL: serverURL}
	return &Service{
		config: cfg,
		client: &http.Client{},
	}
}

func makePredictionServer(responses []PredictionResult) *httptest.Server {
	callCount := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if callCount >= len(responses) {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(responses[callCount])
		callCount++
	}))
}

// ---------------------------------------------------------------------------
// TestNewService
// ---------------------------------------------------------------------------

func TestNewService(t *testing.T) {
	cfg := &config.Config{MLPServiceURL: "http://localhost:8000"}
	svc := NewService(cfg)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.config != cfg {
		t.Error("expected config to be set")
	}
	if svc.client == nil {
		t.Error("expected http.Client to be set")
	}
}

// ---------------------------------------------------------------------------
// TestCallPredictApi
// ---------------------------------------------------------------------------

func TestCallPredictApi(t *testing.T) {
	tests := []struct {
		name        string
		handler     http.HandlerFunc
		fieldType   string
		expectError bool
		wantLabel   string
	}{
		{
			name:      "Success",
			fieldType: "account",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(PredictionResult{Label: "HDFC Savings", Confidence: 0.95})
			},
			expectError: false,
			wantLabel:   "HDFC Savings",
		},
		{
			name:      "Server unreachable",
			fieldType: "payee",
			handler: func(w http.ResponseWriter, r *http.Request) {
				// Close connection immediately to simulate network error
				hj, ok := w.(http.Hijacker)
				if ok {
					conn, _, _ := hj.Hijack()
					conn.Close()
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError: true,
		},
		{
			name:      "Invalid JSON response",
			fieldType: "category",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("not-valid-json"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			svc := newTestPredictionService(server.URL)
			details := &parser.EmailDetails{
				Text:   "Dear Customer...",
				Amount: -100,
			}
			result, err := svc.CallPredictApi(context.Background(), details, tt.fieldType)

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
			if result == nil {
				t.Error("expected result, got nil")
				return
			}
			if tt.wantLabel != "" && result.Label != tt.wantLabel {
				t.Errorf("expected label %q, got %q", tt.wantLabel, result.Label)
			}
			// Verify fieldType was set on emailDetails
			if details.Type != tt.fieldType {
				t.Errorf("expected emailDetails.Type %q, got %q", tt.fieldType, details.Type)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestCallPredictApi_InvalidURL covers the http.NewRequest error path
// (triggered by an invalid URL scheme).
// ---------------------------------------------------------------------------

func TestCallPredictApi_InvalidURL(t *testing.T) {
	// "://bad-url" is not a valid URL and causes http.NewRequest to return an error.
	svc := newTestPredictionService("://bad-url")
	details := &parser.EmailDetails{Text: "Dear Customer...", Amount: -100}
	_, err := svc.CallPredictApi(context.Background(), details, "account")
	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}
}

// TestCallPredictApi_BodyReadError covers the io.ReadAll error path by
// hijacking the connection and closing it after headers are sent, so the
// body read returns an unexpected EOF.
func TestCallPredictApi_BodyReadError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write status + headers, then close the connection abruptly so
		// io.ReadAll sees an unexpected EOF.
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		hj := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer server.Close()

	svc := newTestPredictionService(server.URL)
	details := &parser.EmailDetails{Text: "Dear Customer...", Amount: -100}
	_, err := svc.CallPredictApi(context.Background(), details, "account")
	if err == nil {
		t.Error("expected error from abrupt connection close, got nil")
	}
}

// ---------------------------------------------------------------------------
// TestGetPredictedFields
// ---------------------------------------------------------------------------

func TestGetPredictedFields(t *testing.T) {
	ctx := context.Background()
	emailDetails := func() *parser.EmailDetails {
		return &parser.EmailDetails{
			Text:   "Dear Customer...",
			Amount: -500,
			Date:   "2025-01-01",
		}
	}

	t.Run("All three predictions above threshold", func(t *testing.T) {
		server := makePredictionServer([]PredictionResult{
			{Label: "HDFC Savings", Confidence: 0.9},   // account
			{Label: "Google Cloud", Confidence: 0.85},  // payee
			{Label: "Subscriptions", Confidence: 0.75}, // category
		})
		defer server.Close()

		svc := newTestPredictionService(server.URL)
		result, err := svc.GetPredictedFields(ctx, emailDetails(), "HDFC Savings")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Account.Label != "HDFC Savings" {
			t.Errorf("expected account 'HDFC Savings', got %q", result.Account.Label)
		}
		if result.Payee.Label != "Google Cloud" {
			t.Errorf("expected payee 'Google Cloud', got %q", result.Payee.Label)
		}
		if result.Category.Label != "Subscriptions" {
			t.Errorf("expected category 'Subscriptions', got %q", result.Category.Label)
		}
	})

	t.Run("Account confidence below threshold stops cascade", func(t *testing.T) {
		server := makePredictionServer([]PredictionResult{
			{Label: "HDFC Savings", Confidence: 0.5}, // below threshold
		})
		defer server.Close()

		svc := newTestPredictionService(server.URL)
		result, err := svc.GetPredictedFields(ctx, emailDetails(), "fallback-account")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Account is set to the predicted label even when below threshold
		if result.Account.Label != "HDFC Savings" {
			t.Errorf("expected account label 'HDFC Savings', got %q", result.Account.Label)
		}
		// Payee stays as fallback
		if result.Payee.Label != "Unexpected" {
			t.Errorf("expected fallback payee 'Unexpected', got %q", result.Payee.Label)
		}
		// Category stays as fallback
		if result.Category.Label != "❗ Unexpected expenses" {
			t.Errorf("expected fallback category, got %q", result.Category.Label)
		}
	})

	t.Run("Payee confidence below threshold returns Unexpected payee", func(t *testing.T) {
		server := makePredictionServer([]PredictionResult{
			{Label: "HDFC Savings", Confidence: 0.9}, // account passes
			{Label: "Some Payee", Confidence: 0.5},   // payee below threshold
		})
		defer server.Close()

		svc := newTestPredictionService(server.URL)
		result, err := svc.GetPredictedFields(ctx, emailDetails(), "fallback")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Payee.Label != "Unexpected" {
			t.Errorf("expected payee 'Unexpected', got %q", result.Payee.Label)
		}
		if result.Category.Label != "❗ Unexpected expenses" {
			t.Errorf("expected fallback category, got %q", result.Category.Label)
		}
	})

	t.Run("Category confidence below threshold returns fallback category", func(t *testing.T) {
		server := makePredictionServer([]PredictionResult{
			{Label: "HDFC Savings", Confidence: 0.9},  // account
			{Label: "Google Cloud", Confidence: 0.85}, // payee
			{Label: "Unknown Cat", Confidence: 0.3},   // category below threshold
		})
		defer server.Close()

		svc := newTestPredictionService(server.URL)
		result, err := svc.GetPredictedFields(ctx, emailDetails(), "fallback")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Category.Label != "❗ Unexpected expenses" {
			t.Errorf("expected fallback category, got %q", result.Category.Label)
		}
	})

	t.Run("Account predict API error returns fallbacks with error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invalid-json"))
		}))
		defer server.Close()

		svc := newTestPredictionService(server.URL)
		result, err := svc.GetPredictedFields(ctx, emailDetails(), "fallback-acc")
		if err == nil {
			t.Error("expected error from invalid JSON, got nil")
		}
		// Even on error, result should have fallback values
		if result == nil {
			t.Error("expected non-nil result even on error")
		}
	})

	t.Run("Payee predict API error returns early with error", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount == 1 {
				// account succeeds
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(PredictionResult{Label: "HDFC", Confidence: 0.9})
			} else {
				// payee fails
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("bad-json"))
			}
		}))
		defer server.Close()

		svc := newTestPredictionService(server.URL)
		result, err := svc.GetPredictedFields(ctx, emailDetails(), "fallback-acc")
		if err == nil {
			t.Error("expected error from invalid payee JSON")
		}
		if result == nil {
			t.Error("expected non-nil result even on error")
		}
	})

	t.Run("Category predict API error returns early with error", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			switch callCount {
			case 1:
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(PredictionResult{Label: "HDFC", Confidence: 0.9})
			case 2:
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(PredictionResult{Label: "Google", Confidence: 0.85})
			default:
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("bad-json"))
			}
		}))
		defer server.Close()

		svc := newTestPredictionService(server.URL)
		result, err := svc.GetPredictedFields(ctx, emailDetails(), "fallback-acc")
		if err == nil {
			t.Error("expected error from invalid category JSON")
		}
		if result == nil {
			t.Error("expected non-nil result even on error")
		}
	})
}
