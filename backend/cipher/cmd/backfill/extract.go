package main

import (
	"context"
	"strings"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

// extractAndParse runs Phase 1 LLM extraction on raw email text and builds
// the parsed result. Falls back to raw text splitting if LLM extraction fails.
func (d *BackfillDeps) extractAndParse(ctx context.Context, p resolvedPrediction) *parsedEmailText {
	log := logger.Logger(ctx)

	extracted, err := d.OllamaClient.ExtractEmailData(ctx, p.EmailText)
	if err != nil {
		log.Warn("phase 1 extraction failed, falling back to raw parse", "id", p.ID, "error", err)
		return parseRawEmailText(p.EmailText)
	}

	// Small delay to avoid overwhelming Ollama between inference calls
	time.Sleep(100 * time.Millisecond)

	parsed := buildParsedEmail(extracted, p.TransactionType)
	if parsed == nil {
		log.Warn("extraction returned empty merchant, falling back to raw parse", "id", p.ID)
		return parseRawEmailText(p.EmailText)
	}

	return parsed
}

// buildParsedEmail constructs a parsedEmailText from the Phase 1 LLM extraction output.
func buildParsedEmail(extracted *sharedModel.ExtractedEmailResponse, transactionType string) *parsedEmailText {
	if extracted.Merchant == "" {
		return nil
	}

	cleanedAccountCard := utils.CleanAccountString(extracted.AccountCard)
	upiText, merchantName := utils.CleanUPIText(extracted.Merchant)
	// merchantName = utils.CleanMerchantString(extracted.Merchant)

	return &parsedEmailText{
		TransactionType: transactionType,
		Account:         cleanedAccountCard,
		MerchantString:  extracted.Merchant,
		UPIText:         upiText,
		MerchantName:    merchantName,
	}
}

// parseRawEmailText is the fallback parser when Phase 1 LLM extraction fails.
// Expects pre-cleaned text in format: "<type> <account> <merchant...>"
func parseRawEmailText(text string) *parsedEmailText {
	split := strings.Split(text, " ")
	if len(split) < 3 {
		return nil
	}

	fullMerchantString := strings.Join(split[2:], " ")
	upiText, merchantName := utils.CleanUPIText(fullMerchantString)

	return &parsedEmailText{
		TransactionType: split[0],
		Account:         split[1],
		MerchantString:  fullMerchantString,
		UPIText:         upiText,
		MerchantName:    merchantName,
	}
}
