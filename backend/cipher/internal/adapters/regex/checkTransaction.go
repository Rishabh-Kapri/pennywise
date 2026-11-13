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

// checkTransaction is a ports.CheckTransaction that matches known alert
// formats. A match is a confident yes. No match is a no with no score: the
// rules can't say how sure they are of a format they don't know.
type checkTransaction struct{}

func NewCheckTransaction() ports.CheckTransaction {
	return &checkTransaction{}
}

func (c *checkTransaction) Check(ctx context.Context, msg ports.Message) (ports.TxnVerdict, error) {
	for _, pattern := range transactionPatterns {
		if pattern.MatchString(msg.Body) {
			return ports.TxnVerdict{IsTransaction: true, Confidence: new(1.0)}, nil
		}
	}
	return ports.TxnVerdict{IsTransaction: false}, nil
}
