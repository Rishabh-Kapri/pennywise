package utils

import (
	"context"
	"errors"
	"net/http"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

type contextKey string

const (
	budgetIDKey      contextKey = "budgetId"
	userIDKey        contextKey = "userId"
	userKey          contextKey = "user"
	correlationIDKey contextKey = "correlationId"
	serviceNameKey   contextKey = "serviceName"
	requestMetaKey   contextKey = "requestMetadata"
	internalTokenKey contextKey = "internalAuthToken"
	apiKeyKey        contextKey = "apiKey"

	HeaderCorrelationID   = "X-Correlation-ID"
	HeaderCallerService   = "X-Caller-Service"
	HeaderOriginService   = "X-Origin-Service"
	HeaderInternalToken   = "X-Internal-Token"
	HeaderInternalService = "X-Internal-Service"
	HeaderServiceName     = "X-Service-Name"
	HeaderBudgetID        = "X-Budget-ID"
	HeaderUserID          = "X-User-ID"
	HeaderAPIKey          = "X-API-Key"
)

type RequestMetadata struct {
	CorrelationID    string
	LocalService     string
	CallerService    string
	OriginService    string
	InternalRequest  bool
	VerifiedInternal bool
	BudgetID         *uuid.UUID
	UserID           *uuid.UUID
}

func (metadata RequestMetadata) normalized() RequestMetadata {
	metadata.BudgetID = cloneUUID(metadata.BudgetID)
	metadata.UserID = cloneUUID(metadata.UserID)

	if metadata.OriginService == "" {
		switch {
		case metadata.CallerService != "":
			metadata.OriginService = metadata.CallerService
		case metadata.InternalRequest && metadata.LocalService != "":
			metadata.OriginService = metadata.LocalService
		}
	}

	return metadata
}

func cloneUUID(id *uuid.UUID) *uuid.UUID {
	if id == nil {
		return nil
	}

	value := *id
	return &value
}

func parseHeaderUUID(value string) *uuid.UUID {
	if value == "" {
		return nil
	}

	parsed, err := uuid.Parse(value)
	if err != nil {
		return nil
	}

	return &parsed
}

func updateRequestMetadata(ctx context.Context, update func(*RequestMetadata)) context.Context {
	metadata := RequestMetadataFromContext(ctx)
	update(&metadata)
	return WithRequestMetadata(ctx, metadata)
}

func WithRequestMetadata(ctx context.Context, metadata RequestMetadata) context.Context {
	metadata = metadata.normalized()
	ctx = context.WithValue(ctx, requestMetaKey, metadata)

	if metadata.CorrelationID != "" {
		ctx = context.WithValue(ctx, correlationIDKey, metadata.CorrelationID)
	}
	if metadata.LocalService != "" {
		ctx = context.WithValue(ctx, serviceNameKey, metadata.LocalService)
	}
	if metadata.BudgetID != nil {
		ctx = context.WithValue(ctx, budgetIDKey, *metadata.BudgetID)
	}
	if metadata.UserID != nil {
		ctx = context.WithValue(ctx, userIDKey, *metadata.UserID)
	}

	return ctx
}

func RequestMetadataFromContext(ctx context.Context) RequestMetadata {
	if metadata, ok := ctx.Value(requestMetaKey).(RequestMetadata); ok {
		return metadata.normalized()
	}

	metadata := RequestMetadata{}

	if correlationID, ok := ctx.Value(correlationIDKey).(string); ok {
		metadata.CorrelationID = correlationID
	}
	if serviceName, ok := ctx.Value(serviceNameKey).(string); ok {
		metadata.LocalService = serviceName
	}
	if budgetID, ok := ctx.Value(budgetIDKey).(uuid.UUID); ok {
		metadata.BudgetID = cloneUUID(&budgetID)
	}
	if userID, ok := ctx.Value(userIDKey).(uuid.UUID); ok {
		metadata.UserID = cloneUUID(&userID)
	}

	return metadata.normalized()
}

func RequestMetadataFromHeaders(headers http.Header) RequestMetadata {
	metadata := RequestMetadata{
		CorrelationID: headers.Get(HeaderCorrelationID),
		CallerService: headers.Get(HeaderCallerService),
		OriginService: headers.Get(HeaderOriginService),
		BudgetID:      parseHeaderUUID(headers.Get(HeaderBudgetID)),
		UserID:        parseHeaderUUID(headers.Get(HeaderUserID)),
	}

	if metadata.CallerService == "" {
		metadata.CallerService = headers.Get(HeaderServiceName)
	}
	if headers.Get(HeaderInternalService) == "true" || headers.Get(HeaderInternalToken) != "" {
		metadata.InternalRequest = true
	}

	return metadata.normalized()
}

func RequestMetadataFromHeaderMap(headers map[string][]string) RequestMetadata {
	return RequestMetadataFromHeaders(http.Header(headers))
}

func WithLocalService(ctx context.Context, name string) context.Context {
	return updateRequestMetadata(ctx, func(metadata *RequestMetadata) {
		metadata.LocalService = name
	})
}

func LocalServiceFromContext(ctx context.Context) string {
	return RequestMetadataFromContext(ctx).LocalService
}

func WithCallerService(ctx context.Context, name string) context.Context {
	return updateRequestMetadata(ctx, func(metadata *RequestMetadata) {
		metadata.CallerService = name
	})
}

func CallerServiceFromContext(ctx context.Context) string {
	return RequestMetadataFromContext(ctx).CallerService
}

func WithOriginService(ctx context.Context, name string) context.Context {
	return updateRequestMetadata(ctx, func(metadata *RequestMetadata) {
		metadata.OriginService = name
	})
}

func OriginServiceFromContext(ctx context.Context) string {
	return RequestMetadataFromContext(ctx).OriginService
}

func WithInternalRequest(ctx context.Context, internal bool) context.Context {
	return updateRequestMetadata(ctx, func(metadata *RequestMetadata) {
		metadata.InternalRequest = internal
	})
}

func InternalRequestFromContext(ctx context.Context) bool {
	return RequestMetadataFromContext(ctx).InternalRequest
}

func WithVerifiedInternal(ctx context.Context, verified bool) context.Context {
	return updateRequestMetadata(ctx, func(metadata *RequestMetadata) {
		metadata.VerifiedInternal = verified
	})
}

func VerifiedInternalFromContext(ctx context.Context) bool {
	return RequestMetadataFromContext(ctx).VerifiedInternal
}

func WithInternalAuthToken(ctx context.Context, token string) context.Context {
	if token == "" {
		return ctx
	}

	return context.WithValue(ctx, internalTokenKey, token)
}

func InternalAuthTokenFromContext(ctx context.Context) string {
	if token, ok := ctx.Value(internalTokenKey).(string); ok {
		return token
	}

	return ""
}

func WithAPIKey(ctx context.Context, apiKey *model.APIKey) context.Context {
	if apiKey == nil {
		return ctx
	}
	return context.WithValue(ctx, apiKeyKey, apiKey)
}

func APIKeyFromContext(ctx context.Context) *model.APIKey {
	if apiKey, ok := ctx.Value(apiKeyKey).(*model.APIKey); ok {
		return apiKey
	}

	return nil
}

// WithServiceName returns a new context with the service name set.
func WithServiceName(ctx context.Context, name string) context.Context {
	return WithLocalService(ctx, name)
}

func ServiceNameFromContext(ctx context.Context) string {
	return LocalServiceFromContext(ctx)
}

// WithBudgetID returns a new context with the budget ID set.
func WithBudgetID(ctx context.Context, id uuid.UUID) context.Context {
	return updateRequestMetadata(ctx, func(metadata *RequestMetadata) {
		metadata.BudgetID = cloneUUID(&id)
	})
}

// BudgetIDFromContext extracts the budget ID from the context.
// Returns an error if the budget ID is missing.
func BudgetIDFromContext(ctx context.Context) (uuid.UUID, error) {
	metadata := RequestMetadataFromContext(ctx)
	if metadata.BudgetID == nil {
		return uuid.Nil, errors.New("budget ID not found in context")
	}

	return *metadata.BudgetID, nil
}

// MustBudgetID extracts the budget ID or panics.
func MustBudgetID(ctx context.Context) uuid.UUID {
	id, err := BudgetIDFromContext(ctx)
	if err != nil {
		panic("BudgetIdMiddleware not configured: " + err.Error())
	}
	return id
}

// WithUserID returns a new context with the authenticated user's ID set.
func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return updateRequestMetadata(ctx, func(metadata *RequestMetadata) {
		metadata.UserID = cloneUUID(&id)
	})
}

// UserIDFromContext extracts the authenticated user's ID from the context.
func UserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	metadata := RequestMetadataFromContext(ctx)
	if metadata.UserID == nil {
		return uuid.Nil, errors.New("user ID not found in context")
	}

	return *metadata.UserID, nil
}

func WithUser(ctx context.Context, user *model.AuthUser) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func MustUserID(ctx context.Context) uuid.UUID {
	id, err := UserIDFromContext(ctx)
	if err != nil {
		panic("UserIdMiddleware not configured: " + err.Error())
	}
	return id
}

// WithCorrelationID returns a new context with the correlation ID set.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return updateRequestMetadata(ctx, func(metadata *RequestMetadata) {
		metadata.CorrelationID = id
	})
}

// NewCorrelationID generates a new correlation ID.
func NewCorrelationID() string {
	return uuid.New().String()
}

// CorrelationIDFromContext extracts the correlation ID from the context.
func CorrelationIDFromContext(ctx context.Context) string {
	return RequestMetadataFromContext(ctx).CorrelationID
}

// BuildInternalHeaders returns canonical internal headers plus legacy compatibility headers.
func BuildInternalHeaders(ctx context.Context) map[string][]string {
	metadata := RequestMetadataFromContext(ctx)
	headers := make(map[string][]string)
	callerService := metadata.LocalService
	if callerService == "" {
		callerService = metadata.CallerService
	}
	originService := metadata.OriginService
	if originService == "" && callerService != "" {
		originService = callerService
	}

	// Keep the legacy internal marker until all services adopt the canonical metadata.
	headers[HeaderInternalService] = []string{"true"}

	if metadata.CorrelationID != "" {
		headers[HeaderCorrelationID] = []string{metadata.CorrelationID}
	}
	if callerService != "" {
		headers[HeaderCallerService] = []string{callerService}
		headers[HeaderServiceName] = []string{callerService}
	}
	if originService != "" {
		headers[HeaderOriginService] = []string{originService}
	}

	if metadata.BudgetID != nil {
		headers[HeaderBudgetID] = []string{metadata.BudgetID.String()}
	}
	if metadata.UserID != nil {
		headers[HeaderUserID] = []string{metadata.UserID.String()}
	}
	if token := InternalAuthTokenFromContext(ctx); token != "" {
		headers[HeaderInternalToken] = []string{token}
	}

	return headers
}

// GetHeaders returns a map of headers to be used in the request for internal services.
// Deprecated: prefer BuildInternalHeaders for new call sites.
func GetHeaders(ctx context.Context) map[string][]string {
	return BuildInternalHeaders(ctx)
}

func SanitizeHeadersForLogging(headers map[string][]string) map[string][]string {
	if headers == nil {
		return nil
	}

	sanitized := make(map[string][]string, len(headers))
	for key, values := range headers {
		copied := append([]string(nil), values...)
		if key == HeaderInternalToken && len(copied) > 0 {
			copied[0] = "[REDACTED]"
		}
		sanitized[key] = copied
	}

	return sanitized
}
