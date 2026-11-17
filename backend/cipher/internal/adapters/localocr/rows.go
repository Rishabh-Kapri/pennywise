package localocr

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

// pageRow is one row as the OCR model filled the schema. Bank pages fill
// refNo and balance, card pages rewards.
type pageRow struct {
	Date      string `json:"date"`
	Narration string `json:"narration"`
	RefNo     string `json:"refNo"`
	Rewards   string `json:"rewards"`
	Amount    string `json:"amount"`
	Balance   string `json:"balance"`
	page      int
}

// layout is how one kind of statement is asked for and read.
type layout struct {
	prompt string
	schema json.RawMessage
	rows   func([]pageRow) []sharedModel.StatementRow
}

// The model fills fields in the order they're listed, so they follow the
// page's columns. Neither layout asks for withdrawal and deposit: the model
// can't see which column an amount sat in, and copies one row's guess onto
// the rest of the page.
var (
	// A bank row's direction comes from its balance (bankRows).
	bankLayout = newLayout(bankRows, "date", "narration", "refNo", "amount", "balance")
	// A card has no balance; points and the amount's "+" decide (cardRows).
	// Rewards is asked for even on cards without points: optional, the model
	// skips it where it's printed.
	cardLayout = newLayout(cardRows, "date", "narration", "rewards", "amount")
)

// newLayout writes the prompt (an empty example to fill) and the JSON schema
// by hand: a Go map would sort the fields.
func newLayout(rows func([]pageRow) []sharedModel.StatementRow, fields ...string) layout {
	example := make([]string, len(fields))
	props := make([]string, len(fields))
	for i, f := range fields {
		example[i] = strconv.Quote(f) + `: ""`
		props[i] = strconv.Quote(f) + `: {"type": "string"}`
	}
	required, _ := json.Marshal(fields)
	return layout{
		prompt: `Output the information in the image in the following JSON format:` + "\n" +
			`{"transactions": [{` + strings.Join(example, ", ") + `}]}`,
		schema: json.RawMessage(`{"type": "object", "required": ["transactions"], "properties": {"transactions": ` +
			`{"type": "array", "items": {"type": "object", "required": ` + string(required) +
			`, "properties": {` + strings.Join(props, ", ") + `}}}}}`),
		rows: rows,
	}
}

// bankRows: the balance going up is a deposit, down a withdrawal. The first
// row has no balance before it and is taken as a withdrawal; the API's
// direction repair and the running-balance check see it.
func bankRows(read []pageRow) []sharedModel.StatementRow {
	var rows []sharedModel.StatementRow
	var last *float64
	for _, r := range read {
		row, ok := baseRow(r)
		if !ok {
			continue
		}
		row.Reference = strings.TrimSpace(r.RefNo)
		row.Direction = sharedModel.StatementDebit
		if balance, ok := money(r.Balance); ok {
			if last != nil && balance > *last {
				row.Direction = sharedModel.StatementCredit
			}
			row.Balance, last = &balance, &balance
		}
		rows = append(rows, row)
	}
	return rows
}

// cardRows: points decide, since the model reads them reliably and often
// drops the "+" HDFC prints before a credit. A purchase earns points ("+ 70"),
// a refund takes them back ("- 60"). With none, the amount's "+" or "Cr" is
// all there is.
func cardRows(read []pageRow) []sharedModel.StatementRow {
	var rows []sharedModel.StatementRow
	for _, r := range read {
		// On a card with no points column the model puts the amount in
		// rewards, the field before it.
		if looksLikeMoney(r.Rewards) && strings.TrimSpace(r.Amount) == "" {
			r.Amount, r.Rewards = r.Rewards, ""
		}
		row, ok := baseRow(r)
		if !ok {
			continue
		}
		credit := creditMark(r.Amount)
		if repeated, ok := money(r.Rewards); ok && repeated == row.Amount {
			// The amount repeated into rewards, "+" and all.
			credit = credit || creditMark(r.Rewards)
		} else if points, ok := rewardPoints(r.Rewards); ok {
			credit = points < 0
			row.RewardPoints = &points
		}
		row.Direction = sharedModel.StatementDebit
		if credit {
			row.Direction = sharedModel.StatementCredit
		}
		rows = append(rows, row)
	}
	return rows
}

// baseRow is a row with a date, a narration and a non-zero amount.
func baseRow(r pageRow) (sharedModel.StatementRow, bool) {
	date, ok := parseDate(r.Date)
	amount, hasAmount := money(r.Amount)
	description := strings.Join(strings.Fields(r.Narration), " ")
	if !ok || !hasAmount || amount == 0 || description == "" {
		return sharedModel.StatementRow{}, false
	}
	return sharedModel.StatementRow{
		Date:        date,
		Time:        sharedModel.ParseStatementTime(r.Date),
		Description: description,
		Amount:      amount,
		Page:        r.page,
	}, true
}

var (
	numberRe = regexp.MustCompile(`\d[\d,]*(?:\.\d+)?`)
	// moneyRe: a currency sign (₹, Rs, or HDFC's "C") or paise. Points have neither.
	moneyRe  = regexp.MustCompile(`(?i)₹|\brs\.?|(?:^|[\s+-])c\s*\d|\.\d{2}\b`)
	pointsRe = regexp.MustCompile(`^[+-]?\d[\d,]*$`)
)

// money reads the first number in s: "₹ 2,932.00", "2648.09CR.", "C2,379.46".
func money(s string) (float64, bool) {
	m := numberRe.FindString(s)
	if m == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(m, ",", ""), 64)
	return math.Round(v*100) / 100, err == nil
}

func looksLikeMoney(s string) bool { return moneyRe.MatchString(s) }

// creditMark: "+ ₹ 43.70", "1,234.00 Cr".
func creditMark(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(s, "+") || strings.HasSuffix(strings.TrimSuffix(s, "."), "cr")
}

// rewardPoints reads "+ 70", "- 60", "1,200"; zero isn't points.
func rewardPoints(s string) (int, bool) {
	s = strings.ReplaceAll(s, " ", "")
	if !pointsRe.MatchString(s) {
		return 0, false
	}
	n, err := strconv.Atoi(strings.ReplaceAll(strings.TrimPrefix(s, "+"), ",", ""))
	return n, err == nil && n != 0
}

var (
	numericDateRe = regexp.MustCompile(`(\d{1,2})[/.\-](\d{1,2})[/.\-](\d{4}|\d{2})`)
	textDateRe    = regexp.MustCompile(`(\d{1,2})[\s\-]*([A-Za-z]{3})[A-Za-z]*[\s,'\-]*(\d{4}|\d{2})`)
)

// parseDate reads a day-first date anywhere in s ("06/08/2026| 20:49",
// "28/05/26", "04-Oct-2026", "13 Sep, 2026") as YYYY-MM-DD.
func parseDate(s string) (string, bool) {
	var value, layout string
	if m := numericDateRe.FindStringSubmatch(s); m != nil {
		value, layout = m[1]+"/"+m[2]+"/"+m[3], "2/1/"+yearLayout(m[3])
	} else if m := textDateRe.FindStringSubmatch(s); m != nil {
		month := strings.ToUpper(m[2][:1]) + strings.ToLower(m[2][1:])
		value, layout = m[1]+" "+month+" "+m[3], "2 Jan "+yearLayout(m[3])
	} else {
		return "", false
	}
	t, err := time.Parse(layout, value)
	if err != nil {
		return "", false
	}
	return t.Format("2006-01-02"), true
}

func yearLayout(year string) string {
	if len(year) == 2 {
		return "06"
	}
	return "2006"
}
