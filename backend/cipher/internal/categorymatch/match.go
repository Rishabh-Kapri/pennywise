// Package categorymatch finds the budget category a model meant when it named
// one. Both prediction flows (service and adapters/llm) resolve through it.
package categorymatch

import (
	"context"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
)

// Trigram thresholds for a category name the model got almost right.
//
// pg_trgm treats emoji and punctuation as word separators and folds case, so
// the common near-misses score at or near the top of the range while invented
// names score far below the floor. Measured against the real category list:
// a suggested "Dining Out/Entertainment" scores 1.00 against the stored
// emoji-prefixed name (next best 0.07), lowercase "groceries" scores 1.00
// (next 0.08), and "Uncategorized" -- a name no budget has a category for --
// tops out at 0.10.
//
// The margin guards the other failure mode: sibling categories that differ by a
// suffix both score well against a truncated name -- "Travel" scores exactly
// 0.70 against both "Travel - ST" and "Travel - LT". Picking the higher one
// would silently miscategorize, so when the top two are that close no answer
// beats a coin flip.
const (
	minScore  = 0.45
	minMargin = 0.10
)

// Kind says how a name was matched.
type Kind string

const (
	Exact Kind = "exact"
	Fuzzy Kind = "fuzzy"
)

// Result is the budget's category for a suggested name.
type Result struct {
	ID    uuid.UUID
	Name  string
	Kind  Kind
	Score float64 // 1 for an exact match
}

// Match returns the category in categories named suggested, else the closest
// one by trigram similarity. ok is false when nothing is close enough or the
// two best are too close to call.
func Match(
	ctx context.Context,
	repo repository.CategoryRepository,
	budgetID uuid.UUID,
	categories []sharedModel.CategorySimplified,
	suggested string,
) (result Result, ok bool, err error) {
	if suggested == "" {
		return Result{}, false, nil
	}
	for _, c := range categories {
		if c.Name == suggested {
			return Result{ID: c.ID, Name: c.Name, Kind: Exact, Score: 1}, true, nil
		}
	}

	// Local models routinely return a category that is right but not
	// byte-identical: dropping the emoji prefix, changing case or spacing.
	matches, err := repo.FindClosestSimplified(ctx, budgetID, suggested, 2)
	if err != nil {
		return Result{}, false, err
	}
	if len(matches) == 0 || matches[0].Score < minScore {
		return Result{}, false, nil
	}
	if len(matches) > 1 && matches[0].Score-matches[1].Score < minMargin {
		return Result{}, false, nil
	}
	best := matches[0]
	return Result{ID: best.ID, Name: best.Name, Kind: Fuzzy, Score: best.Score}, true, nil
}
