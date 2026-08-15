package db

import (
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

// Rows written before transactions.status got its NOT NULL default (migration
// 00023) scan as NULL. Reading them into the non-pointer model field is what
// made every single-row lookup of such a transaction fail, so the fallback has
// to hold for each read path.
func TestScannedStatus(t *testing.T) {
	approved := model.TransactionStatusApproved
	empty := model.TransactionStatus("")

	tests := []struct {
		name string
		in   *model.TransactionStatus
		want model.TransactionStatus
	}{
		{"null legacy row", nil, model.TransactionStatusManual},
		{"empty value", &empty, model.TransactionStatusManual},
		{"stored status", &approved, model.TransactionStatusApproved},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scannedStatus(tt.in); got != tt.want {
				t.Errorf("scannedStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}
