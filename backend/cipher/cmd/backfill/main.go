package main

import (
	"context"
	"flag"
	"os"
	"strings"

	db "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"

	"github.com/google/uuid"
)

type backfillTargets struct {
	PayeeRule  bool
	Embeddings bool
}

func parseFlags() (dataPath string, targets backfillTargets) {
	var backfillStr string
	flag.StringVar(&dataPath, "data", "", "path to json file containing prediction data")
	flag.StringVar(
		&backfillStr,
		"backfill",
		"payeeRule,transaction",
		"comma-separated list of targets (payeeRule,transaction)",
	)
	flag.Parse()

	for _, target := range strings.Split(backfillStr, ",") {
		switch strings.TrimSpace(target) {
		case "payeeRule":
			targets.PayeeRule = true
		case "transaction":
			targets.Embeddings = true
		default:
			logger.Fatal("Invalid target: %s", target)
		}
	}

	if !targets.PayeeRule && !targets.Embeddings {
		logger.Fatal("At least one backfill target is required (payeeRule, transaction)")
	}
	return
}

func main() {
	ctx := context.Background()
	log := logger.Logger(ctx)
	cfg := config.Load()

	dataPath, targets := parseFlags()
	log.Info("flags", "data", dataPath, "targets", targets)

	pennywiseAPI := os.Getenv("PENNYWISE_API")
	if pennywiseAPI == "" {
		logger.Fatal("PENNYWISE_API environment variable is required")
	}

	budgetID, err := uuid.Parse(os.Getenv("BUDGET_ID"))
	if err != nil {
		logger.Fatal("Invalid or missing BUDGET_ID: %v", err)
	}

	dbConn, err := db.ConnectWithURL(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer dbConn.Close()

	deps, pennywiseClient := initDeps(cfg, pennywiseAPI, budgetID, dbConn)
	predictions := loadPredictions(ctx, pennywiseClient, budgetID, dataPath)

	log.Info("loaded predictions", "count", len(predictions))
	log.Info("running backfills", "payeeRule", targets.PayeeRule, "embeddings", targets.Embeddings)

	success, failed := 0, 0
	for i, p := range predictions {
		resolved := resolvePrediction(p, budgetID)
		if resolved == nil {
			log.Warn("skipping unresolvable prediction", "id", p.ID)
			failed++
			continue
		}

		// Phase 1: Extract structured data from raw email via local SLM (Gemma/Ollama)
		parsed := deps.extractAndParse(ctx, *resolved)
		log.Info("parsed", "id", p.ID, "parsed", parsed)
		if parsed == nil {
			log.Warn("skipping prediction with unparseable email text", "id", p.ID)
			failed++
			continue
		}

		if targets.Embeddings {
			if err := deps.backfillEmbedding(ctx, *resolved, parsed); err != nil {
				log.Error("embedding backfill failed", "id", p.ID, "error", err)
				failed++
				continue
			}
		}

		if targets.PayeeRule {
			if err := deps.backfillPayeeRules(ctx, *resolved, parsed); err != nil {
				log.Error("payeeRule backfill failed", "id", p.ID, "error", err)
				failed++
				continue
			}
		}

		success++

		if (i+1)%50 == 0 {
			log.Info("progress", "processed", i+1, "total", len(predictions), "success", success, "failed", failed)
		}
	}

	log.Info("backfill complete", "total", len(predictions), "success", success, "failed", failed)
}
