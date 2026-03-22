package service

//
// import (
// 	"context"
// 	"crypto/rand"
// 	"encoding/base64"
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"net/http"
// 	"os"
// 	"strings"
// 	"time"
//
// 	"pennywise-api/internal/model"
// 	"pennywise-api/internal/repository"
//
// 	"github.com/golang-jwt/jwt/v5"
// 	"github.com/google/uuid"
// )
//
// var (
// 	ErrInvalidToken     = errors.New("invalid token")
// 	ErrTokenExpired     = errors.New("token expired")
// 	ErrInvalidGoogleToken = errors.New("invalid google token")
// )
//
// const (
// 	AccessTokenExpiry  = 15 * time.Minute
// 	RefreshTokenExpiry = 7 * 24 * time.Hour
// )
//
// type AuthService interface {
// 	LoginWithGoogle(ctx context.Context, credential string) (*model.LoginResponse, error)
// 	RefreshToken(ctx context.Context, refreshToken string) (*model.RefreshTokenResponse, error)
// 	ValidateAccessToken(tokenString string) (*model.JWTClaims, error)
// 	LogoutAllDevices(ctx context.Context, userID uuid.UUID) error
// 	GetUserByID(ctx context.Context, userID uuid.UUID) (*model.AuthUser, error)
// }
//
// type authService struct {
// 	repo      repository.AuthRepository
// 	jwtSecret []byte
// 	googleClientID string
// }
//
// func NewAuthService(repo repository.AuthRepository) AuthService {
// 	secret := os.Getenv("JWT_SECRET")
// 	if secret == "" {
// 		secret = "pennywise-default-secret-change-in-production"
// 	}
// 	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
// 	return &authService{
// 		repo:           repo,
// 		jwtSecret:      []byte(secret),
// 		googleClientID: googleClientID,
// 	}
// }
//
// func (s *authService) LoginWithGoogle(ctx context.Context, credential string) (*model.LoginResponse, error) {
// 	// Verify Google token
// 	googleUser, err := s.verifyGoogleToken(credential)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to verify google token: %w", err)
// 	}
//
// 	// Find or create user
// 	user, err := s.repo.FindByGoogleID(ctx, googleUser.Sub)
// 	if err != nil {
// 		if errors.Is(err, repository.ErrUserNotFound) {
// 			// Create new user
// 			newUser := model.AuthUser{
// 				GoogleID: googleUser.Sub,
// 				Email:    googleUser.Email,
// 				Name:     googleUser.Name,
// 				Picture:  googleUser.Picture,
// 			}
// 			user, err = s.repo.Create(ctx, newUser)
// 			if err != nil {
// 				return nil, fmt.Errorf("failed to create user: %w", err)
// 			}
// 		} else {
// 			return nil, fmt.Errorf("failed to find user: %w", err)
// 		}
// 	}
//
// 	// Generate tokens
// 	accessToken, err := s.generateAccessToken(user)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to generate access token: %w", err)
// 	}
//
// 	refreshToken, err := s.generateRefreshToken()
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
// 	}
//
// 	return &model.LoginResponse{
// 		User: model.AuthUserResponse{
// 			ID:      user.ID,
// 			Email:   user.Email,
// 			Name:    user.Name,
// 			Picture: user.Picture,
// 		},
// 		AccessToken:  accessToken,
// 		RefreshToken: refreshToken,
// 		ExpiresIn:    int(AccessTokenExpiry.Seconds()),
// 	}, nil
// }
//
// func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*model.RefreshTokenResponse, error) {
// 	// For now, we'll use a simple approach - decode the user ID from refresh token
// 	// In production, you'd want to store refresh tokens in DB
// 	claims, err := s.decodeRefreshToken(refreshToken)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	// Get latest user with current token_version
// 	user, err := s.repo.FindByID(ctx, claims.UserID)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	// Verify token version matches (for logout all devices)
// 	if claims.TokenVersion != user.TokenVersion {
// 		return nil, ErrInvalidToken
// 	}
//
// 	// Generate new access token
// 	accessToken, err := s.generateAccessToken(user)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	return &model.RefreshTokenResponse{
// 		AccessToken: accessToken,
// 		ExpiresIn:   int(AccessTokenExpiry.Seconds()),
// 	}, nil
// }
//
// func (s *authService) ValidateAccessToken(tokenString string) (*model.JWTClaims, error) {
// 	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
// 		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
// 			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
// 		}
// 		return s.jwtSecret, nil
// 	})
//
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
// 		userIDStr, _ := claims["userId"].(string)
// 		userID, err := uuid.Parse(userIDStr)
// 		if err != nil {
// 			return nil, ErrInvalidToken
// 		}
//
// 		email, _ := claims["email"].(string)
// 		tokenVersion := int(claims["tokenVersion"].(float64))
//
// 		return &model.JWTClaims{
// 			UserID:       userID,
// 			Email:        email,
// 			TokenVersion: tokenVersion,
// 		}, nil
// 	}
//
// 	return nil, ErrInvalidToken
// }
//
// func (s *authService) LogoutAllDevices(ctx context.Context, userID uuid.UUID) error {
// 	return s.repo.UpdateTokenVersion(ctx, userID)
// }
//
// func (s *authService) GetUserByID(ctx context.Context, userID uuid.UUID) (*model.AuthUser, error) {
// 	return s.repo.FindByID(ctx, userID)
// }
//
// // Private methods
//
// func (s *authService) verifyGoogleToken(idToken string) (*model.GoogleTokenPayload, error) {
// 	// Verify with Google's tokeninfo endpoint
// 	resp, err := http.Get("https://oauth2.googleapis.com/tokeninfo?id_token=" + idToken)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer resp.Body.Close()
//
// 	if resp.StatusCode != http.StatusOK {
// 		return nil, ErrInvalidGoogleToken
// 	}
//
// 	var payload model.GoogleTokenPayload
// 	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
// 		return nil, err
// 	}
//
// 	// Verify audience matches our client ID
// 	if s.googleClientID != "" && payload.Aud != s.googleClientID {
// 		return nil, fmt.Errorf("token audience mismatch")
// 	}
//
// 	return &payload, nil
// }
//
// func (s *authService) generateAccessToken(user *model.AuthUser) (string, error) {
// 	claims := jwt.MapClaims{
// 		"userId":       user.ID.String(),
// 		"email":        user.Email,
// 		"tokenVersion": user.TokenVersion,
// 		"exp":          time.Now().Add(AccessTokenExpiry).Unix(),
// 		"iat":          time.Now().Unix(),
// 	}
//
// 	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
// 	return token.SignedString(s.jwtSecret)
// }
//
// func (s *authService) generateRefreshToken() (string, error) {
// 	bytes := make([]byte, 32)
// 	if _, err := rand.Read(bytes); err != nil {
// 		return "", err
// 	}
// 	return base64.URLEncoding.EncodeToString(bytes), nil
// }
//
// func (s *authService) decodeRefreshToken(refreshToken string) (*model.JWTClaims, error) {
// 	// Simple implementation - in production, store refresh tokens in DB
// 	// For now, we'll encode user info in the refresh token as well
// 	parts := strings.Split(refreshToken, ".")
// 	if len(parts) < 1 {
// 		return nil, ErrInvalidToken
// 	}
//
// 	// This is a simplified implementation
// 	// In production, you'd store refresh tokens in DB and look them up
// 	return nil, ErrInvalidToken
// }
