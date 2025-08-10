package pennywise

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"gmail-transactions/pkg/config"
	"gmail-transactions/pkg/parser"
	"gmail-transactions/pkg/prediction"
)

type Service struct {
	config config.Config
}

type ParsedTransaction struct {
	Amount   float64
	Date     string
	Payee    string
	Account  string
	Category string
}

type Transaction struct {
	ID                    string  `json:"id,omitempty"`
	Date                  string  `json:"date"`
	PayeeId               string  `json:"payeeId"`
	CategoryId            string  `json:"categoryId"`
	AccountId             string  `json:"accountId"`
	Amount                float64 `json:"amount"`
	Note                  string  `json:"note"`
	Source                string  `json:"source"` // MLP for prediction, PENNYWISE for frontend
	TransferAccountId     string  `json:"transferAccountId,omitempty"`
	TransferTransactionId string  `json:"transferTransactionId,omitempty"`
}

type PredictionReq struct {
	TransactionId      string  `json:"transactionId"`
	EmailText          string  `json:"emailText"`
	Amount             float64 `json:"amount"`
	Account            string  `json:"account"`
	AccountPrediction  float64 `json:"accountPrediction,omitempty"`
	Payee              string  `json:"payee,omitempty"`
	PayeePrediction    float64 `json:"payeePrediction"`
	Category           string  `json:"category,omitempty"`
	CategoryPrediction float64 `json:"categoryPrediction,omitempty"`
}

func NewService() *Service {
	return &Service{}
}

// add query params to url
func (s *Service) getEncodedURL(path string, queryData map[string]string) (string, error) {
	baseUrl, err := url.Parse(s.config.PennywiseApi)
	if err != nil {
		log.Printf("Error while parsing Pennywise API URL: %v", err)
		return "", err
	}
	baseUrl.Path += path
	params := url.Values{}
	for key, value := range queryData {
		params.Add(key, value)
	}
	baseUrl.RawQuery = params.Encode()

	log.Printf("URL: %v", baseUrl.String())
	return baseUrl.String(), nil
}

// makePennywiseApiRequest makes a request to Pennywise API
func (s *Service) makePennywiseRequest(endpoint string, method string, queryData map[string]string, data any) ([]map[string]any, error) {
	url, err := s.getEncodedURL(endpoint, queryData)
	if err != nil {
		log.Printf("Error while encoding url for %v: %v", endpoint, err)
		return nil, err
	}

	var requestBodyBytes []byte
	if data != nil {
		var err error
		requestBodyBytes, err = json.Marshal(data)
		if err != nil {
			log.Printf("Error marshaling JSON for %v: %v", endpoint, err)
			return nil, err
		}
	} else {
		requestBodyBytes = []byte{}
	}

	requestBody := bytes.NewBuffer(requestBodyBytes)
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		log.Printf("Error while creating pennywise api request for %v: %v", endpoint, err.Error())
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	// @TODO: add ability to take this from env
	req.Header.Set("X-Budget-ID", "2166418d-3fa2-4acc-b92c-ab9f36c18d76")

	dump, _ := httputil.DumpRequestOut(req, true)
	log.Printf("FINAL REQUEST:\n%s", dump)
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		log.Printf("Error while sending pennywise api request for %v: %v", endpoint, err.Error())
		return nil, err
	}
	defer res.Body.Close()
	
	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("Error while reading pennywise api response: %v", err.Error())
		return nil, err
	}
	log.Printf("Response received from pennywise api for %v with data %v", endpoint, string(body))
	
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("API error %v: %s", endpoint, res.Status)
	}
	var response []map[string]any
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("Error while unmarshalling pennywise api response: %v", err.Error())
	}
	return response, nil
}

func (s *Service) CreateTransaction(parsedDetails *parser.EmailDetails, predictedFields *prediction.PredictedFields) (*Transaction, error) {
	txnData := ParsedTransaction{
		Amount:   parsedDetails.Amount,
		Date:     parsedDetails.Date,
		Payee:    predictedFields.Payee.Label,
		Account:  predictedFields.Account.Label,
		Category: predictedFields.Category.Label,
	}
	log.Printf("Creating transaction: %+v", txnData)

	accQueryMap := map[string]string{"name": txnData.Account}
	accounts, err := s.makePennywiseRequest("/api/accounts/search", "GET", accQueryMap, nil)
	if err != nil {
		log.Printf("Error while searching for account: %v", err)
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("Account not found for %s", txnData.Account)
	}
	accountId := accounts[0]["id"].(string)
	log.Printf("Account found: %v", accountId)

	// search for payee
	payeeQueryMap := map[string]string{"name": txnData.Payee}
	payees, err := s.makePennywiseRequest("/api/payees/search", "GET", payeeQueryMap, nil)
	if err != nil {
		log.Printf("Error while searching for payee: %v", err)
		return nil, err
	}
	if len(payees) == 0 {
		return nil, fmt.Errorf("Payee not found for %s", txnData.Payee)
	}
	payeeId := payees[0]["id"].(string)
	log.Printf("Payee found: %v", payeeId)

	// search for category
	catQueryMap := map[string]string{"name": txnData.Category}
	categories, err := s.makePennywiseRequest("/api/categories/search", "GET", catQueryMap, nil)
	if err != nil {
		log.Printf("Error while searching for category: %v", err)
		return nil, err
	}
	if len(categories) == 0 {
		return nil, fmt.Errorf("Category not found %s", txnData.Category)
	}
	catId := categories[0]["id"].(string)
	log.Printf("Category found: %v", catId)

	newTxn := Transaction{
		Date:       txnData.Date,
		Amount:     txnData.Amount,
		AccountId:  accountId,
		PayeeId:    payeeId,
		CategoryId: catId,
		Source:     "MLP",
		Note:       "",
	}

	res, err := s.makePennywiseRequest("/api/transactions", "POST", nil, newTxn)
	if err != nil {
		return nil, fmt.Errorf("Error while creating new transaction %s", err.Error())
	}
	log.Printf("Transaction created: %v", res)
	var txn Transaction
	resBytes, err := json.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("Error while marshaling transaction response %s", err.Error())
	}
	if err := json.Unmarshal(resBytes, &txn); err != nil {
		return nil, fmt.Errorf("Error while unmarshaling transaction response %s", err.Error())
	}
	return &txn, nil
}

func (s *Service) CreatePrediction(parsedDetails *parser.EmailDetails, predictedFields *prediction.PredictedFields, txnData *Transaction) error {
	predictionReq := PredictionReq{
		TransactionId: txnData.ID,
		Amount:        txnData.Amount,
		EmailText:     parsedDetails.Text,
		Account:       predictedFields.Account.Label,
	}
	if predictedFields.Account.Confidence != -1 {
		predictionReq.AccountPrediction = predictedFields.Account.Confidence
	}
	if predictedFields.Payee.Confidence != -1 {
		predictionReq.Payee = predictedFields.Payee.Label
		predictionReq.PayeePrediction = predictedFields.Payee.Confidence
	}
	if predictedFields.Category.Confidence != -1 {
		predictionReq.Category = predictedFields.Category.Label
		predictionReq.CategoryPrediction = predictedFields.Category.Confidence
	}
	res, err := s.makePennywiseRequest("/api/predictions", "POST", nil, predictionReq)
	if err != nil {
		return fmt.Errorf("Error while creating prediction %s", err.Error())
	}
	log.Printf("Prediction created %v", res)
	return nil
}
