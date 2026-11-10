// Package narration turns bank statement narrations into the lookup key
// "CHANNEL | MERCHANT" plus the matching fields a line states, and holds the
// one definition of a key's merchant form.
//
// It doesn't read alert emails or SMS. Those change with every bank template,
// so the pipeline's extract and verify steps read them instead. Text that isn't
// a statement line yields a Parsed with only Kind set.
package narration

import (
	"html"
	"regexp"
	"strings"
	"time"
)

const (
	KindEmail         = "EMAIL"
	KindStatementLine = "STATEMENT_LINE"

	ChannelUPI  = "UPI"
	ChannelCard = "CARD"
	ChannelNEFT = "NEFT"
	ChannelIMPS = "IMPS"
	ChannelNACH = "NACH"

	Debit  = "DEBIT"
	Credit = "CREDIT"
)

// Parsed is what one piece of bank text states. Empty strings and nil
// pointers mean the text didn't say.
type Parsed struct {
	Kind        string
	Channel     string
	Merchant    string
	VPA         string
	Ref         string // UPI RRN, IMPS ref, NEFT UTR: unique per payment
	AccountHint string
	Direction   string
	Amount      string // decimal text without separators, e.g. "1196.52"
	Currency    string
	OccurredOn  *time.Time // date in IST
	OccurredAt  *time.Time // set only when the text had a time of day
}

// LookupKey is "CHANNEL | MERCHANT", or "" when either part is unknown.
func (p Parsed) LookupKey() string {
	if p.Channel == "" || p.Merchant == "" {
		return ""
	}
	return p.Channel + " | " + p.Merchant
}

// Hint is what the caller knows about the text beyond the text itself.
type Hint struct {
	// CardAccount: the text belongs to a credit card account, so a statement
	// line without a channel prefix is a card descriptor.
	CardAccount bool
}

// Parse reads a statement line. It never fails: anything else returns a Parsed
// with only Kind set.
func Parse(raw string) Parsed {
	return ParseWith(raw, Hint{})
}

// ParseWith is Parse with what the caller knows about the account.
func ParseWith(raw string, hint Hint) Parsed {
	text := clean(raw)
	if isStatementCell(text) {
		return parseStatementLine(text, hint)
	}
	return Parsed{Kind: KindEmail}
}

// isStatementCell tells a statement narration/description cell from an email
// body: a known channel prefix, or a single short line that isn't addressed
// to anyone.
func isStatementCell(text string) bool {
	upper := strings.ToUpper(text)
	if statementPrefix.MatchString(upper) {
		return true
	}
	return len(text) <= 80 && !strings.Contains(upper, "DEAR ") && !strings.Contains(upper, "HELLO")
}

var (
	tagRe   = regexp.MustCompile(`<[^>]*>`)
	spaceRe = regexp.MustCompile(`\s+`)
)

// clean strips HTML, decodes entities and collapses whitespace.
func clean(raw string) string {
	s := tagRe.ReplaceAllString(raw, " ")
	s = html.UnescapeString(s)
	s = spaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// NormalizeMerchant is the one place merchant strings get their key form.
func NormalizeMerchant(s string) string {
	s = spaceRe.ReplaceAllString(strings.ToUpper(s), " ")
	return strings.Trim(s, " .,-")
}

// LookupMerchant is the merchant half of a pipeline message's lookup key. A UPI
// payment uses its VPA, which the bank states exactly, where a model's merchant
// name can vary between messages.
func LookupMerchant(channel, merchant, vpa string) string {
	if channel == ChannelUPI && strings.TrimSpace(vpa) != "" {
		return NormalizeMerchant(vpa)
	}
	return NormalizeMerchant(merchant)
}

// ---- statement narrations ----

var (
	statementPrefix = regexp.MustCompile(`^(?:UPI-|NEFT (?:CR|DR)-|IMPS-|ACH [CD]-|NACH)`)

	// UPI-<name>-<vpa>-<ifsc>-<rrn>-<remark>. The VPA (has '@') and the RRN
	// (12 digits) anchor the split, so hyphens inside the name, the VPA or
	// the remark don't break it.
	stmtUPI = regexp.MustCompile(`^UPI-(.+?)-([^\s@]+@[^\s-]+)-(?:[A-Z0-9]{11}-)?(\d{12})(?:-(.*))?$`)
	// NEFT CR-<ifsc>-<counterparty>-<beneficiary>-<utr>
	stmtNEFT = regexp.MustCompile(`^NEFT (CR|DR)-[A-Z0-9]{11}-(.+?)(?:-.*)?-([A-Z0-9]+)$`)
	// IMPS-<rrn>-<counterparty>-...
	stmtIMPS = regexp.MustCompile(`^IMPS-(\d+)-([^-]+)-`)
	// ACH D-<mandate name>-<ref>
	stmtACH = regexp.MustCompile(`^(?:ACH|NACH) ?([CD])-\s*([^-]+?)\s*(?:-(.*))?$`)
	// Card network descriptor without a prefix: merchant, city, 2-letter
	// country ("AIRPORT LOUNGE BANGALORE IN"). No digits, which rules out the
	// bank's own lines ("INTEREST PAID TILL 30-SEP-2026").
	stmtCardDescriptor = regexp.MustCompile(`^[A-Z*&'./ -]+ [A-Z]{2}$`)
)

func parseStatementLine(text string, hint Hint) Parsed {
	p := Parsed{Kind: KindStatementLine}
	upper := strings.ToUpper(text)
	switch {
	case stmtUPI.MatchString(upper):
		m := stmtUPI.FindStringSubmatch(upper)
		p.Channel, p.Merchant, p.VPA, p.Ref = ChannelUPI, NormalizeMerchant(m[1]), strings.ToLower(m[2]), m[3]
	case stmtNEFT.MatchString(upper):
		m := stmtNEFT.FindStringSubmatch(upper)
		p.Channel, p.Merchant, p.Ref = ChannelNEFT, NormalizeMerchant(m[2]), m[3]
		p.Direction = directionFromCode(m[1])
	case stmtIMPS.MatchString(upper):
		m := stmtIMPS.FindStringSubmatch(upper)
		p.Channel, p.Ref, p.Merchant = ChannelIMPS, m[1], NormalizeMerchant(m[2])
	case stmtACH.MatchString(upper):
		m := stmtACH.FindStringSubmatch(upper)
		p.Channel, p.Merchant = ChannelNACH, NormalizeMerchant(m[2])
		p.Direction = directionFromCode(m[1])
	case hint.CardAccount || stmtCardDescriptor.MatchString(upper):
		p.Channel, p.Merchant = ChannelCard, NormalizeMerchant(upper)
	}
	return p
}

func directionFromCode(code string) string {
	switch code {
	case "CR", "C":
		return Credit
	case "DR", "D":
		return Debit
	}
	return ""
}
