package temporal

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

type FetchGoogleUsersActivity struct {
	AuthService service.AuthService
}

// ListGoogleUsersNeedingWatchRefresh, UpdateGmailHistoryID and
// UpdateGmailWatchState do nothing since Gmail state moved to mail_connections.
// They stay registered so running workflows finish; remove them with Temporal.
func (a *FetchGoogleUsersActivity) ListGoogleUsersNeedingWatchRefresh(
	ctx context.Context,
) ([]sharedModel.GoogleWatchUser, error) {
	return nil, nil
}

func (a *FetchGoogleUsersActivity) GetGoogleUserByEmail(
	ctx context.Context,
	email string,
) (*sharedModel.GoogleUserInfo, error) {
	user, err := a.AuthService.GetGoogleUserByEmail(ctx, email)
	if err != nil {
		return nil, errs.Wrap(errs.CodeAuthLookupFailed, "failed to get google user by email", err)
	}
	return user, nil
}

func (a *FetchGoogleUsersActivity) UpdateGmailHistoryID(
	ctx context.Context,
	input sharedModel.UpdateGmailHistoryInput,
) error {
	return nil
}

func (a *FetchGoogleUsersActivity) UpdateGmailWatchState(
	ctx context.Context,
	input []sharedModel.GoogleWatchUser,
) error {
	return nil
}
