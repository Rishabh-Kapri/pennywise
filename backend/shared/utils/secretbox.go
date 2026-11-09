package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// SecretBox seals small secrets (user API keys) with AES-256-GCM. The API
// seals and cipher opens, so both must load the same CREDENTIALS_KEY.
type SecretBox struct {
	aead cipher.AEAD
}

// ErrSecretBoxKeyMissing is returned when CREDENTIALS_KEY is unset.
var ErrSecretBoxKeyMissing = errors.New("CREDENTIALS_KEY is not set")

// NewSecretBox takes a base64-encoded 32-byte key
// (generate one with `openssl rand -base64 32`).
func NewSecretBox(base64Key string) (*SecretBox, error) {
	if base64Key == "" {
		return nil, ErrSecretBoxKeyMissing
	}
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("decoding CREDENTIALS_KEY: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("CREDENTIALS_KEY must decode to 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &SecretBox{aead: aead}, nil
}

// Seal returns nonce || ciphertext.
func (b *SecretBox) Seal(plaintext string) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return b.aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// Open reverses Seal.
func (b *SecretBox) Open(sealed []byte) (string, error) {
	n := b.aead.NonceSize()
	if len(sealed) < n {
		return "", errors.New("sealed secret is too short")
	}
	plaintext, err := b.aead.Open(nil, sealed[:n], sealed[n:], nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
