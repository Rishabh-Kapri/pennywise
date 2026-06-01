package db

import (
	"strings"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

func TestBuildUpdateConversationSQLUsesDollarPlaceholders(t *testing.T) {
	title := "Ways I Can Help"
	sql, args, hasUpdates, err := buildUpdateConversationSQL(uuid.New(), model.AgentConversation{
		Title: &title,
	})
	if err != nil {
		t.Fatalf("buildUpdateConversationSQL returned error: %v", err)
	}
	if !hasUpdates {
		t.Fatal("buildUpdateConversationSQL reported no updates")
	}
	if strings.Contains(sql, "?") {
		t.Fatalf("sql contains unsupported postgres placeholder: %s", sql)
	}
	if !strings.Contains(sql, "title = $1") {
		t.Fatalf("sql does not use dollar placeholder for title: %s", sql)
	}
	if !strings.Contains(sql, "id = $2") {
		t.Fatalf("sql does not use dollar placeholder for id: %s", sql)
	}
	if !strings.Contains(sql, "deleted IS NULL") {
		t.Fatalf("sql does not filter active conversations: %s", sql)
	}
	if got, want := len(args), 2; got != want {
		t.Fatalf("arg count = %d, want %d", got, want)
	}
	if got, want := args[0], title; got != want {
		t.Fatalf("first arg = %v, want %v", got, want)
	}
}

func TestBuildUpdateConversationSQLNoUpdates(t *testing.T) {
	sql, args, hasUpdates, err := buildUpdateConversationSQL(uuid.New(), model.AgentConversation{})
	if err != nil {
		t.Fatalf("buildUpdateConversationSQL returned error: %v", err)
	}
	if hasUpdates {
		t.Fatal("buildUpdateConversationSQL reported updates")
	}
	if sql != "" {
		t.Fatalf("sql = %q, want empty", sql)
	}
	if args != nil {
		t.Fatalf("args = %#v, want nil", args)
	}
}
