package job

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

// Metadata is the part of utils.RequestMetadata a job carries from the
// enqueuer to the worker: the same two fields the Temporal propagator
// carries. It never holds the user, budget or internal-verified flag:
// a stored value must not grant trust when it is read back. The job's budget
// is Job.BudgetID.
type Metadata struct {
	CorrelationID string `json:"correlationId,omitempty"`
	OriginService string `json:"originService,omitempty"`
}

// MetadataFromContext captures the enqueuing request's metadata. A ctx
// without a correlation ID gets a new one, so every job can be traced.
func MetadataFromContext(ctx context.Context) Metadata {
	rm := utils.RequestMetadataFromContext(ctx)
	m := Metadata{CorrelationID: rm.CorrelationID, OriginService: rm.OriginService}
	if m.CorrelationID == "" {
		m.CorrelationID = utils.NewCorrelationID()
	}
	if m.OriginService == "" {
		m.OriginService = rm.LocalService // the job started in this service
	}
	return m
}

// Context returns ctx carrying m, for running the job: logs get the
// correlation ID and outbound calls send it on.
func (m Metadata) Context(ctx context.Context) context.Context {
	if m.CorrelationID != "" {
		ctx = utils.WithCorrelationID(ctx, m.CorrelationID)
	}
	if m.OriginService != "" {
		ctx = utils.WithOriginService(ctx, m.OriginService)
	}
	return ctx
}
