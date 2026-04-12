package service

import (
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestValidateTransactionPayload(t *testing.T) {
	service := &transactionService{}
	validAccountID := uuid.New()
	validPayeeID := uuid.New()

	tests := []struct {
		name    string
		txn     model.Transaction
		wantErr bool
	}{
		{
			name: "missing_account_id",
			txn: model.Transaction{
				AccountID: nil,
			},
			wantErr: true,
		},
		{
			name: "missing_payee_id",
			txn: model.Transaction{
				AccountID: &validAccountID,
				PayeeID:   nil,
			},
			wantErr: true,
		},
		{
			name: "invalid_date",
			txn: model.Transaction{
				AccountID: &validAccountID,
				PayeeID:   &validPayeeID,
				Date:      "invalid-date",
			},
			wantErr: true,
		},
		{
			name: "zero_amount",
			txn: model.Transaction{
				AccountID: &validAccountID,
				PayeeID:   &validPayeeID,
				Date:      "2023-10-10",
				Amount:    0,
			},
			wantErr: true,
		},
		{
			name: "negative_amount",
			txn: model.Transaction{
				AccountID: &validAccountID,
				PayeeID:   &validPayeeID,
				Date:      "2023-10-10",
				Amount:    -50.0,
			},
			wantErr: true,
		},
		{
			name: "valid_payload",
			txn: model.Transaction{
				AccountID: &validAccountID,
				PayeeID:   &validPayeeID,
				Date:      "2023-10-10",
				Amount:    50.0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateTransactionPayload(tt.txn)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCategory(t *testing.T) {
	service := &transactionService{}
	transferAccountID := uuid.New()
	categoryID := uuid.New()

	tests := []struct {
		name       string
		categoryID *uuid.UUID
		account    model.Account
		payee      model.Payee
		wantErr    bool
	}{
		{
			name:       "not_a_transfer",
			categoryID: &categoryID,
			account:    model.Account{Type: "savings"},
			payee:      model.Payee{TransferAccountID: nil},
			wantErr:    false,
		},
		{
			name:       "transfer_without_category",
			categoryID: nil,
			account:    model.Account{Type: "savings"},
			payee:      model.Payee{TransferAccountID: &transferAccountID},
			wantErr:    false,
		},
		{
			name:       "transfer_with_category_savings",
			categoryID: &categoryID,
			account:    model.Account{Type: "savings"},
			payee:      model.Payee{TransferAccountID: &transferAccountID},
			wantErr:    true,
		},
		{
			name:       "transfer_with_category_checking",
			categoryID: &categoryID,
			account:    model.Account{Type: "checking"},
			payee:      model.Payee{TransferAccountID: &transferAccountID},
			wantErr:    true,
		},
		{
			name:       "transfer_with_category_creditCard",
			categoryID: &categoryID,
			account:    model.Account{Type: "creditCard"},
			payee:      model.Payee{TransferAccountID: &transferAccountID},
			wantErr:    true,
		},
		{
			name:       "transfer_with_category_other_account_type",
			categoryID: &categoryID,
			account:    model.Account{Type: "loan"},
			payee:      model.Payee{TransferAccountID: &transferAccountID},
			wantErr:    false, // Only savings, checking, creditCard are restricted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateCategory(tt.categoryID, tt.account, tt.payee)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
