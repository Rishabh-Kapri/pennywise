package worker

import (
	"context"
	"encoding/json"
	"strings"

	// "io"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/jobrunner"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/pipeline"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/google/uuid"
	// "golang.org/x/net/html"
)

// The message pipeline, one handler per step (the flow is drawn in
// shared/model/messagePipeline.go). Every handler has the same errors:
//
//   - CodeJobInvalidPayload: no budget, or the payload doesn't decode.
//   - CodeMessage*Failed: the step's port failed, wrapping the port's error.
//   - CodeJobEnqueueFailed: the next job couldn't be queued. Next jobs use the
//     message's idempotency key, so the retry can't queue a second one.
//
// The runner retries every error up to the job's MaxAttempts, then marks the
// job dead, so a port should only return an error for something worth
// retrying. "The answer is no" is a normal result that ends the message with a
// pipeline.report job.

// messageReceived handles model.JobTypeMessageReceived, the entry point for
// every source (email, SMS, API). It asks whether the message reports a
// completed transaction at all, which filters out OTPs, newsletters and
// reminders before any paid extraction.
//
// Yes queues message.extract. No ends the message as not_transaction. An answer
// either way that it wasn't sure of parks the message for a person.
func messageReceived(deps WorkerDeps) jobrunner.HandlerFunc {
	return func(ctx context.Context, j *job.Job) (json.RawMessage, error) {
		ctx, budgetID, err := jobrunner.BudgetContext(ctx, j)
		if err != nil {
			return nil, err
		}
		msg, err := jobrunner.DecodePayload[model.ReceivedMessage](j)
		if err != nil {
			return nil, err
		}

		switch msg.Source {
		case "api":
		case "email":
		case "sms":
			break
		default:
			return nil, errs.New(errs.CodeJobInvalidOption, "invalid message source: %s", msg.Source)
		}

		jobrunner.ReportProgress(ctx, job.Progress{Message: "Checking the message"})
		checker, err := deps.Pipeline.Check(ctx, budgetID)
		if err != nil {
			return nil, errs.Wrap(errs.CodeMessageCheckFailed, "pick check adapter", err)
		}
		verdict, err := checker.Check(ctx, toPortMessage(msg))
		if err != nil {
			return nil, errs.Wrap(errs.CodeMessageCheckFailed, "check transaction", err)
		}
		if deps.shouldPark(ctx, pipeline.StepCheck, verdict.Confidence) {
			// Continuing is the person saying that it is a transaction.
			resume, err := resumeJob(budgetID, j, msg, model.JobTypeMessageExtract, func(m model.ReceivedMessage) any {
				verdict := toModelVerdict(verdict)
				verdict.IsTransaction = true
				return model.MessageExtractPayload{Message: m, Verdict: verdict}
			})
			if err != nil {
				return nil, err
			}
			return park(ctx, deps, j, budgetID, msg, "Not sure this is a transaction.", model.MessageReview{
				Step: string(pipeline.StepCheck), Confidence: verdict.Confidence, Resume: resume,
			}, verdict)
		}
		if !verdict.IsTransaction {
			return finish(ctx, deps, j, budgetID, msg, model.MessageOutcomeNotTransaction, "not a transaction", verdict)
		}

		return next(ctx, deps, j, budgetID, msg, model.JobTypeMessageExtract, model.MessageExtractPayload{
			Message: msg,
			Verdict: toModelVerdict(verdict),
		}, verdict)
	}
}

// messageExtract handles model.JobTypeMessageExtract. It pulls the account,
// merchant, amount and date out of the message text.
//
// Always queues message.verify: this step doesn't judge its own output.
func messageExtract(deps WorkerDeps) jobrunner.HandlerFunc {
	return func(ctx context.Context, j *job.Job) (json.RawMessage, error) {
		ctx, budgetID, err := jobrunner.BudgetContext(ctx, j)
		if err != nil {
			return nil, err
		}
		p, err := jobrunner.DecodePayload[model.MessageExtractPayload](j)
		if err != nil {
			return nil, err
		}

		jobrunner.ReportProgress(ctx, job.Progress{Message: "Reading the details"})
		extractor, err := deps.Pipeline.Extract(ctx, budgetID)
		if err != nil {
			return nil, errs.Wrap(errs.CodeMessageExtractFailed, "pick extract adapter", err)
		}
		entities, err := extractor.Extract(ctx, toPortMessage(p.Message))
		if err != nil {
			return nil, errs.Wrap(errs.CodeMessageExtractFailed, "extract entities", err)
		}

		return next(ctx, deps, j, budgetID, p.Message, model.JobTypeMessageVerify, model.MessageEntitiesPayload{
			Message:  p.Message,
			Verdict:  p.Verdict,
			Entities: toModelEntities(entities),
		}, entities)
	}
}

// messageVerify handles model.JobTypeMessageVerify. It checks the extracted
// fields against the message text, so a bad extraction never becomes a
// transaction with the wrong amount or date.
//
// It also adds what it could read (channel, VPA, ref, a missing account). Valid
// queues message.lookup. Details that don't match, or a match it wasn't sure of,
// park the message for a person.
func messageVerify(deps WorkerDeps) jobrunner.HandlerFunc {
	return func(ctx context.Context, j *job.Job) (json.RawMessage, error) {
		ctx, budgetID, err := jobrunner.BudgetContext(ctx, j)
		if err != nil {
			return nil, err
		}
		p, err := jobrunner.DecodePayload[model.MessageEntitiesPayload](j)
		if err != nil {
			return nil, err
		}

		jobrunner.ReportProgress(ctx, job.Progress{Message: "Checking the details"})
		verifier, err := deps.Pipeline.Verify(ctx, budgetID)
		if err != nil {
			return nil, errs.Wrap(errs.CodeMessageVerifyFailed, "pick verify adapter", err)
		}
		res, err := verifier.Verify(ctx, toPortMessage(p.Message), toPortEntities(p.Entities))
		if err != nil {
			return nil, errs.Wrap(errs.CodeMessageVerifyFailed, "verify entities", err)
		}
		p.Entities.Channel, p.Entities.VPA, p.Entities.Ref = res.Channel, res.VPA, res.Ref
		if strings.TrimSpace(p.Entities.Account) == "" {
			p.Entities.Account = res.Account
		}

		reason := ""
		if !res.Valid {
			reason = "The details read from this message don't match it."
		} else if deps.shouldPark(ctx, pipeline.StepVerify, &res.Confidence) {
			reason = "Not sure the details read from this message are right."
		}
		if reason != "" {
			return parkVerify(ctx, deps, j, budgetID, p, reason, res)
		}

		return next(ctx, deps, j, budgetID, p.Message, model.JobTypeMessageLookup, p, res)
	}
}

// messageLookup handles model.JobTypeMessageLookup. It looks for the payee and
// category in what the budget already knows (payee rules, then similar past
// transactions), which skips the LLM when the merchant has been seen before.
//
// A hit queues transaction.create on the pennywise queue. A miss queues
// message.predict.
func messageLookup(deps WorkerDeps) jobrunner.HandlerFunc {
	return func(ctx context.Context, j *job.Job) (json.RawMessage, error) {
		ctx, budgetID, err := jobrunner.BudgetContext(ctx, j)
		if err != nil {
			return nil, err
		}
		p, err := jobrunner.DecodePayload[model.MessageEntitiesPayload](j)
		if err != nil {
			return nil, err
		}

		res, found, err := deps.Memory.Lookup(ctx, toPortMessage(p.Message), toPortEntities(p.Entities))
		if err != nil {
			return nil, errs.Wrap(errs.CodeMessageLookupFailed, "memory lookup", err)
		}
		if !found {
			return next(ctx, deps, j, budgetID, p.Message, model.JobTypeMessagePredict, p, lookupOutput{})
		}

		return createTransaction(ctx, deps, j, budgetID, p, res, model.MessagePredictionMemory, lookupOutput{Found: true, Match: &res})
	}
}

// messagePredict handles model.JobTypeMessagePredict, reached on a memory miss.
// It asks the Predictor for a payee and category.
//
// A prediction queues transaction.create on the pennywise queue. An empty
// CategoryName ends the message as no_prediction.
func messagePredict(deps WorkerDeps) jobrunner.HandlerFunc {
	return func(ctx context.Context, j *job.Job) (json.RawMessage, error) {
		ctx, budgetID, err := jobrunner.BudgetContext(ctx, j)
		if err != nil {
			return nil, err
		}
		p, err := jobrunner.DecodePayload[model.MessageEntitiesPayload](j)
		if err != nil {
			return nil, err
		}

		jobrunner.ReportProgress(ctx, job.Progress{Message: "Choosing the payee and category"})
		predictor, err := deps.Pipeline.Predict(ctx, budgetID)
		if err != nil {
			return nil, errs.Wrap(errs.CodeMessagePredictFailed, "pick predict adapter", err)
		}
		res, err := predictor.Predict(ctx, toPortMessage(p.Message), toPortEntities(p.Entities))
		if err != nil {
			return nil, errs.Wrap(errs.CodeMessagePredictFailed, "predict", err)
		}
		if res.CategoryName == "" {
			return finish(ctx, deps, j, budgetID, p.Message, model.MessageOutcomeNoPrediction, "", res)
		}

		return createTransaction(ctx, deps, j, budgetID, p, res, model.MessagePredictionPredictor, res)
	}
}

// lookupOutput is what the lookup step stores as its result.
type lookupOutput struct {
	Found bool                    `json:"found"`
	Match *ports.PredictionResult `json:"match,omitempty"`
}

// next queues the following step on cipher's queue.
func next(
	ctx context.Context,
	deps WorkerDeps,
	j *job.Job,
	budgetID uuid.UUID,
	msg model.ReceivedMessage,
	jobType string,
	payload any,
	output any,
) (json.RawMessage, error) {
	return enqueue(ctx, deps, j, budgetID, msg, jobType, model.QueueCipher, payload, "", output)
}

// createTransaction hands a predicted message to go-pennywise-api.
func createTransaction(
	ctx context.Context,
	deps WorkerDeps,
	j *job.Job,
	budgetID uuid.UUID,
	p model.MessageEntitiesPayload,
	res ports.PredictionResult,
	source model.MessagePredictionSource,
	output any,
) (json.RawMessage, error) {
	return enqueue(ctx, deps, j, budgetID, p.Message, model.JobTypeTransactionCreate, model.QueuePennywise,
		model.TransactionCreatePayload{
			Message:    p.Message,
			Verdict:    p.Verdict,
			Entities:   p.Entities,
			Prediction: toModelPrediction(res, source),
			Source:     messageSource(p.Message, p.Entities, p.Verdict),
		}, "", output)
}

// finish ends the message early by queueing its pipeline report.
func finish(
	ctx context.Context,
	deps WorkerDeps,
	j *job.Job,
	budgetID uuid.UUID,
	msg model.ReceivedMessage,
	outcome model.MessageOutcome,
	reason string,
	output any,
) (json.RawMessage, error) {
	return enqueue(ctx, deps, j, budgetID, msg, model.JobTypePipelineReport, model.QueuePennywise,
		model.PipelineReportPayload{Message: msg, Outcome: outcome, Reason: reason}, outcome, output)
}

// verifiedFields are what verify checks. It answers for them as a whole, so
// when it doubts the details, all of them are in question.
var verifiedFields = []string{"amount", "account", "date", "merchant"}

// reviewThreshold is the confidence below which a step's answer goes to a person.
const reviewThreshold = 0.7

// shouldPark reports whether a step's confidence is low enough to park the
// message. Until ParkLowConfidence is on it only logs what it would have parked,
// so the threshold can be checked against real scores first. A zero score isn't
// low: it's how adapters without a model (regex, noop) say they don't know.
func (d WorkerDeps) shouldPark(ctx context.Context, step pipeline.StepID, confidence *float64) bool {
	if confidence == nil || *confidence <= 0 || *confidence >= reviewThreshold {
		return false
	}
	if !d.ParkLowConfidence {
		logger.Logger(ctx).Info("low confidence, would park for review", "step", step, "confidence", *confidence)
		return false
	}
	return true
}

// resumeJob is the job that carries on after a person says continue. The
// parked job's ID makes its run new, so a double click can't queue it twice.
func resumeJob(
	budgetID uuid.UUID,
	j *job.Job,
	msg model.ReceivedMessage,
	jobType string,
	payload func(model.ReceivedMessage) any,
) (*model.MessageResume, error) {
	msg.RunID = "review:" + j.ID.String()
	raw, err := json.Marshal(payload(msg))
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "encode resume payload", err)
	}
	return &model.MessageResume{
		JobType:        jobType,
		Queue:          model.QueueCipher,
		IdempotencyKey: msg.IdempotencyKey(budgetID),
		Payload:        raw,
	}, nil
}

// parkVerify parks a message whose details couldn't be trusted. Without an
// amount, date or account a transaction can't be made from what was read, so
// there's nothing to continue with.
func parkVerify(
	ctx context.Context,
	deps WorkerDeps,
	j *job.Job,
	budgetID uuid.UUID,
	p model.MessageEntitiesPayload,
	reason string,
	res ports.VerifyResult,
) (json.RawMessage, error) {
	entities := p.Entities
	review := model.MessageReview{
		Step: string(pipeline.StepVerify), Confidence: &res.Confidence, Entities: &entities, Unsure: verifiedFields,
	}
	if entities.Amount != 0 && !entities.Date.IsZero() && strings.TrimSpace(entities.Account) != "" {
		resume, err := resumeJob(budgetID, j, p.Message, model.JobTypeMessageLookup, func(m model.ReceivedMessage) any {
			return model.MessageEntitiesPayload{Message: m, Verdict: p.Verdict, Entities: entities}
		})
		if err != nil {
			return nil, err
		}
		review.Resume = resume
	}
	return park(ctx, deps, j, budgetID, p.Message, reason, review, res)
}

// park ends the message's run until a person decides what to do with it.
func park(
	ctx context.Context,
	deps WorkerDeps,
	j *job.Job,
	budgetID uuid.UUID,
	msg model.ReceivedMessage,
	reason string,
	review model.MessageReview,
	output any,
) (json.RawMessage, error) {
	review.Text = displayText(msg)
	return enqueue(ctx, deps, j, budgetID, msg, model.JobTypePipelineReport, model.QueuePennywise,
		model.PipelineReportPayload{
			Message: msg, Outcome: model.MessageOutcomeNeedsReview, Reason: reason, Review: &review,
		}, model.MessageOutcomeNeedsReview, output)
}

func enqueue(
	ctx context.Context,
	deps WorkerDeps,
	j *job.Job,
	budgetID uuid.UUID,
	msg model.ReceivedMessage,
	jobType, queue string,
	payload any,
	outcome model.MessageOutcome,
	output any,
) (json.RawMessage, error) {
	id, err := jobrunner.EnqueueNext(ctx, deps.Broker, j, jobType, queue, msg.IdempotencyKey(budgetID), payload)
	if err != nil {
		return nil, err
	}
	return jobrunner.EncodeResult(model.MessageStepResult{Output: output, NextJobType: jobType, NextJobID: &id, Outcome: outcome})
}
