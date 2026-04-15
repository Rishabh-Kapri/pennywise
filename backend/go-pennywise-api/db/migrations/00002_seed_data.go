package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upSeedData, downSeedData)
}

func uuidOrNull(id *uuid.UUID) any {
	if id != nil && *id == uuid.Nil {
		return nil
	}
	return id
}

func loadFileData[T any](path string) ([]T, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening %s: %w", path, err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", path, err)
	}
	var data []T
	if err := json.Unmarshal(bytes, &data); err != nil {
		return nil, fmt.Errorf("error unmarshaling file %s: %w", path, err)
	}
	log.Printf("Loaded total data for file %v: %v\n", path, len(data))
	return data, nil
}

func upSeedData(ctx context.Context, tx *sql.Tx) error {
	// Budgets
	if budgets, err := loadFileData[model.Budget]("data/budgets.json"); err == nil {
		for _, d := range budgets {
			_, err = tx.ExecContext(ctx, `INSERT INTO budgets (id, name, is_selected, created_at, updated_at) VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`,
				d.ID, d.Name, d.IsSelected, d.CreatedAt, d.UpdatedAt)
			if err != nil {
				return err
			}
		}
	} else {
		log.Printf("Skipping budgets seed: %v", err)
	}

	// Accounts
	if accounts, err := loadFileData[model.Account]("data/accounts.json"); err == nil {
		for _, d := range accounts {
			_, err = tx.ExecContext(ctx, `INSERT INTO accounts (id, name, budget_id, transfer_payee_id, type, closed, deleted, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT DO NOTHING`,
				d.ID, d.Name, d.BudgetID, uuidOrNull(d.TransferPayeeID), d.Type, d.Closed, d.Deleted, d.CreatedAt, d.UpdatedAt)
			if err != nil {
				return err
			}
		}
	} else {
		log.Printf("Skipping accounts seed: %v", err)
	}

	// Users
	if users, err := loadFileData[model.User]("data/users.json"); err == nil {
		for _, d := range users {
			_, err = tx.ExecContext(ctx, `INSERT INTO users (id, budget_id, email, history_id, deleted, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT DO NOTHING`,
				d.ID, d.BudgetID, d.Email, d.HistoryID, d.Deleted, d.CreatedAt, d.UpdatedAt)
			if err != nil {
				return err
			}
		}
	} else {
		log.Printf("Skipping users seed: %v", err)
	}

	// Category Groups
	if groups, err := loadFileData[model.CategoryGroup]("data/categoryGroups.json"); err == nil {
		for _, d := range groups {
			isSystem := d.Name == "Credit Card Payments"
			_, err = tx.ExecContext(ctx, `INSERT INTO category_groups (id, name, budget_id, hidden, is_system, deleted, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) ON CONFLICT DO NOTHING`,
				d.ID, d.Name, d.BudgetID, d.Hidden, isSystem, d.Deleted, d.CreatedAt, d.UpdatedAt)
			if err != nil {
				return err
			}
		}
	} else {
		log.Printf("Skipping category_groups seed: %v", err)
	}

	// Categories and Monthly Budgets
	if categories, err := loadFileData[model.Category]("data/categories.json"); err == nil {
		for _, d := range categories {
			isSystem := d.Name == "Inflow: Ready to Assign"
			if !isSystem {
				for month, budgeted := range d.Budgeted {
					key := strings.Split(month, "-")
					keyFloat, _ := strconv.ParseFloat(key[1], 10)
					keyInt := int(keyFloat) + 1
					newKey := key[0] + "-" + fmt.Sprintf("%02d", keyInt)

					_, err = tx.ExecContext(ctx, `INSERT INTO monthly_budgets (budget_id, category_id, month, budgeted, carryover_balance, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, NOW(), NOW()) ON CONFLICT DO NOTHING`,
						d.BudgetID, d.ID, newKey, budgeted, 0)
					if err != nil {
						return err
					}
				}
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO categories (id, name, budget_id, category_group_id, note, hidden, is_system, deleted, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT DO NOTHING`,
				d.ID, d.Name, d.BudgetID, d.CategoryGroupID, d.Note, d.Hidden, isSystem, d.Deleted, d.CreatedAt, d.UpdatedAt)
			if err != nil {
				return err
			}
		}
	} else {
		log.Printf("Skipping categories seed: %v", err)
	}

	// Inflow Category
	type InflowCategory struct {
		model.Category
	}
	if inflow, err := loadFileData[InflowCategory]("data/inflowCategory.json"); err == nil && len(inflow) > 0 {
		inflowCat := inflow[0]
		_, err = tx.ExecContext(ctx, `INSERT INTO categories (id, name, budget_id, category_group_id, note, hidden, is_system, deleted, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT DO NOTHING`,
			inflowCat.ID, inflowCat.Name, inflowCat.BudgetID, inflowCat.CategoryGroupID, inflowCat.Note, false, true, false, inflowCat.CreatedAt, inflowCat.UpdatedAt)
		if err != nil {
			return err
		}
	} else {
		log.Printf("Skipping inflow_category seed: %v", err)
	}

	return nil
}

func downSeedData(ctx context.Context, tx *sql.Tx) error {
	// Usually seeding down migrations just clear everything,
	// but since it's just tracking state it's safer to do nothing or return nil
	return nil
}
