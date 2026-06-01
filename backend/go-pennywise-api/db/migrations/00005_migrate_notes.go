package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upMigrateNotesToTags, downMigrateNotesToTags)
}

var cityMonthYearRe = regexp.MustCompile(`(?i)^[a-z][\w\s]+ (jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)\s+\d{4}$`)

func isTripTag(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "trip") || cityMonthYearRe.MatchString(name)
}

func upMigrateNotesToTags(ctx context.Context, tx *sql.Tx) error {
	// Find transactions with notes containing ":" pattern
	rows, err := tx.QueryContext(ctx, `
		SELECT id, budget_id, note FROM transactions
		WHERE deleted = FALSE AND note IS NOT NULL AND note LIKE '%:%'
		ORDER BY budget_id
	`)
	if err != nil {
		return fmt.Errorf("error querying transactions with notes: %w", err)
	}
	defer rows.Close()

	type txnNote struct {
		ID       uuid.UUID
		BudgetID uuid.UUID
		Note     string
	}
	var txnNotes []txnNote
	for rows.Next() {
		var t txnNote
		if err := rows.Scan(&t.ID, &t.BudgetID, &t.Note); err != nil {
			return fmt.Errorf("error scanning transaction note: %w", err)
		}
		txnNotes = append(txnNotes, t)
	}

	if len(txnNotes) == 0 {
		log.Printf("No transactions with colon-separated notes found")
		return nil
	}

	// Track created tags per budget to avoid duplicates
	tagCache := make(map[string]uuid.UUID) // key: "budgetId:tagName"

	for _, t := range txnNotes {
		parts := strings.SplitN(t.Note, ":", 2)
		if len(parts) != 2 {
			continue
		}
		tagName := strings.TrimSpace(parts[0])
		remainingNote := strings.TrimSpace(parts[1])
		if tagName == "" {
			continue
		}
		if !isTripTag(tagName) {
			continue
		}
		tagName = strings.ToLower(strings.ReplaceAll(tagName, " ", "-"))

		cacheKey := t.BudgetID.String() + ":" + tagName
		tagId, exists := tagCache[cacheKey]
		if !exists {
			err := tx.QueryRowContext(ctx, `
				INSERT INTO tags (name, budget_id, color, deleted, created_at, updated_at)
				VALUES ($1, $2, '', FALSE, NOW(), NOW())
				ON CONFLICT (name, budget_id) DO UPDATE SET name = EXCLUDED.name
				RETURNING id
			`, tagName, t.BudgetID).Scan(&tagId)
			if err != nil {
				log.Printf("Warning: error creating tag %q for budget %v: %v", tagName, t.BudgetID, err)
				continue
			}
			tagCache[cacheKey] = tagId
		}

		_, err = tx.ExecContext(ctx, `
			UPDATE transactions SET tag_ids = array_append(tag_ids, $1), updated_at = NOW()
			WHERE id = $2 AND NOT ($1 = ANY(tag_ids))
		`, tagId, t.ID)
		if err != nil {
			log.Printf("Warning: error adding tag %v to transaction %v: %v", tagId, t.ID, err)
			continue
		}

		_, err = tx.ExecContext(ctx, `
			UPDATE transactions SET note = $1, updated_at = NOW()
			WHERE id = $2
		`, remainingNote, t.ID)
		if err != nil {
			log.Printf("Warning: error updating note for transaction %v: %v", t.ID, err)
		}
	}

	log.Printf("Note-to-tag migration complete. Processed %d transactions.", len(txnNotes))
	return nil
}

func downMigrateNotesToTags(ctx context.Context, tx *sql.Tx) error {
	return nil
}
