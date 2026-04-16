// Package testdb provides a Postgres-backed test harness.
//
// A single Postgres container is started lazily for the test binary and
// reused across tests. Every call to [New] returns an isolated database
// cloned from a freshly migrated template, so tests see one another's
// schema but never one another's data.
//
// Tests that use this package require a working Docker daemon and are
// skipped in -short mode so `go test -short ./...` stays hermetic.
package testdb

import (
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/mdhender/drynn/db"
)

const (
	postgresImage = "postgres:16-alpine"
	adminUser     = "postgres"
	adminPass     = "postgres"
	adminDB       = "postgres"
	templateDB    = "drynn_template"
)

type shared struct {
	adminDSN  string
	adminPool *pgxpool.Pool
}

var (
	sharedOnce sync.Once
	sharedVal  *shared
	sharedErr  error
	dbCounter  atomic.Uint64
)

// New returns a pgxpool.Pool connected to a fresh, migrated database.
//
// The per-test database and its connection pool are closed automatically
// when the test finishes. The shared container and template database are
// cleaned up by the testcontainers reaper when the test binary exits.
//
// New calls t.Skip in -short mode.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("testdb: skipped in -short mode (requires Docker)")
	}

	s, err := ensureShared()
	if err != nil {
		t.Fatalf("testdb: shared container: %v", err)
	}

	ctx := context.Background()
	name := nextDBName()

	if _, err := s.adminPool.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %s TEMPLATE %s`, quoteIdent(name), quoteIdent(templateDB))); err != nil {
		t.Fatalf("testdb: create database %q: %v", name, err)
	}

	dsn, err := replaceDBName(s.adminDSN, name)
	if err != nil {
		dropDatabase(s.adminPool, name)
		t.Fatalf("testdb: rewrite dsn: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		dropDatabase(s.adminPool, name)
		t.Fatalf("testdb: connect to %q: %v", name, err)
	}

	t.Cleanup(func() {
		pool.Close()
		dropDatabase(s.adminPool, name)
	})

	return pool
}

func ensureShared() (*shared, error) {
	sharedOnce.Do(func() {
		sharedVal, sharedErr = startShared()
	})
	return sharedVal, sharedErr
}

func startShared() (*shared, error) {
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx, postgresImage,
		tcpostgres.WithDatabase(adminDB),
		tcpostgres.WithUsername(adminUser),
		tcpostgres.WithPassword(adminPass),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}

	adminDSN, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("read connection string: %w", err)
	}

	adminPool, err := pgxpool.New(ctx, adminDSN)
	if err != nil {
		return nil, fmt.Errorf("connect admin pool: %w", err)
	}

	if _, err := adminPool.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %s`, quoteIdent(templateDB))); err != nil {
		adminPool.Close()
		return nil, fmt.Errorf("create template database: %w", err)
	}

	templateDSN, err := replaceDBName(adminDSN, templateDB)
	if err != nil {
		adminPool.Close()
		return nil, fmt.Errorf("rewrite template dsn: %w", err)
	}
	if err := applyMigrations(ctx, templateDSN); err != nil {
		adminPool.Close()
		return nil, fmt.Errorf("migrate template: %w", err)
	}

	return &shared{adminDSN: adminDSN, adminPool: adminPool}, nil
}

func applyMigrations(ctx context.Context, dsn string) error {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect template: %w", err)
	}
	defer pool.Close()

	entries, err := fs.ReadDir(db.Migrations, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	if len(files) == 0 {
		return fmt.Errorf("no migration files embedded")
	}
	sort.Strings(files)

	for _, name := range files {
		content, err := fs.ReadFile(db.Migrations, "migrations/"+name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
	}
	return nil
}

func dropDatabase(pool *pgxpool.Pool, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = pool.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %s WITH (FORCE)`, quoteIdent(name)))
}

func nextDBName() string {
	return fmt.Sprintf("drynn_test_%d", dbCounter.Add(1))
}

func replaceDBName(dsn, name string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse dsn: %w", err)
	}
	u.Path = "/" + name
	return u.String(), nil
}

// quoteIdent quotes a SQL identifier for safe interpolation. Database and
// template names here are always generated internally, but we double-quote
// them anyway to keep the intent visible and handle any future callers.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
