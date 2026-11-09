package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/config"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type AuthService interface {
	LoginWithGoogle(ctx context.Context, req model.GoogleLoginRequest) (*model.AuthUserResponse, string, string, error)
	GenerateAccessToken(ctx context.Context, userID uuid.UUID, version int) (string, error)
	GenerateRefreshToken(ctx context.Context, userID uuid.UUID) (string, error)
	ValidateToken(ctx context.Context, tokenString string) (*jwt.Token, error)
	GetUserById(ctx context.Context, userID uuid.UUID) (*model.AuthUser, error)
	GetCurrentUser(ctx context.Context, userID uuid.UUID) (*model.CurrentAuthUserResponse, error)
	GetGoogleUserByEmail(ctx context.Context, email string) (*model.GoogleUserInfo, error)
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
	dbTxn          repository.DBTxn
}

const googleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"

// googleOAuthScopes are the scopes the Android app asks for. Login only signs
// in on either client: mailboxes connect through mail_connections.
var googleOAuthScopes = []string{"https://mail.google.com/", "https://www.googleapis.com/auth/userinfo.email"}

var googleWebLoginScopes = []string{"openid", "profile", "email"}

func (s *authService) getOauth2Config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.config.GoogleClientID,
		ClientSecret: s.config.GoogleClientSecret,
		// RedirectURL:  s.config.CallbackURL,
		RedirectURL: "postmessage",
		Endpoint:    google.Endpoint,
		Scopes:      googleWebLoginScopes,
	}
}

func (s *authService) getMobileOauth2Config(req model.GoogleLoginRequest) (*oauth2.Config, []oauth2.AuthCodeOption, error) {
	if req.RedirectURI == "" || req.CodeVerifier == "" {
		return nil, nil, errs.New(
			errs.CodeInvalidArgument,
			"redirectUri and codeVerifier are required for mobile Google login",
		)
	}
	if s.config.GoogleAndroidClientID == "" {
		return nil, nil, errs.New(
			errs.CodeInvalidArgument,
			"GOOGLE_ANDROID_CLIENT_ID is required for mobile Google login",
		)
	}
	return &oauth2.Config{
		ClientID:    s.config.GoogleAndroidClientID,
		RedirectURL: req.RedirectURI,
		Endpoint:    google.Endpoint,
		Scopes:      googleOAuthScopes,
	}, []oauth2.AuthCodeOption{oauth2.SetAuthURLParam("code_verifier", req.CodeVerifier)}, nil
}

func getOAuthClientTypeForLogin(req model.GoogleLoginRequest) model.GoogleOAuthClientType {
	if req.RedirectURI != "" || req.CodeVerifier != "" {
		return model.GoogleOAuthClientTypeAndroid
	}
	return model.GoogleOAuthClientTypeWeb
}

func (s *authService) getOauth2ConfigForLogin(req model.GoogleLoginRequest) (*oauth2.Config, []oauth2.AuthCodeOption, error) {
	if getOAuthClientTypeForLogin(req) == model.GoogleOAuthClientTypeAndroid {
		return s.getMobileOauth2Config(req)
	}
	return s.getOauth2Config(), nil, nil
}

func NewAuthService(
	r repository.AuthRepository,
	googleProvider repository.GoogleProviderRepository,
	dbTxn repository.DBTxn,
) AuthService {
	return &authService{dbTxn: dbTxn, repo: r, googleProvider: googleProvider, config: config.Load()}
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
	req model.GoogleLoginRequest,
) (*model.AuthUserResponse, string, string, error) {
	logger := logger.Logger(ctx)
	oauthClientType := getOAuthClientTypeForLogin(req)
	oauthConfig, authCodeOptions, err := s.getOauth2ConfigForLogin(req)
	if err != nil {
		return nil, "", "", err
	}
	oauthToken, err := oauthConfig.Exchange(ctx, req.Code, authCodeOptions...)
	if err != nil {
		return nil, "", "", errs.Wrap(errs.CodeInvalidArgument, "token exchange failed", err)
	}

	googleProfile, err := s.fetchGoogleUser(ctx, oauthConfig, oauthToken)
	if err != nil {
		return nil, "", "", err
	}

	userWithCreds, err := s.googleProvider.GetUserByGoogleIDAndClientType(ctx, googleProfile.ID, oauthClientType)
	if err != nil {
		if !errors.Is(err, repository.ErrUserNotFound) {
			return nil, "", "", errs.Wrap(errs.CodeAuthLookupFailed, "failed to find user", err)
		}

		existingUser, existingErr := s.googleProvider.GetUserByGoogleID(ctx, googleProfile.ID)
		if existingErr != nil && !errors.Is(existingErr, repository.ErrUserNotFound) {
			return nil, "", "", errs.Wrap(errs.CodeAuthLookupFailed, "failed to find existing google user", existingErr)
		}

		if existingUser != nil && existingUser.AuthUser != nil {
			userWithCreds, err = s.createGoogleProviderForUser(
				ctx,
				existingUser.AuthUser,
				googleProfile,
				oauthClientType,
			)
		} else {
			userWithCreds, err = s.createGoogleUser(ctx, googleProfile, oauthClientType)
		}
		if err != nil {
			return nil, "", "", err
		}
	} else {
		data := model.GoogleProviderUser{
			OAuthClientType: oauthClientType,
			Name:            googleProfile.Name,
			Picture:         googleProfile.Picture,
		}
		err = s.googleProvider.UpdateUserByGoogleIDAndClientType(ctx, googleProfile.ID, oauthClientType, &data)
		if err != nil {
			return nil, "", "", err
		}
		if userWithCreds.GoogleProvider != nil {
			userWithCreds.GoogleProvider.OAuthClientType = oauthClientType
			userWithCreds.GoogleProvider.Name = googleProfile.Name
			userWithCreds.GoogleProvider.Picture = googleProfile.Picture
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
	oauthClientType model.GoogleOAuthClientType,
) (*model.UserWithCredentials, error) {
	log := logger.Logger(ctx)
	oauthClientType = model.NormalizeGoogleOAuthClientType(oauthClientType)
	log.Info("creating new google user", "googleId", profile.ID, "email", profile.Email, "oauthClientType", oauthClientType)

	var result *model.UserWithCredentials
	err := s.dbTxn.Do(ctx, func(ctx context.Context) error {
		authUser, err := s.repo.CreateUser(ctx)
		if err != nil {
			return errs.Wrap(errs.CodeAuthCreateFailed, "failed to create auth user", err)
		}

		userWithCreds, err := s.googleProvider.Create(
			ctx, authUser.ID,
			profile.ID, oauthClientType, profile.Name, profile.Picture, profile.Email,
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

func (s *authService) createGoogleProviderForUser(
	ctx context.Context,
	authUser *model.AuthUser,
	profile *googleUser,
	oauthClientType model.GoogleOAuthClientType,
) (*model.UserWithCredentials, error) {
	oauthClientType = model.NormalizeGoogleOAuthClientType(oauthClientType)
	log := logger.Logger(ctx)
	log.Info("creating google provider for existing user", "authUserId", authUser.ID, "googleId", profile.ID, "email", profile.Email, "oauthClientType", oauthClientType)

	var result *model.UserWithCredentials
	err := s.dbTxn.Do(ctx, func(ctx context.Context) error {
		userWithCreds, err := s.googleProvider.Create(
			ctx, authUser.ID,
			profile.ID, oauthClientType, profile.Name, profile.Picture, profile.Email,
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

	log.Info("google provider created", "authUserId", result.AuthUser.ID, "email", profile.Email, "oauthClientType", oauthClientType)
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
		return nil, errs.Wrap(errs.CodeAuthLookupFailed, "failed to get user by ID", err)
	}
	return user, nil
}

func (s *authService) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*model.CurrentAuthUserResponse, error) {
	user, err := s.repo.GetUserWithProviders(ctx, userID)
	if err != nil {
		return nil, err
	}

	for i := range user.Providers {
		provider := &user.Providers[i]
		switch provider.ProviderType {
		case model.GoogleAuthProviderType:
			provider.OAuthClientType = model.NormalizeGoogleOAuthClientType(provider.OAuthClientType)
			googleUser, err := s.googleProvider.GetUserByGoogleIDAndClientType(ctx, provider.ProviderID, provider.OAuthClientType)
			if err != nil {
				return nil, err
			}
			if googleUser.GoogleProvider == nil {
				continue
			}

			googleProvider := googleUser.GoogleProvider
			provider.Name = googleProvider.Name
			provider.Email = googleProvider.Email
			provider.Picture = googleProvider.Picture

			if user.Picture == "" && googleProvider.Picture != "" {
				user.Picture = googleProvider.Picture
			}
			if user.Email == "" && googleProvider.Email != "" {
				user.Email = googleProvider.Email
			}
			if user.Name == "" && googleProvider.Name != "" {
				user.Name = googleProvider.Name
			}
		}
	}

	return user, nil
}

func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*model.RefreshTokenResponse, error) {
	// Validate the refresh token JWT
	token, err := s.ValidateToken(ctx, refreshToken)
	if err != nil {
		return nil, errs.Wrap(errs.CodeAuthInvalidCredentials, "invalid refresh token", err)
	}
	logger.Logger(ctx).Debug("token validated", "valid", token.Valid, "claims", token.Claims, "error", err)

	userId, err := token.Claims.GetSubject()
	logger.Logger(ctx).Debug("userId", "userId", userId, "error", err)
	if err != nil {
		return nil, errs.Wrap(errs.CodeAuthInvalidCredentials, "invalid refresh token claims", err)
	}
	userUuid, err := uuid.Parse(userId)
	if err != nil {
		return nil, errs.Wrap(errs.CodeAuthInvalidCredentials, "invalid user ID in refresh token", err)
	}

	// Fetch the user and verify the refresh token hash matches
	user, err := s.GetUserById(ctx, userUuid)
	logger.Logger(ctx).Debug("USER INFO", "user", user, "error", err)
	if errors.Is(err, repository.ErrUserNotFound) {
		return nil, errs.Wrap(errs.CodeAuthInvalidCredentials, "user not found", err)
	}
	if err != nil {
		return nil, errs.Wrap(errs.CodeAuthLookupFailed, "failed to fetch user for refresh token", err)
	}
	if user == nil {
		return nil, errs.New(errs.CodeAuthInvalidCredentials, "user not found")
	}

	storedHash, err := s.repo.GetRefreshTokenHash(ctx, userUuid)
	logger.Logger(ctx).Debug("STORED HASH", "storedHash", storedHash, "error", err)
	if errors.Is(err, repository.ErrUserNotFound) {
		return nil, errs.Wrap(errs.CodeAuthInvalidCredentials, "user not found", err)
	}
	if err != nil {
		return nil, errs.Wrap(errs.CodeAuthLookupFailed, "failed to check refresh token", err)
	}
	// if storedHash == "" || storedHash != utils.Hash(refreshToken) {
	// 	return nil, errs.New(errs.CodeAuthLookupFailed, "refresh token has been revoked")
	// }

	// Generate new access token with the same version
	newAccessToken, err := s.GenerateAccessToken(ctx, user.ID, user.TokenVersion)
	if err != nil {
		return nil, errs.Wrap(errs.CodeAuthCreateFailed, "failed to generate new access token", err)
	}
	return &model.RefreshTokenResponse{
		AccessToken: newAccessToken,
		ExpiresIn:   900, // 15 minutes
	}, nil
}

func (s *authService) GetGoogleUserByEmail(ctx context.Context, email string) (*model.GoogleUserInfo, error) {
	return s.googleProvider.GetUserByEmail(ctx, email)
}
