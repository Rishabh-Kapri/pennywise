package categorymatch

import (
	"context"
	"errors"
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// stubCategoryRepo answers only the fuzzy lookup; every other repository method
// panics, so a test that reaches one is a test that is not doing what it says.
type stubCategoryRepo struct {
	repository.CategoryRepository
	matches []sharedModel.CategoryNameMatch
	err     error
}

func (s *stubCategoryRepo) FindClosestSimplified(
	ctx context.Context,
	budgetId uuid.UUID,
	name string,
	limit int,
) ([]sharedModel.CategoryNameMatch, error) {
	return s.matches, s.err
}

func TestMatchFuzzy(t *testing.T) {
	dining := uuid.New()
	travelST := uuid.New()

	tests := []struct {
		name      string
		suggested string
		matches   []sharedModel.CategoryNameMatch
		wantID    *uuid.UUID
		wantName  string
	}{
		{
			// The reported failure: the model dropped the emoji prefix.
			name:      "near-exact match wins",
			suggested: "Dining Out/Entertainment",
			matches: []sharedModel.CategoryNameMatch{
				{ID: dining, Name: "🍽️ Dining Out/Entertainment", Score: 1},
				{ID: travelST, Name: "🚗 Travel - ST", Score: 0.08},
			},
			wantID:   &dining,
			wantName: "🍽️ Dining Out/Entertainment",
		},
		{
			name:      "clear winner above the floor is accepted",
			suggested: "groceries",
			matches: []sharedModel.CategoryNameMatch{
				{ID: dining, Name: "🛒 Groceries", Score: 0.72},
				{ID: travelST, Name: "🎁 Gift", Score: 0.1},
			},
			wantID:   &dining,
			wantName: "🛒 Groceries",
		},
		{
			// "Uncategorized" and other invented names resolve to nothing.
			name:      "nothing close enough is rejected",
			suggested: "Uncategorized",
			matches: []sharedModel.CategoryNameMatch{
				{ID: dining, Name: "👕 Clothing", Score: 0.19},
			},
		},
		{
			// Sibling categories separated only by a suffix: guessing would
			// silently miscategorize, so the prediction fails instead.
			name:      "ambiguous top two is rejected",
			suggested: "Travel",
			matches: []sharedModel.CategoryNameMatch{
				{ID: travelST, Name: "🚗 Travel - ST", Score: 0.62},
				{ID: dining, Name: "✈️ Travel - LT", Score: 0.60},
			},
		},
		{
			name:      "no categories at all",
			suggested: "Groceries",
			matches:   nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubCategoryRepo{matches: tc.matches}

			match, ok, err := Match(context.Background(), repo, uuid.New(), nil, tc.suggested)
			require.NoError(t, err)
			if tc.wantID == nil {
				require.False(t, ok)
				return
			}
			require.True(t, ok)
			require.Equal(t, *tc.wantID, match.ID)
			require.Equal(t, tc.wantName, match.Name)
			require.Equal(t, Fuzzy, match.Kind)
		})
	}
}

func TestMatchPropagatesRepoError(t *testing.T) {
	repo := &stubCategoryRepo{err: errors.New("connection refused")}

	_, ok, err := Match(context.Background(), repo, uuid.New(), nil, "Groceries")
	require.Error(t, err)
	require.False(t, ok)
}

// An exact name never reaches the fuzzy lookup, which would fail here.
func TestMatchExactNameSkipsTheFuzzyLookup(t *testing.T) {
	groceries := uuid.New()
	repo := &stubCategoryRepo{err: errors.New("should not be called")}

	match, ok, err := Match(context.Background(), repo, uuid.New(),
		[]sharedModel.CategorySimplified{{ID: groceries, Name: "🛒 Groceries"}}, "🛒 Groceries")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, Result{ID: groceries, Name: "🛒 Groceries", Kind: Exact, Score: 1}, match)
}

func TestMatchEmptySuggestionIsNoMatch(t *testing.T) {
	repo := &stubCategoryRepo{err: errors.New("should not be called")}

	_, ok, err := Match(context.Background(), repo, uuid.New(), nil, "")
	require.NoError(t, err)
	require.False(t, ok)
}
