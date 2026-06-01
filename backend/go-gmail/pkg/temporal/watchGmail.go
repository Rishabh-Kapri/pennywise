package temporal

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/gmail"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

type WatchGmailActivity struct {
	Gmail *gmail.Service
}

func (a *WatchGmailActivity) GmailWatchCall(
	ctx context.Context,
	input []model.GoogleWatchUser,
) ([]model.GoogleWatchUser, error) {
	var updatedUsers []model.GoogleWatchUser
	for _, user := range input {
		syncReq := gmail.GmailSyncRequest{
			RefreshToken:    user.RefreshToken,
			OAuthClientType: user.OAuthClientType,
			Email:           user.Email,
		}
		historyId, expiration, err := a.Gmail.WatchHandler(ctx, syncReq)
		if err != nil {
			return nil, err
		}
		updatedUsers = append(updatedUsers, model.GoogleWatchUser{
			ID:              user.ID,
			OAuthClientType: user.OAuthClientType,
			Email:           user.Email,
			GmailHistoryID:  historyId,
			RefreshToken:    user.RefreshToken,
			ExpiryAt:        &expiration,
		})
	}

	return updatedUsers, nil
}
