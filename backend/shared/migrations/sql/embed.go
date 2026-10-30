// Package sql embeds the Postgres migrations so any service binary can apply them.
// Other databases get a sibling package under shared/migrations (e.g. mongodb).
// Migrations are goose files; see README.md in this directory for how to write one.
package sql

import "embed"

// MigrationsDir is the directory inside Migrations that holds the goose files:
// they sit next to this file.
const MigrationsDir = "."

//go:embed *.sql
var Migrations embed.FS
