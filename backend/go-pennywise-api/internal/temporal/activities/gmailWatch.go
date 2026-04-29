package temporal

import (
	"context"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/service"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

type FetchGoogleUsersActivity struct {
	AuthService service.AuthService
}

func (a *FetchGoogleUsersActivity) ListGoogleUsersNeedingWatchRefresh(
	ctx context.Context,
) ([]sharedModel.GoogleWatchUser, error) {
	users, err := a.AuthService.GetAllGoogleUsers(ctx)
	if err != nil {
		return nil, errs.Wrap(errs.CodeAuthLookupFailed, "failed to get all google users", err)
	}
	refreshBefore := time.Now().Add(time.Hour * 24 * 2).UnixMilli()
	var watchUsers []sharedModel.GoogleWatchUser
	for _, user := range users {
		if user.GmailHistoryID == nil {
			continue
		}
		if user.ExpiryAt == nil || *user.ExpiryAt < refreshBefore {
			watchUsers = append(watchUsers, sharedModel.GoogleWatchUser{
				ID:             user.ID,
				Email:          user.Email,
				GmailHistoryID: *user.GmailHistoryID,
				RefreshToken:   user.RefreshToken,
			})
		}
	}
	return watchUsers, nil
}

func (a *FetchGoogleUsersActivity) UpdateGmailWatchState(
	ctx context.Context,
	input []sharedModel.GoogleWatchUser,
) error {
	for _, user := range input {
		err := a.AuthService.UpdateGmailHistoryID(ctx, user.Email, user.GmailHistoryID, user.ExpiryAt)
		if err != nil {
			return err
		}
	}
	return nil
}
