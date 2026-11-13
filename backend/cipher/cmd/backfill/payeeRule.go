package main

import (
	"context"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

// parsedEmailText holds the decomposed parts of an email after Phase 1 LLM extraction.
// Built by extractAndParse() (extract.go) using Ollama to pull structured data
// from raw bank email text.
type parsedEmailText struct {
	TransactionType string // "debited" or "credited"
	Account         string // card/account number (e.g., "HDFC 1234")
	MerchantString  string // full merchant string from LLM (may contain UPI handle)
	UPIText         string // extracted UPI handle (e.g., "swiggy@hdfcbank"), empty if none
	MerchantName    string // cleaned merchant name after stripping UPI handle
}

// backfillPayeeRules handles the payee rule classification backfill for a single prediction.
// For UPI merchants: creates payee_rules linking UPI handles to payees.
// For non-UPI merchants: LLM-based (currently disabled, pending Phase 4 integration).
func (d *BackfillDeps) backfillPayeeRules(ctx context.Context, p resolvedPrediction, parsed *parsedEmailText) error {
	log := logger.Logger(ctx)

	if skipUPIAddresses[parsed.UPIText] {
		return nil
	}

	log.Info("cleaned merchant", "merchant name", parsed.MerchantName, "upiText", parsed.UPIText)

	if parsed.UPIText == "" {
		// Non-UPI merchant: Phase 4 LLM normalization.
		// Currently disabled — enable when the LLM bridge (cipher.md Phase 4) is integrated.
		// See backfillMCCViaLLM() for the implementation.
		return nil
	}

	// UPI merchant: create payee_match linking the UPI handle to a payee
	return d.handleUPIMerchant(ctx, parsed.MerchantName, parsed.UPIText, p.Payee, p.Category)
}

// handleUPIMerchant creates a payee_match record linking a UPI handle to an existing
// or newly created payee. This is the Phase 2 "Fast Path" builder from cipher.md.
func (d *BackfillDeps) handleUPIMerchant(
	ctx context.Context,
	merchantName, upiText, predictedPayee, predictedCategory string,
) error {
	log := logger.Logger(ctx)
	budgetID := d.BudgetID

	// Check if mapping already exists
	// payeeMatch, err := d.PayeeRuleRepo.FindByMatchString(ctx, budgetID, upiText)
	// if err != nil {
	// 	return errs.Wrap(errs.CodeInternalError, "failed to find payee match", err)
	// }
	// if payeeMatch != nil {
	// 	return errs.New(errs.CodeInternalError, "payee match already exists")
	// }

	log.Info("creating payee match", "match", upiText)

	// Look up category
	foundCategory, err := d.getCategory(ctx, budgetID, predictedCategory, false)
	if err != nil {
		return err
	}

	// Find or create payee
	var payee *model.Payee
	foundPayees, err := d.PayeeRepo.Search(ctx, budgetID, predictedPayee)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, "failed to search payee", err)
	}

	if len(foundPayees) == 0 {
		newPayee := model.Payee{
			Name:     merchantName,
			BudgetID: budgetID,
		}
		payee, err = d.PayeeRepo.Create(ctx, newPayee)
		if err != nil {
			return errs.Wrap(errs.CodeInternalError, "failed to create payee", err)
		}
	} else {
		payee = &foundPayees[0]
	}

	// Create the UPI → payee mapping
	data := model.PayeeRule{
		BudgetID:    budgetID,
		PayeeID:     payee.ID,
		CategoryID:  &foundCategory.ID,
		MatchString: upiText,
		MatchType:   "EXACT",
	}
	return d.PayeeRuleRepo.CreatePayeeRule(ctx, data)
}
