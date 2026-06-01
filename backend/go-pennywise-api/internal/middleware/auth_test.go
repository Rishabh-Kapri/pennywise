package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	sharedMiddleware "github.com/Rishabh-Kapri/pennywise/backend/shared/middleware"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mocks
// ─────────────────────────────────────────────────────────────────────────────

type mockMwAuthService struct{ mock.Mock }

func (m *mockMwAuthService) LoginWithGoogle(ctx context.Context, req model.GoogleLoginRequest) (*model.AuthUserResponse, string, string, error) {
	panic("not used")
}
func (m *mockMwAuthService) GenerateAccessToken(ctx context.Context, userID uuid.UUID, version int) (string, error) {
	panic("not used")
}
func (m *mockMwAuthService) GenerateRefreshToken(ctx context.Context, userID uuid.UUID) (string, error) {
	panic("not used")
}
func (m *mockMwAuthService) ValidateToken(ctx context.Context, tokenString string) (*jwt.Token, error) {
	args := m.Called(ctx, tokenString)
	if v := args.Get(0); v != nil {
		return v.(*jwt.Token), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockMwAuthService) GetUserById(ctx context.Context, userID uuid.UUID) (*model.AuthUser, error) {
	args := m.Called(ctx, userID)
	if v := args.Get(0); v != nil {
		return v.(*model.AuthUser), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockMwAuthService) GetAllGoogleUsers(ctx context.Context) ([]model.GoogleProviderUser, error) {
	panic("not used")
}
func (m *mockMwAuthService) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*model.CurrentAuthUserResponse, error) {
	panic("not used")
}
func (m *mockMwAuthService) GetGoogleUserByEmail(ctx context.Context, email string) (*model.GoogleUserInfo, error) {
	panic("not used")
}
func (m *mockMwAuthService) UpdateGmailHistoryID(ctx context.Context, email string, oauthClientType model.GoogleOAuthClientType, historyID uint64, expiryAt *int64) error {
	panic("not used")
}
func (m *mockMwAuthService) RefreshToken(ctx context.Context, refreshToken string) (*model.RefreshTokenResponse, error) {
	panic("not used")
}

type mockMwAPIKeyService struct{ mock.Mock }

func (m *mockMwAPIKeyService) GetByHash(ctx context.Context, hash string) (*model.APIKey, error) {
	args := m.Called(ctx, hash)
	if v := args.Get(0); v != nil {
		return v.(*model.APIKey), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockMwAPIKeyService) Create(ctx context.Context, key *model.APIKey) (string, error) {
	panic("unused")
}
func (m *mockMwAPIKeyService) GetByKeyID(ctx context.Context, keyID string) (*model.APIKey, error) {
	panic("unused")
}
func (m *mockMwAPIKeyService) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	m.Called(ctx, id)
	return nil
}
func (m *mockMwAPIKeyService) Generate() (string, string, error) { panic("unused") }
func (m *mockMwAPIKeyService) ValidateFormat(key string) bool    { return m.Called(key).Bool(0) }
func (m *mockMwAPIKeyService) ParseKey(key string) (string, string, string, error) {
	args := m.Called(key)
	return args.String(0), args.String(1), args.String(2), args.Error(3)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

const testJWTSecret = "middleware-test-secret"

func buildTestJWT(userID uuid.UUID, version int, expiry time.Duration) string {
	cfg := config.Config{JWTSecret: testJWTSecret}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":     userID.String(),
		"iss":     "pennywise",
		"exp":     time.Now().Add(expiry).Unix(),
		"iat":     time.Now().Unix(),
		"version": float64(version),
	})
	tok, _ := t.SignedString([]byte(cfg.JWTSecret))
	return tok
}

func setupAuthRouter(authSvc service.AuthService, apiKeySvc service.APIKeyService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sharedMiddleware.RequestMetadata("pennywise-api"))
	r.Use(AuthMiddleware(authSvc, apiKeySvc))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: internal request bypasses auth
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthMiddleware_InternalRequestBypasses(t *testing.T) {
	authSvc := &mockMwAuthService{}
	apiKeySvc := &mockMwAPIKeyService{}

	r := gin.New()
	gin.SetMode(gin.TestMode)
	r.Use(sharedMiddleware.RequestMetadata("pennywise-api"))
	r.Use(sharedMiddleware.InternalRequestAuth("secret"))
	r.Use(AuthMiddleware(authSvc, apiKeySvc))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(utils.HeaderInternalService, "true")
	req.Header.Set(utils.HeaderInternalToken, "secret")
	req.Header.Set(utils.HeaderCallerService, "cipher")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: no auth header returns 401
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthMiddleware_NoAuthHeader_Returns401(t *testing.T) {
	r := setupAuthRouter(&mockMwAuthService{}, &mockMwAPIKeyService{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: invalid Bearer format
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthMiddleware_InvalidBearerFormat_Returns401(t *testing.T) {
	r := setupAuthRouter(&mockMwAuthService{}, &mockMwAPIKeyService{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Token abc123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: invalid JWT returns 401
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthMiddleware_InvalidJWT_Returns401(t *testing.T) {
	authSvc := &mockMwAuthService{}
	authSvc.On("ValidateToken", mock.Anything, "bad.token").Return(nil, assert.AnError)

	r := setupAuthRouter(authSvc, &mockMwAPIKeyService{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer bad.token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	authSvc.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: valid JWT, valid user, matching token version → 200
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthMiddleware_ValidJWT_Returns200(t *testing.T) {
	userID := uuid.New()
	tokenStr := buildTestJWT(userID, 1, 15*time.Minute)

	authSvc := &mockMwAuthService{}
	// Return a real parsed token from the valid JWT
	parsedToken, _ := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return []byte(testJWTSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	authSvc.On("ValidateToken", mock.Anything, tokenStr).Return(parsedToken, nil)
	authSvc.On("GetUserById", mock.Anything, userID).Return(&model.AuthUser{ID: userID, TokenVersion: 1}, nil)

	r := setupAuthRouter(authSvc, &mockMwAPIKeyService{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	authSvc.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: valid JWT but token version mismatch → 401
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthMiddleware_TokenVersionMismatch_Returns401(t *testing.T) {
	userID := uuid.New()
	tokenStr := buildTestJWT(userID, 1, 15*time.Minute)

	authSvc := &mockMwAuthService{}
	parsedToken, _ := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return []byte(testJWTSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	authSvc.On("ValidateToken", mock.Anything, tokenStr).Return(parsedToken, nil)
	// User has token version 2 but JWT claims version 1
	authSvc.On("GetUserById", mock.Anything, userID).Return(&model.AuthUser{ID: userID, TokenVersion: 2}, nil)

	r := setupAuthRouter(authSvc, &mockMwAPIKeyService{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	authSvc.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: GetUserById fails → 401
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthMiddleware_UserNotFound_Returns401(t *testing.T) {
	userID := uuid.New()
	tokenStr := buildTestJWT(userID, 1, 15*time.Minute)

	authSvc := &mockMwAuthService{}
	parsedToken, _ := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return []byte(testJWTSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	authSvc.On("ValidateToken", mock.Anything, tokenStr).Return(parsedToken, nil)
	authSvc.On("GetUserById", mock.Anything, userID).Return(nil, assert.AnError)

	r := setupAuthRouter(authSvc, &mockMwAPIKeyService{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	authSvc.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: access_token cookie fallback
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthMiddleware_CookieFallback(t *testing.T) {
	userID := uuid.New()
	tokenStr := buildTestJWT(userID, 1, 15*time.Minute)

	authSvc := &mockMwAuthService{}
	parsedToken, _ := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return []byte(testJWTSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	authSvc.On("ValidateToken", mock.Anything, tokenStr).Return(parsedToken, nil)
	authSvc.On("GetUserById", mock.Anything, userID).Return(&model.AuthUser{ID: userID, TokenVersion: 1}, nil)

	r := setupAuthRouter(authSvc, &mockMwAPIKeyService{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: tokenStr})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	authSvc.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: invalid API key format → 401
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthMiddleware_InvalidAPIKeyFormat_Returns401(t *testing.T) {
	authSvc := &mockMwAuthService{}
	apiKeySvc := &mockMwAPIKeyService{}
	apiKeySvc.On("ValidateFormat", "bad-key").Return(false)

	r := setupAuthRouter(authSvc, apiKeySvc)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "bad-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	apiKeySvc.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: API key ParseKey error → 401
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthMiddleware_APIKey_ParseKeyError_Returns401(t *testing.T) {
	authSvc := &mockMwAuthService{}
	apiKeySvc := &mockMwAPIKeyService{}
	apiKeySvc.On("ValidateFormat", "good-format-key").Return(true)
	apiKeySvc.On("ParseKey", "good-format-key").Return("", "", "", assert.AnError)

	r := setupAuthRouter(authSvc, apiKeySvc)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "good-format-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	apiKeySvc.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: API key GetByHash error → 401
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthMiddleware_APIKey_GetByHashError_Returns401(t *testing.T) {
	authSvc := &mockMwAuthService{}
	apiKeySvc := &mockMwAPIKeyService{}
	apiKeySvc.On("ValidateFormat", "valid-fmt-key").Return(true)
	apiKeySvc.On("ParseKey", "valid-fmt-key").Return("kid", "secret", "hash", nil)
	apiKeySvc.On("GetByHash", mock.Anything, "valid-fmt-key").Return(nil, assert.AnError)

	r := setupAuthRouter(authSvc, apiKeySvc)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "valid-fmt-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	apiKeySvc.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: API key IsValid() false → 401
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthMiddleware_APIKey_IsValidFalse_Returns401(t *testing.T) {
	authSvc := &mockMwAuthService{}
	apiKeySvc := &mockMwAPIKeyService{}
	apiKeySvc.On("ValidateFormat", "valid-fmt-key2").Return(true)
	apiKeySvc.On("ParseKey", "valid-fmt-key2").Return("kid", "secret", "hash", nil)
	// Return an inactive key (IsValid returns false when IsActive=false)
	inactiveKey := &model.APIKey{ID: uuid.New(), IsActive: false}
	apiKeySvc.On("GetByHash", mock.Anything, "valid-fmt-key2").Return(inactiveKey, nil)

	r := setupAuthRouter(authSvc, apiKeySvc)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "valid-fmt-key2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	apiKeySvc.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: valid API key, valid user → 200
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthMiddleware_APIKey_ValidKey_Returns200(t *testing.T) {
	userID := uuid.New()
	authSvc := &mockMwAuthService{}
	apiKeySvc := &mockMwAPIKeyService{}

	validKey := &model.APIKey{ID: uuid.New(), UserID: userID, IsActive: true}
	apiKeySvc.On("ValidateFormat", "valid-api-key").Return(true)
	apiKeySvc.On("ParseKey", "valid-api-key").Return("kid", "secret", "hash", nil)
	apiKeySvc.On("GetByHash", mock.Anything, "valid-api-key").Return(validKey, nil)
	apiKeySvc.On("UpdateLastUsed", mock.Anything, validKey.ID).Return(nil)
	authSvc.On("GetUserById", mock.Anything, userID).Return(&model.AuthUser{ID: userID, TokenVersion: 1}, nil)

	r := setupAuthRouter(authSvc, apiKeySvc)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "valid-api-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	authSvc.AssertExpectations(t)
	// Note: UpdateLastUsed is called in a goroutine, so we don't assert it here
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: Bearer token via query param ?token=
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthMiddleware_QueryParamToken(t *testing.T) {
	userID := uuid.New()
	tokenStr := buildTestJWT(userID, 1, 15*time.Minute)

	authSvc := &mockMwAuthService{}
	parsedToken, _ := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return []byte(testJWTSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	authSvc.On("ValidateToken", mock.Anything, tokenStr).Return(parsedToken, nil)
	authSvc.On("GetUserById", mock.Anything, userID).Return(&model.AuthUser{ID: userID, TokenVersion: 1}, nil)

	r := setupAuthRouter(authSvc, &mockMwAPIKeyService{})
	req := httptest.NewRequest(http.MethodGet, "/?token="+tokenStr, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	authSvc.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: JWT claims version not float64 → 401
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthMiddleware_JWTVersionNotFloat64_Returns401(t *testing.T) {
	userID := uuid.New()

	// Build a JWT with version as string instead of float64
	cfg := config.Config{JWTSecret: testJWTSecret}
	rawToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":     userID.String(),
		"iss":     "pennywise",
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
		"iat":     time.Now().Unix(),
		"version": "not-a-number", // string instead of float64
	})
	tokenStr, _ := rawToken.SignedString([]byte(cfg.JWTSecret))

	authSvc := &mockMwAuthService{}
	parsedToken, _ := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return []byte(testJWTSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	authSvc.On("ValidateToken", mock.Anything, tokenStr).Return(parsedToken, nil)

	r := setupAuthRouter(authSvc, &mockMwAPIKeyService{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	authSvc.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: handleUserId: GetUserById returns nil user → 401
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthMiddleware_NilUser_Returns401(t *testing.T) {
	userID := uuid.New()
	tokenStr := buildTestJWT(userID, 1, 15*time.Minute)

	authSvc := &mockMwAuthService{}
	parsedToken, _ := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return []byte(testJWTSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	authSvc.On("ValidateToken", mock.Anything, tokenStr).Return(parsedToken, nil)
	// Return nil user with nil error — hits the "user == nil" branch
	authSvc.On("GetUserById", mock.Anything, userID).Return((*model.AuthUser)(nil), nil)

	r := setupAuthRouter(authSvc, &mockMwAPIKeyService{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	authSvc.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests: handleUserId: invalid UUID in userID → 401
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthMiddleware_InvalidUserIDUUID_Returns401(t *testing.T) {
	// Build a JWT where sub is not a valid UUID
	cfg := config.Config{JWTSecret: testJWTSecret}
	rawToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":     "not-a-uuid",
		"iss":     "pennywise",
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
		"iat":     time.Now().Unix(),
		"version": float64(1),
	})
	tokenStr, _ := rawToken.SignedString([]byte(cfg.JWTSecret))

	authSvc := &mockMwAuthService{}
	parsedToken, _ := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return []byte(testJWTSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	authSvc.On("ValidateToken", mock.Anything, tokenStr).Return(parsedToken, nil)

	r := setupAuthRouter(authSvc, &mockMwAPIKeyService{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	authSvc.AssertExpectations(t)
}
