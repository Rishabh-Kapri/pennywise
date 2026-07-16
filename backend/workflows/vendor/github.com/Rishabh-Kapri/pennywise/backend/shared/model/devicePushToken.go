package model

import (
	"time"

	"github.com/google/uuid"
)

// DevicePushToken is an Expo push token registered by a mobile device.
type DevicePushToken struct {
	ID            uuid.UUID `json:"id"`
	AuthUserID    uuid.UUID `json:"authUserId"`
	ExpoPushToken string    `json:"expoPushToken"`
	Platform      string    `json:"platform"`
	CreatedAt     time.Time `json:"createdAt"`
	LastSeenAt    time.Time `json:"lastSeenAt"`
}

// DevicePushTokenReq is the register/unregister payload.
type DevicePushTokenReq struct {
	ExpoPushToken string `json:"expoPushToken"`
	Platform      string `json:"platform"`
}
