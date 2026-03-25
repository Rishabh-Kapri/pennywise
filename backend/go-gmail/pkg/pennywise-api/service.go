package pennywise

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"gmail-transactions/pkg/config"
	"gmail-transactions/pkg/logger"
	"gmail-transactions/pkg/parser"
	"gmail-transactions/pkg/prediction"
)

type Service struct {
	config *config.Config
	client *http.Client
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
	CategoryId            *string  `json:"categoryId,omitempty"`
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

func NewService(config *config.Config) *Service {
	return &Service{config: config, client: &http.Client{}}
}

// add query params to url
func (s *Service) getEncodedURL(path string, queryData map[string]string) (string, error) {
	pennywiseUrl := s.config.PennywiseApi
	if !strings.HasPrefix(pennywiseUrl, "http://") && !strings.HasPrefix(pennywiseUrl, "https://") {
		pennywiseUrl = "http://" + pennywiseUrl
	}
	baseUrl, err := url.Parse(pennywiseUrl)
	if err != nil {
		return "", err
	}
	baseUrl.Path += path
	params := url.Values{}
	for key, value := range queryData {
		params.Add(key, value)
	}
	baseUrl.RawQuery = params.Encode()

	return baseUrl.String(), nil
}

// makePennywiseApiRequest makes a request to Pennywise API
func (s *Service) makePennywiseRequest(ctx context.Context, endpoint string, method string, queryData map[string]string, data any) ([]map[string]any, error) {
	log := logger.Logger(ctx)

	url, err := s.getEncodedURL(endpoint, queryData)
	log.Info("making request", "url", url)
	if err != nil {
		log.Error("error encoding url", "endpoint", endpoint, "error", err)
		return nil, err
	}

	var requestBodyBytes []byte
	if data != nil {
		var err error
		requestBodyBytes, err = json.Marshal(data)
		if err != nil {
			log.Error("error marshaling JSON", "endpoint", endpoint, "error", err)
			return nil, err
		}
	} else {
		requestBodyBytes = []byte{}
	}

	requestBody := bytes.NewBuffer(requestBodyBytes)
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		log.Error("error creating pennywise api request", "endpoint", endpoint, "error", err)
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	// @TODO: add ability to take this from env
	req.Header.Set("X-Budget-ID", "2166418d-3fa2-4acc-b92c-ab9f36c18d76")

	// Forward correlation ID to downstream service
	if cid := logger.CorrelationIDFromContext(ctx); cid != "" {
		req.Header.Set("X-Correlation-ID", cid)
	}

	res, err := s.client.Do(req)
	if err != nil {
		log.Error("error sending pennywise api request", "endpoint", endpoint, "error", err)
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Error("error reading pennywise api response", "error", err)
		return nil, err
	}
	log.Info("response received from pennywise api", "endpoint", endpoint, "status", res.StatusCode)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("API error %v: %s", endpoint, res.Status)
	}
	var resArr []map[string]any
	var resObj map[string]any
	var response []map[string]any

	err = json.Unmarshal(body, &resArr)
	response = resArr
	if err != nil {
		err = json.Unmarshal(body, &resObj)
		if err != nil {
			return nil, fmt.Errorf("Error while unmarshaling pennywise api response: %v", err.Error())
		}
		response = []map[string]any{resObj}
	}
	return response, nil
}

func (s *Service) CreateTransaction(ctx context.Context, parsedDetails *parser.EmailDetails, predictedFields *prediction.PredictedFields) (*Transaction, error) {
	log := logger.Logger(ctx)
	txnData := ParsedTransaction{
		Amount:   parsedDetails.Amount,
		Date:     parsedDetails.Date,
		Payee:    predictedFields.Payee.Label,
		Account:  predictedFields.Account.Label,
		Category: predictedFields.Category.Label,
	}
	log.Info("creating transaction", "txnData", txnData)
	log.Info("predicted fields", "fields", predictedFields)

	accQueryMap := map[string]string{"name": txnData.Account}
	accounts, err := s.makePennywiseRequest(ctx, "/api/accounts/search", http.MethodGet, accQueryMap, nil)
	if err != nil {
		log.Error("error searching for account", "error", err)
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("Account not found for %s", txnData.Account)
	}
	accountId := accounts[0]["id"].(string)

	// search for payee
	payeeQueryMap := map[string]string{"name": txnData.Payee}
	payees, err := s.makePennywiseRequest(ctx, "/api/payees/search", http.MethodGet, payeeQueryMap, nil)
	if err != nil {
		log.Error("error searching for payee", "error", err)
		return nil, err
	}
	if len(payees) == 0 {
		return nil, fmt.Errorf("Payee not found for %s", txnData.Payee)
	}
	payeeId := payees[0]["id"].(string)

	// search for category
	var catIdPtr *string
	if txnData.Category != "null" && txnData.Category != "" {
		catQueryMap := map[string]string{"name": txnData.Category}
		categories, err := s.makePennywiseRequest(ctx, "/api/categories/search", http.MethodGet, catQueryMap, nil)
		if err != nil {
			log.Error("error searching for category", "error", err)
			return nil, err
		}
		if len(categories) == 0 {
			return nil, fmt.Errorf("Category not found %s", txnData.Category)
		}
		catId := categories[0]["id"].(string)
		catIdPtr = &catId
		log.Info("category found", "categoryId", catId)
	} else {
		log.Info("category is null")
	}

	newTxn := Transaction{
		Date:       txnData.Date,
		Amount:     txnData.Amount,
		AccountId:  accountId,
		PayeeId:    payeeId,
		CategoryId: catIdPtr,
		Source:     "MLP",
		Note:       "",
	}

	res, err := s.makePennywiseRequest(ctx, "/api/transactions", http.MethodPost, nil, newTxn)
	if err != nil {
		return nil, fmt.Errorf("Error while creating new transaction %s", err.Error())
	}
	log.Info("transaction created", "response", res)
	var txns []Transaction
	resBytes, err := json.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("Error while marshaling transaction response %s", err.Error())
	}
	if err := json.Unmarshal(resBytes, &txns); err != nil {
		return nil, fmt.Errorf("Error while unmarshaling transaction response %s", err.Error())
	}
	if len(txns) == 0 {
		return nil, fmt.Errorf("No transactions received")
	}
	return &txns[0], nil
}

func (s *Service) CreatePrediction(ctx context.Context, parsedDetails *parser.EmailDetails, predictedFields *prediction.PredictedFields, txnData *Transaction) error {
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
	res, err := s.makePennywiseRequest(ctx, "/api/predictions", "POST", nil, predictionReq)
	if err != nil {
		return fmt.Errorf("Error while creating prediction %s", err.Error())
	}
	logger.Logger(ctx).Info("prediction created", "response", res)
	return nil
}

func (s *Service) GetUserHistoryId(ctx context.Context, email string) (uint64, error) {
	log := logger.Logger(ctx)
	userQueryMap := map[string]string{"email": email}
	log.Info("getting user history id", "email", email)
	res, err := s.makePennywiseRequest(ctx, "/api/users/search", "GET", userQueryMap, nil)
	if err != nil {
		log.Error("error getting user history id", "error", err)
		return 0, err
	}
	if len(res) == 0 {
		return 0, fmt.Errorf("No user found with email %s", email)
	}
	historyId, ok := res[0]["historyId"].(float64)
	if !ok {
		return 0, fmt.Errorf("Unexpected type for historyId")
	}

	return uint64(historyId), nil
}

func (s *Service) UpdateUserHistoryId(ctx context.Context, email string, historyId uint64) error {
	userData := map[string]any{
		"email":     email,
		"historyId": historyId,
	}
	res, err := s.makePennywiseRequest(ctx, "/api/users", "PATCH", nil, userData)
	if err != nil {
		return err
	}
	logger.Logger(ctx).Info("user historyId updated", "response", res)
	return nil
}

func (s *Service) GetUserRefreshToken(ctx context.Context, email string) (string, error) {
	log := logger.Logger(ctx)
	userQueryMap := map[string]string{"email": email}
	log.Info("getting user refresh token", "email", email)
	res, err := s.makePennywiseRequest(ctx, "/api/users/search", "GET", userQueryMap, nil)
	if err != nil {
		log.Error("error getting user refresh token", "error", err)
		return "", err
	}
	if len(res) == 0 {
		return "", fmt.Errorf("No user found with email %s", email)
	}
	refreshToken, ok := res[0]["gmailRefreshToken"].(string)
	if !ok {
		return "", fmt.Errorf("Unexpected type for gmailRefreshToken")
	}

	return refreshToken, nil
}
