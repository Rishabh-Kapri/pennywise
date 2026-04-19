package db

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MerchantRepository interface {
	CreateGlobalMerchant(ctx context.Context, tx pgx.Tx, data model.GlobalMerchant) (*model.GlobalMerchant, error)
	CreateGlolabMerchantMapping(ctx context.Context, tx pgx.Tx, data model.GlobalMerchantMapping) error
}

type merchantRepository struct {
	BaseRepository
}

func NewMerchantRepository(pool *pgxpool.Pool) MerchantRepository {
	return &merchantRepository{BaseRepository: NewBaseRepository(pool)}
}

func (r *merchantRepository) CreateGlobalMerchant(ctx context.Context, tx pgx.Tx, data model.GlobalMerchant) (*model.GlobalMerchant, error) {
	log := logger.Logger(ctx)
	log.Info("CreateGlobalMerchant", "data", data)
	// find exsiting merchant
	var existingMerchant model.GlobalMerchant
	executor := r.Executor(nil)
	err := executor.QueryRow(
		ctx,
		`SELECT * FROM global_merchants WHERE canonical_name = $1`,
		data.CanonicalName,
	).Scan(
		&existingMerchant.ID,
		&existingMerchant.CanonicalName,
		&existingMerchant.MCCTag,
		&existingMerchant.CreatedAt,
		&existingMerchant.UpdatedAt,
	)
	log.Info("CreateGlobalMerchant", "existingMerchant", &existingMerchant, "error", err)
	if err != nil {
		if err == pgx.ErrNoRows {
			log.Info("CreateGlobalMerchant: no rows found", "data", data)
			var insertedMerchant model.GlobalMerchant
			err := executor.QueryRow(
				ctx,
				`INSERT INTO global_merchants (
				canonical_name, mcc_tag
				) VALUES ($1, $2) RETURNING id, canonical_name, mcc_tag, created_at, updated_at`,
				data.CanonicalName, data.MCCTag,
			).Scan(
				&insertedMerchant.ID,
				&insertedMerchant.CanonicalName,
				&insertedMerchant.MCCTag,
				&insertedMerchant.CreatedAt,
				&insertedMerchant.UpdatedAt,
			)
			log.Info("CreateGlobalMerchant", "insertedMerchant", insertedMerchant, "error", err)
			if err != nil {
				return nil, err
			}
			return &insertedMerchant, nil
		} else {
			return nil, err
		}
	}
	return &existingMerchant, nil
}

func (r *merchantRepository) CreateGlolabMerchantMapping(ctx context.Context, tx pgx.Tx, data model.GlobalMerchantMapping) error {
	executor := r.Executor(nil)
	_, err := executor.Exec(
		ctx,
		`INSERT INTO global_merchant_mappings (
			cleaned_raw_text, merchant_id
		) VALUES ($1, $2)
		ON CONFLICT (cleaned_raw_text) DO NOTHING`,
		data.CleanedRawText, data.MerchantID,
	)
	return err
}
