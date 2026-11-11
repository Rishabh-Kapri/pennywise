package postgres

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionSourceRepo_FindCrossChannel(t *testing.T) {
	pool, repo, s := setup(t)
	ctx := context.Background()
	sources := NewTransactionSourceRepository(pool)
	ist := time.FixedZone("IST", 5*3600+1800)
	on := time.Date(2026, 11, 10, 0, 0, 0, 0, ist)
	at := time.Date(2026, 11, 10, 20, 41, 11, 0, ist)

	create := func(amount float64, account uuid.UUID, hash string) model.Transaction {
		t.Helper()
		in := newTxn(s, "2026-11-10", amount)
		in.AccountID, in.DedupeHash = &account, &hash
		txn, created, err := repo.CreateDeduped(ctx, in)
		require.NoError(t, err)
		require.True(t, created)
		return *txn
	}
	record := func(txn model.Transaction, kind, text string) {
		t.Helper()
		_, err := sources.RecordFromPipeline(ctx, s.budgetID, txn.ID, model.MessageSource{
			Kind: kind, Text: text, Channel: "CARD", Merchant: "POLICYBAZAAR COM", Amount: "7532.00",
			OccurredOn: &on, OccurredAt: &at,
		})
		require.NoError(t, err)
	}
	query := func(kind, text string) []model.SourceMatchCandidate {
		t.Helper()
		found, err := sources.FindCrossChannel(ctx, model.SourceMatchQuery{
			BudgetID: s.budgetID, AccountID: s.accountID, Amount: -7532, Kind: kind, On: on, Text: text,
		})
		require.NoError(t, err)
		return found
	}

	email := create(-7532, s.accountID, "cc-email")
	record(email, "EMAIL", "the email")
	create(-7532, s.account2ID, "cc-other-account")
	create(-100, s.accountID, "cc-other-amount")

	found := query("SMS", "the sms")
	require.Len(t, found, 1)
	assert.Equal(t, email.ID, found[0].TransactionID)
	require.NotNil(t, found[0].OccurredAt)
	assert.True(t, found[0].OccurredAt.Equal(at))

	assert.Empty(t, query("EMAIL", "another email"), "an email doesn't match an email")

	// Once the SMS is attached, a different SMS can't join, but the same one
	// (a retried job) still finds it.
	record(email, "SMS", "the sms")
	assert.Empty(t, query("SMS", "a different sms"))
	assert.Len(t, query("SMS", "the sms"), 1)

	_, err := pool.Exec(ctx, `UPDATE transactions SET deleted = TRUE WHERE id = $1`, email.ID)
	require.NoError(t, err)
	assert.Empty(t, query("SMS", "the sms"), "a deleted transaction isn't matched")
}

func TestTransactionRepo_LockForDedupeSerializes(t *testing.T) {
	pool, repo, s := setup(t)
	ctx := context.Background()
	txn := NewDBTxn(pool)

	held := make(chan struct{})
	release := make(chan struct{})
	var second time.Time
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = txn.Do(ctx, func(ctx context.Context) error {
			require.NoError(t, repo.LockForDedupe(ctx, s.budgetID, s.accountID, -7532))
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	start := time.Now()
	go func() {
		defer wg.Done()
		_ = txn.Do(ctx, func(ctx context.Context) error {
			require.NoError(t, repo.LockForDedupe(ctx, s.budgetID, s.accountID, -7532))
			second = time.Now()
			return nil
		})
	}()
	time.Sleep(300 * time.Millisecond)
	close(release)
	wg.Wait()
	assert.GreaterOrEqual(t, second.Sub(start), 250*time.Millisecond, "the second lock waited for the first")
}

func TestCipherPredictionRepo_LearningReviewAndDelete(t *testing.T) {
	pool, repo, s := setup(t)
	ctx := context.Background()
	predictions := NewCipherPredictionRepository(pool)
	txn := mustCreate(t, repo, newTxn(s, "2026-11-10", -7532))
	_, err := predictions.Create(ctx, model.CipherPredictionRecord{
		BudgetID: s.budgetID, TransactionID: txn.ID, Source: model.PredictionSourceLLM,
	})
	require.NoError(t, err)

	first, err := predictions.ClaimLearning(ctx, s.budgetID, txn.ID)
	require.NoError(t, err)
	again, err := predictions.ClaimLearning(ctx, s.budgetID, txn.ID)
	require.NoError(t, err)
	assert.True(t, first)
	assert.False(t, again, "learned once")

	reviewedAt := func() *time.Time {
		t.Helper()
		var at *time.Time
		require.NoError(t, pool.QueryRow(ctx, `SELECT reviewed_at FROM cipher_predictions WHERE transaction_id = $1`, txn.ID).Scan(&at))
		return at
	}
	require.NoError(t, predictions.MarkReviewed(ctx, s.budgetID, txn.ID))
	reviewed := reviewedAt()
	require.NotNil(t, reviewed)
	require.NoError(t, predictions.MarkReviewed(ctx, s.budgetID, txn.ID))
	assert.True(t, reviewedAt().Equal(*reviewed), "the first review time stays")

	require.NoError(t, predictions.MarkReviewed(ctx, s.budgetID, uuid.New()), "no prediction is no error")

	require.NoError(t, predictions.DeleteByTransactionID(ctx, s.budgetID, txn.ID))
	var deleted bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT deleted FROM cipher_predictions WHERE transaction_id = $1`, txn.ID).Scan(&deleted))
	assert.True(t, deleted)
}

func TestCipherPredictionRepo_CorrectionIsAReview(t *testing.T) {
	pool, repo, s := setup(t)
	ctx := context.Background()
	predictions := NewCipherPredictionRepository(pool)
	txn := mustCreate(t, repo, newTxn(s, "2026-11-10", -7532))
	_, err := predictions.Create(ctx, model.CipherPredictionRecord{
		BudgetID: s.budgetID, TransactionID: txn.ID, Source: model.PredictionSourceLLM,
	})
	require.NoError(t, err)

	require.NoError(t, predictions.MarkUserCorrected(ctx, s.budgetID, txn.ID, &s.payeeID, &s.category2))
	var corrected bool
	var reviewed *time.Time
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT has_user_corrected, reviewed_at FROM cipher_predictions WHERE transaction_id = $1`, txn.ID,
	).Scan(&corrected, &reviewed))
	assert.True(t, corrected)
	assert.NotNil(t, reviewed)
}

func TestCounterpartyAliasRepo_FindMatchSaysWhoMadeIt(t *testing.T) {
	pool, _, s := setup(t)
	ctx := context.Background()
	aliases := NewCounterpartyAliasRepository(pool)
	require.NoError(t, aliases.Learn(ctx, model.CounterpartyAliasLearn{
		BudgetID: s.budgetID, Channel: "CARD", Merchant: "POLICYBAZAAR COM", PayeeID: s.payeeID, CategoryID: &s.categoryID,
	}))
	m, err := aliases.FindMatch(ctx, s.budgetID, "CARD", "POLICYBAZAAR COM")
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, "AUTO_LEARNED", m.Source)

	require.NoError(t, aliases.Confirm(ctx, model.CounterpartyAliasConfirm{
		BudgetID: s.budgetID, Channel: "CARD", Merchant: "POLICYBAZAAR COM", PayeeID: s.payeeID, CategoryID: &s.categoryID,
	}))
	m, err = aliases.FindMatch(ctx, s.budgetID, "CARD", "POLICYBAZAAR COM")
	require.NoError(t, err)
	assert.Equal(t, "MANUAL", m.Source)
}
