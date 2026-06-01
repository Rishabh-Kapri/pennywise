package utils

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestBuildInternalHeadersIncludesCanonicalAndLegacyFields(t *testing.T) {
	t.Parallel()

	budgetID := uuid.New()
	userID := uuid.New()

	ctx := context.Background()
	ctx = WithServiceName(ctx, "cipher")
	ctx = WithCorrelationID(ctx, "corr-123")
	ctx = WithBudgetID(ctx, budgetID)
	ctx = WithUserID(ctx, userID)
	ctx = WithOriginService(ctx, "gmail-watch")

	headers := BuildInternalHeaders(ctx)

	require.Equal(t, []string{"true"}, headers[HeaderInternalService])
	require.Equal(t, []string{"corr-123"}, headers[HeaderCorrelationID])
	require.Equal(t, []string{"cipher"}, headers[HeaderCallerService])
	require.Equal(t, []string{"gmail-watch"}, headers[HeaderOriginService])
	require.Equal(t, []string{"cipher"}, headers[HeaderServiceName])
	require.Equal(t, []string{budgetID.String()}, headers[HeaderBudgetID])
	require.Equal(t, []string{userID.String()}, headers[HeaderUserID])
}

func TestRequestMetadataFromHeadersParsesCanonicalAndLegacyFields(t *testing.T) {
	t.Parallel()

	budgetID := uuid.New()
	userID := uuid.New()

	headers := http.Header{}
	headers.Set(HeaderCorrelationID, "corr-456")
	headers.Set(HeaderServiceName, "legacy-caller")
	headers.Set(HeaderOriginService, "gmail-watch")
	headers.Set(HeaderInternalService, "true")
	headers.Set(HeaderBudgetID, budgetID.String())
	headers.Set(HeaderUserID, userID.String())

	metadata := RequestMetadataFromHeaders(headers)

	require.Equal(t, "corr-456", metadata.CorrelationID)
	require.Equal(t, "legacy-caller", metadata.CallerService)
	require.Equal(t, "gmail-watch", metadata.OriginService)
	require.True(t, metadata.InternalRequest)
	require.NotNil(t, metadata.BudgetID)
	require.NotNil(t, metadata.UserID)
	require.Equal(t, budgetID, *metadata.BudgetID)
	require.Equal(t, userID, *metadata.UserID)
}

func TestRequestMetadataSettersPreserveExistingFields(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ctx = WithServiceName(ctx, "go-pennywise-api")
	ctx = WithCorrelationID(ctx, "corr-789")
	ctx = WithCallerService(ctx, "cipher")
	ctx = WithOriginService(ctx, "gmail-watch")
	ctx = WithInternalRequest(ctx, true)
	ctx = WithVerifiedInternal(ctx, true)

	metadata := RequestMetadataFromContext(ctx)

	require.Equal(t, "go-pennywise-api", metadata.LocalService)
	require.Equal(t, "corr-789", metadata.CorrelationID)
	require.Equal(t, "cipher", metadata.CallerService)
	require.Equal(t, "gmail-watch", metadata.OriginService)
	require.True(t, metadata.InternalRequest)
	require.True(t, metadata.VerifiedInternal)
	require.Equal(t, "go-pennywise-api", ServiceNameFromContext(ctx))
	require.Equal(t, "cipher", CallerServiceFromContext(ctx))
	require.Equal(t, "gmail-watch", OriginServiceFromContext(ctx))
	require.True(t, InternalRequestFromContext(ctx))
	require.True(t, VerifiedInternalFromContext(ctx))
}

func TestSanitizeHeadersForLoggingRedactsInternalToken(t *testing.T) {
	t.Parallel()

	headers := map[string][]string{
		HeaderInternalToken: {"secret-token"},
		HeaderCorrelationID: {"corr-123"},
	}

	sanitized := SanitizeHeadersForLogging(headers)

	require.Equal(t, []string{"[REDACTED]"}, sanitized[HeaderInternalToken])
	require.Equal(t, []string{"corr-123"}, sanitized[HeaderCorrelationID])
	require.Equal(t, []string{"secret-token"}, headers[HeaderInternalToken])
}
