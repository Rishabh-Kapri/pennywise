package migrations

import (
	"math"
	"testing"

	"github.com/pressly/goose/v3"
)

// Two migrations sharing a goose version make goose panic while collecting
// them, which blocks *every* pending migration rather than just the clashing
// pair. That is easy to introduce by merging two branches that each added the
// next sequential number, and the failure shows up far from the cause -- as a
// missing column at runtime.
func TestMigrationVersionsAreUnique(t *testing.T) {
	goose.SetBaseFS(EmbedMigrations)
	t.Cleanup(func() { goose.SetBaseFS(nil) })

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("duplicate migration version: %v", r)
		}
	}()

	collected, err := goose.CollectMigrations(".", 0, math.MaxInt64)
	if err != nil {
		t.Fatalf("collect migrations: %v", err)
	}
	if len(collected) == 0 {
		t.Fatal("no migrations collected; embed pattern is probably wrong")
	}

	seen := make(map[int64]string, len(collected))
	for _, m := range collected {
		if prev, dup := seen[m.Version]; dup {
			t.Errorf("version %d used by both %s and %s", m.Version, prev, m.Source)
		}
		seen[m.Version] = m.Source
	}
}
