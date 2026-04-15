package model

import (
	"time"

	"github.com/google/uuid"
)

// AuthUser represents an authenticated user via Google OAuth
type AuthUser struct {
	ID           uuid.UUID `json:"id"`
	GoogleID     string    `json:"googleId"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	Picture      string    `json:"picture,omitempty"`
	TokenVersion int       `json:"tokenVersion"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
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
	Credential string `json:"credential" binding:"required"`
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

// GoogleTokenPayload represents the decoded Google ID token
type GoogleTokenPayload struct {
	Iss           string `json:"iss"`
	Azp           string `json:"azp"`
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
