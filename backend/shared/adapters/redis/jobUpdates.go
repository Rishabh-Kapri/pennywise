package redis

import (
	"context"
	"encoding/json"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	goredis "github.com/redis/go-redis/v9"
)

// jobUpdatesChannel is a Pub/Sub channel, not a stream: an update is only a
// hint to look again, so one nobody was listening for needn't be kept.
const jobUpdatesChannel = "pennywise:job-updates"

// JobUpdates implements [job.Notifier] and [job.UpdateSource].
type JobUpdates struct {
	client goredis.UniversalClient
}

func NewJobUpdates(client goredis.UniversalClient) *JobUpdates {
	return &JobUpdates{client: client}
}

// Notify implements [job.Notifier].
func (u *JobUpdates) Notify(ctx context.Context, update job.Update) error {
	body, err := json.Marshal(update)
	if err != nil {
		return err
	}
	return u.client.Publish(ctx, jobUpdatesChannel, body).Err()
}

// Listen implements [job.UpdateSource]. go-redis resubscribes after a dropped
// connection.
func (u *JobUpdates) Listen(ctx context.Context, handle func(job.Update)) error {
	sub := u.client.Subscribe(ctx, jobUpdatesChannel)
	defer sub.Close()

	messages := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return nil
		case message, ok := <-messages:
			if !ok {
				return nil
			}
			var update job.Update
			if err := json.Unmarshal([]byte(message.Payload), &update); err != nil {
				logger.Logger(ctx).Warn("skipping unreadable job update", "err", err)
				continue
			}
			handle(update)
		}
	}
}
