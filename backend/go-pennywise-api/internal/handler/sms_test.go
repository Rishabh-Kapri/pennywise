package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type fakeSmsService struct {
	got service.SmsMessage
	err error
}

func (f *fakeSmsService) Ingest(_ context.Context, msg service.SmsMessage) (*service.SmsDecisionEntry, error) {
	f.got = msg
	if f.err != nil {
		return nil, f.err
	}
	return &service.SmsDecisionEntry{Sender: msg.Sender}, nil
}

func smsReq(body map[string]any) map[string]any {
	req := map[string]any{
		"sender":     "AX-HDFCBK",
		"body":       "Rs 500 debited",
		"receivedAt": "2026-10-30T10:00:00.000Z",
	}
	for k, v := range body {
		req[k] = v
	}
	return req
}

func TestSmsHandler_Ingest_PassesMessageToService(t *testing.T) {
	svc := &fakeSmsService{}
	w, c := makeReq(http.MethodPost, "/api/sms/ingest", smsReq(map[string]any{"source": "inbox"}))
	withBudget(c, uuid.New())

	NewSmsHandler(svc).Ingest(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "AX-HDFCBK", svc.got.Sender)
	assert.Equal(t, "Rs 500 debited", svc.got.Body)
	assert.Equal(t, "inbox", svc.got.Source)
}

func TestSmsHandler_Ingest_DecisionsFailureIsBadGateway(t *testing.T) {
	svc := &fakeSmsService{err: errors.New("decisions: 502 upstream")}
	w, c := makeReq(http.MethodPost, "/api/sms/ingest", smsReq(nil))
	withBudget(c, uuid.New())

	NewSmsHandler(svc).Ingest(c)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "502 upstream")
}

func TestSmsHandler_Ingest_RejectsMissingFields(t *testing.T) {
	w, c := makeReq(http.MethodPost, "/api/sms/ingest", map[string]any{"sender": "AX-HDFCBK"})
	withBudget(c, uuid.New())

	NewSmsHandler(&fakeSmsService{}).Ingest(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSmsHandler_Ingest_RejectsUnknownSource(t *testing.T) {
	w, c := makeReq(http.MethodPost, "/api/sms/ingest", smsReq(map[string]any{"source": "carrier-pigeon"}))
	withBudget(c, uuid.New())

	NewSmsHandler(&fakeSmsService{}).Ingest(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
