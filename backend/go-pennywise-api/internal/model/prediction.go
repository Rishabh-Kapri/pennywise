package model

import "time"

type Prediction struct {
	ID                    string    `json:"id"`
	EmailText             string    `json:"emailText"`
	Account               string    `json:"account"`
	AccountPrediction     int       `json:"accountPrediction"`
	Payee                 string    `json:"payee"`
	PayeePrediction       int       `json:"payeePrediction"`
	Category              string    `json:"category"`
	CategoryPrediction    int       `json:"categoryPrediction"`
	HasUserCorrected      bool      `json:"hasUserCorrected"`
	UserCorrectedPayee    string    `json:"userCorrectedPayee"`
	UserCorrectedAccount  string    `json:"userCorrectedAccount"`
	UserCorrectedCategory string    `json:"userCorrectedCategory"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}
