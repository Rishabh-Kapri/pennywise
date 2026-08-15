package db

import (
	"strings"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
)

// The page query appends LIMIT/OFFSET after these args, so a placeholder that
// doesn't line up with its position in the slice silently filters on the wrong
// value instead of erroring.
func TestDocumentSearchConditionsPlaceholders(t *testing.T) {
	budgetId := uuid.New()

	tests := []struct {
		name         string
		filter       model.DocumentFilter
		wantArgs     int
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:       "no filters",
			filter:     model.DocumentFilter{},
			wantArgs:   1,
			wantAbsent: []string{"ILIKE", "transactions.date >=", "mime_type LIKE"},
		},
		{
			name:         "search only",
			filter:       model.DocumentFilter{Search: "mcdonalds"},
			wantArgs:     2,
			wantContains: []string{"file_name ILIKE $2", "payees.name, '') ILIKE $2"},
		},
		{
			name:         "search and both dates",
			filter:       model.DocumentFilter{Search: "x", StartDate: "2026-01-01", EndDate: "2026-02-01"},
			wantArgs:     4,
			wantContains: []string{"ILIKE $2", "transactions.date >= $3", "transactions.date <= $4"},
		},
		{
			name:         "dates without search keep numbering contiguous",
			filter:       model.DocumentFilter{StartDate: "2026-01-01", EndDate: "2026-02-01"},
			wantArgs:     3,
			wantContains: []string{"transactions.date >= $2", "transactions.date <= $3"},
			wantAbsent:   []string{"ILIKE"},
		},
		{
			name:         "end date only",
			filter:       model.DocumentFilter{EndDate: "2026-02-01"},
			wantArgs:     2,
			wantContains: []string{"transactions.date <= $2"},
		},
		{
			// Kind is inlined rather than parameterised, so it must not shift
			// the numbering of the filters that follow it.
			name:         "kind does not consume a placeholder",
			filter:       model.DocumentFilter{Kind: model.DocumentKindImage, StartDate: "2026-01-01"},
			wantArgs:     2,
			wantContains: []string{"mime_type LIKE 'image/%'", "transactions.date >= $2"},
		},
		{
			name:         "pdf kind",
			filter:       model.DocumentFilter{Kind: model.DocumentKindPDF},
			wantArgs:     1,
			wantContains: []string{"mime_type = 'application/pdf'"},
		},
		{
			// A whitespace-only search is the same as no search: it would
			// otherwise burn a placeholder on a '%   %' match.
			name:       "blank search is ignored",
			filter:     model.DocumentFilter{Search: "   "},
			wantArgs:   1,
			wantAbsent: []string{"ILIKE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			where, args := documentSearchConditions(budgetId, tt.filter)

			if len(args) != tt.wantArgs {
				t.Fatalf("got %d args, want %d (%v)", len(args), tt.wantArgs, args)
			}
			if args[0] != budgetId {
				t.Errorf("first arg = %v, want budget id %v", args[0], budgetId)
			}
			if !strings.Contains(where, "transaction_documents.budget_id = $1") {
				t.Errorf("budget scope missing from: %s", where)
			}
			if !strings.Contains(where, "transaction_documents.deleted = FALSE") {
				t.Errorf("soft-delete filter missing from: %s", where)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(where, want) {
					t.Errorf("want %q in: %s", want, where)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(where, absent) {
					t.Errorf("unexpected %q in: %s", absent, where)
				}
			}
		})
	}
}

func TestDocumentSearchWrapsSearchInWildcards(t *testing.T) {
	_, args := documentSearchConditions(uuid.New(), model.DocumentFilter{Search: " Amazon "})

	if len(args) != 2 {
		t.Fatalf("got %d args, want 2", len(args))
	}
	// trimmed, then wrapped -- a padded query should still match
	if args[1] != "%Amazon%" {
		t.Errorf("search arg = %v, want %q", args[1], "%Amazon%")
	}
}

func TestDocumentKindValid(t *testing.T) {
	valid := []model.DocumentKind{model.DocumentKindAll, model.DocumentKindImage, model.DocumentKindPDF}
	for _, kind := range valid {
		if !kind.Valid() {
			t.Errorf("%q should be valid", kind)
		}
	}
	if model.DocumentKind("video").Valid() {
		t.Error("unknown kind should not be valid")
	}
}
