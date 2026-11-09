package worker

import (
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/narration"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
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

// messageSource reads what rules can from the message's text, for learning.
func messageSource(m model.ReceivedMessage) *model.MessageSource {
	text := plainBody(m)
	parsed := narration.Parse(text)
	source := &model.MessageSource{
		Text:        text,
		Channel:     parsed.Channel,
		Merchant:    parsed.Merchant,
		VPA:         parsed.VPA,
		Ref:         parsed.Ref,
		AccountHint: parsed.AccountHint,
		Direction:   parsed.Direction,
		Amount:      parsed.Amount,
		Currency:    parsed.Currency,
		OccurredOn:  parsed.OccurredOn,
		OccurredAt:  parsed.OccurredAt,
	}
	switch m.Source {
	case "email":
		source.Kind = "EMAIL"
	case "sms":
		source.Kind = "SMS"
	case "api":
	  source.Kind = "API"
	}
	return source
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
	return model.MessageEntities{Account: e.Account, Merchant: e.Merchant, Amount: e.Amount, Date: e.Timestamp}
}

func toPortEntities(e model.MessageEntities) ports.ExtractedEntities {
	return ports.ExtractedEntities{Account: e.Account, Merchant: e.Merchant, Amount: e.Amount, Timestamp: e.Date}
}

func toModelPrediction(r ports.PredictionResult, source model.MessagePredictionSource) model.MessagePrediction {
	return model.MessagePrediction{
		PayeeName:    r.PayeeName,
		CategoryName: r.CategoryName,
		Confidence:   r.Confidence,
		Source:       source,
	}
}
