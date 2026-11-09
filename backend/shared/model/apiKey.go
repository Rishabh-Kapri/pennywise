package model

import (
	"encoding/base64"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// API key format: <prefix>_<version>_<random>, e.g. "pwk_v1_<43 base64url chars>".
// go-pennywise-api generates keys in this format and the auth adapter checks
// it, so both read it from here.
const (
	APIKeyPrefix  = "pwk" // PennyWise Key
	APIKeyVersion = "v1"
	APIKeyLength  = 32 // random bytes, before base64url encoding

	// APIKeyIDPrefix and APIKeyIDLength shape the public key id ("k_" + 12
	// hex chars). The id comes from its own random bytes, never from the
	// secret, so it is safe to show, log and look keys up by.
	APIKeyIDPrefix = "k_"
	APIKeyIDLength = 6
)

// ParseAPIKey splits a key into its three parts. ok is false when the key
// doesn't have three underscore-separated parts.
func ParseAPIKey(fullKey string) (prefix, version, random string, ok bool) {
	parts := strings.SplitN(fullKey, "_", 3)
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

// ValidAPIKeyFormat reports whether fullKey has the shape of a key this
// system issues. It doesn't say the key exists.
func ValidAPIKeyFormat(fullKey string) bool {
	prefix, version, random, ok := ParseAPIKey(fullKey)
	return ok &&
		prefix == APIKeyPrefix &&
		version == APIKeyVersion &&
		len(random) == base64.RawURLEncoding.EncodedLen(APIKeyLength)
}

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
	MaskedKey   *string   `json:"maskedKey,omitempty"`
	HashedKey   string    `json:"-"` // dont'expose HashedKey in JSON
	Name        string    `json:"name"`
	Description string    `json:"description"`

	// Ownership
	UserID uuid.UUID `json:"userId"`

	// Permissions
	Scopes []Scope `json:"scopes"`

	// Restrictions
	AllowedIPs      []string `json:"allowedIps"`
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
