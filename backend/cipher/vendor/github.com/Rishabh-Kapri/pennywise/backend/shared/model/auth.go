package model

import (
	"time"

	"github.com/google/uuid"
)

type AuthProviderType string

const (
	GoogleAuthProviderType AuthProviderType = "google"
)

// AuthUser represents an authenticated internal user
// The user is linked to an auth provider (eg, Google, Email, etc)
type AuthUser struct {
	ID               uuid.UUID `json:"id"`
	TokenVersion     int       `json:"tokenVersion"`
	RefreshTokenHash *string   `json:"refreshTokenHash"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	Deleted          bool      `json:"deleted"`
}

type AuthProvider struct {
	ID           uuid.UUID        `json:"id"`
	AuthUserID   uuid.UUID        `json:"authUserId"` // AuthUser.ID
	ProviderType AuthProviderType `json:"providerType"`
	ProviderID   string           `json:"providerId"` // eg, google_id
	VerifiedAt   time.Time        `json:"verifiedAt"` // when the user verified the provider (eg, email verification)
	CreatedAt    time.Time        `json:"createdAt"`
	UpdatedAt    time.Time        `json:"updatedAt"`
}

type GoogleProviderUser struct {
	ID             string     `json:"id"` // AuthProvider.ProviderID
	Name           string     `json:"name"`
	Picture        string     `json:"picture"`
	Email          string     `json:"email"`
	GmailHistoryID *int       `json:"gmailHistoryId"`
	RefreshToken   string     `json:"refreshToken"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	LastGmailSync  *time.Time `json:"lastGmailSync"`
	Deleted        bool       `json:"deleted"`
}

type UserWithCredentials struct {
	AuthUser       *AuthUser           `json:"authUser"`
	GoogleProvider *GoogleProviderUser `json:"googleProvider"`
	// add more auth providers here as support is added
}

// AuthUserResponse is returned to frontend after login
type AuthUserResponse struct {
	ID      uuid.UUID `json:"id"`
	Email   string    `json:"email"`
	Name    string    `json:"name"`
	Picture string    `json:"picture,omitempty"`
}

// GoogleLoginRequest is the request body for Google login
type GoogleLoginRequest struct {
	// Credential string `json:"credential" binding:"required"`
	Code string `json:"code" binding:"required"`
}

// LoginResponse is returned after successful authentication
type LoginResponse struct {
	User         AuthUserResponse `json:"user"`
	AccessToken  string           `json:"accessToken"`
	RefreshToken string           `json:"refreshToken"`
	ExpiresIn    int              `json:"expiresIn"` // seconds
}

// RefreshTokenRequest is the request body for token refresh
type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// RefreshTokenResponse is returned after successful token refresh
type RefreshTokenResponse struct {
	AccessToken string `json:"accessToken"`
	ExpiresIn   int    `json:"expiresIn"`
}

// JWTClaims represents the claims in our JWT token
type JWTClaims struct {
	UserID       uuid.UUID `json:"userId"`
	Email        string    `json:"email"`
	TokenVersion int       `json:"tokenVersion"`
}

// GoogleUserInfo is returned for internal service lookups by email.
// Contains google provider data plus the user's budget context.
type GoogleUserInfo struct {
	GoogleID       string     `json:"googleId"`
	Email          string     `json:"email"`
	GmailHistoryID int        `json:"gmailHistoryId"`
	RefreshToken   string     `json:"refreshToken"`
	LastGmailSync  *time.Time `json:"lastGmailSync"`
	UserID         uuid.UUID  `json:"userId"`
	BudgetID       uuid.UUID  `json:"budgetId"`
}

// UpdateGmailHistoryRequest is the request body for updating gmail history ID
type UpdateGmailHistoryRequest struct {
	Email          string `json:"email"          binding:"required"`
	GmailHistoryID uint64 `json:"gmailHistoryId" binding:"required"`
}

// GoogleTokenPayload represents the decoded Google ID token
type GoogleTokenPayload struct {
	Aud           string `json:"aud"`
	Sub           string `json:"sub"` // Google User ID
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Iat           int64  `json:"iat"`
	Exp           int64  `json:"exp"`
}
