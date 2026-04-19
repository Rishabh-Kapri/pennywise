package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	db "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/client"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"

	"github.com/google/uuid"
)

type Prediction struct {
	ID                    string  `json:"id"`
	BudgetID              string  `json:"budgetId"`
	TransactionID         string  `json:"transactionId"`
	EmailText             string  `json:"emailText"`
	Amount                float64 `json:"amount"`
	Account               *string `json:"account"`
	Payee                 *string `json:"payee"`
	Category              *string `json:"category"`
	HasUserCorrected      *bool   `json:"hasUserCorrected"`
	UserCorrectedPayee    *string `json:"userCorrectedPayee"`
	UserCorrectedAccount  *string `json:"userCorrectedAccount"`
	UserCorrectedCategory *string `json:"userCorrectedCategory"`
}

type normalizedMCC struct {
	CanonicalName string `json:"canonical_name"`
	MCCTag        string `json:"mcc_tag"`
	Reasoning     string `json:"reasoning"`
}

func main() {
	ctx := context.Background()
	log := logger.Logger(ctx)
	cfg := config.Load()

	var dataPath string
	var backfillTargets string
	flag.StringVar(&dataPath, "data", "", "path to json file containing prediction data")
	flag.StringVar(&backfillTargets, "backfill", "mcc,transaction", "comma-separated list of targets (mcc,transaction)")
	flag.Parse()

	targets := strings.Split(backfillTargets, ",")
	runMcc := false
	runTransaction := false

	for _, target := range targets {
		t := strings.TrimSpace(target)
		switch t {
		case "mcc":
			runMcc = true
		case "transaction":
			runTransaction = true
		default:
			logger.Fatal("Invalid target: %s", target)
		}
	}
	if !runMcc && !runTransaction {
		logger.Fatal("At least one backfill target is required (mcc, transaction)")
	}

	pennywiseAPI := os.Getenv("PENNYWISE_API")
	if pennywiseAPI == "" {
		logger.Fatal("PENNYWISE_API environment variable is required")
	}

	budgetIDStr := os.Getenv("BUDGET_ID")
	if budgetIDStr == "" {
		logger.Fatal("BUDGET_ID environment variable is required")
	}
	budgetID, err := uuid.Parse(budgetIDStr)
	if err != nil {
		logger.Fatal("Invalid BUDGET_ID: %v", err)
	}

	dbConn, err := db.ConnectWithURL(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer dbConn.Close()

	// Ollama client via shared transport
	ollamaEngine := httpclient.NewHttpTransport(cfg.OllamaURL)
	ollamaTransport := transport.NewClient("ollama", ollamaEngine)
	ollamaClient := client.NewOllamaClient(ollamaTransport)

	// Pennywise API client via shared transport
	pennywiseEngine := httpclient.NewHttpTransport(pennywiseAPI)
	pennywiseClient := transport.NewClient("pennywise-api", pennywiseEngine)

	embeddingRepo := repository.NewTransactionEmbeddingRepository(dbConn)
	merchantRepo := repository.NewMerchantRepository(dbConn)

	mccTags := "FOOD_DELIVERY, FAST_FOOD, DINING_OUT, COFFEE_SHOP, GROCERIES, QUICK_COMMERCE, PHARMACY, E_COMMERCE, SHOPPING_CLOTHING, SHOPPING_ELECTRONICS, SHOPPING_FURNITURE, SHOPPING_GENERAL, RENT_MORTGAGE, UTILITY_ELECTRICITY, UTILITY_WATER, UTILITY_GAS, UTILITY_BROADBAND, TELECOM_MOBILE, HOME_MAINTENANCE, TRANSPORT_LOCAL, TRANSIT_PUBLIC, TRAVEL_FLIGHTS, TRAVEL_TRAINS, TRAVEL_HOTELS, SUBSCRIPTION_VIDEO, SUBSCRIPTION_AUDIO, SUBSCRIPTION_SOFTWARE, SUBSCRIPTION_DIGITAL, ENTERTAINMENT_MOVIES, ENTERTAINMENT_EVENTS, GAMING, MEDICAL_HOSPITAL, FITNESS_GYM, SPORTS, GROOMING_SALON, BILL_CREDIT_CARD, BILL_EMI, TAX, INSURANCE_LIFE, INSURANCE_HEALTH, INSURANCE_VEHICLE, INVESTMENT_MUTUAL_FUND, INVESTMENT_STOCKS, INVESTMENT_CRYPTO, INVESTMENT_GOLD, INVESTMENT_FD_RD, INVESTMENT_NPS_PPF, EDUCATION_FEES, PET_CARE, CHILDREN, CHARITY_DONATION, GIFTS, INCOME_SALARY, INCOME_FREELANCE, INCOME_BUSINESS, INCOME_REWARD_CASHBACK, INCOME_REFUND, INCOME_INTEREST_DIVIDEND, TRANSFER_SELF, TRANSFER_P2P, CASH_WITHDRAWAL, WALLET_TOPUP, CHARGES_FEES"

	var predictions []Prediction
	var source string
	if dataPath != "" {
		log.Info("loading predictions from file", "path", dataPath)

		fileData, err := os.ReadFile(dataPath)
		if err != nil {
			logger.Fatal("Failed to read data file", "err", err)
		}
		if err := json.Unmarshal(fileData, &predictions); err != nil {
			logger.Fatal("Failed to unmarshal data file", err)
		}
		source = "historical"
	} else {
		// Fetch predictions from go-pennywise-api via transport
		ctx = utils.WithBudgetID(ctx, budgetID)
		var err error
		predictions, err = transport.Get[[]Prediction](ctx, pennywiseClient, "/api/predictions")
		if err != nil {
			logger.Fatal("Failed to fetch predictions", err)
		}
		source = "prediction"
	}

	log.Info("loaded predictions", "count", len(predictions))
	log.Info("Running backfills", "targets", targets)

	success, failed := 0, 0
	for i, p := range predictions {
		if p.EmailText == "" {
			log.Warn("skipping prediction with empty email text", "id", p.ID)
			failed++
			continue
		}
		transactionType := "debited"
		if p.Amount > 0 {
			transactionType = "credited"
		}
		cleanedEmailText := utils.CleanEmailText(p.EmailText, transactionType)
		// slog.Info("email text", "text", p.EmailText, "cleaned", cleanedEmailText)

		// Determine the correct labels (prefer user-corrected values)
		payee := deref(p.Payee)
		category := deref(p.Category)
		account := deref(p.Account)

		if p.HasUserCorrected != nil && *p.HasUserCorrected {
			if p.UserCorrectedPayee != nil {
				payee = *p.UserCorrectedPayee
			}
			if p.UserCorrectedCategory != nil {
				category = *p.UserCorrectedCategory
			}
			if p.UserCorrectedAccount != nil {
				account = *p.UserCorrectedAccount
			}
			source = "user_corrected"
		}

		if payee == "" || category == "" || account == "" {
			log.Warn("skipping prediction with empty labels", "id", p.ID)
			failed++
			continue
		}

		if runTransaction {
			// Generate embedding from cleaned text
			embedding, err := ollamaClient.Embed(ctx, "bge-m3", cleanedEmailText)
			if err != nil {
				log.Error("failed to embed", "id", p.ID, "error", err)
				failed++
				continue
			}

			embeddingStr := db.VectorToString(embedding)

			var txnID *uuid.UUID = nil
			if p.TransactionID != "" {
				val, _ := uuid.Parse(p.TransactionID)
				txnID = &val
			}

			data := model.TransactionEmbedding{
				BudgetID:      budgetID,
				EmbeddingText: cleanedEmailText,
				Payee:         payee,
				Category:      category,
				Account:       account,
				Amount:        p.Amount,
				TransactionID: txnID,
				Source:        source,
			}
			slog.Debug("data", "data", data)

			if err := embeddingRepo.Upsert(ctx, nil, data, embeddingStr); err != nil {
				log.Error("failed to upsert embedding", "id", p.ID, "error", err)
				failed++
				continue
			}
			// Small delay to not overwhelm Ollama
			time.Sleep(100 * time.Millisecond)
		}

		if runMcc {
			split := strings.Split(cleanedEmailText, " ")
			if len(split) < 2 {
				log.Warn("skipping prediction with empty email text", "id", p.ID, "email", cleanedEmailText)
				failed++
				continue
			}
			merchant := strings.Join(split[1:], " ")
			upiText, cleanedMerchant := utils.CleanUPIText(merchant)
			if upiText != "" {
				// handle UPI by creating payee_matches
			}
			log.Info("cleaned merchant", "raw", cleanedEmailText, "cleaned", cleanedMerchant, "upiText", upiText)

			prompt := fmt.Sprintf(`Analyze this raw bank transaction merchant string: "%s"
			Your goal is to identify the underlying merchant and categorize it. Follow these strict rules:
			1. Canonical Name: Extract the widely recognized consumer brand name. Do NOT use the legal corporate entity name if a well-known consumer app/brand exists (e.g., "Novi Digital" -> "Hotstar", "BUNDL TECH" -> "Swiggy", "One97" -> "Paytm").
			2. Clean up: Strip all payment gateways, store codes, locations, and random IDs (e.g., "PYU*Swiggy" -> "Swiggy", "ZOMATO ANDHERI" -> "Zomato"). Use proper Title Case.
			3. Category: STRICTLY Select the single best matching category from this exact list: [%s].
			Output ONLY valid JSON with exact keys "canonical_name" and "mcc_tag". Do not include markdown blocks or explanations.`, cleanedMerchant, mccTags)
			llmModel := "openai/gpt-5.4-mini"
			req := client.PromptReq{
				Model:  llmModel,
				Prompt: prompt,
			}
			log.Info("req", "req", req)
			if false {
				normalizedMCC, err := client.GenericLLMCall[normalizedMCC](ctx, ollamaClient, req)
				if err != nil {
					log.Error("failed to send prompt", "id", p.ID, "error", err)
					failed++
					continue
				}
				log.Info("normalized mcc", "normalized", normalizedMCC)

				merchantData := model.GlobalMerchant{
					CanonicalName: normalizedMCC.CanonicalName,
					MCCTag:        model.GlobalMCCTag(normalizedMCC.MCCTag),
				}

				globalMerchant, err := merchantRepo.CreateGlobalMerchant(ctx, nil, merchantData)
				if err != nil {
					log.Error("failed to create global merchant", "id", p.ID, "error", err)
					failed++
					continue
				}
				if globalMerchant == nil {
					log.Warn("merchant not found", "id", p.ID, "error", err)
					failed++
					continue
				}
				merchantMapping := model.GlobalMerchantMapping{
					CleanedRawText: cleanedMerchant,
					MerchantID:     globalMerchant.ID,
				}
				err = merchantRepo.CreateGlolabMerchantMapping(ctx, nil, merchantMapping)
				if err != nil {
					log.Error("failed to create merchant mapping", "id", p.ID, "error", err)
					failed++
					continue
				}
			}
			success++
		}

		if (i+1)%50 == 0 {
			log.Info("progress", "processed", i+1, "total", len(predictions), "success", success, "failed", failed)
		}
	}

	log.Info("backfill complete", "total", len(predictions), "success", success, "failed", failed)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
