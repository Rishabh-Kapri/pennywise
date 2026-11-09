-- +goose Up
-- Storage for the email -> transaction classification pipeline (cipher): rules,
-- predictions and the embeddings that back vector matching.

CREATE TYPE payee_match_type AS ENUM ('EXACT', 'PATTERN');

CREATE TABLE payee_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    payee_id UUID NOT NULL REFERENCES payees(id) ON DELETE CASCADE,
    category_id UUID REFERENCES categories(id),

    match_string TEXT NOT NULL, -- upi handle or bank string
    match_type payee_match_type NOT NULL DEFAULT 'EXACT',

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted BOOLEAN DEFAULT false,
    -- A user cannot map the exact same string to two different payees in their budget.
    UNIQUE (budget_id, match_string)
);
CREATE INDEX idx_payee_rules_lookup ON payee_rules (budget_id, match_string);

CREATE TABLE predictions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    email_text TEXT,
    amount NUMERIC(12, 2),
    account TEXT,
    account_prediction NUMERIC(10, 2),
    payee TEXT,
    payee_prediction NUMERIC(10, 2),
    category TEXT,
    category_prediction NUMERIC(10, 2),
    has_user_corrected BOOLEAN,
    user_corrected_account TEXT,
    user_corrected_payee TEXT,
    user_corrected_category TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted BOOLEAN DEFAULT false
);
CREATE INDEX idx_predictions_txn ON predictions (budget_id, transaction_id) WHERE deleted = FALSE;

CREATE TYPE prediction_source AS ENUM ('LLM', 'MANUAL', 'RULE', 'VECTOR', 'UNCATEGORIZED');

CREATE TABLE cipher_predictions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    email_text TEXT,
    llm_reasoning TEXT,
    metadata JSONB,
    amount NUMERIC(12, 2),
    -- extracted strings
    extracted_account TEXT,
    extracted_payee TEXT,
    -- predictions
    predicted_payee_id UUID REFERENCES payees(id) ON DELETE SET NULL,
    predicted_category_id UUID REFERENCES categories(id) ON DELETE SET NULL,
    -- ai metrics
    account_confidence NUMERIC(5, 2),
    payee_confidence NUMERIC(5, 2),
    category_confidence NUMERIC(5, 2),
    source prediction_source NOT NULL,

    has_user_corrected BOOLEAN DEFAULT false,
    actual_payee_id UUID REFERENCES payees(id) ON DELETE SET NULL,
    actual_category_id UUID REFERENCES categories(id) ON DELETE SET NULL,

    -- Learning state: learned_at is set once this prediction has taught the
    -- pipeline (payee rule + AUTO_LEARNED embedding), NULL means it never has.
    -- learn_error records why the last attempt failed and is cleared on
    -- success -- learning runs best-effort in a goroutine, so this is the only
    -- place a broken learning step becomes visible.
    learned_at TIMESTAMPTZ,
    learn_error TEXT,

    -- Marks the prediction as triaged in the review queue; NULL = still pending.
    reviewed_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted BOOLEAN DEFAULT false
);
-- drives the learning backfill's "what still needs learning" scan
CREATE INDEX idx_cipher_predictions_unlearned
    ON cipher_predictions (budget_id, created_at DESC)
    WHERE deleted = FALSE AND learned_at IS NULL;
CREATE INDEX idx_cipher_predictions_pending
    ON cipher_predictions (budget_id, created_at DESC)
    WHERE deleted = FALSE AND reviewed_at IS NULL;

CREATE TYPE learning_source AS ENUM ('AUTO_LEARNED', 'MANUAL');

-- Embeddings are budget scoped, so they carry a specific payee_id and category_id.
CREATE TABLE transaction_embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    embedding_text TEXT NOT NULL,
    embedding vector(1024) NOT NULL,
    payee_id UUID NOT NULL REFERENCES payees(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    amount DECIMAL(12, 2) NOT NULL,
    source learning_source NOT NULL DEFAULT 'MANUAL',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (budget_id, embedding_text)
);
-- Postgres must filter down to the budget BEFORE doing vector math.
CREATE INDEX idx_embeddings_budget ON transaction_embeddings (budget_id);
-- HNSW is the state-of-the-art pgvector index; vector_cosine_ops optimises it
-- for cosine distance, the standard for LLM embeddings.
CREATE INDEX idx_embeddings_vector ON transaction_embeddings
    USING hnsw (embedding vector_cosine_ops);

-- Embeddings for budget entities (categories, payees, accounts). Used by
-- cipher's VectorResolver to match free-text query terms to entity IDs without
-- exposing entity names to a cloud LLM.
CREATE TYPE entity_type AS ENUM ('category', 'payee', 'account');

CREATE TABLE entity_embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    entity_type entity_type NOT NULL,
    entity_id UUID NOT NULL,
    -- The text that was embedded, e.g. "category: Dining Out/Entertainment"
    embedding_text TEXT NOT NULL,
    embedding vector(1024) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (budget_id, entity_type, entity_id)
);
CREATE INDEX idx_entity_embeddings_budget ON entity_embeddings (budget_id);
CREATE INDEX idx_entity_embeddings_vector ON entity_embeddings
    USING hnsw (embedding vector_cosine_ops);

-- +goose Down
DROP TABLE entity_embeddings;
DROP TYPE entity_type;
DROP TABLE transaction_embeddings;
DROP TYPE learning_source;
DROP TABLE cipher_predictions;
DROP TYPE prediction_source;
DROP TABLE predictions;
DROP TABLE payee_rules;
DROP TYPE payee_match_type;
