// Package sql embeds the database's SQL so any service binary can apply it.
// Migrations are goose files; see README.md in this directory for how to write one.
package sql

import "embed"

// MigrationsDir is the directory inside Migrations that holds the goose files.
const MigrationsDir = "migrations"

//go:embed migrations/*.sql
var Migrations embed.FS
