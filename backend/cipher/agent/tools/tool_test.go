package tools

import (
	"context"
	"encoding/json"
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
