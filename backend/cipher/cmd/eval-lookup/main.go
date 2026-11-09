// Command eval-lookup compares ways of matching a merchant the budget hasn't
// seen exactly, using its own counterparty_aliases: trigram similarity, and
// optionally embeddings from a local Ollama.
//
// It only reads: everything runs in a read-only database transaction. Merchant
// names go nowhere except the Ollama URL you pass.
//
//	go run ./cmd/eval-lookup [-budget <uuid>] [-ollama http://host:11434] [-model bge-m3] [-show-wrong 20]
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/client"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/factory"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/jackc/pgx/v5"
)

const minSafePrecision = 0.95

func main() {
	budget := flag.String("budget", "", "only this budget (default: every budget)")
	ollamaURL := flag.String("ollama", "", "Ollama URL for embeddings (default: trigram only)")
	embedModel := flag.String("model", "bge-m3", "Ollama embedding model")
	showWrong := flag.Int("show-wrong", 0, "print this many wrong matches per method, most confident first")
	flag.Parse()

	ctx := context.Background()
	cfg := config.Load()
	db, err := factory.ConnectDB(ctx, factory.Config{Driver: cfg.DatabaseDriver, URL: cfg.DatabaseURL})
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer db.Close()
	if db.Pool == nil {
		logger.Fatal("eval-lookup needs the postgres driver")
	}

	tx, err := db.Pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer tx.Rollback(ctx)

	aliases, err := loadAliases(ctx, tx, *budget)
	if err != nil {
		logger.Fatal(err.Error())
	}
	canFind := 0
	for _, ok := range findable(aliases) {
		if ok {
			canFind++
		}
	}
	fmt.Printf("%d aliases, %d have another spelling under the same payee to find\n", len(aliases), canFind)
	if canFind == 0 {
		fmt.Println("Nothing to measure: no payee has two differently spelled merchants.")
		return
	}

	trigram, err := trigramScores(ctx, tx, aliases)
	if err != nil {
		logger.Fatal(err.Error())
	}
	report("Trigram (pg_trgm)", aliases, bestMatches(aliases, trigram), canFind, *showWrong)

	if *ollamaURL == "" {
		fmt.Println("\nEmbeddings skipped: pass -ollama to compare them.")
		return
	}
	embed, err := embeddingScores(ctx, aliases, *ollamaURL, *embedModel)
	if err != nil {
		fmt.Fprintln(os.Stderr, "embedding failed:", err)
		os.Exit(1)
	}
	report("Embeddings ("+*embedModel+")", aliases, bestMatches(aliases, embed), canFind, *showWrong)
}

func thresholds() []float64 {
	out := []float64{}
	for t := 30; t <= 95; t += 5 {
		out = append(out, float64(t)/100)
	}
	return out
}

func report(name string, aliases []alias, matches []match, canFind, showWrong int) {
	results := evaluate(aliases, matches, thresholds())
	printResults(name, results, canFind)
	if best, ok := bestSafe(results, minSafePrecision); ok {
		fmt.Printf("Best at %.0f%%+ precision: threshold %.2f finds %d of %d (%.0f%%), %d wrong\n",
			minSafePrecision*100, best.Threshold, best.Correct, canFind, best.Coverage(canFind)*100, best.Wrong)
	} else {
		fmt.Printf("No threshold reaches %.0f%% precision.\n", minSafePrecision*100)
	}
	printWrong(aliases, matches, showWrong)
}

func printWrong(aliases []alias, matches []match, limit int) {
	if limit <= 0 {
		return
	}
	wrong := []match{}
	for _, m := range matches {
		if m.Candidate >= 0 && aliases[m.Candidate].PayeeID != aliases[m.Query].PayeeID {
			wrong = append(wrong, m)
		}
	}
	for i := 0; i < len(wrong); i++ {
		for j := i + 1; j < len(wrong); j++ {
			if wrong[j].Score > wrong[i].Score {
				wrong[i], wrong[j] = wrong[j], wrong[i]
			}
		}
	}
	fmt.Println("Closest wrong matches:")
	for i, m := range wrong {
		if i == limit {
			break
		}
		q, c := aliases[m.Query], aliases[m.Candidate]
		fmt.Printf("  %.2f  %s (%s) -> %s (%s)\n", m.Score, q.Merchant, q.PayeeName, c.Merchant, c.PayeeName)
	}
}

func loadAliases(ctx context.Context, tx pgx.Tx, budget string) ([]alias, error) {
	rows, err := tx.Query(ctx, `
		SELECT a.budget_id::text, a.merchant, a.payee_id::text, p.name
		FROM counterparty_aliases a
		JOIN payees p ON p.id = a.payee_id AND p.deleted = FALSE
		WHERE $1 = '' OR a.budget_id::text = $1
		ORDER BY a.budget_id, a.merchant, a.channel`, budget)
	if err != nil {
		return nil, fmt.Errorf("load aliases: %w", err)
	}
	defer rows.Close()
	out := []alias{}
	for rows.Next() {
		var a alias
		if err := rows.Scan(&a.Budget, &a.Merchant, &a.PayeeID, &a.PayeeName); err != nil {
			return nil, fmt.Errorf("load aliases: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func trigramScores(ctx context.Context, tx pgx.Tx, aliases []alias) (scoreFn, error) {
	merchants := make([]string, len(aliases))
	for i, a := range aliases {
		merchants[i] = a.Merchant
	}
	scores := make([][]float64, len(aliases))
	for i := range scores {
		scores[i] = make([]float64, len(aliases))
	}
	rows, err := tx.Query(ctx, `
		SELECT a.ord - 1, b.ord - 1, similarity(a.m, b.m)
		FROM unnest($1::text[]) WITH ORDINALITY a(m, ord)
		CROSS JOIN unnest($1::text[]) WITH ORDINALITY b(m, ord)
		WHERE a.ord < b.ord`, merchants)
	if err != nil {
		return nil, fmt.Errorf("trigram similarity: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var i, j int
		var score float64
		if err := rows.Scan(&i, &j, &score); err != nil {
			return nil, fmt.Errorf("trigram similarity: %w", err)
		}
		scores[i][j], scores[j][i] = score, score
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("trigram similarity: %w", err)
	}
	return func(i, j int) float64 { return scores[i][j] }, nil
}

func embeddingScores(ctx context.Context, aliases []alias, url, slug string) (scoreFn, error) {
	embedder, err := client.NewUserEmbedder(model.LLMProviderOllama, url, "", http.DefaultClient)
	if err != nil {
		return nil, err
	}
	vectors := make([][]float64, len(aliases))
	for i, a := range aliases {
		if vectors[i], err = embedder.Embed(ctx, slug, a.Merchant); err != nil {
			return nil, err
		}
	}
	return func(i, j int) float64 { return cosine(vectors[i], vectors[j]) }, nil
}
