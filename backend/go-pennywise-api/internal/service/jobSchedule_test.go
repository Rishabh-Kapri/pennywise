package service

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
)

type fakeScheduleStore struct {
	job.ScheduleStore
	list  []job.ScheduleOverview
	scope job.ScheduleScope
}

func (f *fakeScheduleStore) ListSchedules(_ context.Context, scope job.ScheduleScope) ([]job.ScheduleOverview, error) {
	f.scope = scope
	return f.list, nil
}

type fakeScheduleConnections struct {
	repository.MailConnectionRepository
	connections []model.MailConnection
}

func (f *fakeScheduleConnections) ListByBudget(context.Context, uuid.UUID) ([]model.MailConnection, error) {
	return f.connections, nil
}

func TestJobScheduleListScopesToBudgetMailboxes(t *testing.T) {
	budgetID := uuid.New()
	ctx := utils.WithBudgetID(context.Background(), budgetID)
	connection := model.MailConnection{ID: uuid.New(), Account: "me@example.com"}
	name := model.MailboxWatchScheduleName(connection.ID)
	ranAt := time.Now().Add(-time.Hour)
	store := &fakeScheduleStore{list: []job.ScheduleOverview{{
		JobSchedule: job.JobSchedule{Name: name, Type: model.JobTypeMailboxStartWatch, Cron: "0 3 * * *", Timezone: "UTC", Enabled: true},
		LastRun:     &job.ScheduleRun{JobID: uuid.New(), Status: job.DeadJobStatus, ScheduledFor: ranAt, LastError: "boom"},
	}}}
	connections := &fakeScheduleConnections{connections: []model.MailConnection{connection}}

	list, err := NewJobScheduleService(store, connections).List(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if store.scope.BudgetID != budgetID || !reflect.DeepEqual(store.scope.SystemNames, []string{name}) {
		t.Fatalf("scope = %+v", store.scope)
	}
	if !list.Enabled || len(list.Schedules) != 1 {
		t.Fatalf("list = %+v", list)
	}
	got := list.Schedules[0]
	if got.Label != connection.Account || got.LastRun == nil || got.LastRun.Status != job.DeadJobStatus || got.LastRun.LastError != "boom" {
		t.Fatalf("view = %+v, last run = %+v", got, got.LastRun)
	}
}

func TestJobScheduleListWithoutStore(t *testing.T) {
	list, err := NewJobScheduleService(nil, nil).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if list.Enabled || list.Schedules == nil || len(list.Schedules) != 0 {
		t.Fatalf("list = %+v", list)
	}
}
