package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	db "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/client"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

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

type payeeMatchInput struct {
	budgetID          uuid.UUID
	merchantName      string
	upiText           string
	predictedPayee    string
	predictedCategory string
	payeeRepo         *repository.PayeesRepository
	payeeMatchRepo    *repository.PayeeMatchRepository
	categoryRepo      *repository.CategoryRepository
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
	payeeRepo := repository.NewPayeesRepository(dbConn)
	payeeMatchRepo := repository.NewPayeeMatchRepository(dbConn)
	categoryRepo := repository.NewCategoryRepository(dbConn)

	mccTags := "FOOD_DELIVERY, FAST_FOOD, DINING_OUT, COFFEE_SHOP, GROCERIES, QUICK_COMMERCE, PHARMACY, E_COMMERCE, SHOPPING_CLOTHING, SHOPPING_ELECTRONICS, SHOPPING_FURNITURE, SHOPPING_GENERAL, RENT_MORTGAGE, UTILITY_ELECTRICITY, UTILITY_WATER, UTILITY_GAS, UTILITY_BROADBAND, TELECOM_MOBILE, HOME_MAINTENANCE, TRANSPORT_LOCAL, TRANSIT_PUBLIC, TRAVEL_FLIGHTS, TRAVEL_TRAINS, TRAVEL_HOTELS, SUBSCRIPTION_VIDEO, SUBSCRIPTION_AUDIO, SUBSCRIPTION_SOFTWARE, SUBSCRIPTION_DIGITAL, ENTERTAINMENT_MOVIES, ENTERTAINMENT_EVENTS, GAMING, MEDICAL_HOSPITAL, FITNESS_GYM, SPORTS, GROOMING_SALON, BILL_CREDIT_CARD, BILL_EMI, TAX, INSURANCE_LIFE, INSURANCE_HEALTH, INSURANCE_VEHICLE, INVESTMENT_MUTUAL_FUND, INVESTMENT_STOCKS, INVESTMENT_CRYPTO, INVESTMENT_GOLD, INVESTMENT_FD_RD, INVESTMENT_NPS_PPF, EDUCATION_FEES, PET_CARE, CHILDREN, CHARITY_DONATION, GIFTS, INCOME_SALARY, INCOME_FREELANCE, INCOME_BUSINESS, INCOME_REWARD_CASHBACK, INCOME_REFUND, INCOME_INTEREST_DIVIDEND, TRANSFER_SELF, TRANSFER_P2P, CASH_WITHDRAWAL, WALLET_TOPUP, CHARGES_FEES"
	skipUPI := map[string]bool{
		"zerodhamf@hdfcbank":                    true,
		"novidigitalentautopayrzp@hdfcbank":     true,
		"paytm-blinkit@ptybl":                   true,
		"playstore@axisbank":                    true,
		"paytmqr5eqr9v@ptys":                    true,
		"zeptonow-2bdpg@hdfcbank":               true,
		"bsestarmfrzp@icici":                    true,
		"batukbhaisonsjewelle68103941@hdfcbank": true,
		"paytms1f9myp@pty":                      true,
		"gpay-11170568058@okbizaxis":            true, // platinum super store
		"ubuntusalons99933697@hdfcbank":         true,
		"zerodhabrokingbrk@validaxis":           true,
		"paytmqr5d3f1q@ptys":                    true, // kailash super market
		"11230094718@okbizaxis":                 true,
		"9891771064@okbizicici":                 true,
		"ka57f1731@cnrb":                        true, // bmtc
		"ka57f1814@cnrb":                        true, // bmtc
		"sonypictures14payu@icici":              true,
		"seedlinghospitalityp68025861@hdfcbank": true,
		"mrdiy96160277@hdfcbank":                true, // mr diy
		"paytm-75735390@ptys":                   true, // corridor 7
		"hdfcltd71372996@hdfcbank":              true, // hdfc housing loan
		"zerodhaiccl3brk@validhdfc":             true,
		"bellavita96148647@hdfcbank":            true, // bella vita
		"paytmqr2810050501011dpcfcxv0hc9@paytm": true, // noble chemist
		"Q180767957@ybl":                        true, // numero uno pithoragarh
		"credclub@axisb":                        true, // cred club
		"tickertapepro@yespay":                  true, // ticker tape pro
		"uberindiasystem187204rzp@rxairtel":     true, // uber india
		"indstocksm2p@hdfcbank":                 true, // indmoney
		"9997684099@okbizaxis":                  true, // variety store
		"75735390@ptys":                         true, // corridor 7
		"9897965590@ptaxis":                     true, // bombay optician
		"gpay-11256964070@okbizaxis":            true, // cafe on the rocks
		"hdfclimitedbilldesk@hdfcbank":          true, // hdfc limited
	}

	var predictions []Prediction
	if dataPath != "" {
		log.Info("loading predictions from file", "path", dataPath)

		fileData, err := os.ReadFile(dataPath)
		if err != nil {
			logger.Fatal("Failed to read data file", "err", err)
		}
		if err := json.Unmarshal(fileData, &predictions); err != nil {
			logger.Fatal("Failed to unmarshal data file", err)
		}
	} else {
		// Fetch predictions from go-pennywise-api via transport
		ctx = utils.WithBudgetID(ctx, budgetID)
		var err error
		predictions, err = transport.Get[[]Prediction](ctx, pennywiseClient, "/api/predictions")
		if err != nil {
			logger.Fatal("Failed to fetch predictions", err)
		}
	}

	log.Info("loaded predictions", "count", len(predictions))
	log.Info("Running backfills", "targets", targets)

	success, failed := 0, 0
	for i, p := range predictions {
		source := "AUTO_LEARNED"
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
		// log.Info("email text", "text", p.EmailText, "cleaned", cleanedEmailText)

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
			source = "MANUAL"
		}

		if payee == "" || category == "" || account == "" {
			log.Warn("skipping prediction with empty labels", "id", p.ID)
			failed++
			continue
		}

		/**
		* [0] transaction type (debit, credit)
		* [1] account/card number
		* [3] merchant
		 */
		split := strings.Split(cleanedEmailText, " ")
		if len(split) < 2 {
			log.Warn("skipping prediction with empty email text", "id", p.ID, "email", cleanedEmailText)
			failed++
			continue
		}
		// log.Info("split", "id", p.ID, "text", cleanedEmailText, "split", split)
		fullMerchantString := strings.Join(split[2:], " ")
		upiText, merchantName := utils.CleanUPIText(fullMerchantString)

		if runTransaction {
			foundPayee, err := payeeRepo.Search(ctx, budgetID, payee)
			if err != nil {
				log.Error("failed to search payee", "id", p.ID, "error", err)
				failed++
				continue
			}
			if len(foundPayee) == 0 {
				log.Warn("skipping prediction with empty payee", "id", p.ID, "email", cleanedEmailText)
				failed++
				continue
			}
			foundCategory, err := categoryRepo.Search(ctx, budgetID, category)
			if err != nil {
				log.Error("failed to search category", "id", p.ID, "error", err)
				failed++
				continue
			}
			if len(foundCategory) == 0 {
				log.Warn("skipping prediction with empty category", "id", p.ID, "email", cleanedEmailText)
				failed++
				continue
			}
			// Generate embedding from cleaned text
			embeddingText := split[0] + " " + strings.Join(split[2:], " ")
			log.Info("embedding", "id", p.ID, "text", embeddingText)

			embedding, err := ollamaClient.Embed(ctx, "bge-m3", embeddingText)
			if err != nil {
				log.Error("failed to embed", "id", p.ID, "error", err)
				failed++
				continue
			}

			embeddingStr := db.VectorToString(embedding)

			data := model.TransactionEmbedding{
				BudgetID:      budgetID,
				EmbeddingText: embeddingText,
				PayeeID:       foundPayee[0].ID,
				CategoryID:    foundCategory[0].ID,
				Amount:        p.Amount,
				Source:        source,
			}

			if err := embeddingRepo.Upsert(ctx, nil, data, embeddingStr); err != nil {
				log.Error("failed to upsert embedding", "id", p.ID, "error", err)
				failed++
				continue
			}
			// Small delay to not overwhelm Ollama
			time.Sleep(100 * time.Millisecond)
		}

		if runMcc {
			if skipUPI[upiText] {
				continue
			}
			if upiText != "" {
				// log.Info("cleaned merchant", "raw", cleanedEmailText, "cleaned", cleanedMerchant, "upiText", upiText)
			}
			log.Info("cleaned merchant", "raw", cleanedEmailText, "merchant name", merchantName, "upiText", upiText)

			if skipUPI[upiText] || upiText == "" {
				if false {
					prompt := fmt.Sprintf(`Analyze this raw bank transaction merchant string: "%s"
						Your goal is to identify the underlying merchant and categorize it. Follow these strict rules:
						1. Canonical Name: Extract the widely recognized consumer brand name. Do NOT use the legal corporate entity name if a well-known consumer app/brand exists (e.g., "Novi Digital" -> "Hotstar", "BUNDL TECH" -> "Swiggy", "One97" -> "Paytm").
						2. Clean up: Strip all payment gateways, store codes, locations, and random IDs (e.g., "PYU*Swiggy" -> "Swiggy", "ZOMATO ANDHERI" -> "Zomato"). Use proper Title Case.
						3. Category: STRICTLY Select the single best matching category from this exact list: [%s].
						Output ONLY valid JSON with exact keys "canonical_name" and "mcc_tag". Do not include markdown blocks or explanations.`, merchantName, mccTags)
					llmModel := "openai/gpt-5.4-mini"
					req := client.PromptReq{
						Model:  llmModel,
						Prompt: prompt,
					}
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
					cleanedRawText := ""
					if upiText != "" {
						cleanedRawText = upiText
					} else {
						cleanedRawText = merchantName
					}
					merchantMapping := model.GlobalMerchantMapping{
						CleanedRawText: cleanedRawText,
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
			} else {
				// Handle UPI merchants
				input := payeeMatchInput{
					budgetID:          budgetID,
					merchantName:      merchantName,
					upiText:           upiText,
					predictedPayee:    payee,
					predictedCategory: category,
					payeeRepo:         &payeeRepo,
					payeeMatchRepo:    &payeeMatchRepo,
					categoryRepo:      &categoryRepo,
				}
				err := handleUPIMerchant(ctx, input)
				if err != nil {
					log.Error("failed to handle UPI merchant", "id", p.ID, "error", err)
					failed++
					continue
				}
				// no need for llm call for UPI merchants
				success++
			}
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

func handleUPIMerchant(ctx context.Context, input payeeMatchInput) error {
	log := logger.Logger(ctx)
	budgetID := input.budgetID
	merchantName := input.merchantName
	upiText := input.upiText
	payeeRepo := *input.payeeRepo
	payeeMatchRepo := *input.payeeMatchRepo
	categoryRepo := *input.categoryRepo

	// handle UPI by creating payee_matches
	payeeMatch, err := payeeMatchRepo.FindByMatchString(ctx, budgetID, upiText)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, "failed to find payee match", err)
	}
	if payeeMatch != nil {
		return errs.New(errs.CodeInternalError, "payee match already exists")
	}
	log.Info("creating payee match", "match", upiText)

	var payee *model.Payee

	foundPayee, err := payeeRepo.Search(ctx, budgetID, input.predictedPayee)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, "failed to search payee", err)
	}

	category, err := categoryRepo.Search(ctx, budgetID, input.predictedCategory)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, "failed to search category", err)
	}
	if len(category) == 0 {
		return errs.New(errs.CodeInternalError, "category not found")
	}
	if len(foundPayee) == 0 {
		newPayee := model.Payee{
			Name:                merchantName,
			BudgetID:            budgetID,
			TransferAccountID:   nil,
			CanonicalMerchantID: nil,
			DefaultCategoryID:   &category[0].ID,
		}
		payee, err = payeeRepo.Create(ctx, nil, newPayee)
		if err != nil {
			return errs.Wrap(errs.CodeInternalError, "failed to create payee", err)
		}
	} else {
		payee = &foundPayee[0]
	}
	// create local payee match
	data := model.PayeeMatch{
		BudgetID:    budgetID,
		PayeeID:     payee.ID,
		MatchString: upiText,
	}
	err = payeeMatchRepo.CreatePayeeMatch(ctx, nil, data)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, "failed to create payee match", err)
	}
	return nil
}
