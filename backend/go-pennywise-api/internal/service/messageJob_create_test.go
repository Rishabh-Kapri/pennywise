package service

import (
	"context"
	"testing"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
)

type fakeRunNotifier struct {
	WebsocketService
	events []string
	err    error
}

func (n *fakeRunNotifier) SendNotification(_ context.Context, _ uuid.UUID, name string, _ any) error {
	n.events = append(n.events, name)
	return n.err
}

type createAccounts struct {
	repository.AccountRepository
	account *model.Account
}

func (f createAccounts) GetBySuffix(context.Context, uuid.UUID, string) (*model.Account, error) {
	return f.account, nil
}

type createCategories struct {
	repository.CategoryRepository
	id uuid.UUID
}

func (f createCategories) GetAllSimplified(context.Context, uuid.UUID) ([]model.CategorySimplified, error) {
	return []model.CategorySimplified{{ID: f.id, Name: "Food"}}, nil
}

type createPayees struct {
	PayeeService
	id      uuid.UUID
	created []string
}

func (f *createPayees) Search(context.Context, string) ([]model.Payee, error) {
	return []model.Payee{{ID: f.id, Name: "Swiggy"}}, nil
}

func (f *createPayees) Create(_ context.Context, p model.Payee) (*model.Payee, error) {
	f.created = append(f.created, p.Name)
	return &model.Payee{ID: uuid.New(), Name: p.Name}, nil
}

type createTransactions struct {
	TransactionService
	created bool
	got     model.Transaction
}

func (f *createTransactions) CreateDeduped(_ context.Context, txn model.Transaction) (*model.Transaction, bool, error) {
	f.got = txn
	txn.ID = uuid.New()
	return &txn, f.created, nil
}

type createTransactionRepo struct {
	repository.TransactionRepository
	existing *model.Transaction
	hashes   []string
}

func (f *createTransactionRepo) FindByDedupeHash(_ context.Context, _ uuid.UUID, hash string) (*model.Transaction, error) {
	f.hashes = append(f.hashes, hash)
	return f.existing, nil
}

type createPredictions struct {
	PredictionService
	records []model.CipherPredictionRecord
}

func (f *createPredictions) CreateCipherPrediction(
	_ context.Context, r model.CipherPredictionRecord,
) (*model.CipherPredictionRecord, error) {
	f.records = append(f.records, r)
	return &r, nil
}

type createAliases struct {
	repository.CounterpartyAliasRepository
	learned   []model.CounterpartyAliasLearn
	confirmed []model.CounterpartyAliasConfirm
}

func (f *createAliases) Confirm(_ context.Context, in model.CounterpartyAliasConfirm) error {
	f.confirmed = append(f.confirmed, in)
	return nil
}

func (f *createAliases) Learn(_ context.Context, in model.CounterpartyAliasLearn) error {
	f.learned = append(f.learned, in)
	return nil
}

type createSources struct {
	repository.TransactionSourceRepository
	recorded []model.MessageSource
	existing bool
	key      *model.LookupKey
}

func (f *createSources) RecordFromPipeline(_ context.Context, _, _ uuid.UUID, s model.MessageSource) (bool, error) {
	f.recorded = append(f.recorded, s)
	return !f.existing, nil
}

func (f *createSources) LookupKey(context.Context, uuid.UUID, uuid.UUID) (*model.LookupKey, error) {
	return f.key, nil
}

type createFixture struct {
	svc          MessageJobService
	transactions *createTransactions
	repo         *createTransactionRepo
	payees       *createPayees
	predictions  *createPredictions
	aliases      *createAliases
	sources      *createSources
	notifier     *fakeRunNotifier
}

func newCreateFixture(account *model.Account, created bool) createFixture {
	f := createFixture{
		transactions: &createTransactions{created: created},
		repo:         &createTransactionRepo{},
		payees:       &createPayees{id: uuid.New()},
		predictions:  &createPredictions{},
		aliases:      &createAliases{},
		sources:      &createSources{},
		notifier:     &fakeRunNotifier{},
	}
	f.svc = NewMessageJobService(MessageJobDeps{
		Transactions:        f.transactions,
		TransactionRepo:     f.repo,
		Payees:              f.payees,
		Predictions:         f.predictions,
		Websocket:           f.notifier,
		DBTxn:               fakeDBTxn{},
		Accounts:            createAccounts{account: account},
		Categories:          createCategories{id: uuid.New()},
		CounterpartyAliases: f.aliases,
		TransactionSources:  f.sources,
	})
	return f
}

func createPayload() model.TransactionCreatePayload {
	return model.TransactionCreatePayload{
		Verdict:  model.MessageVerdict{Type: "debit"},
		Entities: model.MessageEntities{Account: "HDFC 4567", Amount: 450, Date: time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)},
		Prediction: model.MessagePrediction{
			PayeeName: "swiggy", CategoryName: "food", Confidence: 0.9, Source: model.MessagePredictionPredictor,
		},
		Source: &model.MessageSource{Kind: "EMAIL", Text: "Rs 450 debited", Channel: "UPI", Merchant: "SWIGGY"},
	}
}

func createContext() context.Context {
	return utils.WithBudgetID(context.Background(), uuid.New())
}

func TestCreateTransactionSavesAndSaysWhatToLearn(t *testing.T) {
	f := newCreateFixture(&model.Account{ID: uuid.New()}, true)
	res, err := f.svc.CreateTransaction(createContext(), createPayload())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.TransactionIDs) != 1 || res.Learn == nil || res.Learn.Source == nil || res.Learn.PayeeID != f.payees.id {
		t.Fatalf("result = %+v, learn = %+v", res, res.Learn)
	}
	txn := f.transactions.got
	if txn.Amount != -450 || string(txn.Date) != "2026-11-08" || txn.Status != model.TransactionStatusUnapproved {
		t.Fatalf("transaction = %+v", txn)
	}
	if *txn.PayeeID != f.payees.id || len(f.payees.created) != 0 {
		t.Fatalf("payee = %v, created %v", *txn.PayeeID, f.payees.created)
	}
	if len(f.predictions.records) != 1 || f.predictions.records[0].Source != model.PredictionSourceLLM {
		t.Fatalf("predictions = %+v", f.predictions.records)
	}
	if len(f.sources.recorded)+len(f.aliases.learned) != 0 {
		t.Fatal("learning belongs to the transaction.learn job")
	}
}

func TestCreateTransactionDuplicateStillQueuesLearning(t *testing.T) {
	f := newCreateFixture(&model.Account{ID: uuid.New()}, false)
	res, err := f.svc.CreateTransaction(createContext(), createPayload())
	if err != nil || len(res.TransactionIDs) != 1 || res.Learn == nil {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if len(f.predictions.records) != 0 {
		t.Fatal("a duplicate must not record a prediction")
	}
}

func TestCreateTransactionNeedsAnAccount(t *testing.T) {
	f := newCreateFixture(nil, true)
	if _, err := f.svc.CreateTransaction(createContext(), createPayload()); !errs.HasCode(err, errs.CodeAccountLookupFailed) {
		t.Fatalf("err = %v", err)
	}
}

func learnPayload() model.TransactionLearnPayload {
	category := uuid.New()
	return model.TransactionLearnPayload{
		TransactionID: uuid.New(),
		PayeeID:       uuid.New(),
		CategoryID:    &category,
		Source:        &model.MessageSource{Kind: "EMAIL", Text: "t", Channel: "UPI", Merchant: "SWIGGY"},
	}
}

func TestLearnTransactionFromSource(t *testing.T) {
	f := newCreateFixture(nil, true)
	if err := f.svc.LearnTransaction(createContext(), learnPayload()); err != nil {
		t.Fatal(err)
	}
	if len(f.sources.recorded) != 1 || len(f.aliases.learned) != 1 || f.aliases.learned[0].Merchant != "SWIGGY" {
		t.Fatalf("sources %+v, aliases %+v", f.sources.recorded, f.aliases.learned)
	}
}

func TestLearnTransactionRetryDoesntCountTwice(t *testing.T) {
	f := newCreateFixture(nil, true)
	f.sources.existing = true
	if err := f.svc.LearnTransaction(createContext(), learnPayload()); err != nil {
		t.Fatal(err)
	}
	if len(f.aliases.learned) != 0 {
		t.Fatalf("learned %+v", f.aliases.learned)
	}
}

func TestLearnTransactionNoKeyStillSavesSource(t *testing.T) {
	f := newCreateFixture(nil, true)
	p := learnPayload()
	p.Source = &model.MessageSource{Kind: "EMAIL", Text: "Payment received"}
	if err := f.svc.LearnTransaction(createContext(), p); err != nil {
		t.Fatal(err)
	}
	if len(f.sources.recorded) != 1 || len(f.aliases.learned) != 0 {
		t.Fatalf("sources %+v, aliases %+v", f.sources.recorded, f.aliases.learned)
	}
}

func TestLearnTransactionConfirmed(t *testing.T) {
	f := newCreateFixture(nil, true)
	f.sources.key = &model.LookupKey{Channel: "UPI", Merchant: "SWIGGY"}
	p := learnPayload()
	p.Source, p.Confirmed = nil, true
	if err := f.svc.LearnTransaction(createContext(), p); err != nil {
		t.Fatal(err)
	}
	if len(f.aliases.confirmed) != 1 || f.aliases.confirmed[0].PayeeID != p.PayeeID {
		t.Fatalf("confirmed %+v", f.aliases.confirmed)
	}

	// A transaction with no key has nothing to confirm.
	f = newCreateFixture(nil, true)
	if err := f.svc.LearnTransaction(createContext(), p); err != nil || len(f.aliases.confirmed) != 0 {
		t.Fatalf("no key: %v, %+v", err, f.aliases.confirmed)
	}
}

func TestCreateTransactionDedupesOnRefAcrossSources(t *testing.T) {
	account := &model.Account{ID: uuid.New()}
	create := func(text string, amount float64, ref string) string {
		t.Helper()
		f := newCreateFixture(account, true)
		p := createPayload()
		p.Source.Text, p.Entities.Amount, p.Entities.Ref = text, amount, ref
		if _, err := f.svc.CreateTransaction(createContext(), p); err != nil {
			t.Fatal(err)
		}
		return *f.transactions.got.DedupeHash
	}

	email := create("Rs.450 debited, reference no.: 900000000001", 450, "900000000001")
	sms := create("Sent Rs.450 Ref 900000000001", 450, " 900000000001 ")
	if email != sms {
		t.Fatal("the same payment's email and SMS got different hashes")
	}
	if other := create("Rs.450 debited, reference no.: 900000000002", 450, "900000000002"); other == email {
		t.Fatal("two payments share a hash")
	}
	if a, b := create("email text", 450, ""), create("sms text", 450, ""); a == b {
		t.Fatal("without a ref, different text must stay different")
	}
}

func TestCreateTransactionWithoutARefSkipsTheOlderHashCheck(t *testing.T) {
	f := newCreateFixture(&model.Account{ID: uuid.New()}, true)
	if _, err := f.svc.CreateTransaction(createContext(), createPayload()); err != nil {
		t.Fatal(err)
	}
	if len(f.repo.hashes) != 0 {
		t.Fatalf("looked up %v", f.repo.hashes)
	}
}

func TestCreateTransactionFindsARowSavedBeforeRefs(t *testing.T) {
	f := newCreateFixture(&model.Account{ID: uuid.New()}, true)
	payee := uuid.New()
	f.repo.existing = &model.Transaction{ID: uuid.New(), PayeeID: &payee, CreatedAt: time.Now().Add(-time.Hour)}
	p := createPayload()
	p.Entities.Ref = "900000000001"

	res, err := f.svc.CreateTransaction(createContext(), p)
	if err != nil {
		t.Fatal(err)
	}
	if res.Created || len(res.TransactionIDs) != 1 || res.TransactionIDs[0] != f.repo.existing.ID {
		t.Fatalf("result %+v", res)
	}
	if f.transactions.got.DedupeHash != nil {
		t.Fatal("created a second transaction")
	}
	if !res.ExistingSince.Equal(f.repo.existing.CreatedAt) {
		t.Fatalf("existing since %v", res.ExistingSince)
	}
}
