package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/google/uuid"
)

type recordingWebsocket struct {
	budgets []uuid.UUID
	events  []string
}

func (r *recordingWebsocket) Connect(context.Context, http.ResponseWriter, *http.Request) error {
	return nil
}
func (r *recordingWebsocket) GetSessions(context.Context) WebsocketSessionsResponse {
	return WebsocketSessionsResponse{}
}
func (r *recordingWebsocket) SendTestEvent(context.Context, string, any, *string) error { return nil }
func (r *recordingWebsocket) SendNotification(_ context.Context, budget uuid.UUID, event string, _ any) error {
	r.budgets, r.events = append(r.budgets, budget), append(r.events, event)
	return nil
}

type accountConnections struct {
	fakeConnections
	budgets []uuid.UUID
}

func (a accountConnections) GetByAccount(context.Context, string, string) (*model.MailConnection, error) {
	return &model.MailConnection{BudgetIDs: a.budgets}, nil
}

func TestJobUpdateForwarder(t *testing.T) {
	budget, other := uuid.New(), uuid.New()

	t.Run("a budget's job goes to that budget", func(t *testing.T) {
		ws := &recordingWebsocket{}
		f := &JobUpdateForwarder{Websocket: ws}
		f.forward(context.Background(), job.Update{JobID: uuid.New(), BudgetID: &budget})
		if len(ws.budgets) != 1 || ws.budgets[0] != budget || ws.events[0] != model.EventJobUpdate {
			t.Fatalf("sent %v %v", ws.budgets, ws.events)
		}
	})

	t.Run("a mailbox job goes to every budget the mailbox feeds", func(t *testing.T) {
		payload, _ := json.Marshal(model.MailboxChanged{Provider: "gmail", Account: "me@gmail.com"})
		id := uuid.New()
		ws := &recordingWebsocket{}
		f := &JobUpdateForwarder{
			Websocket:   ws,
			Store:       &fakeJobStore{jobs: map[uuid.UUID]*job.Job{id: {ID: id, Payload: payload}}},
			Connections: accountConnections{budgets: []uuid.UUID{budget, other}},
		}
		f.forward(context.Background(), job.Update{JobID: id, Type: model.JobTypeMailboxChanged})
		if len(ws.budgets) != 2 {
			t.Fatalf("sent to %v", ws.budgets)
		}
	})

	t.Run("other system jobs go nowhere", func(t *testing.T) {
		ws := &recordingWebsocket{}
		f := &JobUpdateForwarder{Websocket: ws}
		f.forward(context.Background(), job.Update{JobID: uuid.New(), Type: "mailbox.watch.start"})
		if len(ws.budgets) != 0 {
			t.Fatalf("sent to %v", ws.budgets)
		}
	})
}
