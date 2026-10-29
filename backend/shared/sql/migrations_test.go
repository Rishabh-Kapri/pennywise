package sql

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var migrationName = regexp.MustCompile(`^(\d{5})_[a-z0-9]+(_[a-z0-9]+)*\.sql$`)

func migrationFiles(t *testing.T) []string {
	t.Helper()
	entries, err := Migrations.ReadDir(MigrationsDir)
	if err != nil {
		t.Fatalf("read %s: %v", MigrationsDir, err)
	}
	if len(entries) == 0 {
		t.Fatal("no migrations embedded; embed pattern is probably wrong")
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names // ReadDir returns entries sorted by filename
}

// Two migrations sharing a goose version make goose panic while collecting
// them, which blocks *every* pending migration rather than just the clashing
// pair. That is easy to introduce by merging two branches that each added the
// next sequential number, and the failure shows up far from the cause -- as a
// missing column at runtime. Requiring 1..N with no gaps also catches that, and
// a number skipped by a bad rebase.
func TestMigrationVersionsAreSequential(t *testing.T) {
	for i, name := range migrationFiles(t) {
		m := migrationName.FindStringSubmatch(name)
		if m == nil {
			t.Errorf("%s: want NNNNN_snake_case_name.sql", name)
			continue
		}
		version, _ := strconv.Atoi(m[1])
		if want := i + 1; version != want {
			t.Errorf("%s: version %d, want %d (versions must run 1..N with no gaps or duplicates; "+
				"renumber yours to the end after rebasing)", name, version, want)
		}
	}
}

func TestMigrationsHaveUpAndDown(t *testing.T) {
	for _, name := range migrationFiles(t) {
		body, err := Migrations.ReadFile(MigrationsDir + "/" + name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		up := strings.Index(string(body), "-- +goose Up")
		down := strings.Index(string(body), "-- +goose Down")
		if up < 0 || down < 0 || up > down {
			t.Errorf("%s: want a '-- +goose Up' section followed by a '-- +goose Down' section", name)
		}
	}
}
