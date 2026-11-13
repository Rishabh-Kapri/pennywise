package factory

import (
	"context"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/jobrunner"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/pipeline"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
)

// chainLink is one model to try. name is what logs call it, e.g.
// "decisions:typesafe/jev-latest"; model is what the job's progress shows.
type chainLink[P any] struct {
	name  string
	model string
	port  P
}

type checkChain struct {
	links []chainLink[ports.CheckTransaction]
}

// firstSuccess calls each link in order and returns the first answer. Any error
// moves to the next link, unless the job itself was cancelled.
func firstSuccess[P, R any](
	ctx context.Context,
	step pipeline.StepID,
	links []chainLink[P],
	call func(P) (R, error),
) (R, error) {
	log := logger.Logger(ctx).With("step", step)
	log.Debug("chain started", "links", linkNames(links))

	var zero R
	if len(links) == 0 {
		return zero, errs.New(errs.CodeAdapterNotConfigured, "no model configured for %s", step)
	}
	var lastErr error
	for i, link := range links {
		log.Debug("chain trying model", "model", link.name, "position", i+1, "of", len(links))
		if len(links) > 1 {
			jobrunner.ReportProgress(ctx, job.Progress{Message: "Asking " + link.model, Current: i + 1, Total: len(links)})
		}
		startedAt := time.Now()
		res, err := call(link.port)
		elapsedMs := time.Since(startedAt).Milliseconds()
		if err == nil {
			if i == 0 {
				log.Debug("chain answered", "model", link.name, "elapsedMs", elapsedMs)
			} else {
				log.Info("chain answered after fallback", "model", link.name, "position", i+1, "elapsedMs", elapsedMs)
			}
			return res, nil
		}
		if ctx.Err() != nil {
			log.Warn("chain stopped, job cancelled", "model", link.name, "error", ctx.Err())
			return zero, ctx.Err() // don't try next model on context cancel
		}
		lastErr = err
		if i < len(links)-1 {
			log.Warn("model failed, trying next",
				"model", link.name, "next", links[i+1].name, "elapsedMs", elapsedMs, "error", err)
		} else {
			log.Warn("last model failed", "model", link.name, "elapsedMs", elapsedMs, "error", err)
		}
	}
	log.Error("every model in the chain failed", "links", linkNames(links), "error", lastErr)
	return zero, lastErr
}

func (c checkChain) Check(ctx context.Context, msg ports.Message) (ports.TxnVerdict, error) {
	return firstSuccess(ctx, pipeline.StepCheck, c.links, func(link ports.CheckTransaction) (ports.TxnVerdict, error) {
		return link.Check(ctx, msg)
	})
}

type extractChain struct {
	links []chainLink[ports.Extract]
}

func (c extractChain) Extract(ctx context.Context, msg ports.Message) (ports.ExtractedEntities, error) {
	return firstSuccess(ctx, pipeline.StepExtract, c.links, func(link ports.Extract) (ports.ExtractedEntities, error) {
		return link.Extract(ctx, msg)
	})
}

type verifyChain struct {
	links []chainLink[ports.Verifier]
}

func (c verifyChain) Verify(
	ctx context.Context,
	msg ports.Message,
	entities ports.ExtractedEntities,
) (ports.VerifyResult, error) {
	return firstSuccess(ctx, pipeline.StepVerify, c.links, func(link ports.Verifier) (ports.VerifyResult, error) {
		return link.Verify(ctx, msg, entities)
	})
}

type predictChain struct {
	links []chainLink[ports.Predictor]
}

func (c predictChain) Predict(
	ctx context.Context,
	msg ports.Message,
	entities ports.ExtractedEntities,
) (ports.PredictionResult, error) {
	return firstSuccess(ctx, pipeline.StepPredict, c.links, func(link ports.Predictor) (ports.PredictionResult, error) {
		return link.Predict(ctx, msg, entities)
	})
}
