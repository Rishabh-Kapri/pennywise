package errs

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorStringUnchangedByStatusAndDetails(t *testing.T) {
	cause := errors.New("boom")
	plain := Wrap(CodeInternalError, "do thing", cause)
	rich := Wrap(CodeInternalError, "do thing", cause, WithStatus(429), WithDetail("model", "m"))
	if plain.Error() != "INTERNAL_ERROR: do thing: boom" {
		t.Fatalf("Wrap message changed: %q", plain.Error())
	}
	if rich.Error() != plain.Error() {
		t.Fatalf("options leaked into Error(): %q", rich.Error())
	}
	if got := New(CodeInvalidArgument, "bad %s", "id").Error(); got != "INVALID_ARGUMENT: bad id" {
		t.Fatalf("New message changed: %q", got)
	}
	if got := NewWith(CodeInvalidArgument, "100% literal").Error(); got != "INVALID_ARGUMENT: 100% literal" {
		t.Fatalf("NewWith must not format: %q", got)
	}
}

func TestStatusOfWalksTheChain(t *testing.T) {
	upstream := NewWith(CodeHTTPClientError, "POST failed", WithStatus(401))
	wrapped := Wrap(CodeDecisionsRequestFailed, "ask", upstream)
	viaFmt := fmt.Errorf("activity: %w", wrapped)

	for name, err := range map[string]error{"direct": upstream, "wrapped": wrapped, "fmt-wrapped": viaFmt} {
		if got := StatusOf(err); got != 401 {
			t.Errorf("%s: StatusOf = %d, want 401", name, got)
		}
	}
	if got := StatusOf(Wrap(CodeDecisionsRequestFailed, "ask", upstream, WithStatus(502))); got != 502 {
		t.Errorf("outer status should win, got %d", got)
	}
	if StatusOf(errors.New("plain")) != 0 || StatusOf(nil) != 0 || StatusOf(New(CodeInternalError, "x")) != 0 {
		t.Error("StatusOf should be 0 without a status")
	}
}

func TestHasCodeWalksTheChain(t *testing.T) {
	inner := New(CodeJobsUnavailable, "no jobs")
	wrapped := fmt.Errorf("claim: %w", Wrap(CodeJobClaimFailed, "claim", inner))

	if !HasCode(wrapped, CodeJobsUnavailable) || !HasCode(wrapped, CodeJobClaimFailed) {
		t.Fatal("HasCode should find both codes in the chain")
	}
	if HasCode(wrapped, CodeJobNotFound) {
		t.Fatal("HasCode matched a code that isn't there")
	}
	if HasCode(nil, CodeJobsUnavailable) || HasCode(errors.New("plain"), CodeJobsUnavailable) {
		t.Fatal("HasCode should be false without an *Error")
	}
}

func TestDetailsOfMergesOuterFirst(t *testing.T) {
	inner := NewWith(CodeHTTPClientError, "x", WithDetail("model", "inner"), WithDetail("url", "u"))
	outer := Wrap(CodeDecisionsRequestFailed, "y", inner, WithDetails(map[string]any{"model": "outer"}))
	d := DetailsOf(fmt.Errorf("ctx: %w", outer))
	if d["model"] != "outer" || d["url"] != "u" || len(d) != 2 {
		t.Fatalf("DetailsOf = %v", d)
	}
	if DetailsOf(errors.New("plain")) != nil {
		t.Fatal("DetailsOf should be nil without details")
	}
	// DetailsOf returns a copy: changing it must not touch the error.
	d["model"] = "changed"
	if DetailsOf(outer)["model"] != "outer" {
		t.Fatal("DetailsOf leaked the error's map")
	}
}

func TestErrorsAsStillFindsTheCode(t *testing.T) {
	err := fmt.Errorf("wrap: %w", Wrap(CodeCategoryLookupFailed, "lookup", nil, WithStatus(404)))
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeCategoryLookupFailed {
		t.Fatalf("errors.As lost the code: %v", err)
	}
}
