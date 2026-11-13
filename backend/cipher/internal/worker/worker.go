// Package worker runs cipher's jobs. Like internal/handler (HTTP) it is an
// inbound adapter: it turns a claimed job into calls on the ports and queues
// the next step. The decisions live in the adapters behind the ports.
package worker

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/factory"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/jobrunner"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
)

// WorkerDeps are the ports the handlers call.
type WorkerDeps struct {
	Queues []string
	Broker job.Broker
	// Pipeline picks a step's adapter for each budget.
	Pipeline factory.MessagePipeline

	// ParkLowConfidence sends low-confidence check and verify answers to a
	// person. Off, they're only logged. Details that don't match always park.
	ParkLowConfidence bool
}

// Setup builds cipher's worker on the broker main already opened, registers
// its handlers and runs it until ctx is cancelled and in-flight jobs finish.
// With no broker (JOB_DRIVER unset) nothing starts and it returns nil.
func Setup(ctx context.Context, deps WorkerDeps) error {
	if deps.Broker == nil {
		logger.Logger(ctx).Info("job worker off (JOB_DRIVER not set)")
		return nil
	}

	w, err := jobrunner.NewWorker(
		deps.Broker,
		jobrunner.WithConcurrency(3),
		jobrunner.WithQueues(deps.Queues),
		jobrunner.WithPrefix("cipher"),
	)
	if err != nil {
		return errs.Wrap(errs.CodeWorkerCreateError, "create job worker failed", err)
	}

	w.RegisterJobHandler(sharedModel.JobTypeMessageReceived, messageReceived(deps))
	w.RegisterJobHandler(sharedModel.JobTypeMessageExtract, messageExtract(deps))
	w.RegisterJobHandler(sharedModel.JobTypeMessageVerify, messageVerify(deps))
	w.RegisterJobHandler(sharedModel.JobTypeMessageLookup, messageLookup(deps))
	w.RegisterJobHandler(sharedModel.JobTypeMessagePredict, messagePredict(deps))

	if err := w.Run(ctx); err != nil {
		return errs.Wrap(errs.CodeWorkerRunError, "worker run failed", err)
	}
	return nil
}
