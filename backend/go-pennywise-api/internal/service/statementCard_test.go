package service

import (
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Made-up card statement lines.

func classified(desc string, dir model.StatementDirection, card bool) model.StatementRow {
	r := row("", "2026-09-01", desc, 100, dir)
	readNarration(&r, card)
	classifyStatementRow(&r, card)
	return r
}

func TestClassifyCardRows(t *testing.T) {
	cases := []struct {
		desc   string
		dir    model.StatementDirection
		charge string
		emi    string
		number int
	}{
		{"EXAMPLE STORE MUMBAI", model.StatementDebit, "", "", 0},
		{"IGST @18% ON FEE", model.StatementDebit, model.StatementChargeTax, "", 0},
		{"IGST ON EMI INTEREST", model.StatementDebit, model.StatementChargeTax, "", 0},
		{"ANNUAL MEMBERSHIP FEE", model.StatementDebit, model.StatementChargeFee, "", 0},
		{"LATE PAYMENT FEE REVERSAL", model.StatementCredit, model.StatementChargeFee, "", 0},
		{"FINANCE CHARGES", model.StatementDebit, model.StatementChargeInterest, "", 0},
		{"EMI PRINCIPAL 3/6 EXAMPLE PHONE", model.StatementDebit, "", emiPartPrincipal, 3},
		{"EMI INTEREST 3 OF 6", model.StatementDebit, model.StatementChargeInterest, emiPartInterest, 3},
		{"AMOUNT CONVERTED TO EMI", model.StatementCredit, "", emiPartConversion, 0},
		{"EMI 2/12", model.StatementDebit, "", emiPartInstalment, 2},
	}
	for _, c := range cases {
		r := classified(c.desc, c.dir, true)
		assert.Equal(t, c.charge, r.ChargeKind, c.desc)
		if c.emi == "" {
			assert.Nil(t, r.EMI, c.desc)
			continue
		}
		require.NotNil(t, r.EMI, c.desc)
		assert.Equal(t, c.emi, r.EMI.Part, c.desc)
		assert.Equal(t, c.number, r.EMI.Number, c.desc)
	}
}

func TestClassifyBankRows(t *testing.T) {
	// A loan EMI paid from a bank account is a real payment, and interest
	// paid to the account is income.
	assert.Nil(t, classified("EMI 2/12 HOME LOAN", model.StatementDebit, false).EMI)
	assert.Empty(t, classified("INTEREST PAID TILL 30-SEP-2026", model.StatementCredit, false).ChargeKind)
	assert.Equal(t, model.StatementChargeFee, classified("SMS CHARGES", model.StatementDebit, false).ChargeKind)
	// A UPI payment to a merchant called "Charges" is still a payment.
	assert.Empty(t, classified("UPI-EV CHARGES-ev@ybl-YESB0YBLUPI-123412341234-Pay", model.StatementDebit, false).ChargeKind)
}

func TestStatementChargesAndCovered(t *testing.T) {
	rows := []model.StatementRow{
		classified("ANNUAL FEE", model.StatementDebit, true),
		classified("IGST ON ANNUAL FEE", model.StatementDebit, true),
		classified("ANNUAL FEE REVERSAL", model.StatementCredit, true),
		classified("EMI 1/3", model.StatementDebit, true),
		classified("EMI INTEREST 1/3", model.StatementDebit, true),
	}
	totals := statementCharges(rows)
	byKind := map[string]model.StatementChargeTotal{}
	for _, c := range totals {
		byKind[c.Kind] = c
	}
	assert.Equal(t, 0.0, byKind[model.StatementChargeFee].Amount, "fee and its reversal cancel")
	assert.Equal(t, 2, byKind[model.StatementChargeFee].Count)
	assert.Equal(t, 100.0, byKind[model.StatementChargeTax].Amount)
	assert.Equal(t, 100.0, byKind[model.StatementChargeInterest].Amount)

	assert.True(t, coveredByEMI(rows[3]))
	assert.False(t, coveredByEMI(rows[4]), "EMI interest is a cost")
}

func TestChargeCategory(t *testing.T) {
	fees, tax, groceries := uuid.New(), uuid.New(), uuid.New()
	cats := map[uuid.UUID]string{fees: "Bank Charges", tax: "Taxes", groceries: "Groceries"}
	assert.Equal(t, &tax, chargeCategory(model.StatementChargeTax, cats))
	assert.Equal(t, &fees, chargeCategory(model.StatementChargeFee, cats))
	assert.Equal(t, &fees, chargeCategory(model.StatementChargeInterest, cats))
	assert.Nil(t, chargeCategory(model.StatementChargeFee, map[uuid.UUID]string{groceries: "Groceries"}))
}
