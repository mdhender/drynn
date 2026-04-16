// Package db exposes filesystem resources that live alongside the
// database schema and are embedded into binaries at build time.
//
// The primary consumer is the test harness in internal/testdb, which
// applies [Migrations] to a fresh Postgres instance without depending on
// the process working directory or a checked-out repo layout.
package db

import "embed"

// Migrations contains the Atlas-managed SQL migration files under
// db/migrations. Files are named with a sortable timestamp prefix, so
// lexicographic ordering matches the order Atlas applies them.
//
//go:embed migrations/*.sql
var Migrations embed.FS
