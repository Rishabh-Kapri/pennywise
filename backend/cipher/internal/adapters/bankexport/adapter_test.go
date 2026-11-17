package bankexport

import (
	"context"
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Made up, in the shape of HDFC's "Delimited" export: padded fields, a
// preamble, 0.00 for the empty amount column.
const hdfcCSV = `
Date     ,Narration                                  ,Value Dat,Debit Amount       ,Credit Amount      ,Chq/Ref Number   ,Closing Balance
 01/04/26  ,UPI-ACME STORE-ACME@OKBANK-PAYMENT TO ACME  ,01/04/26 ,       780.00     ,           0.00     ,0000398085330460       ,      9220.00
 02/04/26  ,NEFT CR-ACME CORP-SALARY                    ,02/04/26 ,         0.00     ,       1000.50     ,N0000000000001         ,     10220.50
 03/04/26  ,ACH D- ACME LOAN                            ,03/04/26 ,       220.50     ,           0.00     ,0000008648910248       ,     10000.00
`

func TestHDFCDelimited(t *testing.T) {
	res, err := New().ExtractStatement(context.Background(), []byte(hdfcCSV), nil)
	require.NoError(t, err)
	require.Len(t, res.Sections, 1)
	s := res.Sections[0]
	require.Len(t, s.Rows, 3)

	assert.Equal(t, sharedModel.StatementRow{
		Date: "2026-04-01", Description: "UPI-ACME STORE-ACME@OKBANK-PAYMENT TO ACME",
		Amount: 780, Direction: sharedModel.StatementDebit, Balance: s.Rows[0].Balance, Reference: "0000398085330460",
		Line: 2,
	}, s.Rows[0])
	assert.Equal(t, 4, s.Rows[2].Line, "records, with the empty first line skipped like encoding/csv does")
	assert.Equal(t, 9220.0, *s.Rows[0].Balance)
	assert.Equal(t, sharedModel.StatementCredit, s.Rows[1].Direction)
	assert.Equal(t, 1000.5, s.Rows[1].Amount)
	assert.Equal(t, sharedModel.StatementDebit, s.Rows[2].Direction)

	assert.Equal(t, 10000.0, *s.OpeningBalance)
	assert.Equal(t, 10000.0, *s.ClosingBalance)
	assert.Equal(t, "2026-04-01", *s.PeriodStart)
	assert.Equal(t, "2026-04-03", *s.PeriodEnd)
}

func TestSignedAmountColumn(t *testing.T) {
	csv := "Transaction Date,Description,Amount\n2026-04-01,ACME STORE,-12.50\n2026-04-02,REFUND,3\nTotal,,-9.50\n"
	res, err := New().ExtractStatement(context.Background(), []byte(csv), nil)
	require.NoError(t, err)
	rows := res.Sections[0].Rows
	require.Len(t, rows, 2, "the totals line has no date")
	assert.Equal(t, sharedModel.StatementDebit, rows[0].Direction)
	assert.Equal(t, 12.5, rows[0].Amount)
	assert.Equal(t, sharedModel.StatementCredit, rows[1].Direction)
	assert.Nil(t, res.Sections[0].OpeningBalance, "no balance column")
}

func TestRejects(t *testing.T) {
	_, err := New().ExtractStatement(context.Background(), []byte("%PDF-1.7\n..."), nil)
	assert.ErrorContains(t, err, "is a PDF")
	_, err = New().ExtractStatement(context.Background(), []byte("a,b,c\n1,2,3\n"), nil)
	assert.ErrorContains(t, err, "no header row")
}

// Made up, in the shape of HDFC's credit card export.
const hdfcCardExport = `Name~|~A CUSTOMER 
Payment Due Date~|~23/10/2026 
Statement Date~|~03/10/2026 
Total Amount Due~|~1,500.00 
Minimum Amount Due~|~200.00 
Credit Limit~|~5,00,000.00 

Account Summary 
Opening Bal~|~-~|~Payment / Credit~|~+~|~Purchases / Debits~|~+~|~Finance Charges~|~=~|~Total Dues 
1,000.20~|~-~|~1,000.20~|~+~|~1,500.40~|~+~|~0.00~|~=~|~1,500.00 

Card No: XXXX XXXX XXXX 1234 

Past Dues (if any) 
Overlimit~|~3 Months~|~2 Months~|~1 Month~|~ Current Dues~|~Minimum Amount Due 
0.00~|~0.00~|~0.00~|~0.00~|~200.00~|~200.00 

Domestic / International Transactions
Transaction type~|~Primary / Addon Customer Name~|~DATE~|~Description~|~AMT~|~Debit /Credit~|~REWARDS~|~
Domestic~|~A CUSTOMER ~|~03/09/2026 16:45:49~|~ACME SOFTWARE ~|~1,196.52~|~~|~+ 25~|~
Domestic~|~A CUSTOMER ~|~05/09/2026 00:00:00~|~IGST-VPS000 RATE 18.0 ~|~303.88~|~~|~~|~
Domestic~|~A CUSTOMER ~|~10/09/2026 10:00:00~|~CC PAYMENT ~|~1,000.20~|~Cr~|~~|~
Reward Points Summary
Opening Balance~|~Feature + Bonus Reward Points Earned~|~Disbursed~|~Adjusted/Lapsed~|~Closing Balance~|~Points expiring in next 30 days~|~Points expiring in next 60 days 
5,337~|~25~|~0~|~0~|~5,362~|~0~|~0 

Loan Summary
Loan Type~|~Loan Number~|~Loan Booked Date~|~Loan Amount~|~Tenure~|~Rate of Interest~|~Balance Principal outstanding~|~Balance Interest Payable~|~Balance Tenure 
Smart EMI~|~100000001~|~11/02/2026~|~54,403.18~|~9 Months~|~17%~|~6,392.18~|~91.00~|~1 Month 

*Pre-closure charges as applicable. 
`

func TestHDFCCardExport(t *testing.T) {
	res, err := New().ExtractStatement(context.Background(), []byte(hdfcCardExport), nil)
	require.NoError(t, err)
	require.Len(t, res.Sections, 1)
	s := res.Sections[0]

	assert.Equal(t, sharedModel.StatementKindCard, s.Kind)
	assert.Equal(t, "XXXX XXXX XXXX 1234", s.AccountNumber)
	assert.Equal(t, "2026-10-23", *s.DueDate)
	assert.Equal(t, "2026-10-03", *s.StatementDate)
	assert.Equal(t, 1500.0, *s.TotalDue)
	assert.Equal(t, 200.0, *s.MinimumDue)
	assert.Equal(t, 500000.0, *s.CreditLimit)
	assert.Equal(t, 1000.2, *s.OpeningBalance)
	assert.Equal(t, 1500.4, *s.ClosingBalance, "computed: the printed total is rounded")

	require.Len(t, s.Rows, 3, "the EMI table after the rows isn't rows")
	assert.Equal(t, "2026-09-03", s.Rows[0].Date)
	assert.Equal(t, "16:45", s.Rows[0].Time)
	assert.Equal(t, "", s.Rows[1].Time, "00:00:00 is the export's stamp for no time")
	assert.Equal(t, "10:00", s.Rows[2].Time)
	assert.Equal(t, 20, s.Rows[0].Line, "every line counts in a ~|~ export, empty ones too")
	assert.Equal(t, 22, s.Rows[2].Line)
	assert.Equal(t, "ACME SOFTWARE", s.Rows[0].Description)
	assert.Equal(t, 1196.52, s.Rows[0].Amount)
	assert.Equal(t, sharedModel.StatementDebit, s.Rows[0].Direction)
	assert.Equal(t, 25, *s.Rows[0].RewardPoints)
	assert.Nil(t, s.Rows[1].RewardPoints)
	assert.Equal(t, sharedModel.StatementCredit, s.Rows[2].Direction)

	// the rows add up: opening - credits + debits = closing
	net := 0.0
	for _, r := range s.Rows {
		net += r.SignedAmount()
	}
	assert.InDelta(t, *s.ClosingBalance, *s.OpeningBalance-net, 0.001)

	assert.Equal(t, 5337, *s.Rewards.Opening)
	assert.Equal(t, 25, *s.Rewards.Earned)
	assert.Equal(t, 5362, *s.Rewards.Closing)

	require.Len(t, s.EMIPlans, 1)
	p := s.EMIPlans[0]
	assert.Equal(t, "Smart EMI 100000001", p.Description)
	assert.Equal(t, "2026-02-11", p.BookedOn)
	assert.Equal(t, 54403.18, *p.Principal)
	assert.Equal(t, 17.0, *p.InterestRate)
	assert.Equal(t, 9, *p.TenureMonths)
	assert.Equal(t, 1, *p.RemainingMonths)
	assert.Equal(t, 6392.18, *p.Outstanding)
}
