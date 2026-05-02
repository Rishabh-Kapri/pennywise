package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	sharedMiddleware "github.com/Rishabh-Kapri/pennywise/backend/shared/middleware"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type noopAuthService struct {
	t *testing.T
}

func (s noopAuthService) LoginWithGoogle(context.Context, string) (*model.AuthUserResponse, string, string, error) {
	s.t.Fatalf("LoginWithGoogle should not be called for verified internal requests")
	return nil, "", "", nil
}

func (s noopAuthService) GenerateAccessToken(context.Context, uuid.UUID, int) (string, error) {
	s.t.Fatalf("GenerateAccessToken should not be called for verified internal requests")
	return "", nil
}

func (s noopAuthService) GenerateRefreshToken(context.Context, uuid.UUID) (string, error) {
	s.t.Fatalf("GenerateRefreshToken should not be called for verified internal requests")
	return "", nil
}

func (s noopAuthService) ValidateToken(context.Context, string) (*jwt.Token, error) {
	s.t.Fatalf("ValidateToken should not be called for verified internal requests")
	return nil, nil
}

func (s noopAuthService) GetUserById(context.Context, uuid.UUID) (*model.AuthUser, error) {
	s.t.Fatalf("GetUserById should not be called for verified internal requests")
	return nil, nil
}

func (s noopAuthService) GetAllGoogleUsers(context.Context) ([]model.GoogleProviderUser, error) {
	s.t.Fatalf("GetAllGoogleUsers should not be called for verified internal requests")
	return nil, nil
}

func (s noopAuthService) GetCurrentUser(context.Context, uuid.UUID) (*model.CurrentAuthUserResponse, error) {
	s.t.Fatalf("GetCurrentUser should not be called for verified internal requests")
	return nil, nil
}

func (s noopAuthService) GetGoogleUserByEmail(context.Context, string) (*model.GoogleUserInfo, error) {
	s.t.Fatalf("GetGoogleUserByEmail should not be called for verified internal requests")
	return nil, nil
}

func (s noopAuthService) UpdateGmailHistoryID(context.Context, string, uint64, *int64) error {
	s.t.Fatalf("UpdateGmailHistoryID should not be called for verified internal requests")
	return nil
}

func (s noopAuthService) RefreshToken(context.Context, string) (*model.RefreshTokenResponse, error) {
	s.t.Fatalf("RefreshToken should not be called for verified internal requests")
	return nil, nil
}

type noopAPIKeyService struct {
	t *testing.T
}

func (s noopAPIKeyService) Generate() (string, string, error) {
	s.t.Fatalf("Generate should not be called for verified internal requests")
	return "", "", nil
}

func (s noopAPIKeyService) ParseKey(string) (string, string, string, error) {
	s.t.Fatalf("ParseKey should not be called for verified internal requests")
	return "", "", "", nil
}

func (s noopAPIKeyService) ValidateFormat(string) bool {
	s.t.Fatalf("ValidateFormat should not be called for verified internal requests")
	return false
}

func (s noopAPIKeyService) Create(context.Context, *model.APIKey) (string, error) {
	s.t.Fatalf("Create should not be called for verified internal requests")
	return "", nil
}

func (s noopAPIKeyService) GetByKeyID(context.Context, string) (*model.APIKey, error) {
	s.t.Fatalf("GetByKeyID should not be called for verified internal requests")
	return nil, nil
}

func (s noopAPIKeyService) GetByHash(context.Context, string) (*model.APIKey, error) {
	s.t.Fatalf("GetByHash should not be called for verified internal requests")
	return nil, nil
}

func (s noopAPIKeyService) UpdateLastUsed(context.Context, uuid.UUID) error {
	s.t.Fatalf("UpdateLastUsed should not be called for verified internal requests")
	return nil
}

var _ service.AuthService = noopAuthService{}
var _ service.APIKeyService = noopAPIKeyService{}

func TestAuthMiddlewareAllowsVerifiedInternalRequests(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sharedMiddleware.RequestMetadata("pennywise-api"))
	router.Use(sharedMiddleware.InternalRequestAuth("shared-secret"))
	router.Use(AuthMiddleware(noopAuthService{t: t}, noopAPIKeyService{t: t}))

	router.GET("/", func(c *gin.Context) {
		ctx := c.Request.Context()
		require.True(t, utils.VerifiedInternalFromContext(ctx))
		require.Equal(t, "cipher", utils.CallerServiceFromContext(ctx))
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(utils.HeaderInternalService, "true")
	req.Header.Set(utils.HeaderInternalToken, "shared-secret")
	req.Header.Set(utils.HeaderCallerService, "cipher")

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusNoContent, res.Code)
}
