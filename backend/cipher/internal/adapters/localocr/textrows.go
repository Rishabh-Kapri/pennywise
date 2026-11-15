package localocr

import (
	"math"
	"regexp"
	"strings"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

// The OCR model doesn't always answer "Table Recognition:" with a table. On a
// busy page (banners, notices, a summary box) it can write the transactions
// as plain lines: "15/08/2026 | 09:45 SOME SHOP + ₹ 1,234.00". These rules
// read such lines, but only after a line that heads the list (it names a
// date and an amount), so a date and an amount elsewhere on the page don't
// become a row. The text-only second pass (textWithoutHeading) skips that
// wait: the row pattern alone decides.

var (
	// textRowRe: a dated line ending in one amount. The time after the date
	// is optional, as are a sign, a currency, a Cr/Dr marker and a short
	// trailing mark (a "PI" dot, say).
	textRowRe = regexp.MustCompile(`^` +
		`(\d{1,2}[/.-]\d{1,2}[/.-]\d{2,4}|\d{1,2}[ -][A-Za-z]{3,9},?[ -]\d{2,4})` + // date
		`(?:\s*\|?\s*\d{1,2}:\d{2}(?::\d{2})?(?:\s*[AaPp][Mm])?)?` + // time
		`\s+(.+?)\s+` + // description
		`((?:[+-]\s*)?(?:₹|Rs\.?|INR)?\s*\(?-?[\d,]+\.\d{2}\)?(?:\s*(?:Cr|Dr|CR|DR|cr|dr)\.?)?)` + // amount
		`(?:\s+[^\d\s]{1,2})?$`)
	// splitAmountRe splits an amount off the end of a description: the line
	// ended in two amounts, the transaction's and the balance after it.
	splitAmountRe = regexp.MustCompile(`^(.*?)\s+((?:[+-]\s*)?(?:₹|Rs\.?|INR)?\s*[\d,]+\.\d{2}(?:\s*(?:Cr|Dr|CR|DR|cr|dr)\.?)?)$`)
	// creditWordsRe: narration words that mark money in, for a row whose
	// balance can't say (the first one, with no opening balance).
	creditWordsRe = regexp.MustCompile(`(?i)(?:/CR/|\bCR\b|\bcredit(?:ed)?\b|\bdeposit\b|\brefund\b|\breversal\b|\binterest\b|\bsalary\b|\bcashback\b)`)
	// leadingTimeRe is a time the date cell lost to the description.
	leadingTimeRe = regexp.MustCompile(`^\|?\s*\d{1,2}:\d{2}(?::\d{2})?(?:\s*[AaPp][Mm])?\s+`)
)

// isListHeading: a line naming a date column and an amount column, the
// heading of a transaction list written as text.
func isListHeading(line string) bool {
	l := strings.ToLower(line)
	return strings.Contains(l, "date") &&
		(strings.Contains(l, "amount") || strings.Contains(l, "debit") || strings.Contains(l, "credit") ||
			strings.Contains(l, "withdrawal") || strings.Contains(l, "deposit"))
}

// parseTextRows adds the transactions written as plain lines. Table lines
// (markdown pipes, HTML) are left to the table parser.
func (p *tableParser) parseTextRows(page int, text string) {
	listed := p.textWithoutHeading
	for _, raw := range strings.Split(text, "\n") {
		line := strings.Join(strings.Fields(strings.NewReplacer("**", "", "`", "").Replace(raw)), " ")
		if line == "" || strings.HasPrefix(line, "|") || strings.Contains(line, "<") {
			continue
		}
		if !listed {
			listed = isListHeading(line)
			continue
		}
		m := textRowRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		desc := strings.TrimSpace(m[2])
		amountText, balanceText := m[3], ""
		// Two amounts at the end: the transaction, then the balance after it.
		if sm := splitAmountRe.FindStringSubmatch(desc); sm != nil {
			desc, amountText, balanceText = strings.TrimSpace(sm[1]), sm[2], m[3]
		}
		rc := p.rowContext()
		d, ok := parseDate(m[1])
		if !ok {
			d, ok = parseDateNoYear(m[1], rc.ref)
		}
		amount, mk := parseAmount(amountText)
		if !ok || amount <= 0 || desc == "" {
			continue
		}
		r := sharedModel.StatementRow{
			Date:        d,
			Description: desc,
			Amount:      amount,
			Direction:   sharedModel.StatementDebit,
			Page:        page,
		}
		if balanceText != "" {
			p.textRowWithBalance(&r, balanceText)
		} else if mk == markCredit || (mk == markNegative && rc.negativeIsCredit) {
			r.Direction = sharedModel.StatementCredit
		}
		p.rows = append(p.rows, r)
		p.stats.TextRows++
		p.stats.Rows++
	}
}

// textRowWithBalance sets a row's direction from its balance: up from the
// balance before it is money in, down is money out. The first row has no
// balance before it unless the header gave an opening balance; then, or when
// the change doesn't equal the amount, the narration decides.
func (p *tableParser) textRowWithBalance(r *sharedModel.StatementRow, balanceText string) {
	bal, mk := parseAmount(balanceText)
	if mk == markDebit || mk == markNegative {
		bal = -bal // an overdrawn account
	}
	r.Balance = &bal

	prev := p.textBalance
	if prev == nil {
		prev = p.header.opening
	}
	p.textBalance = &bal

	if prev != nil {
		diff := bal - *prev
		if math.Abs(math.Abs(diff)-r.Amount) < 0.005 {
			if diff > 0 {
				r.Direction = sharedModel.StatementCredit
			}
			return
		}
	}
	p.stats.TextDirectionGuessed++
	if creditWordsRe.MatchString(r.Description) {
		r.Direction = sharedModel.StatementCredit
	}
}
