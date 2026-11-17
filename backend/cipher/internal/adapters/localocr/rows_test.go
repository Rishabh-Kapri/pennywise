package localocr

import (
	"encoding/json"
	"strings"
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// All statement text here is made up.

func TestLayoutsKeepColumnOrder(t *testing.T) {
	for _, l := range []layout{bankLayout, cardLayout} {
		var schema map[string]any
		require.NoError(t, json.Unmarshal(l.schema, &schema), "the schema is JSON")
	}
	// glm-ocr fills fields in the order it's shown them.
	assert.Contains(t, cardLayout.prompt, `{"date": "", "narration": "", "rewards": "", "amount": ""}`)
	assert.Less(t, strings.Index(string(bankLayout.schema), `"amount"`), strings.Index(string(bankLayout.schema), `"balance"`))
}

func TestBankDirectionFromBalance(t *testing.T) {
	rows := bankRows([]pageRow{
		{Date: "01/04/2026", Narration: "OPENING SALARY", Amount: "100.00", Balance: "1,100.00", page: 1},
		{Date: "02/04/2026", Narration: "UPI ACME STORE", Amount: "40.00", Balance: "1,060.00", page: 1},
		{Date: "03/04/26", Narration: "NEFT CR REFUND", Amount: "10.00CR.", Balance: "1,070.00CR.", page: 2},
		{Date: "", Narration: "carried forward", Amount: "1,070.00"}, // no date: not a row
	})
	require.Len(t, rows, 3)
	assert.Equal(t, sharedModel.StatementDebit, rows[0].Direction, "no balance before the first row")
	assert.Equal(t, sharedModel.StatementDebit, rows[1].Direction)
	assert.Equal(t, sharedModel.StatementCredit, rows[2].Direction, "balance went up")
	assert.Equal(t, "2026-04-03", rows[2].Date)
	assert.Equal(t, 1070.0, *rows[2].Balance)
	assert.Equal(t, 2, rows[2].Page)
}

func TestCardDirectionFromPointsAndSign(t *testing.T) {
	rows := cardRows([]pageRow{
		{Date: "03/10/2026| 09:18", Narration: "ACME TRAVEL", Rewards: "+ 70", Amount: "₹ 2,932.00"},
		{Date: "03/10/2026", Narration: "Refund ACME TRAVEL", Rewards: "- 60", Amount: "₹ 2,403.95"}, // "+" dropped
		{Date: "15/08/2026", Narration: "CASHBACK", Rewards: "", Amount: "+ ₹ 43.70"},
		{Date: "15/03/2026", Narration: "CC PAYMENT", Rewards: "+ 3,84993.28", Amount: "3,84993.28"},
		// a card without points: the amount lands in rewards
		{Date: "06/08/2026", Narration: "CC PAYMENT", Rewards: "+ ₹ 95,180.00", Amount: ""},
		{Date: "06/08/2026", Narration: "ACME INSURANCE", Rewards: "₹ 12,296.87", Amount: ""},
	})
	require.Len(t, rows, 6)
	want := []sharedModel.StatementDirection{
		sharedModel.StatementDebit, sharedModel.StatementCredit, sharedModel.StatementCredit,
		sharedModel.StatementCredit, sharedModel.StatementCredit, sharedModel.StatementDebit,
	}
	for i, w := range want {
		assert.Equal(t, w, rows[i].Direction, rows[i].Description)
	}
	assert.Equal(t, 70, *rows[0].RewardPoints)
	assert.Equal(t, "09:18", rows[0].Time)
	assert.Equal(t, "", rows[1].Time)
	assert.Equal(t, -60, *rows[1].RewardPoints)
	assert.Nil(t, rows[3].RewardPoints, "the amount repeated isn't points")
	assert.Equal(t, 384993.28, rows[3].Amount)
	assert.Equal(t, 95180.0, rows[4].Amount)
}

func TestParseDate(t *testing.T) {
	for in, want := range map[string]string{
		"06/08/2026| 20:49": "2026-08-06",
		"28/05/26":          "2026-05-28",
		"04-Oct-2026":       "2026-10-04",
		"13 Sep, 2026":      "2026-09-13",
	} {
		got, ok := parseDate(in)
		assert.True(t, ok, in)
		assert.Equal(t, want, got, in)
	}
	_, ok := parseDate("Opening balance")
	assert.False(t, ok)
}
