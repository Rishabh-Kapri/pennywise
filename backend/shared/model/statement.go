package model

import (
	"time"

	"github.com/google/uuid"
)

// Statement reconciliation: a bank or card statement PDF is turned into
// extracted rows (cipher, LLM) and compared against the ledger (API,
// deterministic). Nothing on the ledger is marked by a reconciliation — the
// comparison is recomputed on every read, so creating a missing transaction
// simply turns its row into a match.

type StatementDirection string

const (
	// StatementDebit is money leaving the account holder: a purchase on a
	// card, a withdrawal from a bank account.
	StatementDebit StatementDirection = "debit"
	// StatementCredit is money reaching the account holder: a deposit, a card
	// payment or refund.
	StatementCredit StatementDirection = "credit"
)

func (d StatementDirection) Valid() bool {
	return d == StatementDebit || d == StatementCredit
}

// StatementRow is one transaction line printed on a statement.
type StatementRow struct {
	// ID is assigned by the API once a row is accepted ("<section>-<n>") and is
	// what the create-missing request refers to.
	ID          string `json:"id"`
	Date        string `json:"date"`
	Description string `json:"description"`
	// Payee is the model's cleaned merchant/counterparty name, used as the
	// default payee when the row is created as a transaction.
	Payee     string             `json:"payee"`
	Amount    float64            `json:"amount"`
	Direction StatementDirection `json:"direction"`
	Balance   *float64           `json:"balance,omitempty"`
	Reference string             `json:"reference,omitempty"`
	Page      int                `json:"page,omitempty"`
	// DirectionCorrected is set when the running balance contradicted the
	// model's debit/credit call and the API flipped it.
	DirectionCorrected bool `json:"directionCorrected,omitempty"`
}

// SignedAmount is the row in ledger convention: inflows positive.
func (r StatementRow) SignedAmount() float64 {
	if r.Direction == StatementCredit {
		return r.Amount
	}
	return -r.Amount
}

// StatementSection is one account's block within a statement. A consolidated
// statement has several; most have one.
type StatementSection struct {
	AccountLabel   string         `json:"accountLabel"`
	AccountNumber  string         `json:"accountNumber"`
	Currency       string         `json:"currency,omitempty"`
	PeriodStart    *string        `json:"periodStart,omitempty"`
	PeriodEnd      *string        `json:"periodEnd,omitempty"`
	OpeningBalance *float64       `json:"openingBalance,omitempty"`
	ClosingBalance *float64       `json:"closingBalance,omitempty"`
	Rows           []StatementRow `json:"rows"`
	// AccountID is the Pennywise account this section reconciles against:
	// auto-mapped from the account number's last digits, or chosen by the user.
	AccountID *uuid.UUID `json:"accountId,omitempty"`
}

// StatementExtractRequest is the API → cipher call for one chunk of pages.
type StatementExtractRequest struct {
	Text      string `json:"text"`
	FirstPage int    `json:"firstPage"`
	LastPage  int    `json:"lastPage"`
	// PreviousSection names the account the previous chunk ended in, so rows
	// on a continuation page without a header are attributed to it.
	PreviousSection string `json:"previousSection,omitempty"`
}

type StatementExtractResponse struct {
	Sections []StatementSection `json:"sections"`
}

type StatementImportStatus string

const (
	StatementImportProcessing StatementImportStatus = "processing"
	StatementImportReady      StatementImportStatus = "ready"
	StatementImportFailed     StatementImportStatus = "failed"
)

// StatementImport is one uploaded statement and what was extracted from it.
// The PDF itself is not kept; its text is, so a failed extraction can be
// retried without a re-upload (and without the password again).
type StatementImport struct {
	ID               uuid.UUID             `json:"id"`
	BudgetID         uuid.UUID             `json:"budgetId"`
	FileName         string                `json:"fileName"`
	FileHash         string                `json:"-"`
	Status           StatementImportStatus `json:"status"`
	Error            *string               `json:"error,omitempty"`
	PageCount        int                   `json:"pageCount"`
	PagesProcessed   int                   `json:"pagesProcessed"`
	DefaultAccountID *uuid.UUID            `json:"defaultAccountId,omitempty"`
	Sections         []StatementSection    `json:"sections"`
	// DroppedRows counts rows the model returned that failed validation
	// (unparseable date, non-positive amount), so an incomplete extraction is
	// visible instead of silently short.
	DroppedRows int       `json:"droppedRows"`
	RowCount    int       `json:"rowCount"`
	SourceText  string    `json:"-"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// StatementLedgerTxn is the slice of a ledger transaction a reconciliation
// view needs.
type StatementLedgerTxn struct {
	ID           uuid.UUID         `json:"id"`
	Date         string            `json:"date"`
	Amount       float64           `json:"amount"`
	PayeeName    string            `json:"payeeName"`
	CategoryName string            `json:"categoryName,omitempty"`
	Note         string            `json:"note"`
	Status       TransactionStatus `json:"status"`
}

type StatementMatchKind string

const (
	// StatementMatchExact: same amount, dates within the match window.
	StatementMatchExact StatementMatchKind = "exact"
	// StatementMatchAmountDiffers: close date and similar text, but the amount
	// is off by a small margin (typically FX conversion or a tip).
	StatementMatchAmountDiffers StatementMatchKind = "amount_differs"
)

type StatementMatch struct {
	Row          StatementRow       `json:"row"`
	Transaction  StatementLedgerTxn `json:"transaction"`
	Kind         StatementMatchKind `json:"kind"`
	DateDiffDays int                `json:"dateDiffDays"`
}

// StatementBalanceCheck compares the statement's own opening/closing balance
// with the sum of the extracted rows — a missed or misread row shows up here.
type StatementBalanceCheck struct {
	Computed   float64 `json:"computed"`
	Reported   float64 `json:"reported"`
	Difference float64 `json:"difference"`
	OK         bool    `json:"ok"`
}

type StatementSectionReconciliation struct {
	Index          int                    `json:"index"`
	AccountLabel   string                 `json:"accountLabel"`
	AccountNumber  string                 `json:"accountNumber"`
	Currency       string                 `json:"currency,omitempty"`
	AccountID      *uuid.UUID             `json:"accountId,omitempty"`
	AccountName    string                 `json:"accountName,omitempty"`
	PeriodStart    string                 `json:"periodStart"`
	PeriodEnd      string                 `json:"periodEnd"`
	OpeningBalance *float64               `json:"openingBalance,omitempty"`
	ClosingBalance *float64               `json:"closingBalance,omitempty"`
	BalanceCheck   *StatementBalanceCheck `json:"balanceCheck,omitempty"`
	RowCount       int                    `json:"rowCount"`
	Matched        []StatementMatch       `json:"matched"`
	// Missing are statement rows with no ledger transaction.
	Missing []StatementRow `json:"missing"`
	// LedgerOnly are ledger transactions inside the statement period that do
	// not appear on it.
	LedgerOnly []StatementLedgerTxn `json:"ledgerOnly"`
}

type StatementReconciliation struct {
	Import   StatementImport                  `json:"import"`
	Sections []StatementSectionReconciliation `json:"sections"`
}

// StatementSectionAccountReq maps a section to a ledger account (nil clears).
type StatementSectionAccountReq struct {
	AccountID *uuid.UUID `json:"accountId"`
}

type StatementCreateItem struct {
	RowID string `json:"rowId"`
	// PayeeID wins over PayeeName. With neither, the row's suggested payee is
	// used; a name without an existing payee creates one.
	PayeeID    *uuid.UUID `json:"payeeId,omitempty"`
	PayeeName  string     `json:"payeeName,omitempty"`
	CategoryID *uuid.UUID `json:"categoryId,omitempty"`
}

type StatementCreateReq struct {
	Items []StatementCreateItem `json:"items"`
}

type StatementCreateFailure struct {
	RowID string `json:"rowId"`
	Error string `json:"error"`
}

type StatementCreateResult struct {
	Created []uuid.UUID              `json:"created"`
	Skipped []string                 `json:"skipped"`
	Failed  []StatementCreateFailure `json:"failed"`
}
