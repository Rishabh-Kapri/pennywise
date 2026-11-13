package worker

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/emailtext"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils/narration"
)

// Job payloads use shared model types because they cross services; ports use
// cipher's own. These convert at the boundary.

func toPortMessage(m model.ReceivedMessage) ports.Message {
	return ports.Message{Body: plainBody(m), Source: m.Source, Sender: m.Sender}
}

// plainBody turns an email's HTML into text, so models don't read (or get
// charged for) markup. Ingest sends the HTML part when an email has one.
func plainBody(m model.ReceivedMessage) string {
	if m.Source != model.MessageSourceEmail {
		return m.Body
	}
	text, err := emailtext.Plain(m.Body)
	if err != nil || text == "" {
		return m.Body
	}
	return text
}

var (
	currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)
	// rupeeNames are how banks and models write the rupee.
	rupeeNames = map[string]bool{"RS": true, "RS.": true, "₹": true, "INR": true, "RUPEE": true, "RUPEES": true}
)

// currency is the ISO code of what extract read, or "" if it isn't one.
func currency(raw string) string {
	value := strings.ToUpper(strings.TrimSpace(raw))
	if rupeeNames[value] {
		return "INR"
	}
	if currencyCode.MatchString(value) {
		return value
	}
	return ""
}

// displayText is the message as a person reads it. Unlike plainBody it keeps
// the line breaks, so it mustn't go into hashes or lookup keys.
func displayText(m model.ReceivedMessage) string {
	if m.Source != model.MessageSourceEmail {
		return m.Body
	}
	text, err := emailtext.Display(m.Body)
	if err != nil || text == "" {
		return plainBody(m)
	}
	return text
}

// messageSource is the transaction's source row, from what extract and verify
// read. The API source has no Kind: the table only keeps bank text.
func messageSource(m model.ReceivedMessage, e model.MessageEntities, v model.MessageVerdict) *model.MessageSource {
	source := &model.MessageSource{
		Text:        plainBody(m),
		Channel:     e.Channel,
		Merchant:    narration.LookupMerchant(e.Channel, e.Merchant, e.VPA),
		VPA:         e.VPA,
		Ref:         e.Ref,
		AccountHint: accountHint(e.Account),
	}
	switch v.Type {
	case string(ports.DebitTransactionType):
		source.Direction = narration.Debit
	case string(ports.CreditTransactionType):
		source.Direction = narration.Credit
	}
	if e.Amount != 0 {
		source.Amount = strconv.FormatFloat(math.Abs(e.Amount), 'f', 2, 64)
	}
	source.Currency = currency(e.Currency)
	if !e.Date.IsZero() {
		// Extract keeps the date and clock the message wrote as UTC; they mean
		// that time in the bank's zone.
		d := e.Date
		on := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, narration.BankZone)
		source.OccurredOn = &on
		if d.Hour() != 0 || d.Minute() != 0 || d.Second() != 0 {
			at := time.Date(d.Year(), d.Month(), d.Day(), d.Hour(), d.Minute(), d.Second(), 0, narration.BankZone)
			source.OccurredAt = &at
		}
	}
	switch m.Source {
	case model.MessageSourceEmail:
		source.Kind = narration.KindEmail
	case model.MessageSourceSMS:
		source.Kind = narration.KindSMS
	}
	return source
}

// accountHint is the last four digits of an extracted account.
func accountHint(account string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, account)
	if len(digits) > 4 {
		return digits[len(digits)-4:]
	}
	return digits
}

func toModelVerdict(v ports.TxnVerdict) model.MessageVerdict {
	verdict := model.MessageVerdict{
		MessageType:   model.InboundMessageType(v.MessageType),
		IsTransaction: v.IsTransaction,
		Type:          string(v.Type),
		Confidence:    v.Confidence,
	}
	if v.Usage != nil {
		verdict.Model = v.Usage.Model
	}
	return verdict
}

func toModelEntities(e ports.ExtractedEntities) model.MessageEntities {
	return model.MessageEntities{
		Account: e.Account, Merchant: e.Merchant, Amount: e.Amount, Date: e.Timestamp,
		Currency: e.Currency, Channel: e.Channel, VPA: e.VPA, Ref: e.Ref,
	}
}

func toPortEntities(e model.MessageEntities) ports.ExtractedEntities {
	return ports.ExtractedEntities{
		Account: e.Account, Merchant: e.Merchant, Amount: e.Amount, Timestamp: e.Date,
		Currency: e.Currency, Channel: e.Channel, VPA: e.VPA, Ref: e.Ref,
	}
}

func toModelPrediction(r ports.PredictionResult, source model.MessagePredictionSource) model.MessagePrediction {
	return model.MessagePrediction{
		PayeeName:       r.PayeeName,
		CategoryName:    r.CategoryName,
		Confidence:      r.Confidence,
		Source:          source,
		PayeeConfidence: r.PayeeConfidence,
		Reasoning:       r.Reasoning,
		Model:           r.Model,
	}
}
