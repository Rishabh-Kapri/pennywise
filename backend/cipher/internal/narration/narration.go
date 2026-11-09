// Package narration turns raw bank text (alert emails, statement narrations)
// into the lookup key "CHANNEL | MERCHANT" plus the matching fields a source
// states. It is rule-based and runs locally: private bank text never leaves
// the process.
//
// Only what lookup and reconciliation need is extracted. Text no rule
// recognises yields a Parsed with an empty Channel; callers keep the raw text
// and simply have no key for it.
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

var ist = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		return time.FixedZone("IST", 5*3600+1800)
	}
	return loc
}()

// Hint is what the caller knows about the text beyond the text itself.
type Hint struct {
	// CardAccount: the text belongs to a credit card account, so a statement
	// line without a channel prefix is a card descriptor.
	CardAccount bool
}

// Parse reads raw bank text. It never fails: unknown text returns a Parsed
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
	return parseEmail(text)
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

// normalizeMerchant is the one place merchant strings get their key form.
func normalizeMerchant(s string) string {
	s = spaceRe.ReplaceAllString(strings.ToUpper(s), " ")
	return strings.Trim(s, " .,-")
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
		p.Channel, p.Merchant, p.VPA, p.Ref = ChannelUPI, normalizeMerchant(m[1]), strings.ToLower(m[2]), m[3]
	case stmtNEFT.MatchString(upper):
		m := stmtNEFT.FindStringSubmatch(upper)
		p.Channel, p.Merchant, p.Ref = ChannelNEFT, normalizeMerchant(m[2]), m[3]
		p.Direction = directionFromCode(m[1])
	case stmtIMPS.MatchString(upper):
		m := stmtIMPS.FindStringSubmatch(upper)
		p.Channel, p.Ref, p.Merchant = ChannelIMPS, m[1], normalizeMerchant(m[2])
	case stmtACH.MatchString(upper):
		m := stmtACH.FindStringSubmatch(upper)
		p.Channel, p.Merchant = ChannelNACH, normalizeMerchant(m[2])
		p.Direction = directionFromCode(m[1])
	case hint.CardAccount || stmtCardDescriptor.MatchString(upper):
		p.Channel, p.Merchant = ChannelCard, normalizeMerchant(upper)
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

// ---- alert emails (HDFC formats seen so far) ----

var (
	// "towards VPA x@y (NAME) on 07-10-26" (current) and
	// "to VPA x@y NAME on 06-05-26" (older).
	emailVPA        = regexp.MustCompile(`(?i)\bVPA\s+(\S+@[\w.-]+)\s+(.+?)\s+on\s+\d{2}-\d{2}-\d{2}\b`)
	emailUPIRef     = regexp.MustCompile(`(?i)reference no\.?\s*:?\s*(\d{6,})`)
	emailCardToward = regexp.MustCompile(`(?i)Credit Card ending\.?\s*\d{4}\.?\s+towards\s+(.+?)\.?\s+on\s+\d{1,2} [A-Za-z]{3},? \d{4}`)
	// "for Rs. 149.0. at SPOTIFY SI. on 12-06-2026": anchored on the amount
	// so an earlier "at" (a forwarded header's "2026 at 10:31") can't start it.
	emailCardAt     = regexp.MustCompile(`(?i)\b(?:Rs|INR)\.?\s*[\d,]+(?:\.\d+)?\.?\s+at\s+(.+?)\.?\s+on\s+\d{2}-\d{2}-\d{4}`)
	emailMandate    = regexp.MustCompile(`(?i)Your (.+?) bill, set up through E-mandate`)
	emailMandateDue = regexp.MustCompile(`(?i)E-mandate \(Auto payment\) of [A-Z]{3} [\d,.]+ for (.+?)\.\s+Amount will be debited`)
	emailReversal   = regexp.MustCompile(`(?i)transaction reversal of .*?From Merchant:\.?\s*(.+?)\s*Date Time`)
	emailNACH       = regexp.MustCompile(`(?i)debited from HDFC Bank Account Number \S+ towards (.+?)(?:/\S*)? with UMRN`)

	emailAmount  = regexp.MustCompile(`(?i)\b(Rs|INR|USD|EUR|GBP)\.?\s*([\d,]+(?:\.\d+)?)`)
	emailAccount = regexp.MustCompile(`(?i)(?:ending(?: in)?|Card XX|account|Number X+)\.?\s*(\d{4})\b`)
	// The first of these phrases decides the direction. "for using" and
	// "made a transaction" are card purchase alerts ("Thank you for using your
	// HDFC Bank Card XX2222 for Rs. 12.71 at ..."), which never say "debited".
	emailDirection = regexp.MustCompile(`(?i)\b(debited|paid using|spent|charged|for using|made a transaction|credited|reversal)\b`)
	emailCard      = regexp.MustCompile(`(?i)\b(?:Credit Card|Bank Card|Debit Card)\b`)

	// Dates in the order they are tried; the first that matches wins.
	dateRules = []struct {
		re     *regexp.Regexp
		layout string
	}{
		{regexp.MustCompile(`\b(\d{1,2} [A-Za-z]{3}), (\d{4}) at (\d{2}:\d{2}:\d{2})`), "2 Jan 2006 15:04:05"},
		{regexp.MustCompile(`\b(\d{2}-\d{2}-\d{4}) (\d{2}:\d{2}:\d{2})`), "02-01-2006 15:04:05"},
		{regexp.MustCompile(`\b(\d{1,2} [A-Za-z]{3}),? (\d{4})\b`), "2 Jan 2006"},
		{regexp.MustCompile(`\b(\d{2}-[A-Za-z]{3}-\d{4})\b`), "02-Jan-2006"},
		{regexp.MustCompile(`\b(\d{2}-\d{2}-\d{4})\b`), "02-01-2006"},
		{regexp.MustCompile(`\b(\d{2}/\d{2}/\d{4})\b`), "02/01/2006"},
		{regexp.MustCompile(`\b(\d{2}-\d{2}-\d{2})\b`), "02-01-06"},
	}
)

func parseEmail(text string) Parsed {
	p := Parsed{Kind: KindEmail}

	switch {
	case emailVPA.MatchString(text):
		m := emailVPA.FindStringSubmatch(text)
		p.Channel, p.VPA = ChannelUPI, strings.ToLower(m[1])
		name := strings.TrimSpace(m[2])
		if strings.HasPrefix(name, "(") && strings.HasSuffix(name, ")") {
			name = name[1 : len(name)-1]
		}
		p.Merchant = normalizeMerchant(name)
		if r := emailUPIRef.FindStringSubmatch(text); r != nil {
			p.Ref = r[1]
		}
	case emailMandate.MatchString(text):
		p.Channel, p.Merchant = ChannelCard, normalizeMerchant(emailMandate.FindStringSubmatch(text)[1])
	case emailMandateDue.MatchString(text):
		p.Channel, p.Merchant = ChannelCard, normalizeMerchant(emailMandateDue.FindStringSubmatch(text)[1])
	case emailReversal.MatchString(text):
		p.Channel, p.Merchant = ChannelCard, normalizeMerchant(emailReversal.FindStringSubmatch(text)[1])
	case emailNACH.MatchString(text):
		p.Channel, p.Merchant = ChannelNACH, normalizeMerchant(emailNACH.FindStringSubmatch(text)[1])
	case emailCard.MatchString(text) && emailCardToward.MatchString(text):
		p.Channel, p.Merchant = ChannelCard, normalizeMerchant(emailCardToward.FindStringSubmatch(text)[1])
	case emailCard.MatchString(text) && emailCardAt.MatchString(text):
		p.Channel, p.Merchant = ChannelCard, normalizeMerchant(emailCardAt.FindStringSubmatch(text)[1])
	}

	// Fields any bank alert states, recognised format or not. Only bank
	// alerts get them: a non-bank receipt would yield misleading numbers.
	if p.Channel == "" && !strings.Contains(strings.ToUpper(text), "HDFC BANK") {
		return p
	}
	if m := emailAmount.FindStringSubmatch(text); m != nil {
		p.Currency = strings.ToUpper(m[1])
		if p.Currency == "RS" {
			p.Currency = "INR"
		}
		p.Amount = strings.TrimRight(strings.ReplaceAll(m[2], ",", ""), ".")
	}
	if m := emailAccount.FindStringSubmatch(text); m != nil {
		p.AccountHint = m[1]
	}
	if m := emailDirection.FindStringSubmatch(text); m != nil {
		p.Direction = Debit
		if strings.EqualFold(m[1], "credited") || strings.EqualFold(m[1], "reversal") {
			p.Direction = Credit
		}
	}
	p.OccurredOn, p.OccurredAt = parseDate(text)
	return p
}

func parseDate(text string) (on, at *time.Time) {
	for _, rule := range dateRules {
		m := rule.re.FindStringSubmatch(text)
		if m == nil {
			continue
		}
		t, err := time.ParseInLocation(rule.layout, strings.Join(m[1:], " "), ist)
		if err != nil {
			continue
		}
		day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, ist)
		on = &day
		if strings.Contains(rule.layout, "15:04") {
			at = &t
		}
		return on, at
	}
	return nil, nil
}
