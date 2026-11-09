// Package unimplemented holds stand-ins for pipeline ports that have no real
// adapter yet. Each one returns CodeNotImplemented, so a job reaching that step
// fails visibly instead of finishing as if it had done the work. Delete an
// entry once its real adapter is in factory.NewMessagePorts.
package unimplemented

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
)

type extract struct{}

// NewExtract returns a stand-in for ports.Extract.
func NewExtract() ports.Extract { return extract{} }

func (extract) Extract(context.Context, ports.Message) (ports.ExtractedEntities, error) {
	return ports.ExtractedEntities{}, errs.New(errs.CodeNotImplemented, "no adapter for ports.Extract")
}

type verifier struct{}

// NewVerifier returns a stand-in for ports.Verifier.
func NewVerifier() ports.Verifier { return verifier{} }

func (verifier) Verify(context.Context, ports.Message, ports.ExtractedEntities) (ports.VerifyResult, error) {
	return ports.VerifyResult{}, errs.New(errs.CodeNotImplemented, "no adapter for ports.Verifier")
}

type memoryLookup struct{}

// NewMemoryLookup returns a stand-in for ports.MemoryLookup that never finds
// anything, so every message goes on to the Predictor.
func NewMemoryLookup() ports.MemoryLookup { return memoryLookup{} }

func (memoryLookup) Lookup(
	context.Context, ports.Message, ports.ExtractedEntities,
) (ports.PredictionResult, bool, error) {
	return ports.PredictionResult{}, false, nil
}

type predictor struct{}

// NewPredictor returns a stand-in for ports.Predictor.
func NewPredictor() ports.Predictor { return predictor{} }

func (predictor) Predict(context.Context, ports.Message, ports.ExtractedEntities) (ports.PredictionResult, error) {
	return ports.PredictionResult{}, errs.New(errs.CodeNotImplemented, "no adapter for ports.Predictor")
}
