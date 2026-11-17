package worker

import (
	"context"
	"encoding/json"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/pipeline"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/google/uuid"
)

// A budget can turn off the message steps after extract (verify, lookup,
// predict). Each takes the extracted details, so a step that's off is skipped
// by queueing the next one that's on. The handlers check too, for jobs queued
// before the step was turned off and for a parked message continued later.

// entitiesJobs are the job types of the steps that take the extracted details.
var entitiesJobs = map[pipeline.StepID]string{
	pipeline.StepVerify:  model.JobTypeMessageVerify,
	pipeline.StepLookup:  model.JobTypeMessageLookup,
	pipeline.StepPredict: model.JobTypeMessagePredict,
}

// skippedOutput is what a step that's off records as its result.
type skippedOutput struct {
	Skipped bool   `json:"skipped"`
	Reason  string `json:"reason"`
}

func turnedOff(step pipeline.StepID) skippedOutput {
	return skippedOutput{Skipped: true, Reason: string(step) + " is turned off for this budget"}
}

// sendEntities queues the first of steps the budget runs, with p. With none
// on, nothing is left that can choose a payee and category, so the message is
// parked for a person.
func sendEntities(
	ctx context.Context,
	deps WorkerDeps,
	j *job.Job,
	budgetID uuid.UUID,
	p model.MessageEntitiesPayload,
	output any,
	steps ...pipeline.StepID,
) (json.RawMessage, error) {
	for _, step := range steps {
		on, err := deps.Pipeline.Enabled(ctx, budgetID, step)
		if err != nil {
			return nil, err
		}
		if on {
			return next(ctx, deps, j, budgetID, p.Message, entitiesJobs[step], p, output)
		}
	}
	return parkNoPrediction(ctx, deps, j, budgetID, p, output)
}

// parkNoPrediction parks a message that memory had no answer for while
// predict is turned off. Nothing can continue it: a person picks the payee
// and category and creates it, or dismisses it.
func parkNoPrediction(
	ctx context.Context,
	deps WorkerDeps,
	j *job.Job,
	budgetID uuid.UUID,
	p model.MessageEntitiesPayload,
	output any,
) (json.RawMessage, error) {
	entities := p.Entities
	return park(ctx, deps, j, budgetID, p.Message,
		"Predict is turned off and nothing on record names this payee, so choose the payee and category.",
		model.MessageReview{Step: string(pipeline.StepPredict), Entities: &entities}, output)
}
