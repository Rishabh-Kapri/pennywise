-- +goose Up
-- +goose StatementBegin
ALTER TABLE transactions
	ADD COLUMN IF NOT EXISTS location_lat DOUBLE PRECISION,
	ADD COLUMN IF NOT EXISTS location_lng DOUBLE PRECISION,
	ADD COLUMN IF NOT EXISTS location_name TEXT,
	ADD COLUMN IF NOT EXISTS location_source VARCHAR(10);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE transactions
	DROP COLUMN IF EXISTS location_lat,
	DROP COLUMN IF EXISTS location_lng,
	DROP COLUMN IF EXISTS location_name,
	DROP COLUMN IF EXISTS location_source;
-- +goose StatementEnd
