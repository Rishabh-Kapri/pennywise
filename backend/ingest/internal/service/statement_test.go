package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

type attachmentProvider struct {
	fakeProvider
	fetched []string
}

func (p *attachmentProvider) Attachment(_ context.Context, _ model.MailConnection, _, fileName string) ([]byte, error) {
	p.fetched = append(p.fetched, fileName)
	return []byte("%PDF-1.7"), nil
}

func TestFetchStatementQueuesEachPDFOnTheServiceQueue(t *testing.T) {
	budget := uuid.New()
	conn := &model.MailConnection{ID: uuid.New(), Provider: "gmail", Account: "me@gmail.com", BudgetIDs: []uuid.UUID{budget}}
	provider := &attachmentProvider{}
	queue := &fakeBroker{byKey: map[string]uuid.UUID{}}
	s := NewMailboxService(MailboxDeps{
		Queue: queue, Connections: &fakeConns{conn: conn}, Providers: oneProvider{provider},
	})

	n, err := s.FetchStatement(context.Background(), budget, model.StatementFetchPayload{
		Provider: "gmail", MailAccount: "Me@Gmail.com", MessageID: "m1",
		Attachments: []model.MessageAttachment{
			{FileName: "statement.pdf", MimeType: "application/pdf", Size: 100},
			{FileName: "logo.png", MimeType: "image/png", Size: 100},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(provider.fetched) != 1 || provider.fetched[0] != "statement.pdf" {
		t.Fatalf("queued %d, fetched %v", n, provider.fetched)
	}
	if len(queue.calls) != 1 {
		t.Fatalf("enqueued %d jobs", len(queue.calls))
	}
	call := queue.calls[0]
	if call.Type != model.JobTypeStatementImport || call.Queue != model.QueuePennywise ||
		call.BudgetID == nil || *call.BudgetID != budget {
		t.Fatalf("enqueued %+v", call)
	}
}
