package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

type stubTool struct {
	name string
}

func (s stubTool) Definition() sharedModel.ToolDefiniton {
	return sharedModel.ToolDefiniton{Name: s.name}
}

func (s stubTool) Execute(context.Context, sharedModel.ToolCall) (*sharedModel.ToolResult, error) {
	return nil, nil
}

func (s stubTool) GetNormalizedName(bool) string { return s.name }

func (s stubTool) Normalize(sharedModel.ToolCall, json.RawMessage) (*sharedModel.ToolResultNormalized, error) {
	return nil, nil
}

// Tools render first in the provider prompt, so an unstable order changes the
// prompt prefix on every request and defeats prompt caching. The old
// implementation ranged a Go map, which randomises iteration order by design.
func TestGetAllToolsPreservesRegistrationOrder(t *testing.T) {
	names := []string{"get_today", "get_budget_info", "get_spending_summary", "get_top_transactions", "execute_sql"}

	registry := NewToolRegistry()
	for _, name := range names {
		registry.RegisterTool(stubTool{name: name})
	}

	// Repeat enough times that map-order randomisation would almost certainly show.
	for i := 0; i < 50; i++ {
		got := registry.GetAllTools()
		if len(got) != len(names) {
			t.Fatalf("tool count = %d, want %d", len(got), len(names))
		}
		for j, tool := range got {
			if tool.Definition().Name != names[j] {
				t.Fatalf("iteration %d: tool[%d] = %q, want %q", i, j, tool.Definition().Name, names[j])
			}
		}
	}
}

func TestRegisterToolIgnoresDuplicatesAndKeepsOrder(t *testing.T) {
	registry := NewToolRegistry()
	registry.RegisterTool(stubTool{name: "a"})
	registry.RegisterTool(stubTool{name: "b"})
	registry.RegisterTool(stubTool{name: "a"})

	got := registry.GetAllTools()
	if len(got) != 2 {
		t.Fatalf("tool count = %d, want 2", len(got))
	}
	if got[0].Definition().Name != "a" || got[1].Definition().Name != "b" {
		t.Fatalf("tool order = %q, %q; want a, b", got[0].Definition().Name, got[1].Definition().Name)
	}
}

// An argument the tool does not implement must fail loudly. Silently dropping it
// is how "spending by category tagged nagpur-july-2026" came back as unfiltered
// July spending: the model passed tagName, encoding/json discarded it, and the
// wrong number was reported as though it answered the question.
func TestDecodeToolArgsRejectsUnknownField(t *testing.T) {
	var args spendingSummaryArgs
	raw := json.RawMessage(`{
		"groupBy": "category",
		"dateRange": {"start": "2026-07-01", "end": "2026-07-31"},
		"accountName": "HDFC"
	}`)

	err := decodeToolArgs(spendingSummaryToolName, raw, &args)
	if err == nil {
		t.Fatal("expected unknown field accountName to be rejected, got nil error")
	}
	if !strings.Contains(err.Error(), "accountName") {
		t.Errorf("error should name the offending field so the model can correct it, got: %v", err)
	}
}

func TestDecodeToolArgsAcceptsSupportedFilters(t *testing.T) {
	var args spendingSummaryArgs
	raw := json.RawMessage(`{
		"groupBy": "category",
		"dateRange": {"start": "2026-07-01", "end": "2026-07-31"},
		"tagName": "nagpur-july-2026",
		"categoryName": "Food",
		"payeeName": "Amazon",
		"limit": 10
	}`)

	if err := decodeToolArgs(spendingSummaryToolName, raw, &args); err != nil {
		t.Fatalf("supported filters should decode, got: %v", err)
	}

	if args.TagName != "nagpur-july-2026" {
		t.Errorf("TagName = %q, want nagpur-july-2026", args.TagName)
	}
	if args.CategoryName != "Food" {
		t.Errorf("CategoryName = %q, want Food", args.CategoryName)
	}
	if args.PayeeName != "Amazon" {
		t.Errorf("PayeeName = %q, want Amazon", args.PayeeName)
	}
}

// get_spending_summary and get_top_transactions must accept the same filter
// names. A filter honored by one and rejected by the other pushes the model
// toward execute_sql, or back into the silent-drop failure above.
func TestFilterVocabularyMatchesAcrossQueryTools(t *testing.T) {
	summary := GetSpendingSummaryTool{}.Definition().InputSchema.Properties
	top := GetTopTransactionsTool{}.Definition().InputSchema.Properties

	for _, name := range []string{"categoryName", "payeeName", "tagName"} {
		if _, ok := summary[name]; !ok {
			t.Errorf("get_spending_summary is missing filter %q", name)
		}
		if _, ok := top[name]; !ok {
			t.Errorf("get_top_transactions is missing filter %q", name)
		}
	}
}

func TestValidateReadOnlyQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{name: "select", query: "SELECT 1", wantErr: false},
		{name: "with", query: "WITH x AS (SELECT 1) SELECT * FROM x", wantErr: false},
		{name: "trailing semicolon trimmed", query: "SELECT 1;", wantErr: false},
		{name: "multiple statements", query: "SELECT 1; SELECT 2", wantErr: true},
		{name: "empty", query: "   ", wantErr: true},
		{name: "non select", query: "UPDATE transactions SET amount = 0", wantErr: true},
		{
			// The old keyword blocklist rejected this because the literal contains
			// "UPDATE ". Writes are stopped by the read-only transaction now, so a
			// legitimate query mentioning a keyword must pass.
			name:    "blocked keyword inside a string literal is allowed",
			query:   "SELECT * FROM transactions WHERE note LIKE '%update %'",
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateReadOnlyQuery(tc.query)
			if tc.wantErr && err == nil {
				t.Errorf("expected an error for %q", tc.query)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tc.query, err)
			}
		})
	}
}
