package service

import (
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRawStatementExtractionLenientNumbers(t *testing.T) {
	body := "```json\n" + `{"sections": [{
		"accountLabel": " HDFC Savings ", "accountNumber": "XX1234", "currency": "inr",
		"periodStart": "", "periodEnd": "2026-08-31",
		"openingBalance": "1,00,000.50", "closingBalance": null,
		"rows": [
			{"date": "2026-08-02", "description": "UPI/SWIGGY/\n 1234", "payee": "Swiggy",
			 "amount": "1,234.50", "direction": "Debit", "balance": 98766.0, "reference": 991, "page": "2"},
			{"date": "2026-08-03", "description": "X", "payee": "", "amount": "n/a",
			 "direction": "credit", "balance": null, "reference": null, "page": 2}
		]}]}` + "\n```"

	parsed, err := utils.UnmarshalResponse[rawStatementExtraction]([]byte(body))
	require.NoError(t, err)
	res := parsed.toModel()

	require.Len(t, res.Sections, 1)
	s := res.Sections[0]
	assert.Equal(t, "HDFC Savings", s.AccountLabel)
	assert.Equal(t, "INR", s.Currency)
	assert.Nil(t, s.PeriodStart)
	assert.Equal(t, "2026-08-31", *s.PeriodEnd)
	assert.Equal(t, 100000.50, *s.OpeningBalance)
	assert.Nil(t, s.ClosingBalance)

	require.Len(t, s.Rows, 2)
	assert.Equal(t, 1234.50, s.Rows[0].Amount)
	assert.Equal(t, sharedModel.StatementDebit, s.Rows[0].Direction)
	assert.Equal(t, "UPI/SWIGGY/ 1234", s.Rows[0].Description)
	assert.Equal(t, "991", s.Rows[0].Reference)
	assert.Equal(t, 2, s.Rows[0].Page)
	// unparseable amount decodes as 0 for the API to drop and count
	assert.Equal(t, 0.0, s.Rows[1].Amount)
}
