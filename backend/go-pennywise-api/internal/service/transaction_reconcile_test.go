package service

import (
	"context"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestReconcileTransfer(t *testing.T) {
	ctx := context.Background()
	var mockTx pgx.Tx

	budgetId := uuid.New()
	txnId := uuid.New()
	oldTransferTxnId := uuid.New()
	newTransferTxnId := uuid.New()
	oldPayeeId := uuid.New()
	newPayeeId := uuid.New()
	transferAccountId := uuid.New()

	tests := []struct {
		name          string
		wasTransfer   bool
		isTransfer    bool
		samePayee     bool
		setupMocks    func(*mockTransactionRepo)
		expectedError bool
	}{
		{
			name:        "wasTransfer_notTransfer_success",
			wasTransfer: true,
			isTransfer:  false,
			setupMocks: func(repo *mockTransactionRepo) {
				repo.On("DeleteById", ctx, mockTx, budgetId, oldTransferTxnId).Return(nil).Once()
			},
			expectedError: false,
		},
		{
			name:        "wasTransfer_notTransfer_delete_error",
			wasTransfer: true,
			isTransfer:  false,
			setupMocks: func(repo *mockTransactionRepo) {
				repo.On("DeleteById", ctx, mockTx, budgetId, oldTransferTxnId).Return(assert.AnError).Once()
			},
			expectedError: true,
		},
		{
			name:        "notTransfer_isTransfer_success",
			wasTransfer: false,
			isTransfer:  true,
			setupMocks: func(repo *mockTransactionRepo) {
				// Mock Create for counterpart
				repo.On("Create", ctx, mockTx, mock.Anything).Return([]model.Transaction{{ID: newTransferTxnId}}, nil).Once()
			},
			expectedError: false,
		},
		{
			name:        "notTransfer_isTransfer_create_error",
			wasTransfer: false,
			isTransfer:  true,
			setupMocks: func(repo *mockTransactionRepo) {
				// Mock Create for counterpart
				repo.On("Create", ctx, mockTx, mock.Anything).Return(nil, assert.AnError).Once()
			},
			expectedError: true,
		},
		{
			name:        "wasTransfer_isTransfer_differentPayee_success",
			wasTransfer: true,
			isTransfer:  true,
			samePayee:   false,
			setupMocks: func(repo *mockTransactionRepo) {
				repo.On("DeleteById", ctx, mockTx, budgetId, oldTransferTxnId).Return(nil).Once()
				repo.On("Create", ctx, mockTx, mock.Anything).Return([]model.Transaction{{ID: newTransferTxnId}}, nil).Once()
			},
			expectedError: false,
		},
		{
			name:        "wasTransfer_isTransfer_differentPayee_delete_error",
			wasTransfer: true,
			isTransfer:  true,
			samePayee:   false,
			setupMocks: func(repo *mockTransactionRepo) {
				repo.On("DeleteById", ctx, mockTx, budgetId, oldTransferTxnId).Return(assert.AnError).Once()
			},
			expectedError: true,
		},
		{
			name:        "wasTransfer_isTransfer_differentPayee_create_error",
			wasTransfer: true,
			isTransfer:  true,
			samePayee:   false,
			setupMocks: func(repo *mockTransactionRepo) {
				repo.On("DeleteById", ctx, mockTx, budgetId, oldTransferTxnId).Return(nil).Once()
				repo.On("Create", ctx, mockTx, mock.Anything).Return(nil, assert.AnError).Once()
			},
			expectedError: true,
		},
		{
			name:        "wasTransfer_isTransfer_samePayee_success",
			wasTransfer: true,
			isTransfer:  true,
			samePayee:   true,
			setupMocks: func(repo *mockTransactionRepo) {
				repo.On("Update", ctx, mockTx, budgetId, oldTransferTxnId, mock.Anything).Return(nil).Once()
			},
			expectedError: false,
		},
		{
			name:        "wasTransfer_isTransfer_samePayee_update_error",
			wasTransfer: true,
			isTransfer:  true,
			samePayee:   true,
			setupMocks: func(repo *mockTransactionRepo) {
				repo.On("Update", ctx, mockTx, budgetId, oldTransferTxnId, mock.Anything).Return(assert.AnError).Once()
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockTransactionRepo{}
			service := newTestTransactionService(mockRepo, nil, nil, nil, nil, nil, nil)

			tt.setupMocks(mockRepo)

			var oldTxn model.Transaction
			var newTxn model.Transaction
			var account model.Account
			var payee model.Payee

			oldTxn.ID = txnId
			if tt.wasTransfer {
				oldTxn.TransferTransactionID = &oldTransferTxnId
				oldTxn.TransferAccountID = &transferAccountId
			}
			if tt.samePayee {
				oldTxn.PayeeID = &oldPayeeId
				newTxn.PayeeID = &oldPayeeId
			} else {
				oldTxn.PayeeID = &oldPayeeId
				newTxn.PayeeID = &newPayeeId
			}

			if tt.isTransfer {
				payee.TransferAccountID = &transferAccountId
			}

			newTxn.Amount = 100 // for counterpart amount
			newTxn.Date = "2023-11-11"

			err := service.reconcileTransfer(ctx, mockTx, budgetId, oldTxn, &newTxn, account, payee)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.wasTransfer && !tt.isTransfer {
					assert.Nil(t, newTxn.TransferAccountID)
					assert.Nil(t, newTxn.TransferTransactionID)
					// assert.Nil(t, transferTxnId)
				}
				if tt.isTransfer {
					// assert.NotNil(t, transferTxnId)
					assert.Equal(t, transferAccountId, *newTxn.TransferAccountID)
				}
			}

			mockRepo.AssertExpectations(t)
		})
	}
}
