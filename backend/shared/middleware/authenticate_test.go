package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/factory"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/middleware"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/auth"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

const testSecret = "middleware-test-secret"

// ---- fakes: the adapters only need repository ports, so no database ----

type fakeUsers struct {
	repository.AuthRepository // unused methods panic
	versions                  map[uuid.UUID]int
	err                       error
}

func (f *fakeUsers) GetTokenVersion(_ context.Context, id uuid.UUID) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	v, ok := f.versions[id]
	if !ok {
		return 0, repository.ErrUserNotFound
	}
	return v, nil
}

type fakeKeys struct {
	repository.APIKeyRepository
	byHash map[string]*model.APIKey
}

func (f *fakeKeys) GetByHash(_ context.Context, hash string) (*model.APIKey, error) {
	if k, ok := f.byHash[hash]; ok {
		return k, nil
	}
	return nil, repository.ErrNotFound
}

func (f *fakeKeys) UpdateLastUsed(context.Context, uuid.UUID) error { return nil }

// ---- helpers ----

func signToken(t *testing.T, claims jwt.MapClaims, secret string) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func accessToken(t *testing.T, userID uuid.UUID, version int) string {
	return signToken(t, jwt.MapClaims{
		"sub": userID.String(), "iss": "pennywise", "version": version,
		"iat": time.Now().Unix(), "exp": time.Now().Add(15 * time.Minute).Unix(),
	}, testSecret)
}

type setup struct {
	users *fakeUsers
	keys  *fakeKeys
	// what the handler saw
	principal *auth.Principal
	userID    uuid.UUID
	apiKey    *model.APIKey
}

func newRouter(s *setup, extra ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequestMetadata("pennywise-api"))
	for _, h := range extra {
		r.Use(h)
	}
	authenticators := factory.NewAuthenticators(context.Background(),
		factory.AuthConfig{JWTSecret: testSecret}, s.keys, s.users)
	r.Use(middleware.Authenticate(authenticators...))
	r.GET("/", func(c *gin.Context) {
		ctx := c.Request.Context()
		s.principal = auth.PrincipalFromContext(ctx)
		s.userID, _ = utils.UserIDFromContext(ctx)
		s.apiKey = utils.APIKeyFromContext(ctx)
		c.Status(http.StatusOK)
	})
	return r
}

func newSetup() *setup {
	return &setup{
		users: &fakeUsers{versions: map[uuid.UUID]int{}},
		keys:  &fakeKeys{byHash: map[string]*model.APIKey{}},
	}
}

func do(r *gin.Engine, mutate func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if mutate != nil {
		mutate(req)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ---- tests ----

func TestAuthenticateJWT(t *testing.T) {
	userID := uuid.New()

	cases := []struct {
		name     string
		version  int // user's current token_version
		known    bool
		usersErr error
		req      func(*http.Request)
		want     int
	}{
		{name: "valid header", version: 1, known: true, want: http.StatusOK,
			req: func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+accessToken(t, userID, 1)) }},
		{name: "cookie fallback", version: 1, known: true, want: http.StatusOK,
			req: func(r *http.Request) {
				r.AddCookie(&http.Cookie{Name: "access_token", Value: accessToken(t, userID, 1)})
			}},
		{name: "query param fallback", version: 1, known: true, want: http.StatusOK,
			req: func(r *http.Request) { r.URL.RawQuery = "token=" + accessToken(t, userID, 1) }},
		{name: "no credentials", want: http.StatusUnauthorized},
		{name: "not a bearer header", want: http.StatusUnauthorized,
			req: func(r *http.Request) { r.Header.Set("Authorization", "Token abc123") }},
		{name: "garbage token", want: http.StatusUnauthorized,
			req: func(r *http.Request) { r.Header.Set("Authorization", "Bearer bad.token") }},
		{name: "wrong signing secret", version: 1, known: true, want: http.StatusUnauthorized,
			req: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+signToken(t, jwt.MapClaims{
					"sub": userID.String(), "version": 1, "exp": time.Now().Add(time.Minute).Unix(),
				}, "other-secret"))
			}},
		{name: "expired", version: 1, known: true, want: http.StatusUnauthorized,
			req: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+signToken(t, jwt.MapClaims{
					"sub": userID.String(), "version": 1, "exp": time.Now().Add(-time.Minute).Unix(),
				}, testSecret))
			}},
		{name: "revoked (version bumped)", version: 2, known: true, want: http.StatusUnauthorized,
			req: func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+accessToken(t, userID, 1)) }},
		{name: "user not found", want: http.StatusUnauthorized,
			req: func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+accessToken(t, userID, 1)) }},
		{name: "refresh token has no version", version: 1, known: true, want: http.StatusUnauthorized,
			req: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+signToken(t, jwt.MapClaims{
					"sub": userID.String(), "exp": time.Now().Add(time.Hour).Unix(),
				}, testSecret))
			}},
		{name: "version not a number", version: 1, known: true, want: http.StatusUnauthorized,
			req: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+signToken(t, jwt.MapClaims{
					"sub": userID.String(), "version": "1", "exp": time.Now().Add(time.Minute).Unix(),
				}, testSecret))
			}},
		{name: "subject not a uuid", want: http.StatusUnauthorized,
			req: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer "+signToken(t, jwt.MapClaims{
					"sub": "not-a-uuid", "version": 1, "exp": time.Now().Add(time.Minute).Unix(),
				}, testSecret))
			}},
		{name: "database down is not a logout", version: 1, known: true, usersErr: errors.New("db down"),
			want: http.StatusServiceUnavailable,
			req:  func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+accessToken(t, userID, 1)) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newSetup()
			if tc.known {
				s.users.versions[userID] = tc.version
			}
			s.users.err = tc.usersErr
			w := do(newRouter(s), tc.req)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d (body %s)", w.Code, tc.want, w.Body.String())
			}
			if tc.want != http.StatusOK {
				return
			}
			if s.principal == nil || s.principal.Kind != auth.UserKind || s.userID != userID {
				t.Fatalf("principal = %+v, ctx user = %v", s.principal, s.userID)
			}
		})
	}
}

func TestAuthenticateAPIKey(t *testing.T) {
	userID := uuid.New()
	goodKey := "pwk_v1_" + strings.Repeat("A", 43)
	key := &model.APIKey{ID: uuid.New(), KeyID: "k_1", UserID: userID, IsActive: true,
		Scopes: []model.Scope{model.ScopeRead}, RateLimit: 60}

	t.Run("valid key sets principal, user and key in ctx", func(t *testing.T) {
		s := newSetup()
		s.keys.byHash[utils.Hash(goodKey)] = key
		w := do(newRouter(s), func(r *http.Request) { r.Header.Set(utils.HeaderAPIKey, goodKey) })
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		if s.principal.Kind != auth.APIKeyKind || s.userID != userID || s.apiKey == nil || s.apiKey.ID != key.ID {
			t.Fatalf("principal = %+v, user = %v, key = %v", s.principal, s.userID, s.apiKey)
		}
	})

	t.Run("bad key is 401 even with a valid bearer token", func(t *testing.T) {
		s := newSetup()
		s.users.versions[userID] = 1
		w := do(newRouter(s), func(r *http.Request) {
			r.Header.Set(utils.HeaderAPIKey, "pwk_v1_"+strings.Repeat("Z", 43))
			r.Header.Set("Authorization", "Bearer "+accessToken(t, userID, 1))
		})
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	})

	t.Run("malformed key", func(t *testing.T) {
		s := newSetup()
		w := do(newRouter(s), func(r *http.Request) { r.Header.Set(utils.HeaderAPIKey, "nope") })
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	})
}

func TestAuthenticateInternalRequestPassesThrough(t *testing.T) {
	s := newSetup()
	services, err := factory.NewServiceAuthenticator(context.Background(),
		factory.ServiceAuthConfig{Tokens: "cipher:secret"})
	if err != nil {
		t.Fatal(err)
	}
	r := newRouter(s, middleware.InternalRequestAuth("own", services))
	w := do(r, func(req *http.Request) {
		req.Header.Set(utils.HeaderInternalService, "true")
		req.Header.Set(utils.HeaderInternalToken, "secret")
		req.Header.Set(utils.HeaderCallerService, "cipher")
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if s.principal == nil || s.principal.Kind != auth.ServiceKind || s.principal.UserID != nil {
		t.Fatalf("want a service principal with no user, got %+v", s.principal)
	}
}

func TestEmptyJWTSecretRejectsBearerTokens(t *testing.T) {
	s := newSetup()
	userID := uuid.New()
	s.users.versions[userID] = 1
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Authenticate(factory.NewAuthenticators(context.Background(),
		factory.AuthConfig{}, s.keys, s.users)...))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	// A token signed with the empty key must not get in.
	tok := signToken(t, jwt.MapClaims{"sub": userID.String(), "version": 1,
		"exp": time.Now().Add(time.Minute).Unix()}, "")
	w := do(r, func(req *http.Request) { req.Header.Set("Authorization", "Bearer "+tok) })
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}
