package temporal

import (
	"errors"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/userllm"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"

	"go.temporal.io/sdk/temporal"
)

func TestTerminalIfNoAIProvider(t *testing.T) {
	wrapped := errs.Wrap(errs.CodeInternalError, "extracting email", userllm.ErrNotConfigured)
	var appErr *temporal.ApplicationError
	if !errors.As(terminalIfNoAIProvider(wrapped), &appErr) || !appErr.NonRetryable() {
		t.Fatalf("wrapped not-configured error should be non-retryable")
	}

	other := errors.New("ollama down")
	if got := terminalIfNoAIProvider(other); got != other {
		t.Fatalf("other errors must pass through unchanged, got %v", got)
	}
}
