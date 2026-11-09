// Package aliasAdapter is the lookup step: has this budget seen this merchant
// before? It reads no model, so private bank text stays in the process.
package aliasAdapter

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/narration"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

type lookup struct {
	aliases repository.CounterpartyAliasRepository
}

// NewLookup matches messages against the budget's counterparty aliases.
func NewLookup(aliases repository.CounterpartyAliasRepository) ports.MemoryLookup {
	return &lookup{aliases: aliases}
}

// Lookup implements [ports.MemoryLookup]. It uses the rule-parsed key, not the
// extracted merchant, because that's what aliases are stored under.
func (l *lookup) Lookup(
	ctx context.Context,
	msg ports.Message,
	_ ports.ExtractedEntities,
) (ports.PredictionResult, bool, error) {
	log := logger.Logger(ctx).With("step", "lookup")
	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		return ports.PredictionResult{}, false, err
	}

	parsed := narration.Parse(msg.Body)
	key := parsed.LookupKey()
	if key == "" {
		log.Debug("no lookup key in the message")
		return ports.PredictionResult{}, false, nil
	}

	match, err := l.aliases.FindMatch(ctx, budgetID, parsed.Channel, parsed.Merchant)
	if err != nil {
		return ports.PredictionResult{}, false, errs.Wrap(errs.CodeMessageLookupFailed, "find alias", err)
	}
	if match == nil {
		log.Debug("no alias for key", "key", key)
		return ports.PredictionResult{}, false, nil
	}

	log.Info("alias matched", "key", key, "payee", match.PayeeName, "category", match.CategoryName)
	return ports.PredictionResult{
		PayeeName:    match.PayeeName,
		CategoryName: match.CategoryName,
		Confidence:   1,
	}, true, nil
}
