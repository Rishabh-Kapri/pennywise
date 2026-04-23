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

// backfillPayeeRulesiaLLM normalizes a merchant via LLM and creates global merchant + mapping.
// This implements Phase 4 of the cipher.md classification pipeline.
// Currently not called — enable when LLM bridge is integrated.
//
//nolint:unused
func (d *BackfillDeps) backfillPayeeRulesiaLLM(ctx context.Context, parsed *parsedEmailText) error {
	log := logger.Logger(ctx)
	_ = log

	// TODO: Uncomment when Phase 4 LLM bridge is ready.
	// The flow:
	// 1. Send merchant name to LLM for normalization → {canonical_name, mcc_tag}
	// 2. Create/find global_merchant record
	// 3. Create global_merchant_mapping (cleaned_raw_text → merchant_id)
	//
	// prompt := fmt.Sprintf(`Analyze this raw bank transaction merchant string: "%s" ...`, parsed.MerchantName)
	// req := client.PromptReq{Model: "openai/gpt-5.4-mini", Prompt: prompt}
	// normalized, err := client.GenericLLMCall[normalizedMCC](ctx, d.OllamaClient, req)
	// ...
	// merchantData := model.GlobalMerchant{CanonicalName: normalized.CanonicalName, MCCTag: model.GlobalMCCTag(normalized.MCCTag)}
	// globalMerchant, err := d.MerchantRepo.CreateGlobalMerchant(ctx, nil, merchantData)
	// ...
	// cleanedRawText := parsed.UPIText
	// if cleanedRawText == "" { cleanedRawText = parsed.MerchantName }
	// mapping := model.GlobalMerchantMapping{CleanedRawText: cleanedRawText, MerchantID: globalMerchant.ID}
	// return d.MerchantRepo.CreateGlolabMerchantMapping(ctx, nil, mapping)

	return nil
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
		payee, err = d.PayeeRepo.Create(ctx, nil, newPayee)
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
		CategoryID:  foundCategory.ID,
		MatchString: upiText,
		MatchType:   "EXACT",
	}
	return d.PayeeRuleRepo.CreatePayeeRule(ctx, nil, data)
}
