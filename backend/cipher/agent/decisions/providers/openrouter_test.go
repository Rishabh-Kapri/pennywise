package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

func ptr[T any](v T) *T { return &v }

func testRequest() sharedModel.DecisionRequest {
	return sharedModel.DecisionRequest{
		Model: "typesafe/jev-1.13",
		State: sharedModel.State{Query: "Rs. 450 debited at Blue Tokai"},
		Questions: map[string]sharedModel.Intent{
			"isTransaction": {
				Type:     sharedModel.NoulIntentType,
				Criteria: map[string]*string{"true": ptr("a debit"), "false": ptr("not a debit")},
			},
		},
	}
}

func TestOpenRouterAsk(t *testing.T) {
	var got map[string]any
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{
			"id": "gen-dec-1",
			"model": "typesafe/jev-1.13-20260917",
			"provider": "TypeSafe",
			"answers": {
				"isTransaction": {"type": "noul", "noul": 0.96},
				"other": {"type": "refusal"}
			},
			"usage": {"input_tokens": 476, "output_tokens": 70, "cost": 0.00002,
				"is_byok": true, "input_tokens_details": {"cached_tokens": 12}}
		}`))
	}))
	defer srv.Close()

	c, err := NewOpenRouterClient(srv.URL, "sk-test", srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Ask(context.Background(), testRequest())
	if err != nil {
		t.Fatal(err)
	}

	if auth != "Bearer sk-test" {
		t.Errorf("Authorization = %q", auth)
	}
	if got["model"] != "typesafe/jev-1.13" {
		t.Errorf("model sent = %v", got["model"])
	}
	q := got["questions"].(map[string]any)["isTransaction"].(map[string]any)
	if _, ok := q["instructions"]; ok {
		t.Error("empty instructions should be omitted")
	}

	if res.Model != "typesafe/jev-1.13-20260917" || *res.ID != "gen-dec-1" || *res.Provider != "TypeSafe" {
		t.Errorf("response metadata = %+v", res)
	}
	if a := res.Answers["isTransaction"]; a.Noul == nil || *a.Noul != 0.96 {
		t.Errorf("noul answer = %+v", a)
	}
	if a := res.Answers["other"]; a.Type != "refusal" {
		t.Errorf("refusal answer = %+v", a)
	}
	u := res.Usage
	if !u.Available || u.InputTokens != 476 || u.OutputTokens != 70 || u.TotalTokens != 546 ||
		u.CacheReadTokens != 12 || u.Cost == nil {
		t.Errorf("usage = %+v", u)
	}
}

func TestOpenRouterAskKeepsHTTPStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"error": {"code": 402, "message": "Insufficient credits"}}`))
	}))
	defer srv.Close()

	c, err := NewOpenRouterClient(srv.URL, "sk-test", srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Ask(context.Background(), testRequest())
	if !errs.HasCode(err, errs.CodeDecisionsRequestFailed) {
		t.Fatalf("want %s, got %v", errs.CodeDecisionsRequestFailed, err)
	}
	if got := errs.StatusOf(err); got != http.StatusPaymentRequired {
		t.Errorf("StatusOf = %d, want 402", got)
	}
}

func TestNewOpenRouterClientRequiresSafeClient(t *testing.T) {
	if _, err := NewOpenRouterClient("https://openrouter.ai/api/alpha/decisions", "sk", nil); err == nil {
		t.Fatal("want an error without an http client")
	}
}
