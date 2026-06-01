package model

import (
	"time"

	"github.com/google/uuid"
)

type LoanMetadata struct {
	ID              uuid.UUID  `json:"id"`
	AccountID       uuid.UUID  `json:"accountId"`
	InterestRate    float64    `json:"interestRate"`
	OriginalBalance float64    `json:"originalBalance"`
	MonthlyPayment  float64    `json:"monthlyPayment"`
	LoanStartDate   string     `json:"loanStartDate"`
	CategoryID      *uuid.UUID `json:"categoryId,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}
