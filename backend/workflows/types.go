package workflows

type EmailWorflowInput struct {
	Email     string `json:"email"`
	HistoryId uint64 `json:"historyId"`
}

type ParsedEmail struct {
	MessageId       string  `json:"messageId"`
	EmailText       string  `json:"emailText"`
	Amount          float64 `json:"amount"`
	Date            string  `json:"date"`
	TransactionType string  `json:"transactionType"`
	DefaultAccount  string  `json:"defaultAccount"`
	Payee           string  `json:"payee"`
	Category        string  `json:"category"`
}

type PredictionResult struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
}

type PredictedFields struct {
	Account  PredictionResult `json:"account"`
	Payee    PredictionResult `json:"payee"`
	Category PredictionResult `json:"category"`
}

type CreateTransactionInput struct {
	ParsedData  ParsedEmail     `json:"parsedData"`
	Predictions PredictedFields `json:"predictions"`
}
