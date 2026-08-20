package service

import (
	"context"
	"errors"
	"testing"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// stubCategoryRepo answers only the fuzzy lookup; every other repository method
// panics, so a test that reaches one is a test that is not doing what it says.
type stubCategoryRepo struct {
	repository.CategoryRepository
	matches []sharedModel.CategoryNameMatch
	err     error
	// gotName records the name the service asked about.
	gotName  string
	gotLimit int
}

func (s *stubCategoryRepo) FindClosestSimplified(
	ctx context.Context,
	budgetId uuid.UUID,
	name string,
	limit int,
) ([]sharedModel.CategoryNameMatch, error) {
	s.gotName = name
	s.gotLimit = limit
	return s.matches, s.err
}

func TestResolveCategoryFuzzy(t *testing.T) {
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
			svc := &predictionService{categoryRepo: repo}

			match, err := svc.resolveCategoryFuzzy(context.Background(), uuid.New(), tc.suggested)
			require.NoError(t, err)
			require.Equal(t, tc.suggested, repo.gotName)
			// Two candidates are needed to judge ambiguity.
			require.Equal(t, 2, repo.gotLimit)

			if tc.wantID == nil {
				require.Nil(t, match)
				return
			}
			require.NotNil(t, match)
			require.Equal(t, *tc.wantID, match.ID)
			require.Equal(t, tc.wantName, match.Name)
		})
	}
}

func TestResolveCategoryFuzzyPropagatesRepoError(t *testing.T) {
	repo := &stubCategoryRepo{err: errors.New("connection refused")}
	svc := &predictionService{categoryRepo: repo}

	match, err := svc.resolveCategoryFuzzy(context.Background(), uuid.New(), "Groceries")
	require.Error(t, err)
	require.Nil(t, match)
}
