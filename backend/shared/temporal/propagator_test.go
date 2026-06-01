package temporal

import (
	"context"
	"testing"

	commonpb "go.temporal.io/api/common/v1"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/stretchr/testify/require"
)

type testHeaders map[string]*commonpb.Payload

func (h testHeaders) Set(key string, value *commonpb.Payload) {
	h[key] = value
}

func (h testHeaders) Get(key string) (*commonpb.Payload, bool) {
	value, ok := h[key]
	return value, ok
}

func (h testHeaders) ForEachKey(handler func(string, *commonpb.Payload) error) error {
	for key, value := range h {
		if err := handler(key, value); err != nil {
			return err
		}
	}

	return nil
}

func TestRequestMetadataPropagatorRoundTripContext(t *testing.T) {
	t.Parallel()

	propagator := NewRequestMetadataPropagator()
	headers := testHeaders{}

	ctx := context.Background()
	ctx = utils.WithCorrelationID(ctx, "corr-123")
	ctx = utils.WithOriginService(ctx, "gmail-pubsub")
	ctx = utils.WithCallerService(ctx, "cipher")

	require.NoError(t, propagator.Inject(ctx, headers))

	extractedCtx, err := propagator.Extract(context.Background(), headers)
	require.NoError(t, err)

	metadata := utils.RequestMetadataFromContext(extractedCtx)
	require.Equal(t, "corr-123", metadata.CorrelationID)
	require.Equal(t, "gmail-pubsub", metadata.OriginService)
	require.Empty(t, metadata.CallerService)
	require.False(t, metadata.InternalRequest)
	_, hasBudget := headers[utils.HeaderBudgetID]
	require.False(t, hasBudget)
}

func TestRequestMetadataPropagatorFallsBackToLocalServiceForOrigin(t *testing.T) {
	t.Parallel()

	propagator := NewRequestMetadataPropagator()
	headers := testHeaders{}

	ctx := utils.WithServiceName(context.Background(), "pennywise-api")
	ctx = utils.WithCorrelationID(ctx, "corr-456")

	require.NoError(t, propagator.Inject(ctx, headers))

	extractedCtx, err := propagator.Extract(context.Background(), headers)
	require.NoError(t, err)

	metadata := utils.RequestMetadataFromContext(extractedCtx)
	require.Equal(t, "corr-456", metadata.CorrelationID)
	require.Equal(t, "pennywise-api", metadata.OriginService)
}