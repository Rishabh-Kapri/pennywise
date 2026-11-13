package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Payloads and results of the message pipeline's jobs. One message moves
// through them in order, each job carrying what the steps before it found:
//
//	message.received ──► message.extract ──► message.verify ──► message.lookup
//	  (is it a txn?)                                              │ hit   │ miss
//	                                                              │       ▼
//	                                                              │  message.predict
//	                                                              ▼       │
//	                              pipeline.report ◄── transaction.create ◄┘
//
// Any step that ends the message early goes straight to pipeline.report. Check
// and verify can also park it there for a person (needs_review); continuing
// queues the step after the one that parked.

// MessageOutcome is how a message's trip through the pipeline ended.
type MessageOutcome string

const (
	MessageOutcomeNotTransaction MessageOutcome = "not_transaction"
	MessageOutcomeUnverified     MessageOutcome = "unverified"
	MessageOutcomeNoPrediction   MessageOutcome = "no_prediction"
	// MessageOutcomeNeedsReview: a step wasn't sure, so the message waits for a
	// person. Its report carries a MessageReview.
	MessageOutcomeNeedsReview        MessageOutcome = "needs_review"
	MessageOutcomeTransactionCreated MessageOutcome = "transaction_created"
	// MessageOutcomeTransactionDuplicate: the message matched a transaction that
	// already existed, so nothing new was added.
	MessageOutcomeTransactionDuplicate MessageOutcome = "transaction_duplicate"
)

// MessagePredictionSource is the step that produced a prediction.
type MessagePredictionSource string

const (
	MessagePredictionMemory    MessagePredictionSource = "memory"
	MessagePredictionPredictor MessagePredictionSource = "predictor"
)

type InboundMessageType string

const (
	TransactionMessageType InboundMessageType = "transaction"
	StatementMessageType   InboundMessageType = "statement"
)

// MessageVerdict is the answer of the "is it a transaction" step.
type MessageVerdict struct {
	MessageType   InboundMessageType `json:"messageType"`
	IsTransaction bool               `json:"isTransaction"`
	Type          string             `json:"type"` // debit | credit
	Confidence    *float64           `json:"confidence,omitempty"`
	Model         string             `json:"model,omitempty"` // the model that answered
}

// MessageEntities are the fields extracted from a message.
type MessageEntities struct {
	Account  string    `json:"account"`
	Merchant string    `json:"merchant"`
	Amount   float64   `json:"amount"`
	Date     time.Time `json:"date"`
	Currency string    `json:"currency,omitempty"`
	// The verify step adds these. Blank means it couldn't tell.
	Channel string `json:"channel,omitempty"` // UPI, CARD, NEFT, IMPS or NACH
	VPA     string `json:"vpa,omitempty"`
	Ref     string `json:"ref,omitempty"` // UPI RRN, IMPS ref, NEFT UTR: unique per payment
	// VerifiedConfidence is how sure verify was that the details match the
	// message, and VerifiedBy the model that said so.
	VerifiedConfidence *float64 `json:"verifiedConfidence,omitempty"`
	VerifiedBy         string   `json:"verifiedBy,omitempty"`
}

// MessagePrediction is the payee and category chosen for a message.
type MessagePrediction struct {
	PayeeName    string                  `json:"payeeName"`
	CategoryName string                  `json:"categoryName"`
	Confidence   float64                 `json:"confidence"` // in the category
	Source       MessagePredictionSource `json:"source"`
	// PayeeConfidence is set when the step scored the payee on its own.
	PayeeConfidence *float64 `json:"payeeConfidence,omitempty"`
	Reasoning       string   `json:"reasoning,omitempty"`
	Model           string   `json:"model,omitempty"`
}

// MessageExtractPayload is the payload of JobTypeMessageExtract.
type MessageExtractPayload struct {
	Message ReceivedMessage `json:"message"`
	Verdict MessageVerdict  `json:"verdict"`
}

// MessageEntitiesPayload is the payload of JobTypeMessageVerify,
// JobTypeMessageLookup and JobTypeMessagePredict.
type MessageEntitiesPayload struct {
	Message  ReceivedMessage `json:"message"`
	Verdict  MessageVerdict  `json:"verdict"`
	Entities MessageEntities `json:"entities"`
}

// TransactionCreatePayload is the payload of JobTypeTransactionCreate.
type TransactionCreatePayload struct {
	Message    ReceivedMessage   `json:"message"`
	Verdict    MessageVerdict    `json:"verdict"`
	Entities   MessageEntities   `json:"entities"`
	Prediction MessagePrediction `json:"prediction"`
	// Source is what the bank text itself states, for learning.
	Source *MessageSource `json:"source,omitempty"`
	// DisplayText is the message as a person reads it, for a review.
	DisplayText string `json:"displayText,omitempty"`
	// AllowDuplicate is a person saying this isn't the payment it looked like,
	// so the match on another channel's message is skipped.
	AllowDuplicate bool `json:"allowDuplicate,omitempty"`
}

// MessageSource is the bank text behind a transaction and what the pipeline
// read from it. Blank fields mean it couldn't tell. Merchant is the lookup key's
// merchant (narration.LookupMerchant), so a UPI source holds its VPA there.
type MessageSource struct {
	// Kind is transaction_sources.kind. Blank for messages that aren't bank
	// text from a mailbox or phone (the API), which get no source row.
	Kind        string     `json:"kind,omitempty"`
	Text        string     `json:"text"`
	Channel     string     `json:"channel,omitempty"`
	Merchant    string     `json:"merchant,omitempty"`
	VPA         string     `json:"vpa,omitempty"`
	Ref         string     `json:"ref,omitempty"`
	AccountHint string     `json:"accountHint,omitempty"`
	Direction   string     `json:"direction,omitempty"`
	Amount      string     `json:"amount,omitempty"` // decimal text, e.g. "1196.52"
	Currency    string     `json:"currency,omitempty"`
	OccurredOn  *time.Time `json:"occurredOn,omitempty"`
	OccurredAt  *time.Time `json:"occurredAt,omitempty"`
}

// CounterpartyAliasMatch is what a lookup key resolves to.
type CounterpartyAliasMatch struct {
	PayeeName    string
	CategoryName string
	Source       string // learning_source: AliasSourceManual or AliasSourceLearned
}

// Who made a counterparty alias (counterparty_aliases.learning_source).
const (
	AliasSourceManual  = "MANUAL"
	AliasSourceLearned = "AUTO_LEARNED"
)

// SourceMatchQuery looks for a transaction that another channel's message
// already created: same account and signed amount, a source of the other kind
// on the same day. Text is this message's, so a retry finds what it attached.
type SourceMatchQuery struct {
	BudgetID  uuid.UUID
	AccountID uuid.UUID
	Amount    float64
	Kind      string // this message's kind: EMAIL or SMS
	On        time.Time
	Text      string
}

// SourceMatchCandidate is a transaction SourceMatchQuery found, and when its
// other-channel message says the payment happened (nil without a time).
type SourceMatchCandidate struct {
	TransactionID uuid.UUID
	PayeeID       *uuid.UUID
	CategoryID    *uuid.UUID
	CreatedAt     time.Time
	OccurredAt    *time.Time
}

// IsUncategorized reports whether a category name is the catch-all, which says
// nothing worth learning about a merchant.
func IsUncategorized(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), "Uncategorized")
}

// LookupKey is the "CHANNEL | MERCHANT" a transaction's bank text gave.
type LookupKey struct {
	Channel  string
	Merchant string
}

// CounterpartyAliasConfirm is the user saying who a lookup key is.
type CounterpartyAliasConfirm struct {
	BudgetID   uuid.UUID
	Channel    string
	Merchant   string
	PayeeID    uuid.UUID
	CategoryID *uuid.UUID
}

// CounterpartyAliasLearn is one sighting of a lookup key.
type CounterpartyAliasLearn struct {
	BudgetID   uuid.UUID
	Channel    string
	Merchant   string
	PayeeID    uuid.UUID
	CategoryID *uuid.UUID
}

// PipelineReportPayload is the payload of JobTypePipelineReport: how one
// message ended, for the pipeline run history.
type PipelineReportPayload struct {
	Message ReceivedMessage `json:"message"`
	Outcome MessageOutcome  `json:"outcome"`
	Reason  string          `json:"reason,omitempty"`
	// TransactionIDs is set when the outcome is MessageOutcomeTransactionCreated
	// or MessageOutcomeTransactionDuplicate.
	TransactionIDs []uuid.UUID `json:"transactionIds,omitempty"`
	// Review is set when the outcome is MessageOutcomeNeedsReview.
	Review *MessageReview `json:"review,omitempty"`
	// Learn queues transaction.learn once the outcome is recorded.
	Learn *TransactionLearnPayload `json:"learn,omitempty"`
}

// MessageReview is what a person needs to decide about a parked message.
type MessageReview struct {
	Step       string   `json:"step"` // the step that wasn't sure: check, verify or create
	Confidence *float64 `json:"confidence,omitempty"`
	// Text is the message as a person reads it, keeping its line breaks.
	Text     string           `json:"text"`
	Entities *MessageEntities `json:"entities,omitempty"`
	// Unsure names the entities to double-check: amount, account, date, merchant.
	Unsure []string `json:"unsure,omitempty"`
	// Resume is the job that carries on if the person says continue. Nil when
	// the message lacks what a transaction needs, so it can only be entered by
	// hand or dismissed.
	Resume *MessageResume `json:"resume,omitempty"`
}

// MessageResume is the next job of a parked message, ready to queue. Its payload
// has its own RunID, because the rest of the run would otherwise share keys with
// the jobs of the run that parked.
type MessageResume struct {
	JobType        string          `json:"jobType"`
	Queue          string          `json:"queue"`
	IdempotencyKey string          `json:"idempotencyKey"`
	Payload        json.RawMessage `json:"payload"`
}

// MessageStepResult is the result stored on a pipeline job: what it decided
// and which job, if any, it queued next.
type MessageStepResult struct {
	// Output is what this step produced: the verdict, the extracted details,
	// the prediction. Its shape depends on the step.
	Output      any            `json:"output,omitempty"`
	NextJobType string         `json:"nextJobType,omitempty"`
	NextJobID   *uuid.UUID     `json:"nextJobId,omitempty"`
	Outcome     MessageOutcome `json:"outcome,omitempty"`
}

// TransactionCreateResult is what transaction.create produced.
type TransactionCreateResult struct {
	TransactionIDs []uuid.UUID `json:"transactionIds"`
	// Created is false when the transaction was already there.
	Created bool `json:"created"`
	// Duplicate: it was there before this job started, so this message added
	// nothing. A retry finding the row its own earlier attempt saved isn't one.
	Duplicate bool `json:"duplicate,omitempty"`
	// ExistingSince is when an already-present transaction was saved.
	ExistingSince time.Time `json:"-"`
	// PossibleDuplicateOf is a transaction from another channel that may be
	// this payment, when the times needed to be sure are missing. Nothing was
	// created; the message waits for a person.
	PossibleDuplicateOf *uuid.UUID `json:"possibleDuplicateOf,omitempty"`
	// Learn is what to learn from it. Not stored with the job's result.
	Learn *TransactionLearnPayload `json:"-"`
}

// TransactionLearnPayload is the payload of JobTypeTransactionLearn.
type TransactionLearnPayload struct {
	TransactionID uuid.UUID  `json:"transactionId"`
	PayeeID       uuid.UUID  `json:"payeeId"`
	CategoryID    *uuid.UUID `json:"categoryId,omitempty"`
	// Source is the bank text the pipeline read. Nil when the user is
	// correcting or approving a transaction.
	Source *MessageSource `json:"source,omitempty"`
	// Confirmed: this came from the user, so it replaces what the alias says.
	Confirmed bool `json:"confirmed,omitempty"`
}

// IdempotencyKey identifies msg within a budget. Every pipeline job for the
// message uses it: the dedupe index includes the job type, so a retried step
// re-queues nothing and one key can't collide across steps.
func (m ReceivedMessage) IdempotencyKey(budgetID uuid.UUID) string {
	key := fmt.Sprintf("%s:%s:%s:%s", budgetID, m.Source, m.Provider, m.ExternalID)
	if m.RunID != "" {
		key += ":run:" + m.RunID
	}
	return key
}
