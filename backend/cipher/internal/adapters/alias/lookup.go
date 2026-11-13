// Package aliasAdapter is the lookup step: has this budget seen this merchant
// before? It reads no model, so private bank text stays in the process.
package aliasAdapter

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils/narration"
)

type lookup struct {
	aliases repository.CounterpartyAliasRepository
}

// NewLookup matches messages against the budget's counterparty aliases.
func NewLookup(aliases repository.CounterpartyAliasRepository) ports.MemoryLookup {
	return &lookup{aliases: aliases}
}

// Lookup implements [ports.MemoryLookup]. A UPI payment is tried by its VPA and
// then by name, because aliases learned before VPAs were used are under the name.
func (l *lookup) Lookup(
	ctx context.Context,
	_ ports.Message,
	entities ports.ExtractedEntities,
) (ports.PredictionResult, bool, error) {
	log := logger.Logger(ctx).With("step", "lookup")
	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		return ports.PredictionResult{}, false, err
	}

	merchants := []string{
		narration.LookupMerchant(entities.Channel, entities.Merchant, entities.VPA),
		narration.NormalizeMerchant(entities.Merchant),
	}
	for i, merchant := range merchants {
		if entities.Channel == "" || merchant == "" || (i > 0 && merchant == merchants[0]) {
			continue
		}
		key := entities.Channel + " | " + merchant
		match, err := l.aliases.FindMatch(ctx, budgetID, entities.Channel, merchant)
		if err != nil {
			return ports.PredictionResult{}, false, errs.Wrap(errs.CodeMessageLookupFailed, "find alias", err)
		}
		if match == nil {
			log.Debug("no alias for key", "key", key)
			continue
		}
		if model.IsUncategorized(match.CategoryName) {
			// It knows the payee but not where the money goes, so predict.
			log.Debug("alias has no real category", "key", key)
			continue
		}
		log.Info("alias matched", "key", key, "payee", match.PayeeName, "category", match.CategoryName, "source", match.Source)
		return ports.PredictionResult{
			PayeeName:    match.PayeeName,
			CategoryName: match.CategoryName,
			Confidence:   aliasConfidence(match.Source),
		}, true, nil
	}
	return ports.PredictionResult{}, false, nil
}

// Learned aliases come only from confident predictions, but a person's choice
// is still worth more.
const (
	manualAliasConfidence  = 1.0
	learnedAliasConfidence = 0.8
)

func aliasConfidence(source string) float64 {
	if source == model.AliasSourceManual {
		return manualAliasConfidence
	}
	return learnedAliasConfidence
}
