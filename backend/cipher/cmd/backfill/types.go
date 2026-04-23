package main

import "github.com/google/uuid"

// Prediction represents a prediction record from the API or a JSON file.
type Prediction struct {
	ID                    string  `json:"id"`
	BudgetID              string  `json:"budgetId"`
	TransactionID         string  `json:"transactionId"`
	EmailText             string  `json:"emailText"`
	Amount                float64 `json:"amount"`
	Account               *string `json:"account"`
	Payee                 *string `json:"payee"`
	Category              *string `json:"category"`
	HasUserCorrected      *bool   `json:"hasUserCorrected"`
	UserCorrectedPayee    *string `json:"userCorrectedPayee"`
	UserCorrectedAccount  *string `json:"userCorrectedAccount"`
	UserCorrectedCategory *string `json:"userCorrectedCategory"`
}

// resolvedPrediction holds a prediction with its resolved labels,
// preferring user-corrected values over ML predictions.
type resolvedPrediction struct {
	ID              string
	EmailText       string
	Amount          float64
	Payee           string
	Category        string
	Account         string
	Source          string // "AUTO_LEARNED" or "MANUAL"
	TransactionType string // "debited" or "credited"
	BudgetID        uuid.UUID
}

// skipUPIAddresses contains UPI handles that should be skipped during MCC backfill
// because they are already mapped or irrelevant (subscriptions, known merchants, etc.).
var skipUPIAddresses = map[string]bool{
	"payee055@hdfcbank":                    true,
	"payee039@hdfcbank":     true,
	"paytm-blinkit@ptybl":                   true,
	"payee047@axisbank":                    true,
	"paytmqr5eqr9v@ptys":                    true,
	"payee054@hdfcbank":               true,
	"payee027@icici":                    true,
	"payee025@hdfcbank": true,
	"paytms1f9myp@pty":                      true,
	"gpay-029@okbizaxis":            true, // platinum super store
	"payee053@hdfcbank":         true,
	"zerodhabrokingbrk@validaxis":           true,
	"paytmqr5d3f1q@ptys":                    true, // kailash super market
	"payee002@okbizaxis":                 true,
	"9000000211@okbizicici":                 true,
	"ka57f1731@cnrb":                        true, // bmtc
	"ka57f1814@cnrb":                        true, // bmtc
	"payee050@icici":              true,
	"payee049@hdfcbank": true,
	"payee038@hdfcbank":                true, // mr diy
	"paytm-75735390@ptys":                   true, // corridor 7
	"payee035@hdfcbank":              true, // hdfc housing loan
	"zerodhaiccl3brk@validhdfc":             true,
	"payee026@hdfcbank":            true, // bella vita
	"paytmqr043@paytm": true, // noble chemist
	"Q900000020@ybl":                        true, // numero uno pithoragarh
	"credclub@axisb":                        true, // cred club
	"tickertapepro@yespay":                  true, // ticker tape pro
	"uberindiasystem187204rzp@rxairtel":     true, // uber india
	"payee037@hdfcbank":                 true, // indmoney
	"9000000114@okbizaxis":                  true, // variety store
	"75735390@ptys":                         true, // corridor 7
	"9000000113@ptaxis":                     true, // bombay optician
	"gpay-031@okbizaxis":            true, // cafe on the rocks
	"payee034@hdfcbank":          true, // hdfc limited
}

// resolvePrediction extracts the correct labels from a prediction,
// preferring user-corrected values when available.
func resolvePrediction(p Prediction, budgetID uuid.UUID) *resolvedPrediction {
	if p.EmailText == "" {
		return nil
	}

	source := "AUTO_LEARNED"
	payee := deref(p.Payee)
	category := deref(p.Category)
	account := deref(p.Account)

	if p.HasUserCorrected != nil && *p.HasUserCorrected {
		if p.UserCorrectedPayee != nil {
			payee = *p.UserCorrectedPayee
		}
		if p.UserCorrectedCategory != nil {
			category = *p.UserCorrectedCategory
		}
		if p.UserCorrectedAccount != nil {
			account = *p.UserCorrectedAccount
		}
		source = "MANUAL"
	}

	if payee == "" || category == "" || account == "" {
		return nil
	}

	transactionType := "debit"
	if p.Amount > 0 {
		transactionType = "credit"
	}

	return &resolvedPrediction{
		ID:              p.ID,
		EmailText:       p.EmailText,
		Amount:          p.Amount,
		Payee:           payee,
		Category:        category,
		Account:         account,
		Source:          source,
		TransactionType: transactionType,
		BudgetID:        budgetID,
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
