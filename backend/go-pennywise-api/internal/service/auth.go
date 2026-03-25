package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"pennywise-api/internal/config"
	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"google.golang.org/api/idtoken"
)

type AuthService interface {
	LoginWithGoogle(ctx context.Context, tokenString string) (*model.AuthUser, string, string, error)
	GenerateAccessToken(ctx context.Context, userID uuid.UUID, name string, email string, version int) (string, error)
	GenerateRefreshToken(ctx context.Context, userID uuid.UUID) (string, error)
	ValidateToken(ctx context.Context, tokenString string) (*jwt.Token, error)
	GetUserById(ctx context.Context, userID uuid.UUID) (*model.AuthUser, error)
	RefreshToken(ctx context.Context, refreshToken string) (*model.RefreshTokenResponse, error)
}

type authService struct {
	config config.Config
	repo   repository.AuthRepository
}

func NewAuthService(r repository.AuthRepository) AuthService {
	return &authService{repo: r, config: config.Load()}
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func (s *authService) LoginWithGoogle(ctx context.Context, tokenString string) (*model.AuthUser, string, string, error) {
	payload, err := idtoken.Validate(ctx, tokenString, s.config.GoogleClientID)
	if err != nil {
		return nil, "", "", fmt.Errorf(("token validation failed: %w"), err)
	}

	// fetch existing user
	user, err := s.repo.FindByGoogleID(ctx, payload.Subject)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			// create new user if not found
			newUser := model.AuthUser{
				GoogleID: payload.Subject,
				Email:    payload.Claims["email"].(string),
				Name:     payload.Claims["name"].(string),
				Picture:  payload.Claims["picture"].(string),
			}
			user, err = s.repo.Create(ctx, newUser)
			if err != nil {
				return nil, "", "", fmt.Errorf("failed to create user: %w", err)
			}
		} else {
			// return error if any other error occurs
			return nil, "", "", fmt.Errorf("failed to find user: %w", err)
		}
	}

	// generate access token
	accessToken, err := s.GenerateAccessToken(ctx, user.ID, user.Name, user.Email, user.TokenVersion)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	// generate refresh token
	refreshToken, err := s.GenerateRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// store refresh token hash on user record
	tokenHash := hashToken(refreshToken)
	if err := s.repo.SaveRefreshTokenHash(ctx, user.ID, tokenHash); err != nil {
		return nil, "", "", fmt.Errorf("failed to save refresh token: %w", err)
	}

	return user, accessToken, refreshToken, nil
}

func (s *authService) GenerateAccessToken(ctx context.Context, userID uuid.UUID, name string, email string, version int) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   userID.String(),                         // subject (user ID)
		"iss":   "pennywise",                             // issuer
		"aud":   email,                                    // audience (email)
		"exp":   time.Now().Add(time.Minute * 15).Unix(), // expire in 15 minutes
		"version": version,                               // token version for invalidation
		"iat":   time.Now().Unix(),                       // issued at
	})

	tokenString, err := t.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func (s *authService) GenerateRefreshToken(ctx context.Context, userID uuid.UUID) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID.String(),                           // subject (user ID)
		"iss": "pennywise",                               // issuer
		"exp": time.Now().Add(time.Hour * 24 * 30).Unix(), // expire in 30 days
		"iat": time.Now().Unix(),                         // issued at
	})

	tokenString, err := t.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func (s *authService) ValidateToken(ctx context.Context, tokenString string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		return []byte(s.config.JWTSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))

	slog.DebugContext(ctx, "token parsed", "valid", token.Valid, "error", err)
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return token, nil
}

func (s *authService) GetUserById(ctx context.Context, userID uuid.UUID) (*model.AuthUser, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}
	return user, nil
}

func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*model.RefreshTokenResponse, error) {
	// Validate the refresh token JWT
	token, err := s.ValidateToken(ctx, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	userId, err := token.Claims.GetSubject()
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token claims: %w", err)
	}
	userUuid, err := uuid.Parse(userId)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID in refresh token: %w", err)
	}

	// Fetch the user and verify the refresh token hash matches
	user, err := s.GetUserById(ctx, userUuid)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user for refresh token: %w", err)
	}

	storedHash, err := s.repo.GetRefreshTokenHash(ctx, userUuid)
	if err != nil {
		return nil, fmt.Errorf("failed to check refresh token: %w", err)
	}
	if storedHash == "" || storedHash != hashToken(refreshToken) {
		return nil, fmt.Errorf("refresh token has been revoked")
	}

	// Generate new access token with the same version
	newAccessToken, err := s.GenerateAccessToken(ctx, user.ID, user.Name, user.Email, user.TokenVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new access token: %w", err)
	}
	return &model.RefreshTokenResponse{
		AccessToken: newAccessToken,
		ExpiresIn:   900, // 15 minutes
	}, nil
}