package temporal

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.temporal.io/sdk/testsuite"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
)

// fakeAuthService implements service.AuthService for gmailWatch tests.
type fakeAuthService struct {
	getGoogleUserByEmail func(context.Context, string) (*model.GoogleUserInfo, error)
}

func (f *fakeAuthService) LoginWithGoogle(context.Context, model.GoogleLoginRequest) (*model.AuthUserResponse, string, string, error) {
	return nil, "", "", nil
}
func (f *fakeAuthService) GenerateAccessToken(context.Context, uuid.UUID, int) (string, error) {
	return "", nil
}
func (f *fakeAuthService) GenerateRefreshToken(context.Context, uuid.UUID) (string, error) {
	return "", nil
}
func (f *fakeAuthService) ValidateToken(context.Context, string) (*jwt.Token, error) {
	return nil, nil
}
func (f *fakeAuthService) GetUserById(context.Context, uuid.UUID) (*model.AuthUser, error) {
	return nil, nil
}
func (f *fakeAuthService) GetCurrentUser(context.Context, uuid.UUID) (*model.CurrentAuthUserResponse, error) {
	return nil, nil
}
func (f *fakeAuthService) RefreshToken(context.Context, string) (*model.RefreshTokenResponse, error) {
	return nil, nil
}
func (f *fakeAuthService) GetGoogleUserByEmail(ctx context.Context, email string) (*model.GoogleUserInfo, error) {
	if f.getGoogleUserByEmail == nil {
		return nil, nil
	}
	return f.getGoogleUserByEmail(ctx, email)
}

// helpers to run activities via Temporal test env

func executeGetGoogleUserByEmail(
	t *testing.T,
	act FetchGoogleUsersActivity,
	email string,
) (*model.GoogleUserInfo, error) {
	t.Helper()
	suite := testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(act.GetGoogleUserByEmail)
	encoded, err := env.ExecuteActivity(act.GetGoogleUserByEmail, email)
	if err != nil {
		return nil, err
	}
	var result model.GoogleUserInfo
	if err := encoded.Get(&result); err != nil {
		t.Fatalf("failed to decode activity result: %v", err)
	}
	return &result, nil
}

func executeUpdateGmailWatchState(
	t *testing.T,
	act FetchGoogleUsersActivity,
	input []model.GoogleWatchUser,
) error {
	t.Helper()
	suite := testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(act.UpdateGmailWatchState)
	_, err := env.ExecuteActivity(act.UpdateGmailWatchState, input)
	return err
}

// --- GetGoogleUserByEmail ---

func TestListGoogleUsersNeedingWatchRefresh_SkipsPausedAndDisconnectedUsers(t *testing.T) {
	historyID := uint64(7)
	act := FetchGoogleUsersActivity{AuthService: &fakeAuthService{
		getAllGoogleUsers: func(context.Context) ([]model.GoogleProviderUser, error) {
			return []model.GoogleProviderUser{
				{ID: "paused", GmailHistoryID: &historyID, RefreshToken: "tok", GmailIngestionPaused: true},
				{ID: "disconnected", GmailHistoryID: &historyID},
			}, nil
		},
	}}
	got, err := executeListGoogleUsersNeedingWatchRefresh(t, act)
	if err != nil || len(got) != 0 {
		t.Fatalf("paused and disconnected mailboxes must not renew: users=%v error=%v", got, err)
	}
}

func TestGetGoogleUserByEmail_ReturnsUser(t *testing.T) {
	want := &model.GoogleUserInfo{
		Email:          "test@example.com",
		GmailHistoryID: 42,
		RefreshToken:   "refresh-tok",
	}
	act := FetchGoogleUsersActivity{
		AuthService: &fakeAuthService{
			getGoogleUserByEmail: func(_ context.Context, email string) (*model.GoogleUserInfo, error) {
				if email != want.Email {
					t.Fatalf("unexpected email: %s", email)
				}
				return want, nil
			},
		},
	}
	got, err := executeGetGoogleUserByEmail(t, act, want.Email)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Email != want.Email || got.GmailHistoryID != want.GmailHistoryID {
		t.Fatalf("unexpected user: %+v", got)
	}
}

func TestGetGoogleUserByEmail_WrapsServiceError(t *testing.T) {
	wantErr := errors.New("not found")
	act := FetchGoogleUsersActivity{
		AuthService: &fakeAuthService{
			getGoogleUserByEmail: func(context.Context, string) (*model.GoogleUserInfo, error) {
				return nil, wantErr
			},
		},
	}
	_, err := executeGetGoogleUserByEmail(t, act, "x@y.com")
	if err == nil || !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("expected wrapped error, got %v", err)
	}
	assertErrorCode(t, err, errs.CodeAuthLookupFailed)
}

// The Gmail state these activities tracked moved to mail_connections.
func TestGmailStateActivitiesDoNothing(t *testing.T) {
	act := &FetchGoogleUsersActivity{AuthService: &fakeAuthService{}}
	ctx := context.Background()

	users, err := act.ListGoogleUsersNeedingWatchRefresh(ctx)
	if err != nil || len(users) != 0 {
		t.Fatalf("users = %v, err = %v", users, err)
	}
	if err := act.UpdateGmailHistoryID(ctx, model.UpdateGmailHistoryInput{Email: "a@a.com"}); err != nil {
		t.Fatal(err)
	}
	if err := act.UpdateGmailWatchState(ctx, []model.GoogleWatchUser{{Email: "a@a.com"}}); err != nil {
		t.Fatal(err)
	}
}
