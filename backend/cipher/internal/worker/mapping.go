package worker

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils/narration"
	"jaytaylor.com/html2text"
)

// Job payloads use shared model types because they cross services; ports use
// cipher's own. These convert at the boundary.

func toPortMessage(m model.ReceivedMessage) ports.Message {
	source := ports.Source(m.Source)
	if m.Source == "api" {
		source = ports.TextSource
	}
	return ports.Message{Body: plainBody(m), Source: source, Sender: m.Sender}
}

// plainBody turns an email's HTML into text, so models don't read (or get
// charged for) markup. Ingest sends the HTML part when an email has one.
func plainBody(m model.ReceivedMessage) string {
	if m.Source != "email" {
		return m.Body
	}
	// Same conversion as the older email flow. Stored transactions and learned
	// aliases came from text made this way, so the dedupe hash and lookup keys
	// have to match it.
	text, err := html2text.FromString(m.Body, html2text.Options{PrettyTables: false, OmitLinks: true, TextOnly: true})
	if err != nil {
		return m.Body
	}
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", ""))
	if text == "" {
		return m.Body
	}
	return text
}

var (
	blankLines   = regexp.MustCompile(`\n{3,}`)
	currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)
)

// displayText is the message as a person reads it. Unlike plainBody it keeps
// the line breaks, so it mustn't go into hashes or lookup keys.
func displayText(m model.ReceivedMessage) string {
	if m.Source != "email" {
		return m.Body
	}
	text, err := html2text.FromString(m.Body, html2text.Options{PrettyTables: false, OmitLinks: true, TextOnly: true})
	if err != nil || strings.TrimSpace(text) == "" {
		return plainBody(m)
	}
	return strings.TrimSpace(blankLines.ReplaceAllString(text, "\n\n"))
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
	case "debit":
		source.Direction = narration.Debit
	case "credit":
		source.Direction = narration.Credit
	}
	if e.Amount != 0 {
		source.Amount = strconv.FormatFloat(math.Abs(e.Amount), 'f', 2, 64)
	}
	if currency := strings.ToUpper(strings.TrimSpace(e.Currency)); currencyCode.MatchString(currency) {
		source.Currency = currency
	}
	if !e.Date.IsZero() {
		on := e.Date
		source.OccurredOn = &on
		if on.Hour() != 0 || on.Minute() != 0 || on.Second() != 0 {
			source.OccurredAt = &on
		}
	}
	switch m.Source {
	case "email":
		source.Kind = "EMAIL"
	case "sms":
		source.Kind = "SMS"
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
	return model.MessageVerdict{
		MessageType:   model.InboundMessageType(v.MessageType),
		IsTransaction: v.IsTransaction,
		Type:          string(v.Type),
		Confidence:    v.Confidence,
	}
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
		PayeeName:    r.PayeeName,
		CategoryName: r.CategoryName,
		Confidence:   r.Confidence,
		Source:       source,
	}
}
