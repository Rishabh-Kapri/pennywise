package model

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
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

// StatementInput is the statement.extract payload. ImportID is the
// statement_imports row the rows are saved onto. SealedPassword is the one
// typed at upload, sealed with the LLM_CREDENTIALS_KEY SecretBox (cipher holds
// the same key), so no plain password sits in the jobs table. The accounts'
// saved passwords are not in it: cipher loads them when the job runs, so a
// password saved after the upload counts on a retry.
type StatementInput struct {
	ImportID uuid.UUID `json:"importId"`
	// FileName is the PDF's name, for the job list.
	FileName       string `json:"fileName,omitempty"`
	Data           []byte `json:"data"`
	SealedPassword []byte `json:"sealedPassword,omitempty"`
	// Password is the typed password in plain text, from jobs queued before
	// it was sealed. Read only; nothing writes it.
	Password string `json:"password,omitempty"`
}

// StatementJobRef is the part of a statement.extract payload the job list
// reads. Decoding into it skips the PDF bytes instead of copying them.
type StatementJobRef struct {
	FileName string `json:"fileName"`
}

// StatementSavePayload is the statement.save payload: what statement.extract
// read, for the import it was queued for. OpenedWith is the account whose
// saved password opened the PDF, a strong hint whose statement it is.
type StatementSavePayload struct {
	ImportID   uuid.UUID          `json:"importId"`
	Sections   []StatementSection `json:"sections"`
	OpenedWith *uuid.UUID         `json:"openedWith,omitempty"`
}

// StatementFetchPayload is the statement.fetch payload: the email whose PDFs
// to download, from which mailbox.
type StatementFetchPayload struct {
	Provider    string              `json:"provider"`
	MailAccount string              `json:"mailAccount"`
	MessageID   string              `json:"messageId"`
	Subject     string              `json:"subject,omitempty"`
	Attachments []MessageAttachment `json:"attachments"`
}

// StatementImportPayload is the statement.import payload: one fetched PDF.
// Key identifies the email attachment, so the same file isn't imported twice.
type StatementImportPayload struct {
	Key      string `json:"key"`
	FileName string `json:"fileName"`
	Data     []byte `json:"data"`
}

// StatementRow is one transaction line printed on a statement.
type StatementRow struct {
	// ID is assigned by the API once a row is accepted ("<section>-<n>") and is
	// what the create-missing request refers to.
	ID   string `json:"id"`
	Date string `json:"date"`
	// Time is the clock time printed with the date, "HH:MM" in 24 hours and
	// the bank's zone (narration.BankZone); "" when the row printed none.
	Time        string `json:"time,omitempty"`
	Description string `json:"description"`
	// Payee is the model's cleaned merchant/counterparty name, used as the
	// default payee when the row is created as a transaction.
	Payee     string             `json:"payee"`
	Amount    float64            `json:"amount"`
	Direction StatementDirection `json:"direction"`
	Balance   *float64           `json:"balance,omitempty"`
	Reference string             `json:"reference,omitempty"`
	Page      int                `json:"page,omitempty"`
	// Line is the row's record in a CSV export, 1-based: the file's lines
	// with empty ones skipped (as encoding/csv reads them), so a viewer can
	// point at it. 0 for a PDF.
	Line int `json:"line,omitempty"`
	// DirectionCorrected is set when the running balance contradicted the
	// model's debit/credit call and the API flipped it.
	DirectionCorrected bool `json:"directionCorrected,omitempty"`
	// RewardPoints is what a card statement printed for this row (negative
	// for a reversal); nil when it printed nothing.
	RewardPoints *int `json:"rewardPoints,omitempty"`
	// LinkedTransactionIDs are ledger transactions the user paired with this
	// row by hand, which matching then takes before anything else: the
	// payment first, then any difference booked to make up the amount (a
	// card credited 3,900 for a 3,882 transfer plus 18 of cashback).
	LinkedTransactionIDs []uuid.UUID `json:"linkedTransactionIds,omitempty"`

	// Read from the narration by the API when the rows are saved
	// (narration.ParseStatementRow). Description is then the cleaned text and
	// RawDescription what the statement printed, when they differ.
	RawDescription string `json:"rawDescription,omitempty"`
	Channel        string `json:"channel,omitempty"`  // UPI, CARD, NEFT, IMPS, NACH
	Merchant       string `json:"merchant,omitempty"` // key form, upper case
	VPA            string `json:"vpa,omitempty"`
	// ValueDate is the printed value date; ValueDateMismatch is set when it
	// is more than statementValueDateSlackDays from Date.
	ValueDate         string `json:"valueDate,omitempty"`
	ValueDateMismatch bool   `json:"valueDateMismatch,omitempty"`

	// ChargeKind is set for a bank's own charge: one of the StatementCharge*
	// kinds. Empty for an ordinary transaction.
	ChargeKind string `json:"chargeKind,omitempty"`
	// EMI is set for a card EMI line (an instalment, its principal or its
	// interest).
	EMI *StatementEMILine `json:"emi,omitempty"`
	// EMIOverride is the user's word on whether the row is part of an EMI the
	// budget already paid for, over what the text suggested: false moves a
	// row read as an instalment back to matching, true sets one aside.
	EMIOverride *bool `json:"emiOverride,omitempty"`
	// SuggestedCategoryID is a category for a missing row: what the budget
	// used for this text on an earlier statement, else for a charge one
	// picked by name from the budget's categories. Filled on read, not stored.
	SuggestedCategoryID *uuid.UUID `json:"suggestedCategoryId,omitempty"`
	// SuggestedPayeeName is the payee the budget used for this text on an
	// earlier statement (StatementPastChoice). Filled on read, not stored.
	SuggestedPayeeName string `json:"suggestedPayeeName,omitempty"`
	// Edited holds what the statement printed before the user corrected the
	// row (a misread amount or date); nil when it was never edited.
	Edited *StatementRowOriginal `json:"edited,omitempty"`
}

// StatementRowOriginal is a row's values as read, kept when the user edits it.
type StatementRowOriginal struct {
	Date        string             `json:"date"`
	Time        string             `json:"time,omitempty"`
	Description string             `json:"description"`
	Amount      float64            `json:"amount"`
	Direction   StatementDirection `json:"direction"`
}

// StatementRowEditReq corrects a row read wrong. Omitted fields stay.
type StatementRowEditReq struct {
	Date *string `json:"date,omitempty"`
	// Time is "HH:MM"; "" removes it.
	Time        *string             `json:"time,omitempty"`
	Description *string             `json:"description,omitempty"`
	Amount      *float64            `json:"amount,omitempty"`
	Direction   *StatementDirection `json:"direction,omitempty"`
}

// StatementPastChoice is the payee and category a budget gave a statement
// line with the same text before, from its most recent such transaction.
type StatementPastChoice struct {
	PayeeName  string     `json:"payeeName,omitempty"`
	CategoryID *uuid.UUID `json:"categoryId,omitempty"`
}

// Kinds of bank charges a statement row can be.
const (
	StatementChargeTax      = "tax"      // GST/IGST/CGST/SGST on a fee or interest
	StatementChargeInterest = "interest" // finance charges, EMI interest
	StatementChargeFee      = "fee"      // annual, late, over-limit, cash, markup fees
)

// StatementEMILine says which EMI a row belongs to and which part it is.
type StatementEMILine struct {
	// Part is "instalment" (principal and interest together), "principal"
	// or "interest".
	Part   string `json:"part"`
	Number int    `json:"number,omitempty"` // 3 in "EMI 3/12"
	Of     int    `json:"of,omitempty"`     // 12 in "EMI 3/12"
}

// StatementEMIPlan is one row of a card statement's EMI table.
type StatementEMIPlan struct {
	Description     string   `json:"description"`
	BookedOn        string   `json:"bookedOn,omitempty"` // YYYY-MM-DD
	Principal       *float64 `json:"principal,omitempty"`
	MonthlyAmount   *float64 `json:"monthlyAmount,omitempty"`
	InterestRate    *float64 `json:"interestRate,omitempty"` // percent a year
	TenureMonths    *int     `json:"tenureMonths,omitempty"`
	RemainingMonths *int     `json:"remainingMonths,omitempty"`
	Outstanding     *float64 `json:"outstanding,omitempty"` // principal left
}

// StatementChargeTotal adds up one kind of charge on a statement.
type StatementChargeTotal struct {
	Kind   string  `json:"kind"`
	Amount float64 `json:"amount"` // charged minus reversed
	Count  int     `json:"count"`
}

// SignedAmount is the row in ledger convention: inflows positive.
func (r StatementRow) SignedAmount() float64 {
	if r.Direction == StatementCredit {
		return r.Amount
	}
	return -r.Amount
}

// Statement kinds, as read from the statement's first page.
const (
	StatementKindCard = "card"
	StatementKindBank = "bank"
)

// StatementRewards is a card statement's reward points summary.
type StatementRewards struct {
	Opening  *int `json:"opening,omitempty"`
	Earned   *int `json:"earned,omitempty"`
	Redeemed *int `json:"redeemed,omitempty"`
	Expired  *int `json:"expired,omitempty"`
	Closing  *int `json:"closing,omitempty"`
}

// StatementSection is one account's block within a statement. A consolidated
// statement has several; most have one.
type StatementSection struct {
	AccountLabel string `json:"accountLabel"`
	// AccountNumber is as printed, usually masked ("XXXX XXXX XXXX 1234");
	// its last four digits pick the Pennywise account (accounts.suffix).
	AccountNumber string `json:"accountNumber"`
	// Kind is StatementKindCard or StatementKindBank; empty when the
	// statement didn't make it clear.
	Kind           string   `json:"kind,omitempty"`
	Currency       string   `json:"currency,omitempty"`
	PeriodStart    *string  `json:"periodStart,omitempty"`
	PeriodEnd      *string  `json:"periodEnd,omitempty"`
	StatementDate  *string  `json:"statementDate,omitempty"`
	OpeningBalance *float64 `json:"openingBalance,omitempty"`
	ClosingBalance *float64 `json:"closingBalance,omitempty"`
	// Card statements only.
	DueDate     *string            `json:"dueDate,omitempty"`
	TotalDue    *float64           `json:"totalDue,omitempty"`
	MinimumDue  *float64           `json:"minimumDue,omitempty"`
	CreditLimit *float64           `json:"creditLimit,omitempty"`
	Rewards     *StatementRewards  `json:"rewards,omitempty"`
	EMIPlans    []StatementEMIPlan `json:"emiPlans,omitempty"`
	Rows        []StatementRow     `json:"rows"`
	// AccountID is the Pennywise account this section reconciles against:
	// auto-mapped from the account number's last digits, or chosen by the user.
	AccountID *uuid.UUID `json:"accountId,omitempty"`
}

// StatementSectionSummary is a StatementSection without its rows. Its JSON
// names are the section's, so it decodes from a stored section.
type StatementSectionSummary struct {
	AccountLabel   string     `json:"accountLabel"`
	AccountNumber  string     `json:"accountNumber"`
	Kind           string     `json:"kind,omitempty"`
	Currency       string     `json:"currency,omitempty"`
	PeriodStart    *string    `json:"periodStart,omitempty"`
	PeriodEnd      *string    `json:"periodEnd,omitempty"`
	StatementDate  *string    `json:"statementDate,omitempty"`
	ClosingBalance *float64   `json:"closingBalance,omitempty"`
	DueDate        *string    `json:"dueDate,omitempty"`
	TotalDue       *float64   `json:"totalDue,omitempty"`
	MinimumDue     *float64   `json:"minimumDue,omitempty"`
	AccountID      *uuid.UUID `json:"accountId,omitempty"`
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
	// PasswordIndex is the index of the password that opened the PDF, -1
	// when it needed none.
	PasswordIndex int `json:"-"`
}

type StatementImportStatus string

const (
	StatementImportProcessing StatementImportStatus = "processing"
	StatementImportReady      StatementImportStatus = "ready"
	StatementImportFailed     StatementImportStatus = "failed"
)

// StatementImport is one uploaded statement and what was extracted from it.
// Extraction is the statement.extract job (JobID); retrying the import retries
// that job. The PDF itself is kept in the API's document store (FilePath) so
// the user can open it. Older imports have no job and no file: their text is
// kept instead (SourceText).
type StatementImport struct {
	ID       uuid.UUID             `json:"id"`
	BudgetID uuid.UUID             `json:"budgetId"`
	FileName string                `json:"fileName"`
	FileHash string                `json:"-"`
	Status   StatementImportStatus `json:"status"`
	Error    *string               `json:"error,omitempty"`
	JobID    *uuid.UUID            `json:"jobId,omitempty"`
	// Stage is what the job is doing now ("reading pages"), filled from the
	// job while the import is processing. Not stored.
	Stage            string             `json:"stage,omitempty"`
	PageCount        int                `json:"pageCount"`
	PagesProcessed   int                `json:"pagesProcessed"`
	DefaultAccountID *uuid.UUID         `json:"defaultAccountId,omitempty"`
	Sections         []StatementSection `json:"sections"`
	// DroppedRows counts rows the model returned that failed validation
	// (unparseable date, non-positive amount), so an incomplete extraction is
	// visible instead of silently short.
	DroppedRows int    `json:"droppedRows"`
	RowCount    int    `json:"rowCount"`
	SourceText  string `json:"-"`
	// FilePath is the PDF's key in the document store; empty for imports
	// uploaded before PDFs were kept. HasFile says whether there is one.
	FilePath string `json:"-"`
	HasFile  bool   `json:"hasFile"`
	// AccountIDs are the accounts its sections are mapped to (List only), so
	// a card's screen can show the statements read for it.
	AccountIDs []uuid.UUID `json:"accountIds,omitempty"`
	// Summary is each section without its rows (List only), so a list can say
	// whose statement it is, for when, and what's due.
	Summary []StatementSectionSummary `json:"summary,omitempty"`
	// Progress is how far reconciling it has got (List only, ready imports).
	Progress  *StatementProgress `json:"progress,omitempty"`
	CreatedAt time.Time          `json:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt"`
}

// StatementProgress counts a statement's rows and transactions by where they
// stand. Reconciled: every section has an account and nothing is missing or
// only in the ledger.
type StatementProgress struct {
	Missing    int  `json:"missing"`
	Matched    int  `json:"matched"`
	LedgerOnly int  `json:"ledgerOnly"`
	Covered    int  `json:"covered"`
	Unmapped   int  `json:"unmapped"`
	Reconciled bool `json:"reconciled"`
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
	// Refs are the references its email/SMS sources stated; a statement row
	// printing one of them is the same payment.
	Refs []string `json:"-"`
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
	// Linked: the user paired them (StatementRow.LinkedTransactionIDs).
	// Others are the further transactions linked to the row, such as a
	// booked difference; Kind compares the row with all of them together.
	Linked bool                 `json:"linked,omitempty"`
	Others []StatementLedgerTxn `json:"others,omitempty"`
	// OtherRows are further rows linked to the same transactions: several
	// EMI lines paid by one transaction. Kind and Difference compare all the
	// rows with all the transactions.
	OtherRows []StatementRow `json:"otherRows,omitempty"`
	// Difference is the row's amount minus the linked transactions', for a
	// linked match that doesn't add up yet; 0 otherwise.
	Difference float64 `json:"difference,omitempty"`
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
	Kind           string                 `json:"kind,omitempty"`
	Currency       string                 `json:"currency,omitempty"`
	AccountID      *uuid.UUID             `json:"accountId,omitempty"`
	AccountName    string                 `json:"accountName,omitempty"`
	PeriodStart    string                 `json:"periodStart"`
	PeriodEnd      string                 `json:"periodEnd"`
	OpeningBalance *float64               `json:"openingBalance,omitempty"`
	ClosingBalance *float64               `json:"closingBalance,omitempty"`
	BalanceCheck   *StatementBalanceCheck `json:"balanceCheck,omitempty"`
	StatementDate  *string                `json:"statementDate,omitempty"`
	// Card statements only.
	DueDate     *string            `json:"dueDate,omitempty"`
	TotalDue    *float64           `json:"totalDue,omitempty"`
	MinimumDue  *float64           `json:"minimumDue,omitempty"`
	CreditLimit *float64           `json:"creditLimit,omitempty"`
	Rewards     *StatementRewards  `json:"rewards,omitempty"`
	EMIPlans    []StatementEMIPlan `json:"emiPlans,omitempty"`
	// RewardPointsOnRows adds up the points printed against rows; nil when
	// no row printed any.
	RewardPointsOnRows *int                   `json:"rewardPointsOnRows,omitempty"`
	Charges            []StatementChargeTotal `json:"charges,omitempty"`
	// Cashback is what the statement paid back (credits minus reversals);
	// nil when no row is cashback.
	Cashback *StatementChargeTotal `json:"cashback,omitempty"`
	RowCount int                   `json:"rowCount"`
	Matched  []StatementMatch      `json:"matched"`
	// Missing are statement rows with no ledger transaction.
	Missing []StatementRow `json:"missing"`
	// LedgerOnly are ledger transactions inside the statement period that do
	// not appear on it.
	LedgerOnly []StatementLedgerTxn `json:"ledgerOnly"`
	// Covered are EMI instalment rows on a card statement. An EMI bought on
	// a card is usually budgeted as paid in full when it was bought, so its
	// instalments aren't missing transactions.
	Covered []StatementRow `json:"covered"`
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
	// Note is the new transaction's memo; empty leaves it blank.
	Note string `json:"note,omitempty"`
}

type StatementCreateReq struct {
	Items []StatementCreateItem `json:"items"`
}

// StatementLinkReq adds ledger transactions to a statement row's links: one
// payment split over several transactions, or several payments in one row.
// TransactionID is the single-transaction form older clients send.
type StatementLinkReq struct {
	TransactionIDs []uuid.UUID `json:"transactionIds"`
	TransactionID  uuid.UUID   `json:"transactionId"`
}

// IDs is the request's transactions, in order, without nil or repeats.
func (r StatementLinkReq) IDs() []uuid.UUID {
	var ids []uuid.UUID
	for _, id := range append(r.TransactionIDs, r.TransactionID) {
		if id != uuid.Nil && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids
}

// StatementLinksReq links rows and transactions together: every row gets
// every transaction. One transaction paying several EMI lines, or one row
// paid in parts.
type StatementLinksReq struct {
	RowIDs         []string    `json:"rowIds"`
	TransactionIDs []uuid.UUID `json:"transactionIds"`
}

// StatementRowEMIReq says whether a row is part of an already-paid EMI; null
// takes back the user's word and leaves it to the row's text.
type StatementRowEMIReq struct {
	EMI *bool `json:"emi"`
}

// StatementDifferenceReq books what a linked row's transactions are short of
// the row as its own transaction on the section's account.
type StatementDifferenceReq struct {
	PayeeID    *uuid.UUID `json:"payeeId,omitempty"`
	PayeeName  string     `json:"payeeName,omitempty"`
	CategoryID *uuid.UUID `json:"categoryId,omitempty"`
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

// StatementTextKey is a statement line's text reduced to its lower-case
// letters, so the same line on another statement (other date, other
// reference) has the same key. "" when too little is left to tell lines
// apart. PastStatementChoices computes the same key in SQL.
func StatementTextKey(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		if r >= 'a' && r <= 'z' {
			b.WriteRune(r)
		}
	}
	if b.Len() < 6 {
		return ""
	}
	return b.String()
}

// statementTimeRe finds a clock time: "20:49", "20:49:05", "8:49 PM".
var statementTimeRe = regexp.MustCompile(`(?i)\b(\d{1,2}):(\d{2})(?::(\d{2}))?\s*([ap]\.?m\.?)?(?:\b|$)`)

// ParseStatementTime reads the first clock time in s ("06/08/2026| 20:49",
// "03/09/2026 04:45:49 PM") as "HH:MM" in 24 hours, or "" when s has none
// or it isn't a real time.
func ParseStatementTime(s string) string {
	m := statementTimeRe.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	h, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	if ampm := strings.ToLower(strings.ReplaceAll(m[4], ".", "")); ampm != "" {
		if h < 1 || h > 12 {
			return ""
		}
		if ampm == "pm" && h != 12 {
			h += 12
		} else if ampm == "am" && h == 12 {
			h = 0
		}
	}
	if h > 23 || min > 59 {
		return ""
	}
	return fmt.Sprintf("%02d:%02d", h, min)
}
