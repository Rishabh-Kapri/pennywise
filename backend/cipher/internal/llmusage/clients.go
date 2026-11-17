package llmusage

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/decisions"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/client"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

// errorLimit bounds the provider error kept on a row.
const errorLimit = 500

// Key is the saved key a client runs on: who owns it and which provider it's for.
type Key struct {
	Provider sharedModel.LLMProviderName
	OwnerID  uuid.UUID
}

// call is one model call being timed; finish turns it into a row.
type call struct {
	ctx     context.Context
	key     Key
	purpose sharedModel.LLMUsagePurpose
	model   string
	started time.Time
}

func (k Key) start(ctx context.Context, purpose sharedModel.LLMUsagePurpose, model string) call {
	return call{ctx: ctx, key: k, purpose: purpose, model: model, started: time.Now()}
}

// finish builds the row. model is what the provider said answered ("" keeps
// the requested one); usage may be nil when the call returned none.
func (c call) finish(model string, usage *sharedModel.Usage, stopReason sharedModel.StopReason, err error) sharedModel.LLMUsage {
	row := sharedModel.LLMUsage{
		Purpose:   c.purpose,
		Provider:  string(c.key.Provider),
		Model:     c.model,
		LatencyMs: int(time.Since(c.started).Milliseconds()),
		Status:    sharedModel.LLMUsageStatusOK,
	}
	if model != "" {
		row.Model = model
	}
	if c.key.OwnerID != uuid.Nil {
		owner := c.key.OwnerID
		row.UserID = &owner
	}
	if budgetID, berr := utils.BudgetIDFromContext(c.ctx); berr == nil && budgetID != uuid.Nil {
		row.BudgetID = &budgetID
	}
	if runID, ok := AgentRunFrom(c.ctx); ok {
		row.AgentRunID = &runID
	}
	if version := PromptVersionFrom(c.ctx); version != "" {
		row.PromptVersion = &version
	}
	if stopReason != "" {
		reason := string(stopReason)
		row.StopReason = &reason
	}
	if err != nil {
		row.Status = sharedModel.LLMUsageStatusError
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || c.ctx.Err() != nil {
			row.Status = sharedModel.LLMUsageStatusCancelled
		}
		msg := []rune(err.Error())
		if len(msg) > errorLimit {
			msg = msg[:errorLimit]
		}
		text := string(msg)
		row.Error = &text
	}
	if usage != nil {
		applyUsage(&row, *usage)
	}
	// A local model costs nothing per call.
	if c.key.Provider == sharedModel.LLMProviderOllama && row.CostMicros == nil {
		zero := int64(0)
		row.CostMicros = &zero
	}
	return row
}

// applyUsage copies the counts a provider reported. A provider that reported
// nothing leaves them NULL rather than claiming zero.
func applyUsage(row *sharedModel.LLMUsage, u sharedModel.Usage) {
	if u.Available || u.InputTokens > 0 || u.OutputTokens > 0 || u.CacheReadTokens > 0 || u.CacheWriteTokens > 0 {
		input, output, cacheRead, cacheWrite := u.InputTokens, u.OutputTokens, u.CacheReadTokens, u.CacheWriteTokens
		row.InputTokens = &input
		row.OutputTokens = &output
		row.CacheReadTokens = &cacheRead
		row.CacheWriteTokens = &cacheWrite
	}
	if u.Cost != nil && *u.Cost >= 0 && !math.IsNaN(*u.Cost) && !math.IsInf(*u.Cost, 0) {
		micros := int64(math.Round(*u.Cost * 1_000_000))
		row.CostMicros = &micros
	}
}

// LLM records every Chat and Stream call of a chat client.
type LLM struct {
	inner    llm.LLM
	key      Key
	recorder Recorder
}

// WrapLLM returns inner unchanged when there's no recorder.
func WrapLLM(inner llm.LLM, key Key, recorder Recorder) llm.LLM {
	if recorder == nil {
		return inner
	}
	return &LLM{inner: inner, key: key, recorder: recorder}
}

func (l *LLM) Chat(
	ctx context.Context,
	req sharedModel.ChatRequest,
	conversationID *string,
) (*sharedModel.ChatResponse, error) {
	c := l.key.start(ctx, PurposeFrom(ctx, sharedModel.LLMPurposeOther), req.Model)
	res, err := l.inner.Chat(ctx, req, conversationID)
	if res == nil {
		l.recorder.Record(ctx, c.finish("", nil, "", err))
		return res, err
	}
	l.recorder.Record(ctx, c.finish(res.Model, &res.Usage, res.StopReason, err))
	return res, err
}

func (l *LLM) Stream(
	ctx context.Context,
	req sharedModel.ChatRequest,
	conversationID *string,
) <-chan sharedModel.StreamChunk {
	c := l.key.start(ctx, PurposeFrom(ctx, sharedModel.LLMPurposeOther), req.Model)
	upstream := l.inner.Stream(ctx, req, conversationID)
	out := make(chan sharedModel.StreamChunk)

	go func() {
		defer close(out)
		var (
			usage      *sharedModel.Usage
			model      string
			stopReason sharedModel.StopReason
			streamErr  error
			completed  bool
		)
		// Recorded before out closes, so the row exists by the time the caller
		// sees the stream end.
		defer func() {
			if streamErr == nil && !completed {
				streamErr = errors.New("llm stream closed without a completion event")
			}
			l.recorder.Record(ctx, c.finish(model, usage, stopReason, streamErr))
		}()

		for {
			select {
			case chunk, ok := <-upstream:
				if !ok {
					return
				}
				switch chunk.Type {
				case sharedModel.ChunkEventCompleted:
					completed = true
					u := chunk.Usage
					usage = &u
					model = chunk.Model
					stopReason = chunk.StopReason
				case sharedModel.ChunkEventError:
					streamErr = errors.New(chunk.Text)
				}
				select {
				case out <- chunk:
				case <-ctx.Done():
					streamErr = ctx.Err()
					go drain(upstream)
					return
				}
			case <-ctx.Done():
				streamErr = ctx.Err()
				go drain(upstream)
				return
			}
		}
	}()
	return out
}

// drain lets the provider's goroutine finish once nobody reads its stream.
func drain(ch <-chan sharedModel.StreamChunk) {
	for range ch {
	}
}

// Decisions records every Ask of a decisions client.
type Decisions struct {
	inner    decisions.Decisions
	key      Key
	recorder Recorder
}

// WrapDecisions returns inner unchanged when there's no recorder.
func WrapDecisions(inner decisions.Decisions, key Key, recorder Recorder) decisions.Decisions {
	if recorder == nil {
		return inner
	}
	return &Decisions{inner: inner, key: key, recorder: recorder}
}

func (d *Decisions) Ask(ctx context.Context, req sharedModel.DecisionRequest) (*sharedModel.DecisionResponse, error) {
	c := d.key.start(ctx, PurposeFrom(ctx, sharedModel.LLMPurposeDecision), req.Model)
	res, err := d.inner.Ask(ctx, req)
	if res == nil {
		d.recorder.Record(ctx, c.finish("", nil, "", err))
		return res, err
	}
	d.recorder.Record(ctx, c.finish(res.Model, &res.Usage, "", err))
	return res, err
}

// Embedder records every Embed call. Embedding APIs here report no usage, so
// its rows carry latency and status only.
type Embedder struct {
	inner    client.Embedder
	key      Key
	recorder Recorder
}

// WrapEmbedder returns inner unchanged when there's no recorder.
func WrapEmbedder(inner client.Embedder, key Key, recorder Recorder) client.Embedder {
	if recorder == nil {
		return inner
	}
	return &Embedder{inner: inner, key: key, recorder: recorder}
}

func (e *Embedder) Embed(ctx context.Context, model string, text string) ([]float64, error) {
	// Always "embedding", even inside an agent run or a pipeline step: an
	// embedding is never one of the step's own model turns.
	c := e.key.start(ctx, sharedModel.LLMPurposeEmbedding, model)
	vector, err := e.inner.Embed(ctx, model, text)
	e.recorder.Record(ctx, c.finish("", nil, "", err))
	return vector, err
}
