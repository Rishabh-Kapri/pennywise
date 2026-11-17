package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
	"unicode/utf8"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
)

// toolPayloadLimit bounds a tool invocation's stored input and output. Most
// results are a few KB; a larger one (a wide SQL result) keeps a preview.
const toolPayloadLimit = 32 << 10

// toolPayloadPreview is how much of an oversized payload is kept.
const toolPayloadPreview = 16 << 10

// promptVersionLength is how many hex characters of the hash are kept.
const promptVersionLength = 12

// PromptVersion identifies the prompt a run is made with: a hash of the
// system prompt (the static part and the dynamic template, before its values
// are filled in) and the tool definitions. Any edit to either gives a new
// version, so runs and evals can be compared per version.
func PromptVersion(prompt SystemPrompt, tools []sharedModel.ToolDefiniton) string {
	h := sha256.New()
	h.Write([]byte(prompt.Static))
	h.Write([]byte{0})
	h.Write([]byte(prompt.Dynamic))
	h.Write([]byte{0})
	// Struct fields marshal in order and map keys sorted, so this is stable.
	if encoded, err := json.Marshal(tools); err == nil {
		h.Write(encoded)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))[:promptVersionLength]
}

// newToolInvocation builds the row for one tool call, bounding what's stored.
func newToolInvocation(
	sequence int,
	turn int,
	call sharedModel.ToolCall,
	isError bool,
	resultJSON []byte,
	duration time.Duration,
) sharedModel.ToolInvocation {
	input := call.Arguments
	if len(bytes.TrimSpace(input)) == 0 {
		input = json.RawMessage(`{}`)
	}
	storedInput, inputBytes, inputCut := boundToolJSON(input)
	storedOutput, outputBytes, outputCut := boundToolJSON(resultJSON)
	durationMs := int(duration.Milliseconds())
	return sharedModel.ToolInvocation{
		Sequence:    sequence,
		Turn:        max(turn, 1),
		ToolCallID:  call.ID,
		ToolName:    call.Name,
		Input:       storedInput,
		Output:      storedOutput,
		IsError:     isError,
		InputBytes:  inputBytes,
		OutputBytes: outputBytes,
		Truncated:   inputCut || outputCut,
		DurationMs:  &durationMs,
	}
}

// truncatedPayload is what an oversized input or output is stored as.
type truncatedPayload struct {
	Truncated bool   `json:"truncated"`
	Bytes     int    `json:"bytes"`
	Preview   string `json:"preview"`
}

// boundToolJSON returns raw as JSON Postgres will store, cut to
// toolPayloadLimit, with its original size. Text that isn't valid JSON (a
// model's malformed arguments) is stored as a JSON string.
func boundToolJSON(raw []byte) (json.RawMessage, int, bool) {
	size := len(raw)
	if !json.Valid(raw) {
		encoded, err := json.Marshal(string(raw))
		if err != nil {
			encoded = []byte(`""`)
		}
		raw = encoded
	}
	// jsonb refuses the NUL character.
	raw = bytes.ReplaceAll(raw, []byte(`\u0000`), []byte(`�`))
	if len(raw) <= toolPayloadLimit {
		return json.RawMessage(raw), size, false
	}
	preview := raw[:toolPayloadPreview]
	for len(preview) > 0 && !utf8.Valid(preview) {
		preview = preview[:len(preview)-1]
	}
	encoded, err := json.Marshal(truncatedPayload{Truncated: true, Bytes: size, Preview: string(preview)})
	if err != nil {
		return json.RawMessage(`{"truncated":true}`), size, true
	}
	return json.RawMessage(encoded), size, true
}

// TruncatedPreview reports whether a stored payload was cut, and the part kept.
func TruncatedPreview(stored json.RawMessage) (string, bool) {
	var payload truncatedPayload
	if err := json.Unmarshal(stored, &payload); err != nil || !payload.Truncated {
		return "", false
	}
	return payload.Preview, true
}

// recordRunResult saves the run's tool invocations and fills its metric
// columns. Failing to is logged: the user already has their answer.
func (a *Agent) recordRunResult(ctx context.Context, payload runFinishedContext) {
	if a.runStore == nil {
		return
	}
	log := logger.Logger(ctx)
	runID, err := uuid.Parse(payload.runID)
	if err != nil {
		log.Error("run id is not a uuid, not recording the run's result", "runId", payload.runID, "error", err)
		return
	}
	budgetID, err := utils.BudgetIDFromContext(ctx)
	if err != nil {
		log.Error("no budget on the run's context, not recording the run's result", "error", err)
		return
	}
	if err := a.runStore.RecordRunResult(utils.DetachedRequestContext(ctx), repository.RecordAgentRunResult{
		RunID:           runID,
		BudgetID:        budgetID,
		StopReason:      payload.stopReason,
		PromptVersion:   payload.promptVersion,
		ToolInvocations: payload.toolInvocations,
	}); err != nil {
		log.Error("recording the run's result failed", "runId", payload.runID, "error", err)
	}
}
