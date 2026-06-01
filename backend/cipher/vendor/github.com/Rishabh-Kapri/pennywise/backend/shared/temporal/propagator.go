package temporal

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"go.temporal.io/sdk/converter"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

type workflowContextKey string

const requestMetadataWorkflowKey workflowContextKey = "requestMetadata"

type RequestMetadataPropagator struct {
	dataConverter converter.DataConverter
}

func NewRequestMetadataPropagator() sdkworkflow.ContextPropagator {
	return &RequestMetadataPropagator{dataConverter: converter.GetDefaultDataConverter()}
}

func ContextPropagators() []sdkworkflow.ContextPropagator {
	return []sdkworkflow.ContextPropagator{NewRequestMetadataPropagator()}
}

func RequestMetadataFromWorkflowContext(ctx sdkworkflow.Context) utils.RequestMetadata {
	metadata, ok := ctx.Value(requestMetadataWorkflowKey).(utils.RequestMetadata)
	if !ok {
		return utils.RequestMetadata{}
	}

	return normalizeTemporalMetadata(metadata)
}

func WithRequestMetadata(ctx sdkworkflow.Context, metadata utils.RequestMetadata) sdkworkflow.Context {
	return sdkworkflow.WithValue(ctx, requestMetadataWorkflowKey, normalizeTemporalMetadata(metadata))
}

func (p *RequestMetadataPropagator) Inject(ctx context.Context, writer sdkworkflow.HeaderWriter) error {
	return p.injectMetadata(utils.RequestMetadataFromContext(ctx), writer)
}

func (p *RequestMetadataPropagator) Extract(ctx context.Context, reader sdkworkflow.HeaderReader) (context.Context, error) {
	metadata, err := p.extractMetadata(reader)
	if err != nil {
		return ctx, err
	}

	return applyContextMetadata(ctx, metadata), nil
}

func (p *RequestMetadataPropagator) InjectFromWorkflow(ctx sdkworkflow.Context, writer sdkworkflow.HeaderWriter) error {
	return p.injectMetadata(RequestMetadataFromWorkflowContext(ctx), writer)
}

func (p *RequestMetadataPropagator) ExtractToWorkflow(ctx sdkworkflow.Context, reader sdkworkflow.HeaderReader) (sdkworkflow.Context, error) {
	metadata, err := p.extractMetadata(reader)
	if err != nil {
		return ctx, err
	}

	return WithRequestMetadata(ctx, metadata), nil
}

func (p *RequestMetadataPropagator) injectMetadata(metadata utils.RequestMetadata, writer sdkworkflow.HeaderWriter) error {
	metadata = normalizeTemporalMetadata(metadata)

	if metadata.CorrelationID != "" {
		payload, err := p.dataConverter.ToPayload(metadata.CorrelationID)
		if err != nil {
			return err
		}
		writer.Set(utils.HeaderCorrelationID, payload)
	}

	if metadata.OriginService != "" {
		payload, err := p.dataConverter.ToPayload(metadata.OriginService)
		if err != nil {
			return err
		}
		writer.Set(utils.HeaderOriginService, payload)
	}

	return nil
}

func (p *RequestMetadataPropagator) extractMetadata(reader sdkworkflow.HeaderReader) (utils.RequestMetadata, error) {
	metadata := utils.RequestMetadata{}

	if payload, ok := reader.Get(utils.HeaderCorrelationID); ok {
		if err := p.dataConverter.FromPayload(payload, &metadata.CorrelationID); err != nil {
			return utils.RequestMetadata{}, err
		}
	}

	if payload, ok := reader.Get(utils.HeaderOriginService); ok {
		if err := p.dataConverter.FromPayload(payload, &metadata.OriginService); err != nil {
			return utils.RequestMetadata{}, err
		}
	}

	return normalizeTemporalMetadata(metadata), nil
}

func normalizeTemporalMetadata(metadata utils.RequestMetadata) utils.RequestMetadata {
	result := utils.RequestMetadata{CorrelationID: metadata.CorrelationID, OriginService: metadata.OriginService}

	if result.OriginService == "" {
		switch {
		case metadata.LocalService != "":
			result.OriginService = metadata.LocalService
		case metadata.OriginService != "":
			result.OriginService = metadata.OriginService
		case metadata.CallerService != "":
			result.OriginService = metadata.CallerService
		}
	}

	return result
}

func applyContextMetadata(ctx context.Context, metadata utils.RequestMetadata) context.Context {
	metadata = normalizeTemporalMetadata(metadata)

	if metadata.CorrelationID != "" {
		ctx = utils.WithCorrelationID(ctx, metadata.CorrelationID)
	}
	if metadata.OriginService != "" {
		ctx = utils.WithOriginService(ctx, metadata.OriginService)
	}

	return ctx
}