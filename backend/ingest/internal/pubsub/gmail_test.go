package pubsub

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
)

type fakeMailbox struct {
	got []model.MailboxChanged
	err error
}

func (f *fakeMailbox) Changed(_ context.Context, n model.MailboxChanged) (job.EnqueueResult, error) {
	f.got = append(f.got, n)
	return job.EnqueueResult{JobID: uuid.New()}, f.err
}

func TestHandle(t *testing.T) {
	ctx := context.Background()

	t.Run("valid push is queued and acked", func(t *testing.T) {
		f := &fakeMailbox{}
		ack := NewGmailSubscriber(Config{}, f).handle(ctx, []byte(`{"emailAddress":"Me@Gmail.com","historyId":12345}`))
		if !ack || len(f.got) != 1 {
			t.Fatalf("ack=%v calls=%d", ack, len(f.got))
		}
		if n := f.got[0]; n.Provider != "gmail" || n.Account != "Me@Gmail.com" || n.Cursor != "12345" {
			t.Fatalf("notification = %+v", n)
		}
	})

	for name, data := range map[string]string{
		"not json":      `nope`,
		"no email":      `{"historyId":1}`,
		"no history id": `{"emailAddress":"me@gmail.com"}`,
	} {
		t.Run(name+" is dropped (acked, not queued)", func(t *testing.T) {
			f := &fakeMailbox{}
			if ack := NewGmailSubscriber(Config{}, f).handle(ctx, []byte(data)); !ack || len(f.got) != 0 {
				t.Fatalf("ack=%v calls=%d", ack, len(f.got))
			}
		})
	}

	t.Run("enqueue failure is nacked for redelivery", func(t *testing.T) {
		f := &fakeMailbox{err: errors.New("db down")}
		if ack := NewGmailSubscriber(Config{}, f).handle(ctx, []byte(`{"emailAddress":"me@gmail.com","historyId":1}`)); ack {
			t.Fatal("want nack")
		}
	})
}
