package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// Hash returns the SHA256 hash of the given value.
// Use a more secure hashing algorithm for production.
func Hash(value string) string {
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:])
}
