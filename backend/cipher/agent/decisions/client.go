package decisions

import (
	"context"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
)

type Decisions interface {
	Ask(ctx context.Context, req sharedModel.DecisionRequest) (*sharedModel.DecisionResponse, error)
}

type ObservedDecisions struct {
	client     Decisions
	telelmetry otelSDK.TelemetryProvider
}

func NewObservedDecisions(client Decisions, tel otelSDK.TelemetryProvider) Decisions {
	return &ObservedDecisions{client: client, telelmetry: tel}
}

func (o *ObservedDecisions) Ask(
	ctx context.Context,
	req sharedModel.DecisionRequest,
) (*sharedModel.DecisionResponse, error) {
	res, err := o.client.Ask(ctx, req)
	if err != nil {}

	return res, err
}
