-- +goose Up
-- API keys were created with key_id = '' (the id was never generated), so only
-- the first key per database could be stored. Give those rows a real id, and
-- refuse empty ids from now on. Authentication matches on hashed_key, so
-- existing keys keep working.
UPDATE api_keys
SET key_id = 'k_' || substr(md5(random()::text || id::text), 1, 12)
WHERE key_id = '';

ALTER TABLE api_keys ADD CONSTRAINT api_keys_key_id_not_empty CHECK (key_id <> '');

-- +goose Down
ALTER TABLE api_keys DROP CONSTRAINT api_keys_key_id_not_empty;
