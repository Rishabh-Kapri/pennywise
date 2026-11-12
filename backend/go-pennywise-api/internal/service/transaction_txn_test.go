package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// These tests pin down the transaction boundary of the transaction service:
// which calls run inside repository.DBTxn.Do, that a failure anywhere in the
// unit of work rolls the whole unit back, and that nested units join instead of
// opening a second transaction.

type txMarker struct{}

// inTx reports whether ctx was handed out by recordingDBTxn.Do.
func inTx(ctx context.Context) bool {
	v, _ := ctx.Value(txMarker{}).(bool)
	return v
}

// recordingDBTxn behaves like the Postgres DBTxn (join when ctx already carries
// a transaction, commit on nil, roll back on error) and records what happened.
type recordingDBTxn struct {
	mu         sync.Mutex
	beginErr   error
	begun      int
	committed  int
	rolledBack int
}

func (r *recordingDBTxn) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if inTx(ctx) {
		return fn(ctx)
	}
	r.mu.Lock()
	if r.beginErr != nil {
		r.mu.Unlock()
		return r.beginErr
	}
	r.begun++
	r.mu.Unlock()

	err := fn(context.WithValue(ctx, txMarker{}, true))

	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		r.rolledBack++
	} else {
		r.committed++
	}
	return err
}

func (r *recordingDBTxn) counts() (begun, committed, rolledBack int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.begun, r.committed, r.rolledBack
}

// txCtx / outsideTx match the ctx argument of a mocked repository call.
var (
	txCtx     = mock.MatchedBy(func(ctx context.Context) bool { return inTx(ctx) })
	outsideTx = mock.MatchedBy(func(ctx context.Context) bool { return !inTx(ctx) })
)

type upsertCarryoverCall struct {
	categoryID uuid.UUID
	monthKey   string
	delta      float64
	inTx       bool
}

// fakeMBService records carryover changes instead of computing them.
type fakeMBService struct {
	mu            sync.Mutex
	err           error
	upserts       []upsertCarryoverCall
	updateCalls   int
	updateInTx    bool
	lastUpdateOld *model.Transaction
}

func (f *fakeMBService) UpsertCarryover(ctx context.Context, _ uuid.UUID, categoryID uuid.UUID, monthKey string, delta float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upserts = append(f.upserts, upsertCarryoverCall{categoryID, monthKey, delta, inTx(ctx)})
	return f.err
}

func (f *fakeMBService) ApplyCarryoverOps(context.Context, uuid.UUID, *txnDiff, carryoverCase) error {
	return f.err
}

func (f *fakeMBService) UpdateCarryovers(ctx context.Context, _ uuid.UUID, oldTxn, _ *model.Transaction, _ uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateCalls++
	f.updateInTx = inTx(ctx)
	f.lastUpdateOld = oldTxn
	return f.err
}

// fakeGeocoder answers Reverse with a fixed name or error.
type fakeGeocoder struct {
	name  string
	err   error
	calls int
}

func (g *fakeGeocoder) Reverse(context.Context, float64, float64) (string, error) {
	g.calls++
	return g.name, g.err
}

func (g *fakeGeocoder) Search(context.Context, string, int, *Coords) ([]PlaceResult, error) {
	return nil, nil
}

type txnSvcDeps struct {
	dbTxn     *recordingDBTxn
	repo      *mockTransactionRepo
	budget    *mockBudgetRepo
	account   *mockAccountRepo
	payee     *mockPayeesRepo
	cipher    *mockCipherPredictionRepo
	payeeRule *mockPayeeRuleRepo
	embedding *mockTransactionEmbeddingRepo
	client    *mockCipherClient
	mb        *fakeMBService
	geo       *fakeGeocoder
}

func newTxnSvc() (*transactionService, *txnSvcDeps) {
	d := &txnSvcDeps{
		dbTxn:     &recordingDBTxn{},
		repo:      &mockTransactionRepo{},
		budget:    &mockBudgetRepo{},
		account:   &mockAccountRepo{},
		payee:     &mockPayeesRepo{},
		cipher:    &mockCipherPredictionRepo{},
		payeeRule: &mockPayeeRuleRepo{},
		embedding: &mockTransactionEmbeddingRepo{},
		client:    &mockCipherClient{},
		mb:        &fakeMBService{},
		geo:       &fakeGeocoder{name: "Blue Tokai, Indiranagar"},
	}
	svc := &transactionService{
		repo:                 d.repo,
		budgetRepo:           d.budget,
		accountRepo:          d.account,
		payeeRepo:            d.payee,
		cipherPredictionRepo: d.cipher,
		payeeRuleRepo:        d.payeeRule,
		txnEmbeddingRepo:     d.embedding,
		cipherClient:         d.client,
		mbService:            d.mb,
		geocodeService:       d.geo,
		dbTxn:                d.dbTxn,
	}
	return svc, d
}

func (d *txnSvcDeps) assertExpectations(t *testing.T) {
	t.Helper()
	d.repo.AssertExpectations(t)
	d.budget.AssertExpectations(t)
	d.account.AssertExpectations(t)
	d.payee.AssertExpectations(t)
	d.cipher.AssertExpectations(t)
	d.payeeRule.AssertExpectations(t)
	d.embedding.AssertExpectations(t)
	d.client.AssertExpectations(t)
}

type txnFixture struct {
	ctx        context.Context
	budgetID   uuid.UUID
	txnID      uuid.UUID
	accountID  uuid.UUID
	payeeID    uuid.UUID
	categoryID uuid.UUID
	inflowID   uuid.UUID
	budget     *model.Budget
	account    *model.Account
	payee      *model.Payee
}

func newTxnFixture() txnFixture {
	f := txnFixture{
		budgetID:   uuid.New(),
		txnID:      uuid.New(),
		accountID:  uuid.New(),
		payeeID:    uuid.New(),
		categoryID: uuid.New(),
		inflowID:   uuid.New(),
	}
	f.ctx = utils.WithBudgetID(context.Background(), f.budgetID)
	f.budget = &model.Budget{ID: f.budgetID, Metadata: model.BudgetMetadata{InflowCategoryID: f.inflowID}}
	f.account = &model.Account{ID: f.accountID, Type: "checking"}
	f.payee = &model.Payee{ID: f.payeeID}
	return f
}

func (f txnFixture) txn() model.Transaction {
	return model.Transaction{
		BudgetID:   f.budgetID,
		AccountID:  &f.accountID,
		PayeeID:    &f.payeeID,
		CategoryID: &f.categoryID,
		Amount:     -250,
		Date:       "2026-10-12",
	}
}

// expectDependencies sets up the budget/account/payee reads, all inside the tx.
func (f txnFixture) expectDependencies(d *txnSvcDeps) {
	d.budget.On("GetById", txCtx, f.budgetID).Return(f.budget, nil).Once()
	d.account.On("GetById", txCtx, f.budgetID, f.accountID).Return(f.account, nil).Once()
	d.payee.On("GetById", txCtx, f.budgetID, f.payeeID).Return(f.payee, nil).Once()
}

func appCode(t *testing.T, err error) errs.Code {
	t.Helper()
	var appErr *errs.Error
	require.ErrorAs(t, err, &appErr)
	return appErr.Code
}

// ─── Create ──────────────────────────────────────────────────────────────────

func TestTxnCreate_RunsEveryWriteInOneCommittedTransaction(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	f.expectDependencies(d)

	created := f.txn()
	created.ID = f.txnID
	d.repo.On("Create", txCtx, mock.MatchedBy(func(txn model.Transaction) bool {
		return txn.Status == model.TransactionStatusManual && txn.TransferAccountID == nil
	})).Return([]model.Transaction{created}, nil).Once()
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&created, nil).Once()

	res, err := svc.Create(f.ctx, f.txn())

	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, f.txnID, res[0].ID)
	begun, committed, rolledBack := d.dbTxn.counts()
	assert.Equal(t, [3]int{1, 1, 0}, [3]int{begun, committed, rolledBack})
	require.Len(t, d.mb.upserts, 1)
	assert.Equal(t, upsertCarryoverCall{f.categoryID, "2026-10", -250, true}, d.mb.upserts[0])
	d.assertExpectations(t)
}

func TestTxnCreate_RepoFailureRollsBackAndSkipsSideEffects(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	f.expectDependencies(d)
	d.repo.On("Create", txCtx, mock.Anything).Return(nil, errors.New("unique violation")).Once()

	_, err := svc.Create(f.ctx, f.txn())

	require.Error(t, err)
	assert.Equal(t, errs.CodeTransactionCreateFailed, appCode(t, err))
	begun, committed, rolledBack := d.dbTxn.counts()
	assert.Equal(t, [3]int{1, 0, 1}, [3]int{begun, committed, rolledBack})
	assert.Empty(t, d.mb.upserts, "no carryover may be applied for a transaction that was never written")
	d.assertExpectations(t)
}

func TestTxnCreate_CarryoverFailureRollsBackTheInsert(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	f.expectDependencies(d)
	created := f.txn()
	created.ID = f.txnID
	d.repo.On("Create", txCtx, mock.Anything).Return([]model.Transaction{created}, nil).Once()
	d.mb.err = errors.New("carryover write failed")

	_, err := svc.Create(f.ctx, f.txn())

	require.ErrorIs(t, err, d.mb.err)
	_, committed, rolledBack := d.dbTxn.counts()
	assert.Equal(t, 0, committed)
	assert.Equal(t, 1, rolledBack)
	d.repo.AssertNotCalled(t, "GetById", mock.Anything, mock.Anything, mock.Anything)
}

func TestTxnCreate_BeginFailureTouchesNoRepository(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	d.dbTxn.beginErr = errors.New("connection refused")

	_, err := svc.Create(f.ctx, f.txn())

	require.ErrorIs(t, err, d.dbTxn.beginErr)
	d.assertExpectations(t) // no expectations set: any repository call would panic
}

func TestTxnCreate_InvalidPayloadNeverOpensATransaction(t *testing.T) {
	tests := map[string]func(*model.Transaction){
		"missing account": func(txn *model.Transaction) { txn.AccountID = nil },
		"missing payee":   func(txn *model.Transaction) { txn.PayeeID = nil },
		"bad date":        func(txn *model.Transaction) { txn.Date = "12/10/2026" },
		"empty date":      func(txn *model.Transaction) { txn.Date = "" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			svc, d := newTxnSvc()
			f := newTxnFixture()
			txn := f.txn()
			mutate(&txn)

			_, err := svc.Create(f.ctx, txn)

			require.Error(t, err)
			begun, _, _ := d.dbTxn.counts()
			assert.Zero(t, begun)
		})
	}
}

// The budget always comes from the request context: a payload naming another
// budget is written to the caller's budget, never to the one it names.
func TestTxnCreate_StampsTheContextBudget(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	f.expectDependencies(d)
	txn := f.txn()
	txn.BudgetID = uuid.New()
	created := f.txn()
	created.ID = f.txnID
	d.repo.On("Create", txCtx, mock.MatchedBy(func(t model.Transaction) bool { return t.BudgetID == f.budgetID })).
		Return([]model.Transaction{created}, nil).Once()
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&created, nil).Once()

	_, err := svc.Create(f.ctx, txn)

	require.NoError(t, err)
	d.assertExpectations(t)
}

func TestTxnCreate_InflowCategorySkipsCarryover(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	f.expectDependencies(d)
	txn := f.txn()
	txn.CategoryID = &f.inflowID
	txn.Amount = 5000
	created := txn
	created.ID = f.txnID
	d.repo.On("Create", txCtx, mock.Anything).Return([]model.Transaction{created}, nil).Once()
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&created, nil).Once()

	_, err := svc.Create(f.ctx, txn)

	require.NoError(t, err)
	assert.Empty(t, d.mb.upserts)
	d.assertExpectations(t)
}

func TestTxnCreate_TransferCreatesAndLinksCounterpartInTheSameTransaction(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	savingsID, savingsPayeeID, checkingPayeeID := uuid.New(), uuid.New(), uuid.New()
	f.account.TransferPayeeID = &checkingPayeeID
	f.payee.TransferAccountID = &savingsID
	d.budget.On("GetById", txCtx, f.budgetID).Return(f.budget, nil).Once()
	d.account.On("GetById", txCtx, f.budgetID, f.accountID).Return(f.account, nil).Once()
	d.payee.On("GetById", txCtx, f.budgetID, f.payeeID).Return(f.payee, nil).Once()
	d.account.On("GetById", txCtx, f.budgetID, savingsID).
		Return(&model.Account{ID: savingsID, Type: "savings", TransferPayeeID: &savingsPayeeID}, nil).Once()

	txn := f.txn()
	txn.CategoryID = nil // budget → budget transfers carry no category
	main := txn
	main.ID = f.txnID
	counterpartID := uuid.New()

	d.repo.On("Create", txCtx, mock.MatchedBy(func(t model.Transaction) bool { return *t.AccountID == f.accountID })).
		Return([]model.Transaction{main}, nil).Once()
	d.repo.On("Create", txCtx, mock.MatchedBy(func(t model.Transaction) bool {
		return *t.AccountID == savingsID && t.Amount == 250 && *t.PayeeID == checkingPayeeID &&
			*t.TransferTransactionID == f.txnID && t.CategoryID == nil
	})).Return([]model.Transaction{{ID: counterpartID}}, nil).Once()
	d.repo.On("Update", txCtx, f.budgetID, f.txnID, mock.MatchedBy(func(t model.Transaction) bool {
		return t.TransferTransactionID != nil && *t.TransferTransactionID == counterpartID &&
			*t.TransferAccountID == savingsID
	})).Return(nil).Once()
	linked := main
	linked.TransferTransactionID = &counterpartID
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&linked, nil).Once()

	res, err := svc.Create(f.ctx, txn)

	require.NoError(t, err)
	assert.Equal(t, counterpartID, *res[0].TransferTransactionID)
	begun, committed, _ := d.dbTxn.counts()
	assert.Equal(t, 1, begun, "both legs of a transfer must commit together")
	assert.Equal(t, 1, committed)
	d.assertExpectations(t)
}

func TestTxnCreate_BudgetTransferWithCategoryIsRejected(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	savingsID := uuid.New()
	f.payee.TransferAccountID = &savingsID
	d.budget.On("GetById", txCtx, f.budgetID).Return(f.budget, nil).Once()
	d.account.On("GetById", txCtx, f.budgetID, f.accountID).Return(f.account, nil).Once()
	d.payee.On("GetById", txCtx, f.budgetID, f.payeeID).Return(f.payee, nil).Once()
	d.account.On("GetById", txCtx, f.budgetID, savingsID).Return(&model.Account{ID: savingsID, Type: "savings"}, nil).Once()

	_, err := svc.Create(f.ctx, f.txn())

	require.Error(t, err)
	assert.Equal(t, errs.CodeInvalidArgument, appCode(t, err))
	_, _, rolledBack := d.dbTxn.counts()
	assert.Equal(t, 1, rolledBack)
	d.assertExpectations(t)
}

func TestTxnCreateInTx_JoinsTheCallersTransaction(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	f.expectDependencies(d)
	created := f.txn()
	created.ID = f.txnID
	d.repo.On("Create", txCtx, mock.Anything).Return([]model.Transaction{created}, nil).Once()
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&created, nil).Once()

	err := d.dbTxn.Do(f.ctx, func(ctx context.Context) error {
		_, err := svc.CreateInTx(ctx, f.txn())
		return err
	})

	require.NoError(t, err)
	begun, committed, _ := d.dbTxn.counts()
	assert.Equal(t, 1, begun, "CreateInTx inside an open transaction must not begin another")
	assert.Equal(t, 1, committed)
	d.assertExpectations(t)
}

func TestTxnCreateInTx_FailureRollsBackTheCallersWork(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	f.expectDependencies(d)
	d.repo.On("Create", txCtx, mock.Anything).Return(nil, errors.New("boom")).Once()
	callerWrote := false

	err := d.dbTxn.Do(f.ctx, func(ctx context.Context) error {
		callerWrote = true // stands in for the caller's own writes earlier in the unit
		_, err := svc.CreateInTx(ctx, f.txn())
		return err
	})

	require.Error(t, err)
	assert.True(t, callerWrote)
	_, committed, rolledBack := d.dbTxn.counts()
	assert.Equal(t, 0, committed)
	assert.Equal(t, 1, rolledBack)
}

// ─── CreateDeduped ───────────────────────────────────────────────────────────

func TestTxnCreateDeduped_NewRowAppliesSideEffectsAndReloads(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	f.expectDependencies(d)
	stored := f.txn()
	stored.ID = f.txnID
	stored.Status = model.TransactionStatusManual
	d.repo.On("CreateDeduped", txCtx, mock.MatchedBy(func(t model.Transaction) bool {
		return t.Status == model.TransactionStatusManual
	})).Return(&stored, true, nil).Once()
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&stored, nil).Once()

	got, created, err := svc.CreateDeduped(f.ctx, f.txn())

	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, f.txnID, got.ID)
	assert.Len(t, d.mb.upserts, 1)
	begun, committed, _ := d.dbTxn.counts()
	assert.Equal(t, [2]int{1, 1}, [2]int{begun, committed})
	d.assertExpectations(t)
}

func TestTxnCreateDeduped_DuplicateReturnsExistingWithoutSideEffects(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	f.expectDependencies(d)
	existing := f.txn()
	existing.ID = f.txnID
	d.repo.On("CreateDeduped", txCtx, mock.Anything).Return(&existing, false, nil).Once()

	got, created, err := svc.CreateDeduped(f.ctx, f.txn())

	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, f.txnID, got.ID)
	assert.Empty(t, d.mb.upserts, "a duplicate push must not apply the carryover a second time")
	d.repo.AssertNotCalled(t, "GetById", mock.Anything, mock.Anything, mock.Anything)
	d.assertExpectations(t)
}

func TestTxnCreateDeduped_KeepsAnExplicitStatus(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	f.expectDependencies(d)
	txn := f.txn()
	txn.Status = model.TransactionStatusUnapproved
	d.repo.On("CreateDeduped", txCtx, mock.MatchedBy(func(t model.Transaction) bool {
		return t.Status == model.TransactionStatusUnapproved
	})).Return(&model.Transaction{ID: f.txnID}, false, nil).Once()

	_, _, err := svc.CreateDeduped(f.ctx, txn)

	require.NoError(t, err)
	d.assertExpectations(t)
}

func TestTxnCreateDeduped_Failures(t *testing.T) {
	t.Run("repository error rolls back", func(t *testing.T) {
		svc, d := newTxnSvc()
		f := newTxnFixture()
		f.expectDependencies(d)
		d.repo.On("CreateDeduped", txCtx, mock.Anything).Return(nil, false, errors.New("boom")).Once()

		_, created, err := svc.CreateDeduped(f.ctx, f.txn())

		require.Error(t, err)
		assert.False(t, created)
		assert.Equal(t, errs.CodeTransactionCreateFailed, appCode(t, err))
		_, _, rolledBack := d.dbTxn.counts()
		assert.Equal(t, 1, rolledBack)
	})

	t.Run("invalid payload", func(t *testing.T) {
		svc, d := newTxnSvc()
		f := newTxnFixture()
		txn := f.txn()
		txn.AccountID = nil

		_, _, err := svc.CreateDeduped(f.ctx, txn)

		require.Error(t, err)
		assert.Equal(t, errs.CodeInvalidArgument, appCode(t, err))
		d.assertExpectations(t)
	})

	t.Run("dependency lookup fails", func(t *testing.T) {
		svc, d := newTxnSvc()
		f := newTxnFixture()
		d.budget.On("GetById", txCtx, f.budgetID).Return(nil, repository.ErrNotFound).Once()

		_, _, err := svc.CreateDeduped(f.ctx, f.txn())

		require.Error(t, err)
		assert.Equal(t, errs.CodeBudgetLookupFailed, appCode(t, err))
		assert.ErrorIs(t, err, repository.ErrNotFound)
	})

	t.Run("reload fails after insert", func(t *testing.T) {
		svc, d := newTxnSvc()
		f := newTxnFixture()
		f.expectDependencies(d)
		d.repo.On("CreateDeduped", txCtx, mock.Anything).Return(&model.Transaction{ID: f.txnID}, true, nil).Once()
		d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(nil, errors.New("lost connection")).Once()

		_, _, err := svc.CreateDeduped(f.ctx, f.txn())

		require.Error(t, err)
		assert.Equal(t, errs.CodeTransactionLookupFailed, appCode(t, err))
		_, committed, _ := d.dbTxn.counts()
		assert.Zero(t, committed)
	})
}

// ─── Update ──────────────────────────────────────────────────────────────────

func TestTxnUpdate_WriteFailureRollsBackCarryoverChanges(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	found := f.txn()
	found.ID = f.txnID
	found.Status = model.TransactionStatusManual
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&found, nil).Once()
	f.expectDependencies(d)
	d.repo.On("Update", txCtx, f.budgetID, f.txnID, mock.Anything).Return(errors.New("deadlock detected")).Once()

	edited := f.txn()
	edited.Amount = -400
	err := svc.Update(f.ctx, f.txnID, edited)

	require.Error(t, err)
	assert.Equal(t, errs.CodeTransactionUpdateFailed, appCode(t, err))
	assert.Equal(t, 1, d.mb.updateCalls)
	assert.True(t, d.mb.updateInTx, "carryover changes must be part of the same transaction")
	_, committed, rolledBack := d.dbTxn.counts()
	assert.Equal(t, [2]int{0, 1}, [2]int{committed, rolledBack})
	d.assertExpectations(t)
}

func TestTxnUpdate_UnchangedTransactionCommitsWithoutWrites(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	found := f.txn()
	found.ID = f.txnID
	found.Status = model.TransactionStatusManual
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&found, nil).Once()

	err := svc.Update(f.ctx, f.txnID, f.txn())

	require.NoError(t, err)
	assert.Zero(t, d.mb.updateCalls)
	d.repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	_, committed, _ := d.dbTxn.counts()
	assert.Equal(t, 1, committed)
}

func TestTxnUpdate_ApprovingAnUnapprovedTransaction(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	found := f.txn()
	found.ID = f.txnID
	found.Status = model.TransactionStatusUnapproved
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&found, nil).Once()
	f.expectDependencies(d)
	// confirmation looks up the prediction; none exists for a manual row
	d.cipher.On("GetByTransactionID", txCtx, f.budgetID, f.txnID).Return(nil, repository.ErrNotFound).Once()
	d.repo.On("Update", txCtx, f.budgetID, f.txnID, mock.MatchedBy(func(t model.Transaction) bool {
		return t.Status == model.TransactionStatusApproved
	})).Return(nil).Once()

	err := svc.Update(f.ctx, f.txnID, f.txn())

	require.NoError(t, err)
	d.assertExpectations(t)
}

func TestTxnUpdate_LookupFailures(t *testing.T) {
	t.Run("missing transaction", func(t *testing.T) {
		svc, d := newTxnSvc()
		f := newTxnFixture()
		d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(nil, repository.ErrNotFound).Once()

		err := svc.Update(f.ctx, f.txnID, f.txn())

		require.ErrorIs(t, err, repository.ErrNotFound)
		assert.Equal(t, errs.CodeTransactionLookupFailed, appCode(t, err))
	})

	t.Run("nil transaction", func(t *testing.T) {
		svc, d := newTxnSvc()
		f := newTxnFixture()
		d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(nil, nil).Once()

		err := svc.Update(f.ctx, f.txnID, f.txn())

		assert.Equal(t, errs.CodeTransactionLookupFailed, appCode(t, err))
	})

	t.Run("id mismatch", func(t *testing.T) {
		svc, d := newTxnSvc()
		f := newTxnFixture()
		d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&model.Transaction{ID: uuid.New()}, nil).Once()

		err := svc.Update(f.ctx, f.txnID, f.txn())

		assert.Equal(t, errs.CodeTransactionLookupFailed, appCode(t, err))
	})
}

// ─── DeleteById ──────────────────────────────────────────────────────────────

func TestTxnDelete_TransferRemovesBothLegsAndReversesCarryover(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	counterpartID := uuid.New()
	found := f.txn()
	found.ID = f.txnID
	found.TransferTransactionID = &counterpartID
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&found, nil).Once()
	d.budget.On("GetById", txCtx, f.budgetID).Return(f.budget, nil).Once()
	d.repo.On("DeleteById", txCtx, f.budgetID, counterpartID).Return(nil).Once()
	d.repo.On("DeleteById", txCtx, f.budgetID, f.txnID).Return(nil).Once()

	err := svc.DeleteById(f.ctx, f.txnID)

	require.NoError(t, err)
	require.Len(t, d.mb.upserts, 1)
	assert.Equal(t, upsertCarryoverCall{f.categoryID, "2026-10", 250, true}, d.mb.upserts[0])
	begun, committed, _ := d.dbTxn.counts()
	assert.Equal(t, [2]int{1, 1}, [2]int{begun, committed})
	d.assertExpectations(t)
}

func TestTxnDelete_FailureRollsBack(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	found := f.txn()
	found.ID = f.txnID
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&found, nil).Once()
	d.budget.On("GetById", txCtx, f.budgetID).Return(f.budget, nil).Once()
	d.repo.On("DeleteById", txCtx, f.budgetID, f.txnID).Return(errors.New("fk violation")).Once()

	err := svc.DeleteById(f.ctx, f.txnID)

	require.Error(t, err)
	assert.Equal(t, errs.CodeTransactionDeleteFailed, appCode(t, err))
	_, committed, rolledBack := d.dbTxn.counts()
	assert.Equal(t, [2]int{0, 1}, [2]int{committed, rolledBack})
	assert.Len(t, d.mb.upserts, 1, "the reversed carryover was applied inside the rolled-back transaction")
	assert.True(t, d.mb.upserts[0].inTx)
}

func TestTxnDelete_MissingTransaction(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(nil, nil).Once()

	err := svc.DeleteById(f.ctx, f.txnID)

	assert.Equal(t, errs.CodeTransactionLookupFailed, appCode(t, err))
	d.repo.AssertNotCalled(t, "DeleteById", mock.Anything, mock.Anything, mock.Anything)
}

// ─── UpdateStatus ────────────────────────────────────────────────────────────

// countingBroker counts queued jobs, to check learning is queued exactly once.
type countingBroker struct {
	job.Broker
	queued int
}

func (b *countingBroker) Enqueue(context.Context, string, []byte, ...job.EnqueueOption) (job.EnqueueResult, error) {
	b.queued++
	return job.EnqueueResult{JobID: uuid.New()}, nil
}

// pendingTxn is a pipeline transaction waiting for review, with bank text.
func (f txnFixture) pendingTxn() model.Transaction {
	raw := "Rs 250 debited at BLUE TOKAI"
	txn := f.txn()
	txn.ID = f.txnID
	txn.Status = model.TransactionStatusUnapproved
	txn.RawBankText = &raw
	return txn
}

func TestTxnUpdateStatus_RejectMovesInsideATransaction(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	found := f.pendingTxn()
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&found, nil).Once()
	d.repo.On("TransitionStatus", txCtx, f.budgetID, f.txnID,
		model.TransactionStatusUnapproved, model.TransactionStatusRejected).Return(true, nil).Once()

	require.NoError(t, svc.UpdateStatus(f.ctx, f.txnID, model.TransactionStatusRejected))

	_, committed, _ := d.dbTxn.counts()
	assert.Equal(t, 1, committed)
	d.assertExpectations(t)
}

func TestTxnUpdateStatus_ApprovalWithoutPredictionDoesNotLearn(t *testing.T) {
	svc, d := newTxnSvc()
	broker := &countingBroker{}
	svc.jobBroker = broker
	f := newTxnFixture()
	found := f.pendingTxn()
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&found, nil).Once()
	d.cipher.On("GetByTransactionID", txCtx, f.budgetID, f.txnID).Return(nil, repository.ErrNotFound).Once()
	d.repo.On("TransitionStatus", txCtx, f.budgetID, f.txnID,
		model.TransactionStatusUnapproved, model.TransactionStatusApproved).Return(true, nil).Once()

	require.NoError(t, svc.UpdateStatus(f.ctx, f.txnID, model.TransactionStatusApproved))
	assert.Equal(t, 0, broker.queued)
	d.assertExpectations(t)
}

func TestTxnUpdateStatus_ApprovingAPredictionLearnsOnce(t *testing.T) {
	svc, d := newTxnSvc()
	broker := &countingBroker{}
	svc.jobBroker = broker
	f := newTxnFixture()
	pending := f.pendingTxn()
	approved := pending
	approved.Status = model.TransactionStatusApproved
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&pending, nil).Once()
	d.cipher.On("GetByTransactionID", txCtx, f.budgetID, f.txnID).
		Return(&model.CipherPredictionRecord{TransactionID: f.txnID}, nil).Once()
	d.repo.On("TransitionStatus", txCtx, f.budgetID, f.txnID,
		model.TransactionStatusUnapproved, model.TransactionStatusApproved).Return(true, nil).Once()

	require.NoError(t, svc.UpdateStatus(f.ctx, f.txnID, model.TransactionStatusApproved))
	assert.Equal(t, 1, broker.queued)

	// Approving again finds it approved: no write, no second round of learning.
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&approved, nil).Once()
	require.NoError(t, svc.UpdateStatus(f.ctx, f.txnID, model.TransactionStatusApproved))
	assert.Equal(t, 1, broker.queued)
	d.repo.AssertNumberOfCalls(t, "TransitionStatus", 1)
	d.assertExpectations(t)
}

func TestTxnUpdateStatus_RefusesMovesOutsideTheTable(t *testing.T) {
	tests := []struct {
		from, to model.TransactionStatus
	}{
		{model.TransactionStatusManual, model.TransactionStatusUnapproved},
		{model.TransactionStatusManual, model.TransactionStatusApproved},
		{model.TransactionStatusApproved, model.TransactionStatusUnapproved},
		{model.TransactionStatusApproved, model.TransactionStatusRejected},
		{model.TransactionStatusRejected, model.TransactionStatusApproved},
		{model.TransactionStatusUnapproved, model.TransactionStatusManual},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			svc, d := newTxnSvc()
			f := newTxnFixture()
			found := f.pendingTxn()
			found.Status = tt.from
			d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&found, nil).Once()

			err := svc.UpdateStatus(f.ctx, f.txnID, tt.to)

			assert.Equal(t, errs.CodeTransactionInvalidStatus, appCode(t, err))
			d.repo.AssertNotCalled(t, "TransitionStatus",
				mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			_, committed, rolledBack := d.dbTxn.counts()
			assert.Equal(t, [2]int{0, 1}, [2]int{committed, rolledBack})
		})
	}
}

func TestTxnUpdateStatus_UndoRejection(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	found := f.pendingTxn()
	found.Status = model.TransactionStatusRejected
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&found, nil).Once()
	d.repo.On("TransitionStatus", txCtx, f.budgetID, f.txnID,
		model.TransactionStatusRejected, model.TransactionStatusUnapproved).Return(true, nil).Once()

	require.NoError(t, svc.UpdateStatus(f.ctx, f.txnID, model.TransactionStatusUnapproved))
	d.assertExpectations(t)
}

func TestTxnUpdateStatus_UnknownStatusIsInvalid(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()

	err := svc.UpdateStatus(f.ctx, f.txnID, model.TransactionStatus("MAYBE"))

	assert.Equal(t, errs.CodeInvalidArgument, appCode(t, err))
	d.repo.AssertNotCalled(t, "GetById", mock.Anything, mock.Anything, mock.Anything)
}

func TestTxnUpdateStatus_MissingTransaction(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(nil, repository.ErrNotFound).Once()

	err := svc.UpdateStatus(f.ctx, f.txnID, model.TransactionStatusRejected)

	assert.Equal(t, errs.CodeTransactionLookupFailed, appCode(t, err))
}

func TestTxnUpdateStatus_TransferMovesBothHalves(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	counterpartID := uuid.New()
	found := f.pendingTxn()
	found.TransferTransactionID = &counterpartID
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&found, nil).Once()
	d.repo.On("TransitionStatus", txCtx, f.budgetID, f.txnID,
		model.TransactionStatusUnapproved, model.TransactionStatusRejected).Return(true, nil).Once()
	d.repo.On("TransitionStatus", txCtx, f.budgetID, counterpartID,
		model.TransactionStatusUnapproved, model.TransactionStatusRejected).Return(true, nil).Once()

	require.NoError(t, svc.UpdateStatus(f.ctx, f.txnID, model.TransactionStatusRejected))
	d.assertExpectations(t)
}

func TestTxnUpdateStatus_LosingARaceToTheSameStatusSucceedsWithoutLearning(t *testing.T) {
	svc, d := newTxnSvc()
	broker := &countingBroker{}
	svc.jobBroker = broker
	f := newTxnFixture()
	pending := f.pendingTxn()
	approved := pending
	approved.Status = model.TransactionStatusApproved
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&pending, nil).Once()
	d.cipher.On("GetByTransactionID", txCtx, f.budgetID, f.txnID).
		Return(&model.CipherPredictionRecord{TransactionID: f.txnID}, nil).Once()
	d.repo.On("TransitionStatus", txCtx, f.budgetID, f.txnID,
		model.TransactionStatusUnapproved, model.TransactionStatusApproved).Return(false, nil).Once()
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&approved, nil).Once()

	require.NoError(t, svc.UpdateStatus(f.ctx, f.txnID, model.TransactionStatusApproved))
	assert.Equal(t, 0, broker.queued, "the request that won the race queued the learning")
	d.assertExpectations(t)
}

func TestTxnUpdateStatus_LosingARaceToAnotherStatusConflicts(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	pending := f.pendingTxn()
	rejected := pending
	rejected.Status = model.TransactionStatusRejected
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&pending, nil).Once()
	d.cipher.On("GetByTransactionID", txCtx, f.budgetID, f.txnID).Return(nil, repository.ErrNotFound).Once()
	d.repo.On("TransitionStatus", txCtx, f.budgetID, f.txnID,
		model.TransactionStatusUnapproved, model.TransactionStatusApproved).Return(false, nil).Once()
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&rejected, nil).Once()

	err := svc.UpdateStatus(f.ctx, f.txnID, model.TransactionStatusApproved)

	assert.Equal(t, errs.CodeTransactionInvalidStatus, appCode(t, err))
}

func TestTxnUpdateStatus_WriteFailureIsWrapped(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	found := f.pendingTxn()
	d.repo.On("GetById", txCtx, f.budgetID, f.txnID).Return(&found, nil).Once()
	d.cipher.On("GetByTransactionID", txCtx, f.budgetID, f.txnID).
		Return(&model.CipherPredictionRecord{TransactionID: f.txnID}, nil).Once()
	d.repo.On("TransitionStatus", txCtx, f.budgetID, f.txnID,
		model.TransactionStatusUnapproved, model.TransactionStatusApproved).Return(false, errors.New("boom")).Once()

	err := svc.UpdateStatus(f.ctx, f.txnID, model.TransactionStatusApproved)

	assert.Equal(t, errs.CodeTransactionUpdateFailed, appCode(t, err))
	d.client.AssertNotCalled(t, "GenerateTransactionEmbedding", mock.Anything, mock.Anything)
}

// ─── UpdateLocation ──────────────────────────────────────────────────────────

func ptr[T any](v T) *T { return &v }

func TestTxnUpdateLocation_Validation(t *testing.T) {
	tests := map[string]model.TransactionLocationReq{
		"lat without lng":   {Lat: ptr(12.97)},
		"lng without lat":   {Lng: ptr(77.59)},
		"lat out of range":  {Lat: ptr(91.0), Lng: ptr(77.59)},
		"lng out of range":  {Lat: ptr(12.97), Lng: ptr(-181.0)},
		"both out of range": {Lat: ptr(-90.5), Lng: ptr(180.5)},
	}
	for name, req := range tests {
		t.Run(name, func(t *testing.T) {
			svc, d := newTxnSvc()
			f := newTxnFixture()

			_, err := svc.UpdateLocation(f.ctx, f.txnID, req)

			assert.Equal(t, errs.CodeInvalidArgument, appCode(t, err))
			begun, _, _ := d.dbTxn.counts()
			assert.Zero(t, begun)
			assert.Zero(t, d.geo.calls)
		})
	}
}

func TestTxnUpdateLocation_GeocodesMissingNameAndDefaultsSource(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	lat, lng := 12.97, 77.64
	d.repo.On("UpdateLocation", txCtx, f.budgetID, f.txnID, &lat, &lng,
		mock.MatchedBy(func(n *string) bool { return n != nil && *n == "Blue Tokai, Indiranagar" }),
		mock.MatchedBy(func(s *model.LocationSource) bool { return s != nil && *s == model.LocationSourceManual }),
	).Return(nil).Once()
	reloaded := &model.Transaction{ID: f.txnID}
	d.repo.On("GetById", outsideTx, f.budgetID, f.txnID).Return(reloaded, nil).Once()

	got, err := svc.UpdateLocation(f.ctx, f.txnID, model.TransactionLocationReq{Lat: &lat, Lng: &lng, Source: "bogus"})

	require.NoError(t, err)
	assert.Same(t, reloaded, got)
	assert.Equal(t, 1, d.geo.calls)
	d.assertExpectations(t)
}

func TestTxnUpdateLocation_KeepsGivenNameAndSource(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	lat, lng, name := 12.97, 77.64, "Office"
	auto := model.LocationSourceAuto
	d.repo.On("UpdateLocation", txCtx, f.budgetID, f.txnID, &lat, &lng, &name, &auto).Return(nil).Once()
	d.repo.On("GetById", mock.Anything, f.budgetID, f.txnID).Return(&model.Transaction{ID: f.txnID}, nil).Once()

	_, err := svc.UpdateLocation(f.ctx, f.txnID, model.TransactionLocationReq{Lat: &lat, Lng: &lng, Name: &name, Source: auto})

	require.NoError(t, err)
	assert.Zero(t, d.geo.calls)
	d.assertExpectations(t)
}

func TestTxnUpdateLocation_GeocodeFailureStillSavesCoordinates(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	d.geo.err = errors.New("nominatim timeout")
	lat, lng := 12.97, 77.64
	d.repo.On("UpdateLocation", txCtx, f.budgetID, f.txnID, &lat, &lng, (*string)(nil), mock.Anything).Return(nil).Once()
	d.repo.On("GetById", mock.Anything, f.budgetID, f.txnID).Return(&model.Transaction{ID: f.txnID}, nil).Once()

	_, err := svc.UpdateLocation(f.ctx, f.txnID, model.TransactionLocationReq{Lat: &lat, Lng: &lng})

	require.NoError(t, err)
	d.assertExpectations(t)
}

func TestTxnUpdateLocation_ClearingPassesNils(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	d.repo.On("UpdateLocation", txCtx, f.budgetID, f.txnID,
		(*float64)(nil), (*float64)(nil), (*string)(nil), (*model.LocationSource)(nil)).Return(nil).Once()
	d.repo.On("GetById", mock.Anything, f.budgetID, f.txnID).Return(&model.Transaction{ID: f.txnID}, nil).Once()

	_, err := svc.UpdateLocation(f.ctx, f.txnID, model.TransactionLocationReq{})

	require.NoError(t, err)
	assert.Zero(t, d.geo.calls)
	d.assertExpectations(t)
}

func TestTxnUpdateLocation_Failures(t *testing.T) {
	t.Run("write fails", func(t *testing.T) {
		svc, d := newTxnSvc()
		f := newTxnFixture()
		d.repo.On("UpdateLocation", txCtx, f.budgetID, f.txnID, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(repository.ErrNotFound).Once()

		_, err := svc.UpdateLocation(f.ctx, f.txnID, model.TransactionLocationReq{})

		assert.Equal(t, errs.CodeTransactionUpdateFailed, appCode(t, err))
		assert.ErrorIs(t, err, repository.ErrNotFound)
		d.repo.AssertNotCalled(t, "GetById", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("reload fails", func(t *testing.T) {
		svc, d := newTxnSvc()
		f := newTxnFixture()
		d.repo.On("UpdateLocation", txCtx, f.budgetID, f.txnID, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).Once()
		d.repo.On("GetById", mock.Anything, f.budgetID, f.txnID).Return(nil, errors.New("boom")).Once()

		_, err := svc.UpdateLocation(f.ctx, f.txnID, model.TransactionLocationReq{})

		assert.Equal(t, errs.CodeTransactionLookupFailed, appCode(t, err))
	})
}

// ─── enrichLocation / predictionConfirmed ────────────────────────────────────

func TestTxnEnrichLocation(t *testing.T) {
	lat, lng := 12.97, 77.64
	auto := model.LocationSourceAuto
	bogus := model.LocationSource("bogus")

	tests := []struct {
		name       string
		txn        model.Transaction
		geo        *fakeGeocoder
		wantName   *string
		wantSource *model.LocationSource
		wantCalls  int
	}{
		{name: "no coordinates leaves the row alone", txn: model.Transaction{}, geo: &fakeGeocoder{name: "x"}},
		{name: "one coordinate leaves the row alone", txn: model.Transaction{LocationLat: &lat}, geo: &fakeGeocoder{name: "x"}},
		{
			name:       "missing name is geocoded, source defaults to manual",
			txn:        model.Transaction{LocationLat: &lat, LocationLng: &lng},
			geo:        &fakeGeocoder{name: "Indiranagar"},
			wantName:   ptr("Indiranagar"),
			wantSource: ptr(model.LocationSourceManual),
			wantCalls:  1,
		},
		{
			name:       "invalid source is replaced, existing name kept",
			txn:        model.Transaction{LocationLat: &lat, LocationLng: &lng, LocationName: ptr("Home"), LocationSource: &bogus},
			geo:        &fakeGeocoder{name: "x"},
			wantName:   ptr("Home"),
			wantSource: ptr(model.LocationSourceManual),
		},
		{
			name:       "valid source is kept",
			txn:        model.Transaction{LocationLat: &lat, LocationLng: &lng, LocationName: ptr("Home"), LocationSource: &auto},
			geo:        &fakeGeocoder{name: "x"},
			wantName:   ptr("Home"),
			wantSource: &auto,
		},
		{
			name:       "geocode failure keeps coordinates only",
			txn:        model.Transaction{LocationLat: &lat, LocationLng: &lng},
			geo:        &fakeGeocoder{err: errors.New("timeout")},
			wantSource: ptr(model.LocationSourceManual),
			wantCalls:  1,
		},
		{
			name:       "no geocoder configured",
			txn:        model.Transaction{LocationLat: &lat, LocationLng: &lng},
			wantSource: ptr(model.LocationSourceManual),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &transactionService{}
			if tt.geo != nil {
				svc.geocodeService = tt.geo
			}
			txn := tt.txn

			svc.enrichLocation(context.Background(), &txn)

			assert.Equal(t, tt.wantName, txn.LocationName)
			assert.Equal(t, tt.wantSource, txn.LocationSource)
			if tt.geo != nil {
				assert.Equal(t, tt.wantCalls, tt.geo.calls)
			}
		})
	}
}

func TestTxnPredictionConfirmed(t *testing.T) {
	txn := func(s model.TransactionStatus) *model.Transaction { return &model.Transaction{Status: s} }
	tests := []struct {
		name     string
		old, new *model.Transaction
		want     bool
	}{
		{"unapproved to approved", txn(model.TransactionStatusUnapproved), txn(model.TransactionStatusApproved), true},
		{"approved re-saved", txn(model.TransactionStatusApproved), txn(model.TransactionStatusApproved), false},
		{"manual to approved", txn(model.TransactionStatusManual), txn(model.TransactionStatusApproved), false},
		{"unapproved to rejected", txn(model.TransactionStatusUnapproved), txn(model.TransactionStatusRejected), false},
		{"no old", nil, txn(model.TransactionStatusApproved), false},
		{"no new", txn(model.TransactionStatusUnapproved), nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, predictionConfirmed(tt.old, tt.new))
		})
	}
}

// ─── applySideEffects: prediction feedback on update ─────────────────────────

func TestTxnApplySideEffects_UpdatePredictionFeedback(t *testing.T) {
	f := newTxnFixture()
	newCategory := uuid.New()
	oldTxn := f.txn()
	oldTxn.ID = f.txnID
	newTxn := oldTxn
	newTxn.CategoryID = &newCategory // a correction
	input := func() sideEffectInput {
		o, n := oldTxn, newTxn
		return sideEffectInput{budgetId: f.budgetID, oldTxn: &o, newTxn: &n, budget: f.budget, account: f.account, payee: f.payee}
	}

	t.Run("no prediction repository configured", func(t *testing.T) {
		svc, _ := newTxnSvc()
		svc.cipherPredictionRepo = nil

		require.NoError(t, svc.applySideEffects(f.ctx, input()))
	})

	t.Run("prediction not found is not an error", func(t *testing.T) {
		svc, d := newTxnSvc()
		d.cipher.On("GetByTransactionID", mock.Anything, f.budgetID, f.txnID).Return(nil, repository.ErrNotFound).Once()

		require.NoError(t, svc.applySideEffects(f.ctx, input()))
		d.cipher.AssertNotCalled(t, "MarkUserCorrected", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("lookup failure is wrapped", func(t *testing.T) {
		svc, d := newTxnSvc()
		d.cipher.On("GetByTransactionID", mock.Anything, f.budgetID, f.txnID).Return(nil, errors.New("boom")).Once()

		err := svc.applySideEffects(f.ctx, input())

		assert.Equal(t, errs.CodeTransactionLookupFailed, appCode(t, err))
	})

	t.Run("correction is recorded and queued for learning with the stored bank text", func(t *testing.T) {
		svc, d := newTxnSvc()
		raw := "Rs 250 debited at BLUE TOKAI"
		in := input()
		in.oldTxn.RawBankText = &raw
		var queued *model.Transaction
		in.queueLearning = func(txn model.Transaction) { queued = &txn }
		d.cipher.On("GetByTransactionID", mock.Anything, f.budgetID, f.txnID).
			Return(&model.CipherPredictionRecord{TransactionID: f.txnID}, nil).Once()
		d.cipher.On("MarkUserCorrected", mock.Anything, f.budgetID, f.txnID, &f.payeeID, &newCategory).Return(nil).Once()

		require.NoError(t, svc.applySideEffects(f.ctx, in))

		require.NotNil(t, queued)
		assert.Equal(t, &raw, queued.RawBankText)
		assert.Equal(t, &newCategory, queued.CategoryID)
		d.cipher.AssertExpectations(t)
	})

	t.Run("recording the correction fails", func(t *testing.T) {
		svc, d := newTxnSvc()
		d.cipher.On("GetByTransactionID", mock.Anything, f.budgetID, f.txnID).
			Return(&model.CipherPredictionRecord{TransactionID: f.txnID}, nil).Once()
		d.cipher.On("MarkUserCorrected", mock.Anything, f.budgetID, f.txnID, mock.Anything, mock.Anything).
			Return(errors.New("boom")).Once()

		err := svc.applySideEffects(f.ctx, input())

		assert.Equal(t, errs.CodeTransactionUpdateFailed, appCode(t, err))
	})
}

// ─── LearnFromTransaction ────────────────────────────────────────────────────

func TestTxnLearnMapping_WritesRuleAndEmbeddingInOneTransaction(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	raw := "Rs 250 debited at BLUE TOKAI"
	txn := f.txn()
	txn.ID = f.txnID
	txn.RawBankText = &raw

	d.client.On("GenerateTransactionEmbedding", outsideTx, TransactionEmbeddingRequest{RawBankText: raw, Amount: -250}).
		Return(&TransactionEmbeddingResponse{MatchString: "blue tokai", EmbeddingText: "debit blue tokai", Embedding: "[0.25, 0.5]"}, nil).Once()
	d.payeeRule.On("CreatePayeeRule", txCtx, mock.MatchedBy(func(r model.PayeeRule) bool {
		return r.BudgetID == f.budgetID && r.MatchString == "blue tokai" && r.PayeeID == f.payeeID
	})).Return(nil).Once()
	d.embedding.On("Upsert", txCtx, mock.Anything, []float64{0.25, 0.5}).Return(nil).Once()
	done := make(chan struct{})
	d.cipher.On("MarkLearned", txCtx, f.budgetID, f.txnID).Return(nil).Once().Run(func(mock.Arguments) { close(done) })

	_ = svc.learnTransactionMapping(f.ctx, f.budgetID, txn)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("learning never finished")
	}
	assert.Eventually(t, func() bool { _, c, _ := d.dbTxn.counts(); return c == 1 }, time.Second, 10*time.Millisecond)
	d.assertExpectations(t)
}

func TestTxnLearnMapping_EmbeddingFailureRollsBackTheRule(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	raw := "Rs 250 debited at BLUE TOKAI"
	txn := f.txn()
	txn.ID = f.txnID
	txn.RawBankText = &raw

	d.client.On("GenerateTransactionEmbedding", mock.Anything, mock.Anything).
		Return(&TransactionEmbeddingResponse{MatchString: "blue tokai", Embedding: "[0.25]"}, nil).Once()
	d.payeeRule.On("CreatePayeeRule", txCtx, mock.Anything).Return(nil).Once()
	d.embedding.On("Upsert", txCtx, mock.Anything, mock.Anything).Return(errors.New("vector dim mismatch")).Once()
	done := make(chan struct{})
	d.cipher.On("MarkLearnFailed", outsideTx, f.budgetID, f.txnID, mock.MatchedBy(func(reason string) bool {
		return reason != ""
	})).Return(nil).Once().Run(func(mock.Arguments) { close(done) })

	_ = svc.learnTransactionMapping(f.ctx, f.budgetID, txn)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("learning failure was never recorded")
	}
	_, committed, rolledBack := d.dbTxn.counts()
	assert.Equal(t, 0, committed)
	assert.Equal(t, 1, rolledBack, "the payee rule must not survive without its embedding")
	d.cipher.AssertNotCalled(t, "MarkLearned", mock.Anything, mock.Anything, mock.Anything)
}

func TestTxnLearnMapping_UnreadableEmbeddingIsRecordedNotWritten(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	raw := "Rs 250 debited at BLUE TOKAI"
	txn := f.txn()
	txn.ID = f.txnID
	txn.RawBankText = &raw

	d.client.On("GenerateTransactionEmbedding", mock.Anything, mock.Anything).
		Return(&TransactionEmbeddingResponse{MatchString: "blue tokai", Embedding: "not-a-vector"}, nil).Once()
	done := make(chan struct{})
	d.cipher.On("MarkLearnFailed", mock.Anything, f.budgetID, f.txnID, mock.Anything).
		Return(nil).Once().Run(func(mock.Arguments) { close(done) })

	_ = svc.learnTransactionMapping(f.ctx, f.budgetID, txn)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("learning failure was never recorded")
	}
	begun, _, _ := d.dbTxn.counts()
	assert.Zero(t, begun)
	d.payeeRule.AssertNotCalled(t, "CreatePayeeRule", mock.Anything, mock.Anything)
}

func TestTxnLearnFromTransaction_DoesNotEmbed(t *testing.T) {
	svc, d := newTxnSvc()
	f := newTxnFixture()
	raw := "Rs 250 debited at BLUE TOKAI"
	txn := f.txn()
	txn.ID = f.txnID
	txn.RawBankText = &raw

	svc.LearnFromTransaction(f.ctx, txn)

	d.client.AssertNotCalled(t, "GenerateTransactionEmbedding", mock.Anything, mock.Anything)
}
