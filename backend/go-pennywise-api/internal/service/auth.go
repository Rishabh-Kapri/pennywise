package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/config"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type AuthService interface {
	LoginWithGoogle(ctx context.Context, tokenString string) (*model.AuthUserResponse, string, string, error)
	GenerateAccessToken(ctx context.Context, userID uuid.UUID, version int) (string, error)
	GenerateRefreshToken(ctx context.Context, userID uuid.UUID) (string, error)
	ValidateToken(ctx context.Context, tokenString string) (*jwt.Token, error)
	GetUserById(ctx context.Context, userID uuid.UUID) (*model.AuthUser, error)
	GetGoogleUserByEmail(ctx context.Context, email string) (*model.GoogleUserInfo, error)
	UpdateGmailHistoryID(ctx context.Context, email string, historyID uint64) error
	RefreshToken(ctx context.Context, refreshToken string) (*model.RefreshTokenResponse, error)
}

type googleUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
}

type authService struct {
	config         config.Config
	repo           repository.AuthRepository
	googleProvider repository.GoogleProviderRepository
	transport      *transport.Client
}

const googleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"

func (s *authService) getOauth2Config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.config.GoogleClientID,
		ClientSecret: s.config.GoogleClientSecret,
		// RedirectURL:  s.config.CallbackURL,
		RedirectURL: "postmessage",
		Endpoint:    google.Endpoint,
		Scopes:      []string{"https://mail.google.com/", "https://www.googleapis.com/auth/userinfo.email"},
	}
}

type gmailSyncRequest struct {
	RefreshToken string `json:"refreshToken"`
	Email        string `json:"email"`
	IsStop       bool   `json:"isStop"`
}

type gmailSyncResponse struct {
	HistoryID uint64 `json:"historyID"`
}

func NewAuthService(
	r repository.AuthRepository,
	googleProvider repository.GoogleProviderRepository,
	transport *transport.Client,
) AuthService {
	return &authService{repo: r, googleProvider: googleProvider, config: config.Load(), transport: transport}
}

func (s *authService) SetupGmailWatch(ctx context.Context, googleID string, refreshToken string, email string) {
	logger := logger.Logger(ctx)
	logger.Info("watching gmail", "email", email)

	reqData := gmailSyncRequest{
		RefreshToken: refreshToken,
		Email:        email,
		IsStop:       false,
	}
	// headers := utils.GetHeaders(ctx)
	var headers map[string][]string
	res, err := transport.Post[gmailSyncResponse](ctx, s.transport, "/api/watch", headers, reqData)
	if err != nil {
		// TODO: handle error
		logger.Error("error watching gmail", "email", email, "error", err)
		return
	}
	logger.Info("gmail watch done", "historyId", res.HistoryID)
	if err = s.googleProvider.UpdateHistoryID(ctx, googleID, res.HistoryID); err != nil {
		logger.Error("error updating gmail history id", "email", email, "error", err)
		return
	}
}

func (s *authService) fetchGoogleUser(
	ctx context.Context,
	oauthConfig *oauth2.Config,
	token *oauth2.Token,
) (*googleUser, error) {
	oauthClient := oauthConfig.Client(ctx, token)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return nil, errs.Wrap(errs.CodeAuthLookupFailed, "failed to create user info request", err)
	}

	resp, err := oauthClient.Do(request)
	if err != nil {
		return nil, errs.Wrap(errs.CodeAuthLookupFailed, "failed to get user info", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, errs.Wrap(errs.CodeAuthLookupFailed, "failed to read google user info error response", readErr)
		}

		return nil, errs.New(
			errs.CodeAuthLookupFailed,
			"google user info request failed with status %s: %s",
			resp.Status,
			string(body),
		)
	}

	var profile googleUser
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, errs.Wrap(errs.CodeAuthLookupFailed, "failed to decode user info", err)
	}

	if profile.ID == "" {
		return nil, errs.New(errs.CodeAuthLookupFailed, "google user info missing id")
	}

	return &profile, nil
}

func (s *authService) LoginWithGoogle(
	ctx context.Context,
	tokenString string,
) (*model.AuthUserResponse, string, string, error) {
	logger := logger.Logger(ctx)
	oauthConfig := s.getOauth2Config()
	oauthToken, err := oauthConfig.Exchange(ctx, tokenString)
	if err != nil {
		return nil, "", "", errs.Wrap(errs.CodeInvalidArgument, "token exchange failed", err)
	}

	googleProfile, err := s.fetchGoogleUser(ctx, oauthConfig, oauthToken)
	if err != nil {
		return nil, "", "", err
	}

	userWithCreds, err := s.googleProvider.GetUserByGoogleID(ctx, googleProfile.ID)
	if err != nil {
		if !errors.Is(err, repository.ErrUserNotFound) {
			return nil, "", "", errs.Wrap(errs.CodeAuthLookupFailed, "failed to find user", err)
		}

		// User not found — create auth_user + auth_provider + google_provider_user in a transaction
		userWithCreds, err = s.createGoogleUser(ctx, googleProfile, oauthToken.RefreshToken)
		if err != nil {
			return nil, "", "", err
		}
	}
	logger.Info("user found", "authUser", *userWithCreds.AuthUser, "googleProvider", *userWithCreds.GoogleProvider)

	accessToken, err := s.GenerateAccessToken(ctx, userWithCreds.AuthUser.ID, userWithCreds.AuthUser.TokenVersion)
	if err != nil {
		return nil, "", "", errs.Wrap(errs.CodeAuthCreateFailed, "failed to generate access token", err)
	}

	refreshToken, err := s.GenerateRefreshToken(ctx, userWithCreds.AuthUser.ID)
	if err != nil {
		return nil, "", "", errs.Wrap(errs.CodeAuthCreateFailed, "failed to generate refresh token", err)
	}

	tokenHash := refreshToken
	if err := s.repo.SaveRefreshTokenHash(ctx, userWithCreds.AuthUser.ID, tokenHash); err != nil {
		return nil, "", "", errs.Wrap(errs.CodeAuthCreateFailed, "failed to save refresh token", err)
	}

	// setup gmail watch
	lastGmailSync := userWithCreds.GoogleProvider.LastGmailSync
	if lastGmailSync == nil || lastGmailSync.Before(time.Now().Add(-time.Hour*24*5)) {
		googleRefreshToken := userWithCreds.GoogleProvider.RefreshToken
		detachedCtx := context.WithoutCancel(ctx)
		go s.SetupGmailWatch(
			detachedCtx,
			userWithCreds.GoogleProvider.ID,
			googleRefreshToken,
			userWithCreds.GoogleProvider.Email,
		)
	}

	resUser := model.AuthUserResponse{
		ID:      userWithCreds.AuthUser.ID,
		Email:   userWithCreds.GoogleProvider.Email,
		Name:    userWithCreds.GoogleProvider.Name,
		Picture: userWithCreds.GoogleProvider.Picture,
	}
	return &resUser, accessToken, refreshToken, nil
}

func (s *authService) createGoogleUser(
	ctx context.Context,
	profile *googleUser,
	refreshToken string,
) (*model.UserWithCredentials, error) {
	log := logger.Logger(ctx)
	log.Info("creating new google user", "googleId", profile.ID, "email", profile.Email)

	var result *model.UserWithCredentials
	err := utils.WithTx(ctx, s.repo.GetDB(), func(tx pgx.Tx) error {
		authUser, err := s.repo.CreateUser(ctx, tx)
		if err != nil {
			return errs.Wrap(errs.CodeAuthCreateFailed, "failed to create auth user", err)
		}

		userWithCreds, err := s.googleProvider.Create(
			ctx, tx, authUser.ID,
			profile.ID, profile.Name, profile.Picture, profile.Email,
			refreshToken,
		)
		if err != nil {
			return errs.Wrap(errs.CodeAuthCreateFailed, "failed to create google provider user", err)
		}

		userWithCreds.AuthUser = authUser
		result = userWithCreds
		return nil
	})
	if err != nil {
		return nil, err
	}

	log.Info("google user created", "authUserId", result.AuthUser.ID, "email", profile.Email)
	return result, nil
}

func (s *authService) GenerateAccessToken(ctx context.Context, userID uuid.UUID, version int) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":     userID.String(),                         // subject (user ID)
		"iss":     "pennywise",                             // issuer
		"exp":     time.Now().Add(time.Minute * 15).Unix(), // expire in 15 minutes
		"version": version,                                 // token version for invalidation
		"iat":     time.Now().Unix(),                       // issued at
	})

	tokenString, err := t.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func (s *authService) GenerateRefreshToken(ctx context.Context, userID uuid.UUID) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID.String(),                            // subject (user ID)
		"iss": "pennywise",                                // issuer
		"exp": time.Now().Add(time.Hour * 24 * 30).Unix(), // expire in 30 days
		"iat": time.Now().Unix(),                          // issued at
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
	if err != nil {
		logger.Logger(ctx).Debug("token parse failed", "error", err)
		return nil, err
	}

	logger.Logger(ctx).Debug("token parsed", "valid", token.Valid)
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return token, nil
}

func (s *authService) GetUserById(ctx context.Context, userID uuid.UUID) (*model.AuthUser, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, errs.New(errs.CodeAuthLookupFailed, "failed to get user by ID", err)
	}
	return user, nil
}

func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*model.RefreshTokenResponse, error) {
	// Validate the refresh token JWT
	token, err := s.ValidateToken(ctx, refreshToken)
	if err != nil {
		return nil, errs.New(errs.CodeAuthLookupFailed, "invalid refresh token", err)
	}
	logger.Logger(ctx).Debug("token validated", "valid", token.Valid, "claims", token.Claims, "error", err)

	userId, err := token.Claims.GetSubject()
	logger.Logger(ctx).Debug("userId", "userId", userId, "error", err)
	if err != nil {
		return nil, errs.New(errs.CodeAuthLookupFailed, "invalid refresh token claims", err)
	}
	userUuid, err := uuid.Parse(userId)
	if err != nil {
		return nil, errs.New(errs.CodeAuthLookupFailed, "invalid user ID in refresh token", err)
	}

	// Fetch the user and verify the refresh token hash matches
	user, err := s.GetUserById(ctx, userUuid)
	logger.Logger(ctx).Debug("USER INFO", "user", user, "error", err)
	if err != nil {
		return nil, errs.New(errs.CodeAuthLookupFailed, "failed to fetch user for refresh token", err)
	}
	if user == nil {
		return nil, errs.New(errs.CodeAuthLookupFailed, "user not found")
	}

	storedHash, err := s.repo.GetRefreshTokenHash(ctx, userUuid)
	logger.Logger(ctx).Debug("STORED HASH", "storedHash", storedHash, "error", err)
	if err != nil {
		return nil, errs.New(errs.CodeAuthLookupFailed, "failed to check refresh token", err)
	}
	// if storedHash == "" || storedHash != utils.Hash(refreshToken) {
	// 	return nil, errs.New(errs.CodeAuthLookupFailed, "refresh token has been revoked")
	// }

	// Generate new access token with the same version
	newAccessToken, err := s.GenerateAccessToken(ctx, user.ID, user.TokenVersion)
	if err != nil {
		return nil, errs.New(errs.CodeAuthCreateFailed, "failed to generate new access token", err)
	}
	return &model.RefreshTokenResponse{
		AccessToken: newAccessToken,
		ExpiresIn:   900, // 15 minutes
	}, nil
}

func (s *authService) GetGoogleUserByEmail(ctx context.Context, email string) (*model.GoogleUserInfo, error) {
	return s.googleProvider.GetUserByEmail(ctx, email)
}

func (s *authService) UpdateGmailHistoryID(ctx context.Context, email string, historyID uint64) error {
	return s.googleProvider.UpdateHistoryIDByEmail(ctx, email, historyID)
}
