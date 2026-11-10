package service

import (
	"context"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/google/uuid"
)

// JobScheduleRunView is the latest job a schedule queued.
type JobScheduleRunView struct {
	JobID        uuid.UUID     `json:"jobId"`
	Status       job.JobStatus `json:"status"`
	ScheduledFor time.Time     `json:"scheduledFor"`
	CompletedAt  *time.Time    `json:"completedAt,omitempty"`
	LastError    string        `json:"lastError,omitempty"`
}

// JobScheduleView is one row of the Schedules tab. Label says what the schedule
// is for, such as the mailbox address.
type JobScheduleView struct {
	ID        uuid.UUID           `json:"id"`
	Type      string              `json:"type"`
	Label     string              `json:"label,omitempty"`
	Cron      string              `json:"cron"`
	Timezone  string              `json:"timezone"`
	Enabled   bool                `json:"enabled"`
	NextRunAt time.Time           `json:"nextRunAt"`
	LastRun   *JobScheduleRunView `json:"lastRun,omitempty"`
}

// JobScheduleList is the Schedules tab. Enabled is false when this service has
// no schedule store (JOB_DRIVER is unset or memory).
type JobScheduleList struct {
	Enabled   bool              `json:"enabled"`
	Schedules []JobScheduleView `json:"schedules"`
}

// JobScheduleService shows the budget's schedules. The budget comes from ctx.
type JobScheduleService interface {
	List(ctx context.Context) (JobScheduleList, error)
}

type jobScheduleService struct {
	store       job.ScheduleStore
	connections repository.MailConnectionRepository
}

func NewJobScheduleService(
	store job.ScheduleStore,
	connections repository.MailConnectionRepository,
) JobScheduleService {
	return &jobScheduleService{store: store, connections: connections}
}

// List returns the budget's own schedules and its mailboxes' watch renewals,
// which are system schedules with no budget.
func (s *jobScheduleService) List(ctx context.Context) (JobScheduleList, error) {
	list := JobScheduleList{Schedules: []JobScheduleView{}}
	if s.store == nil {
		return list, nil
	}
	list.Enabled = true

	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		return JobScheduleList{}, err
	}
	connections, err := s.connections.ListByBudget(ctx, budgetID)
	if err != nil {
		return JobScheduleList{}, err
	}
	names := make([]string, 0, len(connections))
	labels := make(map[string]string, len(connections))
	for _, connection := range connections {
		name := model.MailboxWatchScheduleName(connection.ID)
		names = append(names, name)
		labels[name] = connection.Account
	}

	schedules, err := s.store.ListSchedules(ctx, job.ScheduleScope{BudgetID: budgetID, SystemNames: names})
	if err != nil {
		return JobScheduleList{}, err
	}
	for _, schedule := range schedules {
		view := JobScheduleView{
			ID:        schedule.ID,
			Type:      schedule.Type,
			Label:     labels[schedule.Name],
			Cron:      schedule.Cron,
			Timezone:  schedule.Timezone,
			Enabled:   schedule.Enabled,
			NextRunAt: schedule.NextRunAt,
		}
		if run := schedule.LastRun; run != nil {
			view.LastRun = &JobScheduleRunView{
				JobID:        run.JobID,
				Status:       run.Status,
				ScheduledFor: run.ScheduledFor,
				CompletedAt:  run.CompletedAt,
				LastError:    run.LastError,
			}
		}
		list.Schedules = append(list.Schedules, view)
	}
	return list, nil
}
