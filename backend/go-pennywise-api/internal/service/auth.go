package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"pennywise-api/internal/config"
	"pennywise-api/internal/model"
	"pennywise-api/internal/repository"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"google.golang.org/api/idtoken"
)

type AuthService interface {
	LoginWithGoogle(ctx context.Context, tokenString string) (string, error)
	GenerateAccessToken(ctx context.Context, userID uuid.UUID, name string) (string, error)
	GenerateRefreshToken(ctx context.Context, userID uuid.UUID) (string, error)
}

type authService struct {
	config config.Config
	repo   repository.AuthRepository
}

func NewAuthService(r repository.AuthRepository) AuthService {
	return &authService{repo: r, config: config.Load()}
}

func (s *authService) LoginWithGoogle(ctx context.Context, tokenString string) (string, error) {
	payload, err := idtoken.Validate(ctx, tokenString, s.config.GoogleClientID)
	if err != nil {
		return "", fmt.Errorf(("token validation failed: %w"), err)
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
				return "", fmt.Errorf("failed to create user: %w", err)
			}
		} else {
			// return error if any other error occurs
			return "", fmt.Errorf("failed to find user: %w", err)
		}
	}

	// generate access token
	accessToken, err := s.GenerateAccessToken(ctx, user.ID, user.Name)
	if err != nil {
		return "", fmt.Errorf("failed to generate access token: %w", err)
	}

	// generate refresh token
	// refreshToken, err := s.GenerateRefreshToken(ctx, user.ID)
	if err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return accessToken, nil
}

func (s *authService) GenerateAccessToken(ctx context.Context, userID uuid.UUID, name string) (string, error) {
	claims := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID.String(),                         // subject (user ID)
		"iss": "pennywise",                             // issuer
		"aud": name,                                    // audience (username)
		"exp": time.Now().Add(time.Minute * 15).Unix(), // expire in 15 minutes
		"iat": time.Now().Unix(),                       // issued at
	})
	
	tokenString, err := claims.SignedString(s.config.JWTSecret)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func (s *authService) GenerateRefreshToken(ctx context.Context, userID uuid.UUID) (string, error) {
	// claims := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
	// 	"sub": userID.String(),                         // subject (user ID)
	// 	"iss": "pennywise",                             // issuer
	// 	"exp": time.Now().Add(time.Minute * 15).Unix(), // expire in 15 minutes
	// 	"iat": time.Now().Unix(),                       // issued at
	// })
	return "", nil
}
