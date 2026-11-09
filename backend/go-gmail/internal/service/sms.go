package service

import (
	"context"
	"time"
)

type SmsMessage struct {
	Sender     string
	Body       string
	ReceivedAt time.Time
	Source     string
}

type SmsService interface {
	Ingest(ctx context.Context, message SmsMessage) error
}
