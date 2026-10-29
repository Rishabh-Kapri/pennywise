// Package regex holds deterministic, pattern-based adapters.
package regex

import (
	"context"
	"regexp"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
)

// transactionPatterns are the known transaction alert formats.
var transactionPatterns = []*regexp.Regexp{
	// HDFC credit card email: "Rs. 2440.95 has been debited from your HDFC Bank Credit Card ending 2222"
	regexp.MustCompile(`(?i)Rs\.?\s*[\d,]+(?:\.\d{1,2})? has been debited from your HDFC Bank Credit Card ending \d{4}`),
	// HDFC credit card SMS: "Spent Rs.3782 On HDFC Bank Card 2222 At"
	regexp.MustCompile(`(?i)Spent Rs\.?\s*[\d,]+(?:\.\d{1,2})? On HDFC Bank Card \d{4} At`),
}

// CheckTransaction is a ports.CheckTransaction that matches known alert formats.
// A match is a confident yes. No match means "unknown" (confidence 0), so a
// chain falls through to the next adapter.
type checkTransaction struct {}


func NewCheckTransaction() *checkTransaction {
	return &checkTransaction{}
}

func (c *checkTransaction) IsTransaction(ctx context.Context, msg ports.Message) (ports.TxnVerdict, error) {
	for _, pattern := range transactionPatterns {
		if pattern.MatchString(msg.Body) {
			return ports.TxnVerdict{IsTransaction: true, Confidence: 1}, nil
		}
	}
	return ports.TxnVerdict{IsTransaction: false, Confidence: 0}, nil
}
