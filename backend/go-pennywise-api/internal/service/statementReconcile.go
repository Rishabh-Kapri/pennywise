package service

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
)

// Everything in this file is deterministic: the model only reads the PDF.
// Merging its per-chunk output, validating rows, and deciding what matches the
// ledger happen here, where they can be tested and do not drift between runs.

const (
	// statementMatchWindowDays: a card purchase posts days after it is made,
	// and the ledger usually holds the email's (purchase) date.
	statementMatchWindowDays = 5
	// Near matches (amount differs) need a tighter date and textual evidence:
	// an amount mismatch alone would pair unrelated rows.
	statementNearWindowDays    = 3
	statementNearAmountRatio   = 0.05
	statementNearMinSimilarity = 0.5
	amountEpsilon              = 0.005
)

// mergeExtractedSections folds the per-chunk sections into one list per
// account. Sections are keyed by the account number's last four digits, or by
// label when there is no number; a section with neither continues the previous
// one (a page that does not repeat its header). Opening balance and period
// start come from the first chunk that reports them, closing balance and
// period end from the last.
func mergeExtractedSections(chunks [][]model.StatementSection) []model.StatementSection {
	merged := make([]model.StatementSection, 0)
	index := map[string]int{}
	last := -1

	for _, sections := range chunks {
		for _, s := range sections {
			key := statementSectionKey(s)
			target := -1
			switch {
			case key != "":
				if i, ok := index[key]; ok {
					target = i
				}
			case last >= 0:
				target = last
			}

			if target < 0 {
				s.Rows = append([]model.StatementRow(nil), s.Rows...)
				merged = append(merged, s)
				target = len(merged) - 1
				if key != "" {
					index[key] = target
				}
				last = target
				continue
			}

			m := &merged[target]
			if m.AccountLabel == "" {
				m.AccountLabel = s.AccountLabel
			}
			if m.AccountNumber == "" && s.AccountNumber != "" {
				m.AccountNumber = s.AccountNumber
				if k := statementSectionKey(*m); k != "" {
					index[k] = target
				}
			}
			if m.Currency == "" {
				m.Currency = s.Currency
			}
			if m.PeriodStart == nil {
				m.PeriodStart = s.PeriodStart
			}
			if s.PeriodEnd != nil {
				m.PeriodEnd = s.PeriodEnd
			}
			if m.OpeningBalance == nil {
				m.OpeningBalance = s.OpeningBalance
			}
			if s.ClosingBalance != nil {
				m.ClosingBalance = s.ClosingBalance
			}
			m.Rows = append(m.Rows, s.Rows...)
			last = target
		}
	}
	return merged
}

func statementSectionKey(s model.StatementSection) string {
	if suffix := accountNumberSuffix(s.AccountNumber); suffix != "" {
		return "n:" + suffix
	}
	if label := strings.ToLower(strings.TrimSpace(s.AccountLabel)); label != "" {
		return "l:" + label
	}
	return ""
}

// accountNumberSuffix is the last four digits of a printed (often masked)
// account number — the same thing accounts.suffix stores.
func accountNumberSuffix(number string) string {
	var digits []rune
	for _, r := range number {
		if unicode.IsDigit(r) {
			digits = append(digits, r)
		}
	}
	if len(digits) < 4 {
		return ""
	}
	return string(digits[len(digits)-4:])
}

// cleanStatementSections validates every row, drops sections left empty with
// no balances, repairs debit/credit calls the running balance contradicts, and
// assigns stable row IDs. It returns how many rows were dropped.
func cleanStatementSections(sections []model.StatementSection) ([]model.StatementSection, int) {
	dropped := 0
	out := make([]model.StatementSection, 0, len(sections))
	for _, s := range sections {
		s.PeriodStart = validDatePtr(s.PeriodStart)
		s.PeriodEnd = validDatePtr(s.PeriodEnd)

		rows := make([]model.StatementRow, 0, len(s.Rows))
		for _, r := range s.Rows {
			if _, err := time.Parse("2006-01-02", r.Date); err != nil {
				dropped++
				continue
			}
			r.Amount = roundCents(r.Amount)
			if r.Amount <= 0 || !r.Direction.Valid() {
				dropped++
				continue
			}
			if r.Description == "" {
				r.Description = r.Payee
			}
			if r.Description == "" {
				dropped++
				continue
			}
			rows = append(rows, r)
		}
		s.Rows = rows
		if len(s.Rows) == 0 && s.OpeningBalance == nil && s.ClosingBalance == nil {
			continue
		}
		repairDirectionsFromBalance(s.Rows)
		out = append(out, s)
	}
	for i := range out {
		for j := range out[i].Rows {
			out[i].Rows[j].ID = fmt.Sprintf("%d-%d", i, j)
		}
	}
	return out, dropped
}

func validDatePtr(d *string) *string {
	if d == nil {
		return nil
	}
	if _, err := time.Parse("2006-01-02", *d); err != nil {
		return nil
	}
	return d
}

// repairDirectionsFromBalance uses the printed running balance to check the
// model's debit/credit calls. Between two rows that both show a balance, the
// change equals the second row's amount with one sign or the other. Which sign
// means "credit" depends on the statement — a bank balance rises with credits,
// a card's outstanding balance falls — so the convention is taken from the
// majority of rows, and only the minority that contradicts it is flipped.
// With fewer than three checkable rows nothing is changed.
func repairDirectionsFromBalance(rows []model.StatementRow) {
	type check struct {
		index int
		sign  int // +1: delta == signed amount, -1: delta == -signed amount
	}
	var checks []check
	agree, disagree := 0, 0
	for i := 1; i < len(rows); i++ {
		prev, cur := rows[i-1].Balance, rows[i].Balance
		if prev == nil || cur == nil {
			continue
		}
		delta := roundCents(*cur - *prev)
		signed := rows[i].SignedAmount()
		switch {
		case math.Abs(delta-signed) < amountEpsilon*2:
			checks = append(checks, check{i, 1})
			agree++
		case math.Abs(delta+signed) < amountEpsilon*2:
			checks = append(checks, check{i, -1})
			disagree++
		}
	}
	if len(checks) < 3 || agree == disagree {
		return
	}
	convention := 1
	if disagree > agree {
		convention = -1
	}
	for _, c := range checks {
		if c.sign == convention {
			continue
		}
		r := &rows[c.index]
		if r.Direction == model.StatementCredit {
			r.Direction = model.StatementDebit
		} else {
			r.Direction = model.StatementCredit
		}
		r.DirectionCorrected = true
	}
}

// statementBalanceCheck compares opening + rows with the reported closing
// balance under either sign convention (see repairDirectionsFromBalance).
func statementBalanceCheck(s model.StatementSection) *model.StatementBalanceCheck {
	if s.OpeningBalance == nil || s.ClosingBalance == nil {
		return nil
	}
	sum := 0.0
	for _, r := range s.Rows {
		sum += r.SignedAmount()
	}
	asBank := roundCents(*s.OpeningBalance + sum)
	asCard := roundCents(*s.OpeningBalance - sum)
	reported := roundCents(*s.ClosingBalance)
	computed := asBank
	if math.Abs(asCard-reported) < math.Abs(asBank-reported) {
		computed = asCard
	}
	diff := roundCents(reported - computed)
	return &model.StatementBalanceCheck{
		Computed:   computed,
		Reported:   reported,
		Difference: diff,
		OK:         math.Abs(diff) < amountEpsilon*2,
	}
}

// statementPeriod is the declared period widened to cover every row: a
// statement's first rows can predate its printed period (posting lag).
func statementPeriod(s model.StatementSection) (string, string) {
	start, end := "", ""
	if s.PeriodStart != nil {
		start = *s.PeriodStart
	}
	if s.PeriodEnd != nil {
		end = *s.PeriodEnd
	}
	for _, r := range s.Rows {
		if start == "" || r.Date < start {
			start = r.Date
		}
		if end == "" || r.Date > end {
			end = r.Date
		}
	}
	return start, end
}

// reconcileStatementRows pairs statement rows with ledger transactions one to
// one. Exact pairs (same signed amount, dates within the window) are taken
// first, closest date and most similar text winning; then near pairs (amount
// within 5%, close date, similar text) among what is left. used carries ledger
// IDs already claimed by another section mapped to the same account.
//
// ledgerOnly lists unmatched ledger rows dated inside [periodStart, periodEnd];
// txns may extend past the period by the match window so a lagging posting can
// still pair, but those extra rows are not "missing from the statement".
func reconcileStatementRows(
	rows []model.StatementRow,
	txns []model.StatementLedgerTxn,
	periodStart, periodEnd string,
	used map[uuid.UUID]bool,
) (matched []model.StatementMatch, missing []model.StatementRow, ledgerOnly []model.StatementLedgerTxn) {
	type candidate struct {
		row, txn   int
		dateDiff   int
		similarity float64
	}

	rowTaken := make([]bool, len(rows))
	txnTaken := make([]bool, len(txns))
	for j, t := range txns {
		if used[t.ID] {
			txnTaken[j] = true
		}
	}

	assign := func(cands []candidate, kind model.StatementMatchKind) {
		for _, c := range cands {
			if rowTaken[c.row] || txnTaken[c.txn] {
				continue
			}
			rowTaken[c.row] = true
			txnTaken[c.txn] = true
			used[txns[c.txn].ID] = true
			matched = append(matched, model.StatementMatch{
				Row:          rows[c.row],
				Transaction:  txns[c.txn],
				Kind:         kind,
				DateDiffDays: c.dateDiff,
			})
		}
	}

	var exact, near []candidate
	for i, r := range rows {
		signed := r.SignedAmount()
		for j, t := range txns {
			if txnTaken[j] {
				continue
			}
			dd := absDayDiff(r.Date, t.Date)
			if dd < 0 {
				continue
			}
			if math.Abs(signed-t.Amount) < amountEpsilon && dd <= statementMatchWindowDays {
				exact = append(exact, candidate{i, j, dd, statementTextSimilarity(r, t)})
				continue
			}
			if dd > statementNearWindowDays || (signed < 0) != (t.Amount < 0) || t.Amount == 0 {
				continue
			}
			if math.Abs(signed-t.Amount)/math.Abs(signed) > statementNearAmountRatio {
				continue
			}
			if sim := statementTextSimilarity(r, t); sim >= statementNearMinSimilarity {
				near = append(near, candidate{i, j, dd, sim})
			}
		}
	}

	sort.SliceStable(exact, func(a, b int) bool {
		if exact[a].dateDiff != exact[b].dateDiff {
			return exact[a].dateDiff < exact[b].dateDiff
		}
		return exact[a].similarity > exact[b].similarity
	})
	assign(exact, model.StatementMatchExact)

	sort.SliceStable(near, func(a, b int) bool {
		if near[a].similarity != near[b].similarity {
			return near[a].similarity > near[b].similarity
		}
		return near[a].dateDiff < near[b].dateDiff
	})
	assign(near, model.StatementMatchAmountDiffers)

	for i, r := range rows {
		if !rowTaken[i] {
			missing = append(missing, r)
		}
	}
	for j, t := range txns {
		if txnTaken[j] || used[t.ID] {
			continue
		}
		d := ledgerDate(t.Date)
		if (periodStart == "" || d >= periodStart) && (periodEnd == "" || d <= periodEnd) {
			ledgerOnly = append(ledgerOnly, t)
		}
	}

	sort.SliceStable(matched, func(a, b int) bool { return matched[a].Row.Date < matched[b].Row.Date })
	if matched == nil {
		matched = []model.StatementMatch{}
	}
	if missing == nil {
		missing = []model.StatementRow{}
	}
	if ledgerOnly == nil {
		ledgerOnly = []model.StatementLedgerTxn{}
	}
	return matched, missing, ledgerOnly
}

// ledgerDate normalises a scanned DATE to YYYY-MM-DD.
func ledgerDate(d string) string {
	if len(d) > 10 {
		return d[:10]
	}
	return d
}

func absDayDiff(a, b string) int {
	ta, err := time.Parse("2006-01-02", ledgerDate(a))
	if err != nil {
		return -1
	}
	tb, err := time.Parse("2006-01-02", ledgerDate(b))
	if err != nil {
		return -1
	}
	days := int(math.Round(ta.Sub(tb).Hours() / 24))
	if days < 0 {
		days = -days
	}
	return days
}

// statementTextSimilarity scores 0..1 how well a statement row's text agrees
// with a ledger transaction's payee and note. Bank narrations glue words
// together ("MC DONALDSPUNE"), so a payee contained in the narration once
// spaces are removed counts as a full match; otherwise it is the share of the
// shorter side's words found in the other.
func statementTextSimilarity(r model.StatementRow, t model.StatementLedgerTxn) float64 {
	rowText := r.Description + " " + r.Payee
	ledgerText := t.PayeeName + " " + t.Note

	rowCompact := compactAlnum(rowText)
	if p := compactAlnum(t.PayeeName); len(p) >= 4 && strings.Contains(rowCompact, p) {
		return 1
	}
	if p := compactAlnum(r.Payee); len(p) >= 4 && strings.Contains(compactAlnum(ledgerText), p) {
		return 1
	}

	a, b := wordSet(rowText), wordSet(ledgerText)
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	common := 0
	for w := range a {
		if b[w] {
			common++
		}
	}
	return float64(common) / float64(min(len(a), len(b)))
}

func compactAlnum(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func wordSet(s string) map[string]bool {
	words := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(w) >= 3 && !isAllDigits(w) {
			words[w] = true
		}
	}
	return words
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func roundCents(v float64) float64 {
	return math.Round(v*100) / 100
}
