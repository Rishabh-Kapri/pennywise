package utils

import (
	"regexp"
	"strings"
)

var (
	// Boilerplate removal
	greetingRe     = regexp.MustCompile(`(?i)^Dear\s+(Customer|Card\s*Member|Card\s*Holder)\s*,?\s*`)
	bankNameRe     = regexp.MustCompile(`(?i)(Greetings from\s+\w+\s+Bank!?\s*|Thank you for using\s+)`)
	bankStandaloneRe = regexp.MustCompile(`(?i)\b[A-Za-z]+\s+Bank\b`)

	// Amount/date/noise removal
	amountRe  = regexp.MustCompile(`(?i)(Rs\.?\s?|INR\s?)([\d,]+\.\d+)`)
	dateRe    = regexp.MustCompile(`(?i)\s*(on|Date:)\s*(\d{2}[-/]\d{2}[-/]\d{2,4}|\d{2}\s*\w+,\s*\d{4})\.?`)
	htmlTagRe = regexp.MustCompile(`<[^>]+>`)

	// Account number masking
	cardNumRe    = regexp.MustCompile(`(?i)(Credit\s+Card|account)\s*(ending\s+)?\**(\d{4})`)
	accountNumRe = regexp.MustCompile(`(?i)(account)\s*\**(\d{4})`)

	// Pattern 1: "Your {merchant} bill, set up through E-mandate"
	// Matches: "Your Spotify bill, set up through..."
	// Matches: "Your Adobe Systems Software Ireland Ltd bill, set up through..."
	billMandateRe = regexp.MustCompile(`(?i)Your\s+(.+?)\s+bill,?\s+set up through E-mandate`)
	// Pattern 2: "upcoming E-mandate...for {merchant}"
	// Matches: "upcoming E-mandate (Auto payment) of INR 18.89 for Google Cloud"
	upcomingMandateRe = regexp.MustCompile(`(?i)upcoming E-mandate\s*\(Auto payment\).*?for\s+(.+?)\.`)

	// Filler phrases
	fillerPhrases = []string{
		"has been successfully paid using your",
		"is debited from your",
		"is successfully credited to your",
		"has been debited from",
		"has been credited to",
		"towards",
		"to VPA",
		"by VPA",
		"Transaction Details: Amount:",
		"set up through E-mandate (Auto payment),",
		"bill,",
	}
	fillerWordsRe = regexp.MustCompile(`(?i)\b(at|for|Your)\b`)

	multiSpaceRe = regexp.MustCompile(`\s{2,}`)
)

// CleanEmailText strips boilerplate, amounts, dates, and filler phrases from
// a bank transaction email, leaving only the merchant/payee signal suitable
// for embedding.
//
// Examples:
//
//	Raw:   "Dear Customer, Greetings from HDFC Bank! Rs.438.87 is debited from your HDFC Bank Credit Card ending 9876 towards OPENAI on 11 Aug, 2025."
//	Clean: "debit OPENAI"
//
//	Raw:   "Dear Customer, Rs.500.00 has been debited from account 4567 to VPA 9876543210@ybl JOHN DOE on 14-07-25."
//	Clean: "debit 9876543210@ybl JOHN DOE"
func CleanEmailText(text string, transactionType string) string {
	cleaned := text

	// Remove HTML tags
	cleaned = htmlTagRe.ReplaceAllString(cleaned, " ")

	// Determine transaction type signal
	txnType := "debit"
	if strings.EqualFold(transactionType, "credited") {
		txnType = "credit"
	}

	// Extract account/card number before removing it
	accountId := ""
	if match := cardNumRe.FindStringSubmatch(cleaned); match != nil {
		accountId = match[3]
	} else if match := accountNumRe.FindStringSubmatch(cleaned); match != nil {
		accountId = match[2]
	}

	// Handle bill/mandate templates FIRST (early return)
	if match := billMandateRe.FindStringSubmatch(cleaned); match != nil {
		merchant := strings.TrimSpace(match[1])
		return txnType + " " + accountId + " " + merchant
	}

	if match := upcomingMandateRe.FindStringSubmatch(cleaned); match != nil {
		merchant := strings.TrimSpace(match[1])
		return txnType + " " + accountId + " " + merchant
	}

	// Remove greeting
	cleaned = greetingRe.ReplaceAllString(cleaned, "")
	// Remove bank name boilerplate
	cleaned = bankNameRe.ReplaceAllString(cleaned, "")
	// Remove amounts
	cleaned = amountRe.ReplaceAllString(cleaned, "")
	// Remove dates
	cleaned = dateRe.ReplaceAllString(cleaned, "")
	// Remove card/account numbers EARLY (before fillers destroy the pattern)
	cleaned = cardNumRe.ReplaceAllString(cleaned, "")
	cleaned = accountNumRe.ReplaceAllString(cleaned, "")
	// Remove standalone bank names (e.g. "HDFC Bank" left after card pattern removal)
	cleaned = bankStandaloneRe.ReplaceAllString(cleaned, "")
	// Remove filler phrases (case-insensitive)
	for _, phrase := range fillerPhrases {
		cleaned = replaceInsensitive(cleaned, phrase, " ")
	}
	cleaned = fillerWordsRe.ReplaceAllString(cleaned, " ")

	// Clean up
	cleaned = strings.ReplaceAll(cleaned, ".", "")
	cleaned = strings.ReplaceAll(cleaned, ",", "")
	cleaned = multiSpaceRe.ReplaceAllString(cleaned, " ")
	cleaned = strings.TrimSpace(cleaned)

	if cleaned == "" {
		return txnType
	}
	// Prepend transaction type + account number
	prefix := txnType
	if accountId != "" {
		prefix += " " + accountId
	}
	return prefix + " " + cleaned
}

func replaceInsensitive(s, old, replacement string) string {
	lower := strings.ToLower(s)
	oldLower := strings.ToLower(old)
	idx := strings.Index(lower, oldLower)
	if idx == -1 {
		return s
	}
	return s[:idx] + replacement + s[idx+len(old):]
}
