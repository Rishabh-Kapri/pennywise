-- +goose Up
-- The password that opens an account's statement PDFs, so statements fetched
-- from email can be read without asking. Sealed with the API's credentials key
-- (utils.SecretBox); the API decrypts it only to queue a statement.extract.
-- Its own table, not a column on accounts, because the agent role can read
-- accounts (00009) and must not see these. Not granted to the agent role.
CREATE TABLE account_statement_passwords (
    account_id UUID PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    budget_id  UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    sealed     BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_account_statement_passwords_budget ON account_statement_passwords (budget_id);

-- +goose Down
DROP TABLE account_statement_passwords;
