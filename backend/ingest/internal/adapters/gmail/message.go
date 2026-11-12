package gmail

import (
	"encoding/base64"
	"strings"
	"time"

	gmailv1 "google.golang.org/api/gmail/v1"

	"github.com/Rishabh-Kapri/pennywise/backend/ingest/internal/ports"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

// toInbound turns a Gmail message into the common message shape.
func toInbound(msg *gmailv1.Message) ports.InboundMessage {
	in := ports.InboundMessage{
		Source:     model.SourceEmail,
		Provider:   ports.ProviderGmail,
		ExternalID: msg.Id,
		ReceivedAt: time.UnixMilli(msg.InternalDate),
	}
	if msg.Payload != nil {
		in.Sender = headerValue(msg.Payload.Headers, "From")
		in.Subject = headerValue(msg.Payload.Headers, "Subject")
		in.Body = body(msg.Payload)
	}
	return in
}

// body returns the message's HTML, or its plain text when it has no HTML.
// go-gmail only looked one level down for HTML parts, so single-part emails
// (body directly on the payload) and nested multipart emails came out empty.
func body(payload *gmailv1.MessagePart) string {
	if html := collect(payload, "text/html"); html != "" {
		return html
	}
	return collect(payload, "text/plain")
}

// collect joins the decoded bodies of every part of mimeType, depth first.
func collect(part *gmailv1.MessagePart, mimeType string) string {
	if part == nil {
		return ""
	}
	var b strings.Builder
	if part.MimeType == mimeType && part.Body != nil && part.Body.Data != "" {
		b.WriteString(decode(part.Body.Data))
	}
	for _, child := range part.Parts {
		b.WriteString(collect(child, mimeType))
	}
	return b.String()
}

// decode reads Gmail's URL-safe base64, with or without padding.
func decode(data string) string {
	if raw, err := base64.URLEncoding.DecodeString(data); err == nil {
		return string(raw)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(data)
	return string(raw)
}

// headerValue returns the first matching header, case-insensitively.
func headerValue(headers []*gmailv1.MessagePartHeader, name string) string {
	for _, h := range headers {
		if h != nil && strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}
