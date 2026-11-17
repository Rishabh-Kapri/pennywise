package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/llmusage"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/progress"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/statementcheck"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/jobrunner"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/pipeline"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/google/uuid"
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
		case model.MessageSourceAPI, model.MessageSourceEmail, model.MessageSourceSMS:
		default:
			return nil, errs.New(errs.CodeJobInvalidOption, "invalid message source: %s", msg.Source)
		}

		jobrunner.ReportProgress(ctx, job.Progress{Message: "Checking the message"})
		checker, err := deps.Pipeline.Check(ctx, budgetID)
		if err != nil {
			return nil, errs.Wrap(errs.CodeMessageCheckFailed, "pick check adapter", err)
		}
		verdict, err := checker.Check(llmusage.WithPurpose(ctx, model.LLMPurposePipelineCheck), toPortMessage(msg))
		if err != nil {
			return nil, errs.Wrap(errs.CodeMessageCheckFailed, "check transaction", err)
		}
		// A statement isn't a transaction: its PDF is read on its own path.
		if verdict.MessageType == ports.StatementMessageType {
			return routeStatement(ctx, deps, j, budgetID, msg, verdict)
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

// routeStatement sends a statement email's PDFs to ingest (statement.fetch),
// which downloads them and queues their import. The message itself ends here
// as a statement. One without a PDF (a "your statement is ready" notice) ends
// as not a transaction. With the statement pipeline's email step off, the
// PDFs aren't fetched: the budget reads its statements from uploads only.
func routeStatement(
	ctx context.Context,
	deps WorkerDeps,
	j *job.Job,
	budgetID uuid.UUID,
	msg model.ReceivedMessage,
	verdict ports.TxnVerdict,
) (json.RawMessage, error) {
	var pdfs []model.MessageAttachment
	for _, a := range msg.Attachments {
		if a.IsPDF() {
			pdfs = append(pdfs, a)
		}
	}
	if msg.Source != "email" || msg.MailAccount == "" || len(pdfs) == 0 {
		return finish(ctx, deps, j, budgetID, msg, model.MessageOutcomeNotTransaction,
			"a statement notice without a PDF", toModelVerdict(verdict))
	}
	if deps.StatementImport != nil {
		on, err := deps.StatementImport.Enabled(ctx, budgetID, pipeline.StepStatementEmail)
		if err != nil {
			return nil, err
		}
		if !on {
			return finish(ctx, deps, j, budgetID, msg, model.MessageOutcomeStatement,
				"a statement: reading statements from email is turned off, so its PDF wasn't fetched",
				toModelVerdict(verdict))
		}
	}
	if _, err := jobrunner.EnqueueNext(ctx, deps.Broker, j,
		model.JobTypeStatementFetch, model.QueueIngest, msg.IdempotencyKey(budgetID),
		model.StatementFetchPayload{
			Provider:    msg.Provider,
			MailAccount: msg.MailAccount,
			MessageID:   msg.ExternalID,
			Subject:     msg.Subject,
			Attachments: pdfs,
		},
	); err != nil {
		return nil, err
	}
	return finish(ctx, deps, j, budgetID, msg, model.MessageOutcomeStatement,
		"a statement: its PDF is being read", toModelVerdict(verdict))
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
		entities, err := extractor.Extract(
			llmusage.WithPurpose(ctx, model.LLMPurposePipelineExtract),
			toPortMessage(p.Message),
		)
		if err != nil {
			return nil, errs.Wrap(errs.CodeMessageExtractFailed, "extract entities", err)
		}

		return sendEntities(ctx, deps, j, budgetID, model.MessageEntitiesPayload{
			Message:  p.Message,
			Verdict:  p.Verdict,
			Entities: toModelEntities(entities),
		}, entities, pipeline.StepVerify, pipeline.StepLookup, pipeline.StepPredict)
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

		if on, err := deps.Pipeline.Enabled(ctx, budgetID, pipeline.StepVerify); err != nil || !on {
			if err != nil {
				return nil, err
			}
			return sendEntities(ctx, deps, j, budgetID, p, turnedOff(pipeline.StepVerify),
				pipeline.StepLookup, pipeline.StepPredict)
		}

		jobrunner.ReportProgress(ctx, job.Progress{Message: "Checking the details"})
		verifier, err := deps.Pipeline.Verify(ctx, budgetID)
		if err != nil {
			return nil, errs.Wrap(errs.CodeMessageVerifyFailed, "pick verify adapter", err)
		}
		res, err := verifier.Verify(
			llmusage.WithPurpose(ctx, model.LLMPurposePipelineVerify),
			toPortMessage(p.Message),
			toPortEntities(p.Entities),
		)
		if err != nil {
			return nil, errs.Wrap(errs.CodeMessageVerifyFailed, "verify entities", err)
		}
		p.Entities.Channel, p.Entities.VPA, p.Entities.Ref = res.Channel, res.VPA, res.Ref
		p.Entities.VerifiedConfidence, p.Entities.VerifiedBy = &res.Confidence, res.Model
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

		return sendEntities(ctx, deps, j, budgetID, p, res, pipeline.StepLookup, pipeline.StepPredict)
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

		if on, err := deps.Pipeline.Enabled(ctx, budgetID, pipeline.StepLookup); err != nil || !on {
			if err != nil {
				return nil, err
			}
			return sendEntities(ctx, deps, j, budgetID, p, turnedOff(pipeline.StepLookup), pipeline.StepPredict)
		}

		memory, err := deps.Pipeline.Lookup(ctx, budgetID)
		if err != nil {
			return nil, errs.Wrap(errs.CodeMessageLookupFailed, "pick lookup adapter", err)
		}
		res, found, err := memory.Lookup(ctx, toPortMessage(p.Message), toPortEntities(p.Entities))
		if err != nil {
			return nil, errs.Wrap(errs.CodeMessageLookupFailed, "memory lookup", err)
		}
		if !found {
			return sendEntities(ctx, deps, j, budgetID, p, lookupOutput{}, pipeline.StepPredict)
		}

		return createTransaction(
			ctx,
			deps,
			j,
			budgetID,
			p,
			res,
			model.MessagePredictionMemory,
			lookupOutput{Found: true, Match: &res},
		)
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

		if on, err := deps.Pipeline.Enabled(ctx, budgetID, pipeline.StepPredict); err != nil || !on {
			if err != nil {
				return nil, err
			}
			return parkNoPrediction(ctx, deps, j, budgetID, p, turnedOff(pipeline.StepPredict))
		}

		jobrunner.ReportProgress(ctx, job.Progress{Message: "Choosing the payee and category"})
		predictor, err := deps.Pipeline.Predict(ctx, budgetID)
		if err != nil {
			return nil, errs.Wrap(errs.CodeMessagePredictFailed, "pick predict adapter", err)
		}
		res, err := predictor.Predict(
			llmusage.WithPurpose(ctx, model.LLMPurposePipelinePredict),
			toPortMessage(p.Message),
			toPortEntities(p.Entities),
		)
		if err != nil {
			return nil, errs.Wrap(errs.CodeMessagePredictFailed, "predict", err)
		}
		if res.CategoryName == "" {
			return finish(ctx, deps, j, budgetID, p.Message, model.MessageOutcomeNoPrediction, "", res)
		}

		return createTransaction(ctx, deps, j, budgetID, p, res, model.MessagePredictionPredictor, res)
	}
}

func statementExtract(deps WorkerDeps) jobrunner.HandlerFunc {
	return func(ctx context.Context, j *job.Job) (json.RawMessage, error) {
		ctx, budgetID, err := jobrunner.BudgetContext(ctx, j)
		if err != nil {
			return nil, err
		}
		p, err := jobrunner.DecodePayload[model.StatementInput](j)
		if err != nil {
			return nil, err
		}
		if p.ImportID == uuid.Nil {
			return nil, errs.New(errs.CodeJobInvalidPayload, "statement.extract has no importId")
		}

		jobrunner.ReportProgress(ctx, job.Progress{Message: "starting extraction"})

		// The typed password first, then each account's saved one, loaded
		// now so one saved after the upload counts when the job is retried.
		passwords := []string{}
		if typedPassword := openTypedPassword(ctx, deps, p); typedPassword != "" {
			passwords = append(passwords, typedPassword)
		}
		typed := len(passwords)
		saved := savedStatementPasswords(ctx, deps, budgetID)
		for _, c := range saved {
			passwords = append(passwords, c.password)
		}
		res, err := readStatement(ctx, deps, budgetID, p.Data, passwords)
		if err != nil {
			return nil, err
		}
		save := model.StatementSavePayload{ImportID: p.ImportID, Sections: res.Sections}
		if i := res.PasswordIndex - typed; res.PasswordIndex >= 0 && i >= 0 && i < len(saved) {
			save.OpenedWith = &saved[i].accountID
		}

		// The API saves the rows onto the import; the key is the import, so a
		// retried extract gets back the save it already queued.
		saveID, err := jobrunner.EnqueueNext(ctx, deps.Broker, j,
			model.JobTypeStatementSave, model.QueuePennywise,
			"statement:"+p.ImportID.String(), save,
		)
		if err != nil {
			return nil, err
		}

		rows := 0
		for _, s := range res.Sections {
			rows += len(s.Rows)
		}
		return jobrunner.EncodeResult(map[string]any{"rows": rows, "nextJobId": saveID})
	}
}

// readStatement reads the PDF with the budget's extract step and checks the
// rows against the balances the statement printed. When they don't check out,
// or it printed nothing to check against, the fallback step reads it again and
// the better of the two reads is kept. A fallback that can't run or fails
// leaves the first read. A file that isn't a PDF (a bank's CSV export) gets
// no fallback.
func readStatement(
	ctx context.Context,
	deps WorkerDeps,
	budgetID uuid.UUID,
	pdf []byte,
	passwords []string,
) (model.StatementExtractResponse, error) {
	log := logger.Logger(ctx).With("budgetId", budgetID)

	extractor, err := deps.StatementImport.Extract(ctx, budgetID)
	if err != nil {
		return model.StatementExtractResponse{}, err
	}
	res, err := extractor.ExtractStatement(
		llmusage.WithPurpose(withJobProgress(ctx), model.LLMPurposeStatementExtract),
		pdf,
		passwords,
	)
	if err != nil {
		return model.StatementExtractResponse{}, err
	}
	check := statementcheck.Check(res)
	// Counts and reasons only: the rows are the user's statement.
	log.Info("statement read", "step", "extract", "ok", check.OK(), "rows", check.Rows,
		"verified", check.Verified, "sections", check.Sections, "reasons", check.Reasons)
	if check.OK() {
		return res, nil
	}
	// The fallback readers read PDFs; a bank's CSV export is read exactly once.
	if !bytes.HasPrefix(bytes.TrimLeft(pdf, "\x00\t\r\n "), []byte("%PDF-")) {
		return res, nil
	}
	if on, err := deps.StatementImport.Enabled(ctx, budgetID, pipeline.StepStatementFallback); err != nil || !on {
		if err != nil {
			return model.StatementExtractResponse{}, err
		}
		log.Info("statement fallback is turned off, keeping the first read")
		return res, nil
	}

	jobrunner.ReportProgress(ctx, job.Progress{Message: "rows don't check out, reading again"})
	fallback, err := deps.StatementImport.Fallback(ctx, budgetID)
	if err != nil {
		log.Warn("statement fallback can't run, keeping the first read", "error", err)
		return res, nil
	}
	again, err := fallback.ExtractStatement(
		llmusage.WithPurpose(withJobProgress(ctx), model.LLMPurposeStatementFallback),
		pdf,
		passwords,
	)
	if err != nil {
		if ctx.Err() != nil {
			return model.StatementExtractResponse{}, ctx.Err()
		}
		log.Warn("statement fallback failed, keeping the first read", "error", err)
		return res, nil
	}
	againCheck := statementcheck.Check(again)
	keep := statementcheck.Better(againCheck, check)
	log.Info("statement read", "step", "fallback", "ok", againCheck.OK(), "rows", againCheck.Rows,
		"verified", againCheck.Verified, "sections", againCheck.Sections, "reasons", againCheck.Reasons,
		"kept", map[bool]string{true: "fallback", false: "extract"}[keep])
	if keep {
		return again, nil
	}
	return res, nil
}

// withJobProgress forwards progress reports from services and adapters to the
// running job's progress, so they don't need to know they run in a job.
func withJobProgress(ctx context.Context) context.Context {
	return progress.With(ctx, func(s progress.Step) {
		jobrunner.ReportProgress(ctx, job.Progress{Message: s.Name, Current: s.Current, Total: s.Total})
	})
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
			Message:     p.Message,
			Verdict:     p.Verdict,
			Entities:    p.Entities,
			Prediction:  toModelPrediction(res, source),
			Source:      messageSource(p.Message, p.Entities, p.Verdict),
			DisplayText: displayText(p.Message),
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
// so the threshold can be checked against real scores first. No score means
// the adapter doesn't give one.
func (d WorkerDeps) shouldPark(ctx context.Context, step pipeline.StepID, confidence *float64) bool {
	if confidence == nil || *confidence >= reviewThreshold {
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
	return jobrunner.EncodeResult(
		model.MessageStepResult{Output: output, NextJobType: jobType, NextJobID: &id, Outcome: outcome},
	)
}

type savedStatementPassword struct {
	accountID uuid.UUID
	password  string
}

// openTypedPassword returns the password typed at upload. The API seals it;
// jobs queued before that carry it in plain text. One that can't be opened
// (no key, or a different one) is logged and skipped, so the saved passwords
// still get their turn.
func openTypedPassword(ctx context.Context, deps WorkerDeps, p model.StatementInput) string {
	if len(p.SealedPassword) == 0 {
		return p.Password
	}
	log := logger.Logger(ctx)
	if deps.SecretBox == nil {
		log.Warn("statement has a typed password, but LLM_CREDENTIALS_KEY isn't set on cipher")
		return ""
	}
	password, err := deps.SecretBox.Open(p.SealedPassword)
	if err != nil {
		log.Warn("couldn't open the typed statement password", "error", err)
		return ""
	}
	return password
}

// savedStatementPasswords opens the budget's saved statement passwords, in
// account order so retries try them the same way. One that won't open (the
// key changed) is skipped: the PDF may not need it. Only counts are logged.
func savedStatementPasswords(ctx context.Context, deps WorkerDeps, budgetID uuid.UUID) []savedStatementPassword {
	if deps.StatementPasswords == nil || deps.SecretBox == nil {
		logger.Logger(ctx).Warn("saved statement passwords unavailable (no repository or LLM_CREDENTIALS_KEY)")
		return nil
	}
	sealed, err := deps.StatementPasswords.ListByBudget(ctx, budgetID)
	if err != nil {
		logger.Logger(ctx).Warn("saved statement passwords not loaded", "error", err)
		return nil
	}
	out := make([]savedStatementPassword, 0, len(sealed))
	for accountID, box := range sealed {
		pw, err := deps.SecretBox.Open(box)
		if err != nil {
			logger.Logger(ctx).Warn("saved statement password not opened", "accountId", accountID, "error", err)
			continue
		}
		out = append(out, savedStatementPassword{accountID, pw})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].accountID.String() < out[b].accountID.String() })
	logger.Logger(ctx).Info("saved statement passwords loaded", "saved", len(sealed), "opened", len(out))
	return out
}
