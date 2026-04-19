package model

import (
	"slices"
	"time"

	"github.com/google/uuid"
)

// Scope represents permission scope for the API Key
type Scope string

const (
	ScopeRead       Scope = "read"
	ScopeWrite      Scope = "write"
	ScopeDelete     Scope = "delete"
	ScopeAdmin      Scope = "admin"
	ScopeMonitoring Scope = "monitoring"
	ScopeAlerts     Scope = "alerts"
)

type APIKey struct {
	ID          uuid.UUID `json:"id"`
	KeyID       string    `json:"keyId"` // short identifier for lookup
	HashedKey   string    `json:"-"`     // dont'expose HashedKey in JSON
	Name        string    `json:"name"`
	Description string    `json:"description"`

	// Ownership
	UserID uuid.UUID `json:"userId"`

	// Permissions
	Scopes []Scope `json:"scopes"`

	// Restrictions
	AllowedIPs       []string `json:"allowedIps"`
	AllowedReferers []string `json:"allowedReferers"`

	// Rate Limiting
	RateLimit int `json:"rateLimit"` // requests per minute

	// Lifecycle
	CreatedAt  *time.Time `json:"createdAt"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	RevokedAt  *time.Time `json:"revokedAt"`

	// Rotation
	RotationEnabled *bool      `json:"rotationEnabled,omitempty"`
	RotatedFromID   *uuid.UUID `json:"rotatedFromId,omitempty"`
	RotationDueAt   *time.Time `json:"rotationDueAt,omitempty"`

	// Status
	IsActive bool `json:"isActive"`
}

// Checks whether the API Key has the specific scope
// Returns true for "admin" scope
func (k *APIKey) HasScope(scope Scope) bool {
	for _, s := range k.Scopes {
		if s == ScopeAdmin || s == scope {
			return true
		}
	}
	return false
}

// Checks whether the API Key has any of the scopes
func (k *APIKey) CheckScope(scopes ...Scope) bool {
	return slices.ContainsFunc(scopes, k.HasScope)
}

// Check where API Key is valid
func (k *APIKey) IsValid() bool {
	if !k.IsActive {
		return false
	}

	if k.RevokedAt != nil {
		return false
	}

	if k.ExpiresAt != nil && time.Now().After(*k.ExpiresAt) {
		return false
	}

	return true
}

func (k *APIKey) NeedRotation() bool {
	if k.RotationDueAt == nil {
		return false
	}

	return time.Now().After(*k.RotationDueAt)
}
