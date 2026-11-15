package worker

import (
	"context"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

func TestOpenTypedPassword(t *testing.T) {
	box, err := utils.NewSecretBox("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	if err != nil {
		t.Fatal(err)
	}
	other, err := utils.NewSecretBox("ZmVkY2JhOTg3NjU0MzIxMGZlZGNiYTk4NzY1NDMyMTA=")
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.Seal("hunter2")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	tests := []struct {
		name string
		deps WorkerDeps
		in   model.StatementInput
		want string
	}{
		{"sealed", WorkerDeps{SecretBox: box}, model.StatementInput{SealedPassword: sealed}, "hunter2"},
		{"plain, from an older job", WorkerDeps{SecretBox: box}, model.StatementInput{Password: "old"}, "old"},
		{"none", WorkerDeps{SecretBox: box}, model.StatementInput{}, ""},
		{"no key on cipher", WorkerDeps{}, model.StatementInput{SealedPassword: sealed}, ""},
		{"different key", WorkerDeps{SecretBox: other}, model.StatementInput{SealedPassword: sealed}, ""},
	}
	for _, tt := range tests {
		if got := openTypedPassword(ctx, tt.deps, tt.in); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
