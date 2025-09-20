package prediction

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"gmail-transactions/pkg/config"
	"gmail-transactions/pkg/parser"
)

const CONFIDENCE_THRESHOLD = 0.7

type Service struct {
	config *config.Config
	client *http.Client
}

type PredictionResult struct {
	Label      string
	Confidence float64
}

type PredictedFields struct {
	Account  PredictionResult
	Payee    PredictionResult
	Category PredictionResult
}

func NewService(config *config.Config) *Service {
	return &Service{
		config: config,
		client: &http.Client{},
	}
}

func (s *Service) CallPredictApi(emailDetails *parser.EmailDetails, fieldType string) (*PredictionResult, error) {
	emailDetails.Type = fieldType
	url := s.config.MLPApi + "/predict"

	requestBody, err := json.Marshal(emailDetails)
	if err != nil {
		return nil, fmt.Errorf("CallPredictApi:Error marshalling request: %w", err)
	}

	var prediction PredictionResult
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("CallPredictApi:Error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("CallPredictApi:Error making request: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("CallPredictApi:Error reading response: %w", err)
	}
	err = json.Unmarshal(body, &prediction)
	if err != nil {
		return nil, fmt.Errorf("CallPredictApi:Error unmarshalling response: %w", err)
	}
	return &prediction, nil
}

func (s *Service) GetPredictedFields(parsedDetails *parser.EmailDetails, fallbackAccount string) (*PredictedFields, error) {
	predicted := &PredictedFields{
		Account: PredictionResult{
			Label:      fallbackAccount,
			Confidence: -1,
		},
		Payee: PredictionResult{
			Label:      "Unexpected",
			Confidence: -1,
		},
		Category: PredictionResult{
			Label:      "❗ Unexpected expenses",
			Confidence: -1,
		},
	}

	accountPrediction, err := s.CallPredictApi(parsedDetails, "account")
	if err != nil {
		return predicted, err
	}
	log.Printf("Predicted account: %v\n", accountPrediction)
	predicted.Account.Label = accountPrediction.Label
	predicted.Account.Confidence = accountPrediction.Confidence

	if accountPrediction.Confidence < CONFIDENCE_THRESHOLD {
		return predicted, nil
	}

	parsedDetails.Account = accountPrediction.Label

	payeePrediction, err := s.CallPredictApi(parsedDetails, "payee")
	if err != nil {
		return predicted, err
	}
	log.Printf("Predicted payee: %v\n", payeePrediction)
	predicted.Payee.Label = payeePrediction.Label
	predicted.Payee.Confidence = payeePrediction.Confidence

	if payeePrediction.Confidence < CONFIDENCE_THRESHOLD {
		predicted.Payee.Label = "Unexpected"
		return predicted, nil
	}

	parsedDetails.Payee = payeePrediction.Label

	categoryPrediction, err := s.CallPredictApi(parsedDetails, "category")
	if err != nil {
		return predicted, err
	}
	log.Printf("Predicted category: %v\n", categoryPrediction)
	predicted.Category.Label = categoryPrediction.Label
	predicted.Category.Confidence = categoryPrediction.Confidence

	if categoryPrediction.Confidence < CONFIDENCE_THRESHOLD {
		predicted.Category.Label = "❗ Unexpected expenses"
		return predicted, nil
	}

	return predicted, nil
}
