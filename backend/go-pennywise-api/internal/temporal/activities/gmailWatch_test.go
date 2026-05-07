package temporal

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.temporal.io/sdk/testsuite"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
)

// fakeAuthService implements service.AuthService for gmailWatch tests.
type fakeAuthService struct {
	getAllGoogleUsers    func(context.Context) ([]model.GoogleProviderUser, error)
	getGoogleUserByEmail func(context.Context, string) (*model.GoogleUserInfo, error)
	updateGmailHistoryID func(context.Context, string, uint64, *int64) error
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
func (f *fakeAuthService) GetAllGoogleUsers(ctx context.Context) ([]model.GoogleProviderUser, error) {
	if f.getAllGoogleUsers == nil {
		return nil, nil
	}
	return f.getAllGoogleUsers(ctx)
}
func (f *fakeAuthService) GetGoogleUserByEmail(ctx context.Context, email string) (*model.GoogleUserInfo, error) {
	if f.getGoogleUserByEmail == nil {
		return nil, nil
	}
	return f.getGoogleUserByEmail(ctx, email)
}
func (f *fakeAuthService) UpdateGmailHistoryID(ctx context.Context, email string, historyID uint64, expiryAt *int64) error {
	if f.updateGmailHistoryID == nil {
		return nil
	}
	return f.updateGmailHistoryID(ctx, email, historyID, expiryAt)
}

// helpers to run activities via Temporal test env

func executeListGoogleUsersNeedingWatchRefresh(
	t *testing.T,
	act FetchGoogleUsersActivity,
) ([]model.GoogleWatchUser, error) {
	t.Helper()
	suite := testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(act.ListGoogleUsersNeedingWatchRefresh)
	encoded, err := env.ExecuteActivity(act.ListGoogleUsersNeedingWatchRefresh)
	if err != nil {
		return nil, err
	}
	var result []model.GoogleWatchUser
	if err := encoded.Get(&result); err != nil {
		t.Fatalf("failed to decode activity result: %v", err)
	}
	return result, nil
}

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

func executeUpdateGmailHistoryID(
	t *testing.T,
	act FetchGoogleUsersActivity,
	input model.UpdateGmailHistoryInput,
) error {
	t.Helper()
	suite := testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(act.UpdateGmailHistoryID)
	_, err := env.ExecuteActivity(act.UpdateGmailHistoryID, input)
	return err
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

// --- ListGoogleUsersNeedingWatchRefresh ---

func TestListGoogleUsersNeedingWatchRefresh_ServiceError(t *testing.T) {
	wantErr := errors.New("db failure")
	act := FetchGoogleUsersActivity{
		AuthService: &fakeAuthService{
			getAllGoogleUsers: func(context.Context) ([]model.GoogleProviderUser, error) {
				return nil, wantErr
			},
		},
	}
	_, err := executeListGoogleUsersNeedingWatchRefresh(t, act)
	if err == nil || !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("expected wrapped service error, got %v", err)
	}
	assertErrorCode(t, err, errs.CodeAuthLookupFailed)
}

func TestListGoogleUsersNeedingWatchRefresh_SkipsUsersWithNoHistoryID(t *testing.T) {
	act := FetchGoogleUsersActivity{
		AuthService: &fakeAuthService{
			getAllGoogleUsers: func(context.Context) ([]model.GoogleProviderUser, error) {
				// user with no history id should be skipped
				return []model.GoogleProviderUser{
					{Email: "no-history@example.com", GmailHistoryID: nil},
				}, nil
			},
		},
	}
	got, err := executeListGoogleUsersNeedingWatchRefresh(t, act)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 watch users, got %d", len(got))
	}
}

func TestListGoogleUsersNeedingWatchRefresh_IncludesExpiredUsers(t *testing.T) {
	historyID := uint64(12345)
	// expiry in the past — should always be included
	pastExpiry := time.Now().Add(-time.Hour).UnixMilli()
	act := FetchGoogleUsersActivity{
		AuthService: &fakeAuthService{
			getAllGoogleUsers: func(context.Context) ([]model.GoogleProviderUser, error) {
				return []model.GoogleProviderUser{
					{
						ID:             "gid-1",
						Email:          "user@example.com",
						GmailHistoryID: &historyID,
						RefreshToken:   "tok",
						ExpiryAt:       &pastExpiry,
					},
				}, nil
			},
		},
	}
	got, err := executeListGoogleUsersNeedingWatchRefresh(t, act)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 watch user, got %d", len(got))
	}
	if got[0].Email != "user@example.com" || got[0].GmailHistoryID != historyID {
		t.Fatalf("unexpected watch user: %+v", got[0])
	}
}

func TestListGoogleUsersNeedingWatchRefresh_ExcludesFarFutureExpiry(t *testing.T) {
	historyID := uint64(99)
	// expiry far in the future — should NOT be refreshed
	futureExpiry := time.Now().Add(time.Hour * 24 * 30).UnixMilli()
	act := FetchGoogleUsersActivity{
		AuthService: &fakeAuthService{
			getAllGoogleUsers: func(context.Context) ([]model.GoogleProviderUser, error) {
				return []model.GoogleProviderUser{
					{
						ID:             "gid-2",
						Email:          "fresh@example.com",
						GmailHistoryID: &historyID,
						ExpiryAt:       &futureExpiry,
					},
				}, nil
			},
		},
	}
	got, err := executeListGoogleUsersNeedingWatchRefresh(t, act)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 watch users (not expiring soon), got %d", len(got))
	}
}

func TestListGoogleUsersNeedingWatchRefresh_IncludesNilExpiry(t *testing.T) {
	historyID := uint64(7)
	act := FetchGoogleUsersActivity{
		AuthService: &fakeAuthService{
			getAllGoogleUsers: func(context.Context) ([]model.GoogleProviderUser, error) {
				return []model.GoogleProviderUser{
					{
						ID:             "gid-3",
						Email:          "nil-expiry@example.com",
						GmailHistoryID: &historyID,
						ExpiryAt:       nil, // nil means refresh is needed
					},
				}, nil
			},
		},
	}
	got, err := executeListGoogleUsersNeedingWatchRefresh(t, act)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 watch user (nil expiry), got %d", len(got))
	}
}

// --- GetGoogleUserByEmail ---

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

// --- UpdateGmailHistoryID ---

func TestUpdateGmailHistoryID_CallsService(t *testing.T) {
	called := false
	act := FetchGoogleUsersActivity{
		AuthService: &fakeAuthService{
			updateGmailHistoryID: func(_ context.Context, email string, historyID uint64, expiryAt *int64) error {
				called = true
				if email != "u@example.com" || historyID != 111 || expiryAt != nil {
					t.Fatalf("unexpected args: email=%s historyID=%d expiryAt=%v", email, historyID, expiryAt)
				}
				return nil
			},
		},
	}
	err := executeUpdateGmailHistoryID(t, act, model.UpdateGmailHistoryInput{
		Email:          "u@example.com",
		GmailHistoryID: 111,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("expected service to be called")
	}
}

func TestUpdateGmailHistoryID_PropagatesError(t *testing.T) {
	wantErr := errors.New("update failed")
	act := FetchGoogleUsersActivity{
		AuthService: &fakeAuthService{
			updateGmailHistoryID: func(context.Context, string, uint64, *int64) error {
				return wantErr
			},
		},
	}
	err := executeUpdateGmailHistoryID(t, act, model.UpdateGmailHistoryInput{Email: "e@e.com"})
	if err == nil || !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("expected propagated error, got %v", err)
	}
}

// --- UpdateGmailWatchState ---

func TestUpdateGmailWatchState_CallsServiceForEachUser(t *testing.T) {
	calls := 0
	act := FetchGoogleUsersActivity{
		AuthService: &fakeAuthService{
			updateGmailHistoryID: func(_ context.Context, email string, historyID uint64, expiryAt *int64) error {
				calls++
				return nil
			},
		},
	}
	expiry := int64(9999999)
	input := []model.GoogleWatchUser{
		{Email: "a@a.com", GmailHistoryID: 1, ExpiryAt: &expiry},
		{Email: "b@b.com", GmailHistoryID: 2, ExpiryAt: nil},
	}
	err := executeUpdateGmailWatchState(t, act, input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 service calls, got %d", calls)
	}
}

func TestUpdateGmailWatchState_StopsOnFirstError(t *testing.T) {
	calls := 0
	wantErr := errors.New("watch state update failed")
	act := FetchGoogleUsersActivity{
		AuthService: &fakeAuthService{
			updateGmailHistoryID: func(context.Context, string, uint64, *int64) error {
				calls++
				return wantErr
			},
		},
	}
	input := []model.GoogleWatchUser{
		{Email: "a@a.com"},
		{Email: "b@b.com"},
	}
	err := executeUpdateGmailWatchState(t, act, input)
	if err == nil || !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("expected error from service, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call before stopping, got %d", calls)
	}
}

func TestUpdateGmailWatchState_EmptyInput(t *testing.T) {
	act := FetchGoogleUsersActivity{
		AuthService: &fakeAuthService{
			updateGmailHistoryID: func(context.Context, string, uint64, *int64) error {
				t.Fatal("should not be called with empty input")
				return nil
			},
		},
	}
	err := executeUpdateGmailWatchState(t, act, []model.GoogleWatchUser{})
	if err != nil {
		t.Fatalf("expected no error for empty input, got %v", err)
	}
}
