package model

import "testing"

func TestTransactionStatusCanMoveTo(t *testing.T) {
	all := []TransactionStatus{
		TransactionStatusManual, TransactionStatusUnapproved, TransactionStatusApproved, TransactionStatusRejected,
	}
	allowed := map[[2]TransactionStatus]bool{
		{TransactionStatusUnapproved, TransactionStatusApproved}: true,
		{TransactionStatusUnapproved, TransactionStatusRejected}: true,
		{TransactionStatusRejected, TransactionStatusUnapproved}: true,
	}
	for _, from := range all {
		for _, to := range all {
			want := allowed[[2]TransactionStatus{from, to}]
			if got := from.CanMoveTo(to); got != want {
				t.Errorf("%s -> %s: CanMoveTo = %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestTransactionStatusValid(t *testing.T) {
	for _, s := range []TransactionStatus{
		TransactionStatusManual, TransactionStatusUnapproved, TransactionStatusApproved, TransactionStatusRejected,
	} {
		if !s.Valid() {
			t.Errorf("%s should be valid", s)
		}
	}
	for _, s := range []TransactionStatus{"", "approved", "MAYBE"} {
		if s.Valid() {
			t.Errorf("%q should be invalid", s)
		}
	}
}
