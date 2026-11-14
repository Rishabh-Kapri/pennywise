package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

// CounterpartyAliasRepository stores what the budget has learned: which payee
// and category a "CHANNEL | MERCHANT" key belongs to.
type CounterpartyAliasRepository interface {
	// FindMatch returns the alias's payee and category names. It returns nil
	// when there's no alias, or its payee or category is missing or deleted:
	// a transaction needs both.
	FindMatch(ctx context.Context, budgetID uuid.UUID, channel, merchant string) (*model.CounterpartyAliasMatch, error)
	// Learn records one more sighting of a key. A new key is added. An
	// existing one keeps its payee, and gets a category only if it had none.
	Learn(ctx context.Context, in model.CounterpartyAliasLearn) error
	// Confirm records what the user said a key is. Unlike Learn it replaces the
	// payee and category, and marks the alias MANUAL.
	Confirm(ctx context.Context, in model.CounterpartyAliasConfirm) error
}

// TransactionSourceRepository stores the bank text behind transactions.
type TransactionSourceRepository interface {
	// RecordFromPipeline saves the source the pipeline created transactionID
	// from, and reports whether it added a row. A second call for the same
	// transaction and kind changes nothing and returns false.
	RecordFromPipeline(
		ctx context.Context,
		budgetID, transactionID uuid.UUID,
		source model.MessageSource,
	) (bool, error)
	// FindCrossChannel lists transactions another channel's message may have
	// created for the same payment, newest first.
	FindCrossChannel(ctx context.Context, q model.SourceMatchQuery) ([]model.SourceMatchCandidate, error)
	// LookupKey returns the key of the transaction's oldest source that has
	// one, or nil when none does.
	LookupKey(ctx context.Context, budgetID, transactionID uuid.UUID) (*model.LookupKey, error)
	// ListByTransaction returns every source of the transaction, oldest first.
	// It is empty for a transaction made by hand.
	ListByTransaction(ctx context.Context, budgetID, transactionID uuid.UUID) ([]model.TransactionSource, error)
	// RefsByTransaction returns the references the transactions' messages
	// (email, SMS) stated, per transaction. Statement lines are left out, so
	// a statement can't confirm its own match.
	RefsByTransaction(ctx context.Context, budgetID uuid.UUID, transactionIDs []uuid.UUID) (map[uuid.UUID][]string, error)
	// ReplaceStatementLines makes lines the statement lines importID recorded:
	// the import's earlier ones are removed, and a transaction already linked
	// to another statement's line is moved to this one.
	ReplaceStatementLines(ctx context.Context, budgetID, importID uuid.UUID, lines []model.StatementLineSource) error
}
