package model

import (
	"time"

	"github.com/google/uuid"
)

type Payee struct {
	ID                uuid.UUID  `json:"id"`
	Name              string     `json:"name"`
	BudgetID          uuid.UUID  `json:"budgetId"`
	TransferAccountID *uuid.UUID `json:"transferAccountId,omitempty"`
	Deleted           bool       `json:"deleted"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type PayeeRule struct {
	ID          uuid.UUID  `json:"id"`
	BudgetID    uuid.UUID  `json:"budgetId"`
	PayeeID     uuid.UUID  `json:"payeeId"`
	CategoryID  *uuid.UUID `json:"categoryId,omitempty"`
	MatchString string     `json:"matchString"`
	MatchType   string     `json:"matchType"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	Deleted     bool       `json:"deleted"`
}

type PayeeRuleDetails struct {
	ID           uuid.UUID  `json:"id"`
	BudgetID     uuid.UUID  `json:"budgetId"`
	PayeeID      uuid.UUID  `json:"payeeId"`
	CategoryID   *uuid.UUID `json:"categoryId,omitempty"`
	CategoryName *string    `json:"categoryName,omitempty"`
	PayeeName    *string    `json:"payeeName,omitempty"`
	MatchString  string     `json:"matchString"`
	MatchType    string     `json:"matchType"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// PayeeStats is per-payee usage across a budget's non-deleted transactions.
type PayeeStats struct {
	PayeeID          uuid.UUID  `json:"payeeId"`
	TransactionCount int        `json:"transactionCount"`
	TotalInflow      float64    `json:"totalInflow"`
	TotalOutflow     float64    `json:"totalOutflow"`
	LastUsed         *string    `json:"lastUsed,omitempty"`
	TopCategoryID    *uuid.UUID `json:"topCategoryId,omitempty"`
	RuleCount        int        `json:"ruleCount"`
}

type PayeeMergeRequest struct {
	TargetID  uuid.UUID   `json:"targetId" binding:"required"`
	SourceIDs []uuid.UUID `json:"sourceIds" binding:"required,min=1"`
}

// PayeeMergeResult counts the rows moved onto the merge target.
type PayeeMergeResult struct {
	TargetID     uuid.UUID `json:"targetId"`
	MergedPayees int       `json:"mergedPayees"`
	Transactions int64     `json:"transactions"`
	Rules        int64     `json:"rules"`
	Recurring    int64     `json:"recurring"`
	Embeddings   int64     `json:"embeddings"`
}
