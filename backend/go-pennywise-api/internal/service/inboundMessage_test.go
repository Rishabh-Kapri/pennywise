package service

import (
	"context"
	"testing"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
)

func TestRecordReportWritesTheMessagesStatus(t *testing.T) {
	budget := uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budget)
	messages := newFakeMessages()
	svc := NewMessageJobService(MessageJobDeps{InboundMessages: messages})
	msg := model.ReceivedMessage{Source: "email", Provider: "gmail", ExternalID: "m1"}
	parked := model.PipelineReportPayload{
		Message: msg, Outcome: model.MessageOutcomeNeedsReview, Review: &model.MessageReview{Step: "verify"},
	}

	first := uuid.New()
	if err := svc.RecordReport(ctx, first, time.Now(), parked); err != nil {
		t.Fatal(err)
	}
	if got := messages.status(budget); got != model.InboundMessageNeedsReview {
		t.Fatalf("status %s", got)
	}
	// A retried report job saves nothing new.
	if err := svc.RecordReport(ctx, first, time.Now(), parked); err != nil {
		t.Fatal(err)
	}
	// A stray report while the message waits for a person is refused, not retried.
	created := model.PipelineReportPayload{Message: msg, Outcome: model.MessageOutcomeTransactionCreated}
	if err := svc.RecordReport(ctx, uuid.New(), time.Now(), created); err != nil {
		t.Fatal(err)
	}
	if got := messages.status(budget); got != model.InboundMessageNeedsReview {
		t.Fatalf("status %s, want the person's decision kept", got)
	}

	// Once continued, the run's report lands.
	row := messages.rows[msg.Key(budget)]
	row.Status = model.InboundMessageProcessing
	if err := svc.RecordReport(ctx, uuid.New(), time.Now(), created); err != nil {
		t.Fatal(err)
	}
	if got := messages.status(budget); got != model.InboundMessageDone {
		t.Fatalf("status %s", got)
	}
}

func TestRecordReportWithoutAStoreDoesNothing(t *testing.T) {
	ctx := utils.WithBudgetID(context.Background(), uuid.New())
	if err := NewMessageJobService(MessageJobDeps{}).RecordReport(ctx, uuid.New(), time.Now(), model.PipelineReportPayload{}); err != nil {
		t.Fatal(err)
	}
}

func TestInboundMessageServiceList(t *testing.T) {
	budget := uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budget)
	messages := newFakeMessages()
	for i, outcome := range []model.MessageOutcome{
		model.MessageOutcomeNeedsReview, model.MessageOutcomeNeedsReview, model.MessageOutcomeNotTransaction,
	} {
		key := model.InboundMessageKey{BudgetID: budget, Source: "sms", Provider: "android", ExternalID: string(rune('a' + i))}
		review := &model.MessageReview{Step: "check"}
		if outcome != model.MessageOutcomeNeedsReview {
			review = nil
		}
		_, _ = messages.RecordReport(ctx, model.InboundMessageReport{
			Key: key, Outcome: outcome, Review: review, ReportJobID: uuid.New(),
		})
	}
	svc := NewInboundMessageService(messages)

	list, err := svc.List(ctx, model.InboundMessageNeedsReview, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 || list.Counts[model.InboundMessageNeedsReview] != 2 || list.Counts[model.InboundMessageDone] != 1 {
		t.Fatalf("%+v", list)
	}
	if _, err := svc.List(ctx, "waiting", 0); !errs.HasCode(err, errs.CodeInvalidArgument) {
		t.Fatalf("unknown status: %v", err)
	}
}
