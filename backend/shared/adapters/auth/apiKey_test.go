package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	authPort "github.com/Rishabh-Kapri/pennywise/backend/shared/ports/auth"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

// validKey has the right shape: pwk_v1_ + 43 base64url chars (32 bytes).
var validKey = "pwk_v1_" + strings.Repeat("A", 43)

type fakeKeyRepo struct {
	repository.APIKeyRepository                          // unused methods panic
	keys                        map[string]*model.APIKey // by hash
	err                         error
	touched                     chan uuid.UUID
}

func (f *fakeKeyRepo) GetByHash(_ context.Context, hash string) (*model.APIKey, error) {
	if f.err != nil {
		return nil, f.err
	}
	if k, ok := f.keys[hash]; ok {
		return k, nil
	}
	return nil, repository.ErrNotFound
}

func (f *fakeKeyRepo) UpdateLastUsed(_ context.Context, id uuid.UUID) error {
	f.touched <- id
	return nil
}

func TestAPIKeyAuthenticate(t *testing.T) {
	userID := uuid.New()
	active := &model.APIKey{ID: uuid.New(), KeyID: "k_1", UserID: userID, IsActive: true,
		Scopes: []model.Scope{model.ScopeWrite}}
	revokedAt := time.Now().Add(-time.Hour)
	revoked := &model.APIKey{ID: uuid.New(), KeyID: "k_2", UserID: userID, IsActive: true, RevokedAt: &revokedAt}
	revokedKey := "pwk_v1_" + strings.Repeat("B", 43)

	newRepo := func() *fakeKeyRepo {
		return &fakeKeyRepo{
			keys:    map[string]*model.APIKey{utils.Hash(validKey): active, utils.Hash(revokedKey): revoked},
			touched: make(chan uuid.UUID, 1),
		}
	}

	t.Run("no key is not mine", func(t *testing.T) {
		p, err := NewAPIKeyAdapter(newRepo()).Authenticate(context.Background(), authPort.Credentials{BearerToken: "x"})
		if p != nil || err != nil {
			t.Fatalf("got (%v, %v), want (nil, nil)", p, err)
		}
	})

	for name, key := range map[string]string{
		"malformed":    "not-a-key",
		"wrong prefix": "abc_v1_" + strings.Repeat("A", 43),
		"unknown":      "pwk_v1_" + strings.Repeat("C", 43),
		"revoked":      revokedKey,
	} {
		t.Run(name+" is rejected", func(t *testing.T) {
			p, err := NewAPIKeyAdapter(newRepo()).Authenticate(context.Background(), authPort.Credentials{APIKey: key})
			if p != nil || !errs.HasCode(err, errs.CodeAuthInvalidCredentials) {
				t.Fatalf("got (%v, %v), want invalid credentials", p, err)
			}
			if err != nil && strings.Contains(err.Error(), key) {
				t.Fatal("error leaks the key")
			}
		})
	}

	t.Run("lookup failure is not a bad key", func(t *testing.T) {
		repo := newRepo()
		repo.err = errors.New("db down")
		_, err := NewAPIKeyAdapter(repo).Authenticate(context.Background(), authPort.Credentials{APIKey: validKey})
		if !errs.HasCode(err, errs.CodeAuthLookupFailed) {
			t.Fatalf("err = %v, want lookup failed", err)
		}
	})

	t.Run("valid key gives principal and touches last_used", func(t *testing.T) {
		repo := newRepo()
		ctx, cancel := context.WithCancel(context.Background())
		p, err := NewAPIKeyAdapter(repo).Authenticate(ctx, authPort.Credentials{APIKey: validKey})
		cancel() // request finished: the last_used write must still happen
		if err != nil {
			t.Fatal(err)
		}
		if p.Kind != authPort.APIKeyKind || p.UserID == nil || *p.UserID != userID || len(p.Scopes) != 1 {
			t.Fatalf("principal = %+v", p)
		}
		select {
		case id := <-repo.touched:
			if id != active.ID {
				t.Fatalf("touched %v, want %v", id, active.ID)
			}
		case <-time.After(time.Second):
			t.Fatal("last_used not updated")
		}
	})
}
