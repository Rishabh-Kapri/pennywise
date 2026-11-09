package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/google/uuid"
)

type learnBroker struct {
	job.Broker
	jobType string
	opts    job.EnqueueOptions
	payload []byte
}

func (b *learnBroker) Enqueue(
	_ context.Context, jobType string, payload []byte, opts ...job.EnqueueOption,
) (job.EnqueueResult, error) {
	resolved, err := job.ResolveEnqueueOptions(jobType, payload, opts...)
	if err != nil {
		return job.EnqueueResult{}, err
	}
	b.jobType, b.opts, b.payload = jobType, resolved, payload
	return job.EnqueueResult{JobID: uuid.New()}, nil
}

func TestQueueAliasConfirm(t *testing.T) {
	payee, category, budget := uuid.New(), uuid.New(), uuid.New()
	txn := model.Transaction{ID: uuid.New(), PayeeID: &payee, CategoryID: &category}

	broker := &learnBroker{}
	svc := &transactionService{jobBroker: broker}
	svc.queueAliasConfirm(context.Background(), budget, txn)

	if broker.jobType != model.JobTypeTransactionLearn || broker.opts.Queue != model.QueuePennywise ||
		broker.opts.BudgetID == nil || *broker.opts.BudgetID != budget {
		t.Fatalf("queued %s with %+v", broker.jobType, broker.opts)
	}
	var got model.TransactionLearnPayload
	if err := json.Unmarshal(broker.payload, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Confirmed || got.TransactionID != txn.ID || got.PayeeID != payee || *got.CategoryID != category {
		t.Fatalf("payload = %+v", got)
	}

	// No payee, or no queue: nothing to do.
	broker = &learnBroker{}
	svc = &transactionService{jobBroker: broker}
	svc.queueAliasConfirm(context.Background(), budget, model.Transaction{ID: uuid.New()})
	if broker.jobType != "" {
		t.Fatalf("queued %s without a payee", broker.jobType)
	}
	(&transactionService{}).queueAliasConfirm(context.Background(), budget, txn)
}
