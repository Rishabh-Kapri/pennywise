// adapter using the decisions api endpoints
package decisionsAdapter

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/decisions"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils/narration"
)

type adapter struct {
	client decisions.Decisions
	model  string
}

// askTimeout limits one call. Decision models answer quickly, so a slow one
// should fail soon enough for the next model in a chain to run.
const askTimeout = 15 * time.Second

type DecisionAdapter interface {
	ports.CheckTransaction
	ports.Verifier
}

func NewDecisionsAdapter(c decisions.Decisions, model string) DecisionAdapter {
	return &adapter{client: c, model: model}
}

// Check implements [ports.CheckTransaction].
func (d *adapter) Check(ctx context.Context, msg ports.Message) (ports.TxnVerdict, error) {
	questions := map[string]sharedModel.Intent{
		"messageType": {
			Type:         sharedModel.ChoiceIntentType,
			Instructions: "What kind of message this is?",
			Criteria: map[string]*string{
				"transaction": new(
					"A direct bank or card issuer alert reporting a completed debit, credit, purchase, transfer, or refund on the customer's account or card.",
				),
				"statement": new(
					"A direct bank or card issuer message reporting a statement for partticular account or card.",
				),
				"none": new(
					"When message is none of transaction or statement",
				),
			},
		},
		"isTransaction": {
			Type:         sharedModel.NoulIntentType,
			Instructions: "Should this text create a transaction from a bank account or card alert?",
			Criteria: map[string]*string{
				"true": new(
					"A direct bank or card issuer alert reporting a completed debit, credit, purchase, transfer, or refund on the customer's account or card.",
				),
				"false": new(
					"A newsletter, merchant receipt, account or card notice, OTP texts, upcoming payment notice, or e-mandate/auto-payment confirmation. Exclude e-mandate confirmations even when they say 'successfully paid'; a separate bank alert is used for that payment.",
				),
			},
		},
		"transactionType": {
			Type:         sharedModel.ChoiceIntentType,
			Instructions: "For the completed transaction reported in this text, did money move out of or into the customer's finances? Use the purchase or payment statement. Ignore card product names and fraud, dispute, and help instructions.",
			Criteria: map[string]*string{
				"debit": new(
					`Money moved out: a purchase, card spend, withdrawal, or payment to a merchant. 'Used your card for Rs. X at MERCHANT' is debit even when it says 'credit card'.`,
				),
				"credit": new(
					`Money moved in: an account credit, incoming transfer, refund, cashback, or reversal of a previous charge.`,
				),
				"none": new(
					`No completed movement is reported, or its direction cannot be determined. Includes upcoming or paid e-mandates and payment reminders.`,
				),
			},
		},
	}
	req := sharedModel.DecisionRequest{
		State: sharedModel.State{
			Query: msg.Body,
		},
		Model:     d.model,
		Questions: questions,
	}
	ctx, cancel := context.WithTimeout(ctx, askTimeout)
	defer cancel()
	resp, err := d.client.Ask(ctx, req)
	if err != nil {
		return ports.TxnVerdict{}, err
	}
	res := resp.Answers

	ans, ok := res["isTransaction"]

	if !ok || ans.Noul == nil {
		return ports.TxnVerdict{}, errs.NewWith(
			errs.CodeDecisionsAnswerMissing, "no noul answer for isTransaction",
			errs.WithDetails(map[string]any{
				"question":   "isTransaction",
				"answerType": string(ans.Type),
			}),
		)
	}

	p := *ans.Noul

	typeAns, ok := res["transactionType"]
	if !ok || typeAns.Choice == nil {
		return ports.TxnVerdict{}, errs.NewWith(
			errs.CodeDecisionsAnswerMissing, "no choice answer for transactionType",
			errs.WithDetails(map[string]any{
				"question":   "transactionType",
				"answerType": string(typeAns.Type),
			}),
		)
	}

	messageTypeAns, ok := res["messageType"]
	if !ok || messageTypeAns.Choice == nil {
		return ports.TxnVerdict{}, errs.NewWith(
			errs.CodeDecisionsAnswerMissing, "no choice answer for messageType",
			errs.WithDetails(map[string]any{
				"question":   "messageType",
				"answerType": string(messageTypeAns.Type),
			}),
		)
	}

	confidence := ans.Confidence
	if confidence == nil {
		// How far the answer is from a coin toss, for models that send no score.
		confidence = new(math.Max(p, 1-p))
	}

	return ports.TxnVerdict{
		IsTransaction: p > 0.5,
		MessageType:   ports.MessageType(*messageTypeAns.Choice),
		Confidence:    confidence,
		Type:          ports.TransactionType(*typeAns.Choice),
		Usage:         portUsage(resp),
	}, nil
}

// Verify implements [ports.Verifier]. Details with no amount or date can't
// become a transaction, so they fail here without a paid call.
func (d *adapter) Verify(
	ctx context.Context,
	msg ports.Message,
	entities ports.ExtractedEntities,
) (ports.VerifyResult, error) {
	if entities.Amount == 0 || entities.Timestamp.IsZero() {
		return ports.VerifyResult{}, nil
	}

	refs, vpas := refCandidates(msg.Body), vpaCandidates(msg.Body)
	var accounts []string
	if strings.TrimSpace(entities.Account) == "" {
		accounts = accountCandidates(msg.Body)
	}

	req := sharedModel.DecisionRequest{
		State: sharedModel.State{Query: fmt.Sprintf(
			"Message:\n%s\n\nExtracted details:\nAccount: %s\nMerchant: %s\nAmount: %.2f\nDate: %s",
			msg.Body, entities.Account, entities.Merchant, entities.Amount, entities.Timestamp.Format("2006-01-02"),
		)},
		Model:     d.model,
		Questions: verifyQuestions(refs, vpas, accounts),
	}

	ctx, cancel := context.WithTimeout(ctx, askTimeout)
	defer cancel()
	resp, err := d.client.Ask(ctx, req)
	if err != nil {
		return ports.VerifyResult{}, err
	}
	ans, ok := resp.Answers["detailsMatch"]
	if !ok || ans.Noul == nil {
		return ports.VerifyResult{}, errs.NewWith(
			errs.CodeDecisionsAnswerMissing, "no noul answer for detailsMatch",
			errs.WithDetails(map[string]any{"question": "detailsMatch", "answerType": string(ans.Type)}),
		)
	}
	confidence := *ans.Noul
	if ans.Confidence != nil {
		confidence = *ans.Confidence
	}
	return ports.VerifyResult{
		Valid:      *ans.Noul > 0.5,
		Confidence: confidence,
		Channel:    pickChannel(resp.Answers["channel"]),
		VPA:        strings.ToLower(pickCandidate(resp.Answers["vpa"], vpas, minFieldConfidence, false)),
		Ref:        pickCandidate(resp.Answers["ref"], refs, minRefConfidence, true),
		Account:    pickCandidate(resp.Answers["account"], accounts, minRefConfidence, true),
	}, nil
}

// verifyQuestions asks everything verify needs in one call. A decision model
// can only choose, so anything it fills in is picked from candidates found in
// the message, and a question with no candidates isn't asked.
func verifyQuestions(refs, vpas, accounts []string) map[string]sharedModel.Intent {
	questions := map[string]sharedModel.Intent{
		"detailsMatch": {
			Type:         sharedModel.NoulIntentType,
			Instructions: "Do the extracted details agree with the message? A negative amount only means money went out.",
			Criteria: map[string]*string{
				"true":  new("The amount, date, merchant and account are all what the message says. A blank merchant or account is fine when the message doesn't give one."),
				"false": new("The amount, date, merchant or account differs from the message, or isn't in it."),
			},
		},
		"channel": {
			Type:         sharedModel.ChoiceIntentType,
			Instructions: "Which payment method carried this transaction?",
			Criteria: map[string]*string{
				narration.ChannelUPI:  new("A UPI payment, usually with a VPA such as name@bank."),
				narration.ChannelCard: new("A credit or debit card purchase, including card e-mandates."),
				narration.ChannelNEFT: new("A NEFT transfer."),
				narration.ChannelIMPS: new("An IMPS transfer."),
				narration.ChannelNACH: new("A NACH or ECS auto-debit, usually with a UMRN."),
				noneChoice:            new("The message doesn't show the method."),
			},
		},
	}
	if len(refs) > 0 {
		questions["ref"] = candidateQuestion(
			"Which value is the bank's unique reference for this one payment (UPI RRN, IMPS reference, NEFT UTR)? Never pick a phone number, account or card number, UMRN, or part of a merchant name.",
			refs, "None of these is this payment's reference.",
		)
	}
	if len(vpas) > 0 {
		questions["vpa"] = candidateQuestion(
			"Which value is the UPI ID of the other party in this payment? Never pick an email address, including the bank's support address.",
			vpas, "None of these is the other party's UPI ID.",
		)
	}
	if len(accounts) > 0 {
		questions["account"] = candidateQuestion(
			"Which value is the last four digits of the customer's own account or card this payment was made on? Never pick part of a phone number, a year, or an amount.",
			accounts, "None of these is the customer's account or card.",
		)
	}
	return questions
}

func candidateQuestion(instructions string, candidates []string, none string) sharedModel.Intent {
	criteria := make(map[string]*string, len(candidates)+1)
	for _, candidate := range candidates {
		criteria[candidate] = new("Found in the message.")
	}
	criteria[noneChoice] = new(none)
	return sharedModel.Intent{Type: sharedModel.ChoiceIntentType, Instructions: instructions, Criteria: criteria}
}

func portUsage(res *sharedModel.DecisionResponse) *ports.Usage {
	if !res.Usage.Available {
		return nil
	}
	usage := &ports.Usage{
		Model:           res.Model,
		InputTokens:     res.Usage.InputTokens,
		OutputTokens:    res.Usage.OutputTokens,
		CacheReadTokens: &res.Usage.CacheReadTokens,
		Cost:            res.Usage.Cost,
	}
	if res.Provider != nil {
		usage.Provider = *res.Provider
	}
	return usage
}
