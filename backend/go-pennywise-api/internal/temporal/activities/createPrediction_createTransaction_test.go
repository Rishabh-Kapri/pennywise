package temporal

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
	"go.temporal.io/sdk/testsuite"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
)

type fakePredictionService struct {
	createCipherPrediction func(context.Context, model.CipherPredictionRecord) (*model.CipherPredictionRecord, error)
}

func (f *fakePredictionService) GetAll(context.Context) ([]model.Prediction, error) {
	return nil, nil
}

func (f *fakePredictionService) Create(context.Context, model.Prediction) ([]model.Prediction, error) {
	return nil, nil
}

func (f *fakePredictionService) Update(context.Context, uuid.UUID, model.Prediction) error {
	return nil
}

func (f *fakePredictionService) DeleteById(context.Context, uuid.UUID) error {
	return nil
}

func (f *fakePredictionService) CreateCipherPrediction(
	ctx context.Context,
	p model.CipherPredictionRecord,
) (*model.CipherPredictionRecord, error) {
	if f.createCipherPrediction == nil {
		return &p, nil
	}
	return f.createCipherPrediction(ctx, p)
}

type fakeTransactionService struct {
	create func(context.Context, model.Transaction) ([]model.Transaction, error)
}

func (f *fakeTransactionService) GetAll(context.Context) ([]model.Transaction, error) {
	return nil, nil
}

func (f *fakeTransactionService) GetAllNormalized(context.Context, *uuid.UUID) ([]model.Transaction, error) {
	return nil, nil
}

func (f *fakeTransactionService) Update(context.Context, uuid.UUID, model.Transaction) error {
	return nil
}

func (f *fakeTransactionService) Create(ctx context.Context, txn model.Transaction) ([]model.Transaction, error) {
	if f.create == nil {
		return []model.Transaction{txn}, nil
	}
	return f.create(ctx, txn)
}

func (f *fakeTransactionService) DeleteById(context.Context, uuid.UUID) error {
	return nil
}

type fakePayeeService struct {
	create func(context.Context, model.Payee) (*model.Payee, error)
}

func (f *fakePayeeService) GetAll(context.Context) ([]model.Payee, error) {
	return nil, nil
}

func (f *fakePayeeService) Search(context.Context, string) ([]model.Payee, error) {
	return nil, nil
}

func (f *fakePayeeService) GetById(context.Context, uuid.UUID) (*model.Payee, error) {
	return nil, nil
}

func (f *fakePayeeService) Create(ctx context.Context, payee model.Payee) (*model.Payee, error) {
	if f.create == nil {
		return &payee, nil
	}
	return f.create(ctx, payee)
}

func (f *fakePayeeService) DeleteById(context.Context, uuid.UUID) error {
	return nil
}

func (f *fakePayeeService) Update(context.Context, uuid.UUID, model.Payee) error {
	return nil
}

func assertErrorCode(t *testing.T, err error, code errs.Code) {
	t.Helper()
	var appErr *errs.Error
	if !errors.As(err, &appErr) {
		if strings.Contains(err.Error(), string(code)+":") {
			return
		}
		t.Fatalf("expected error code %s, got %T: %v", code, err, err)
	}
	if appErr.Code != code {
		t.Fatalf("expected error code %s, got %s", code, appErr.Code)
	}
}

func valueOrNil[T any](value *T) any {
	if value == nil {
		return nil
	}
	return *value
}

func executeCreateCipherPredictionActivity(
	t *testing.T,
	act CreateCipherPredictionActivity,
	input model.CreateCipherPredictionInput,
) error {
	t.Helper()
	suite := testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(act.CreateCipherPrediction)
	encoded, err := env.ExecuteActivity(act.CreateCipherPrediction, input)
	if err != nil {
		return err
	}
	if encoded.HasValue() {
		var result any
		if err := encoded.Get(&result); err != nil {
			t.Fatalf("failed to decode activity result: %v", err)
		}
	}
	return nil
}

func executeCreateTransactionActivity(
	t *testing.T,
	act CreateTransactionActivity,
	input model.PredictionResultInput,
) ([]model.Transaction, error) {
	t.Helper()
	suite := testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(act.CreateTransaction)
	encoded, err := env.ExecuteActivity(act.CreateTransaction, input)
	if err != nil {
		return nil, err
	}
	var result []model.Transaction
	if err := encoded.Get(&result); err != nil {
		t.Fatalf("failed to decode activity result: %v", err)
	}
	return result, nil
}

func TestCreateCipherPredictionValidation(t *testing.T) {
	budgetID := uuid.New()
	txn := model.Transaction{ID: uuid.New()}
	prediction := model.CipherPredictionResult{PayeeID: uuid.New(), CategoryID: uuid.New()}

	tests := []struct {
		name      string
		input     model.CreateCipherPredictionInput
		wantCode  errs.Code
		wantCalls int
	}{
		{
			name: "missing budget id",
			input: model.CreateCipherPredictionInput{
				Transactions: []model.Transaction{txn},
				Predictions:  []model.CipherPredictionResult{prediction},
			},
			wantCode: errs.CodeInvalidArgument,
		},
		{
			name:      "no transactions",
			input:     model.CreateCipherPredictionInput{BudgetID: budgetID},
			wantCalls: 0,
		},
		{
			name:     "mismatched input lengths",
			input:    model.CreateCipherPredictionInput{BudgetID: budgetID, Transactions: []model.Transaction{txn}},
			wantCode: errs.CodeInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			activity := CreateCipherPredictionActivity{
				PredictionService: &fakePredictionService{
					createCipherPrediction: func(context.Context, model.CipherPredictionRecord) (*model.CipherPredictionRecord, error) {
						calls++
						return nil, nil
					},
				},
			}

			err := executeCreateCipherPredictionActivity(t, activity, tt.input)
			if tt.wantCode != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				assertErrorCode(t, err, tt.wantCode)
			} else if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if calls != tt.wantCalls {
				t.Fatalf("expected %d service calls, got %d", tt.wantCalls, calls)
			}
		})
	}
}

func TestCreateCipherPredictionCreatesRecords(t *testing.T) {
	budgetID := uuid.New()
	txnID := uuid.New()
	payeeID := uuid.New()
	categoryID := uuid.New()
	rawText := "bank email text"
	amount := 42.75

	activity := CreateCipherPredictionActivity{
		PredictionService: &fakePredictionService{
			createCipherPrediction: func(ctx context.Context, record model.CipherPredictionRecord) (*model.CipherPredictionRecord, error) {
				ctxBudgetID, err := utils.BudgetIDFromContext(ctx)
				if err != nil {
					t.Fatalf("expected budget id in context: %v", err)
				}
				if ctxBudgetID != budgetID {
					t.Fatalf("expected context budget id %s, got %s", budgetID, ctxBudgetID)
				}
				if record.BudgetID != budgetID || record.TransactionID != txnID {
					t.Fatalf("unexpected record ids: %+v", record)
				}
				if record.EmailText == nil || *record.EmailText != rawText {
					t.Fatalf("expected raw text %q, got %v", rawText, record.EmailText)
				}
				if record.AccountConfidence == nil || math.Abs(*record.AccountConfidence-100) > 0.0001 {
					t.Fatalf("expected account confidence 100, got %v", valueOrNil(record.AccountConfidence))
				}
				if record.PayeeConfidence == nil || math.Abs(*record.PayeeConfidence-92.5) > 0.0001 {
					t.Fatalf("expected payee confidence 92.5, got %v", valueOrNil(record.PayeeConfidence))
				}
				if record.CategoryConfidence == nil || math.Abs(*record.CategoryConfidence-92.5) > 0.0001 {
					t.Fatalf("expected category confidence 92.5, got %v", valueOrNil(record.CategoryConfidence))
				}
				if record.ExtractedAccount == nil || *record.ExtractedAccount != "Checking" {
					t.Fatalf("expected extracted account, got %v", record.ExtractedAccount)
				}
				if record.ExtractedMerchant == nil || *record.ExtractedMerchant != "Merchant" {
					t.Fatalf("expected extracted merchant, got %v", record.ExtractedMerchant)
				}
				if record.PredictedPayeeID == nil || *record.PredictedPayeeID != payeeID {
					t.Fatalf("expected payee id %s, got %v", payeeID, record.PredictedPayeeID)
				}
				if record.PredictedCategoryID == nil || *record.PredictedCategoryID != categoryID {
					t.Fatalf("expected category id %s, got %v", categoryID, record.PredictedCategoryID)
				}
				if record.Amount == nil || *record.Amount != amount {
					t.Fatalf("expected amount %.2f, got %v", amount, record.Amount)
				}
				if record.Source != model.PredictionSourceLLM {
					t.Fatalf("expected source %s, got %s", model.PredictionSourceLLM, record.Source)
				}
				return &record, nil
			},
		},
	}

	err := executeCreateCipherPredictionActivity(t, activity, model.CreateCipherPredictionInput{
		BudgetID: budgetID,
		Transactions: []model.Transaction{{
			ID:          txnID,
			RawBankText: &rawText,
		}},
		Predictions: []model.CipherPredictionResult{{
			OriginalRawText: rawText,
			Account:         "Checking",
			Payee:           "Merchant",
			PayeeID:         payeeID,
			CategoryID:      categoryID,
			Amount:          amount,
			Confidence:      "92.50%",
			Source:          model.PredictionSourceLLM,
		}},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestCreateCipherPredictionOmitsNilPredictionIDs(t *testing.T) {
	activity := CreateCipherPredictionActivity{
		PredictionService: &fakePredictionService{createCipherPrediction: func(_ context.Context, record model.CipherPredictionRecord) (*model.CipherPredictionRecord, error) {
			if record.PredictedPayeeID != nil {
				t.Fatalf("expected nil predicted payee id, got %v", record.PredictedPayeeID)
			}
			if record.PredictedCategoryID != nil {
				t.Fatalf("expected nil predicted category id, got %v", record.PredictedCategoryID)
			}
			return &record, nil
		}},
	}

	err := executeCreateCipherPredictionActivity(t, activity, model.CreateCipherPredictionInput{
		BudgetID:     uuid.New(),
		Transactions: []model.Transaction{{ID: uuid.New()}},
		Predictions:  []model.CipherPredictionResult{{}},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestCreateCipherPredictionUsesCreatedTransactionIDsWhenPredictionIDsNil(t *testing.T) {
	payeeID := uuid.New()
	categoryID := uuid.New()
	activity := CreateCipherPredictionActivity{
		PredictionService: &fakePredictionService{createCipherPrediction: func(_ context.Context, record model.CipherPredictionRecord) (*model.CipherPredictionRecord, error) {
			if record.PredictedPayeeID == nil || *record.PredictedPayeeID != payeeID {
				t.Fatalf("expected predicted payee id %s, got %v", payeeID, record.PredictedPayeeID)
			}
			if record.PredictedCategoryID == nil || *record.PredictedCategoryID != categoryID {
				t.Fatalf("expected predicted category id %s, got %v", categoryID, record.PredictedCategoryID)
			}
			return &record, nil
		}},
	}

	err := executeCreateCipherPredictionActivity(t, activity, model.CreateCipherPredictionInput{
		BudgetID: uuid.New(),
		Transactions: []model.Transaction{{
			ID:         uuid.New(),
			PayeeID:    &payeeID,
			CategoryID: &categoryID,
		}},
		Predictions: []model.CipherPredictionResult{{}},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestCreateCipherPredictionReturnsServiceError(t *testing.T) {
	wantErr := errors.New("create cipher prediction failed")
	activity := CreateCipherPredictionActivity{
		PredictionService: &fakePredictionService{
			createCipherPrediction: func(context.Context, model.CipherPredictionRecord) (*model.CipherPredictionRecord, error) {
				return nil, wantErr
			},
		},
	}

	err := executeCreateCipherPredictionActivity(t, activity, model.CreateCipherPredictionInput{
		BudgetID:     uuid.New(),
		Transactions: []model.Transaction{{ID: uuid.New()}},
		Predictions:  []model.CipherPredictionResult{{PayeeID: uuid.New(), CategoryID: uuid.New()}},
	})
	if err == nil || !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("expected service error %v, got %v", wantErr, err)
	}
}

func TestCreateTransactionValidation(t *testing.T) {
	prediction := model.CipherPredictionResult{
		AccountID:  uuid.New(),
		PayeeID:    uuid.New(),
		Payee:      "Merchant",
		CategoryID: uuid.New(),
	}

	tests := []struct {
		name      string
		input     model.PredictionResultInput
		wantCode  errs.Code
		wantTxns  int
		wantCalls int
	}{
		{
			name:      "no predictions",
			input:     model.PredictionResultInput{BudgetID: uuid.New()},
			wantTxns:  0,
			wantCalls: 0,
		},
		{
			name:     "missing budget id",
			input:    model.PredictionResultInput{Predictions: []model.CipherPredictionResult{prediction}},
			wantCode: errs.CodeInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			activity := CreateTransactionActivity{
				TransactionService: &fakeTransactionService{
					create: func(context.Context, model.Transaction) ([]model.Transaction, error) {
						calls++
						return nil, nil
					},
				},
				PayeeService: &fakePayeeService{},
			}

			got, err := executeCreateTransactionActivity(t, activity, tt.input)
			if tt.wantCode != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				assertErrorCode(t, err, tt.wantCode)
			} else if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if len(got) != tt.wantTxns {
				t.Fatalf("expected %d transactions, got %d", tt.wantTxns, len(got))
			}
			if calls != tt.wantCalls {
				t.Fatalf("expected %d transaction calls, got %d", tt.wantCalls, calls)
			}
		})
	}
}

func TestCreateTransactionCreatesTransactionWithExistingPayee(t *testing.T) {
	budgetID := uuid.New()
	accountID := uuid.New()
	payeeID := uuid.New()
	categoryID := uuid.New()
	rawText := "raw bank email"
	amount := 19.99
	date := "2026-04-27"
	createdTxnID := uuid.New()
	payeeCalls := 0

	activity := CreateTransactionActivity{
		TransactionService: &fakeTransactionService{
			create: func(ctx context.Context, txn model.Transaction) ([]model.Transaction, error) {
				ctxBudgetID, err := utils.BudgetIDFromContext(ctx)
				if err != nil {
					t.Fatalf("expected budget id in context: %v", err)
				}
				if ctxBudgetID != budgetID {
					t.Fatalf("expected context budget id %s, got %s", budgetID, ctxBudgetID)
				}
				if txn.BudgetID != budgetID {
					t.Fatalf("expected transaction budget id %s, got %s", budgetID, txn.BudgetID)
				}
				if txn.AccountID == nil || *txn.AccountID != accountID {
					t.Fatalf("expected account id %s, got %v", accountID, txn.AccountID)
				}
				if txn.PayeeID == nil || *txn.PayeeID != payeeID {
					t.Fatalf("expected payee id %s, got %v", payeeID, txn.PayeeID)
				}
				if txn.CategoryID == nil || *txn.CategoryID != categoryID {
					t.Fatalf("expected category id %s, got %v", categoryID, txn.CategoryID)
				}
				if txn.Amount != amount || txn.Date != model.Date(date) ||
					txn.Status != model.TransactionStatusUnapproved {
					t.Fatalf("unexpected transaction fields: %+v", txn)
				}
				if txn.RawBankText == nil || *txn.RawBankText != rawText {
					t.Fatalf("expected raw text %q, got %v", rawText, txn.RawBankText)
				}
				wantHash := utils.Hash(accountID.String() + date + fmt.Sprintf("%.2f", amount) + rawText)
				if txn.DedupeHash == nil || *txn.DedupeHash != wantHash {
					t.Fatalf("expected dedupe hash %s, got %v", wantHash, txn.DedupeHash)
				}

				txn.ID = createdTxnID
				return []model.Transaction{txn}, nil
			},
		},
		PayeeService: &fakePayeeService{create: func(context.Context, model.Payee) (*model.Payee, error) {
			payeeCalls++
			return nil, nil
		}},
	}

	got, err := executeCreateTransactionActivity(t, activity, model.PredictionResultInput{
		BudgetID: budgetID,
		Predictions: []model.CipherPredictionResult{{
			OriginalRawText: rawText,
			AccountID:       accountID,
			PayeeID:         payeeID,
			CategoryID:      categoryID,
			Payee:           "Merchant",
			Date:            date,
			Amount:          amount,
		}},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if payeeCalls != 0 {
		t.Fatalf("expected no payee creation calls, got %d", payeeCalls)
	}
	if len(got) != 1 || got[0].ID != createdTxnID {
		t.Fatalf("unexpected created transactions: %+v", got)
	}
}

func TestCreateTransactionKeepsExistingPayeeIDWhenPayeeNameEmpty(t *testing.T) {
	budgetID := uuid.New()
	payeeID := uuid.New()
	payeeCalls := 0

	activity := CreateTransactionActivity{
		TransactionService: &fakeTransactionService{create: func(_ context.Context, txn model.Transaction) ([]model.Transaction, error) {
			if txn.PayeeID == nil || *txn.PayeeID != payeeID {
				t.Fatalf("expected existing payee id %s, got %v", payeeID, txn.PayeeID)
			}
			return []model.Transaction{txn}, nil
		}},
		PayeeService: &fakePayeeService{create: func(context.Context, model.Payee) (*model.Payee, error) {
			payeeCalls++
			return &model.Payee{ID: uuid.New()}, nil
		}},
	}

	_, err := executeCreateTransactionActivity(t, activity, model.PredictionResultInput{
		BudgetID: budgetID,
		Predictions: []model.CipherPredictionResult{{
			OriginalRawText: "raw",
			AccountID:       uuid.New(),
			PayeeID:         payeeID,
			CategoryID:      uuid.New(),
			Date:            "2026-04-27",
			Amount:          1.23,
		}},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if payeeCalls != 0 {
		t.Fatalf("expected no payee creation calls, got %d", payeeCalls)
	}
}

func TestCreateTransactionCreatesMissingPayee(t *testing.T) {
	budgetID := uuid.New()
	newPayeeID := uuid.New()
	var createdPayee model.Payee

	activity := CreateTransactionActivity{
		PayeeService: &fakePayeeService{create: func(ctx context.Context, payee model.Payee) (*model.Payee, error) {
			ctxBudgetID, err := utils.BudgetIDFromContext(ctx)
			if err != nil {
				t.Fatalf("expected budget id in context: %v", err)
			}
			if ctxBudgetID != budgetID {
				t.Fatalf("expected context budget id %s, got %s", budgetID, ctxBudgetID)
			}
			createdPayee = payee
			return &model.Payee{ID: newPayeeID, Name: payee.Name}, nil
		}},
		TransactionService: &fakeTransactionService{
			create: func(_ context.Context, txn model.Transaction) ([]model.Transaction, error) {
				if txn.PayeeID == nil || *txn.PayeeID != newPayeeID {
					t.Fatalf("expected new payee id %s, got %v", newPayeeID, txn.PayeeID)
				}
				return []model.Transaction{txn}, nil
			},
		},
	}

	_, err := executeCreateTransactionActivity(t, activity, model.PredictionResultInput{
		BudgetID: budgetID,
		Predictions: []model.CipherPredictionResult{{
			OriginalRawText: "raw",
			AccountID:       uuid.New(),
			CategoryID:      uuid.New(),
			Date:            "2026-04-27",
			Amount:          12.34,
		}},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if createdPayee.Name != "Unknown Payee" {
		t.Fatalf("expected unknown payee name, got %q", createdPayee.Name)
	}
}

func TestCreateTransactionReturnsWrappedPayeeError(t *testing.T) {
	wantErr := errors.New("payee create failed")
	activity := CreateTransactionActivity{
		PayeeService: &fakePayeeService{create: func(context.Context, model.Payee) (*model.Payee, error) {
			return nil, wantErr
		}},
		TransactionService: &fakeTransactionService{},
	}

	_, err := executeCreateTransactionActivity(t, activity, model.PredictionResultInput{
		BudgetID: uuid.New(),
		Predictions: []model.CipherPredictionResult{{
			AccountID:  uuid.New(),
			CategoryID: uuid.New(),
			Date:       "2026-04-27",
		}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertErrorCode(t, err, errs.CodePayeeCreateFailed)
	if !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("expected wrapped payee error %v, got %v", wantErr, err)
	}
}

func TestCreateTransactionReturnsTransactionErrors(t *testing.T) {
	baseInput := model.PredictionResultInput{
		BudgetID: uuid.New(),
		Predictions: []model.CipherPredictionResult{{
			OriginalRawText: "raw",
			AccountID:       uuid.New(),
			PayeeID:         uuid.New(),
			Payee:           "Merchant",
			CategoryID:      uuid.New(),
			Date:            "2026-04-27",
			Amount:          1.23,
		}},
	}

	t.Run("service error", func(t *testing.T) {
		wantErr := errors.New("transaction create failed")
		activity := CreateTransactionActivity{
			TransactionService: &fakeTransactionService{
				create: func(context.Context, model.Transaction) ([]model.Transaction, error) {
					return nil, wantErr
				},
			},
			PayeeService: &fakePayeeService{},
		}

		_, err := executeCreateTransactionActivity(t, activity, baseInput)
		if err == nil || !strings.Contains(err.Error(), wantErr.Error()) {
			t.Fatalf("expected transaction service error %v, got %v", wantErr, err)
		}
	})

	t.Run("no created transaction", func(t *testing.T) {
		activity := CreateTransactionActivity{
			TransactionService: &fakeTransactionService{
				create: func(context.Context, model.Transaction) ([]model.Transaction, error) {
					return nil, nil
				},
			},
			PayeeService: &fakePayeeService{},
		}

		_, err := executeCreateTransactionActivity(t, activity, baseInput)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		assertErrorCode(t, err, errs.CodeTransactionNotCreated)
	})
}
