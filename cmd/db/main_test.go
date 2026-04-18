package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peterbourgon/ff/v4"

	"github.com/mdhender/drynn/internal/config"
)

// failOnOpenDatabase installs an openDatabaseFn that fails the test if
// called. Used by every test that expects the selected command to not
// touch the database.
func failOnOpenDatabase(t *testing.T) {
	t.Helper()
	prev := openDatabaseFn
	openDatabaseFn = func(ctx context.Context, configPath string) (config.Config, *pgxpool.Pool, error) {
		t.Errorf("openDatabase should not be called; configPath=%q", configPath)
		return config.Config{}, nil, fmt.Errorf("unexpected openDatabase call")
	}
	t.Cleanup(func() { openDatabaseFn = prev })
}

// captureOutput redirects os.Stdout and os.Stderr while fn runs.
func captureOutput(t *testing.T, fn func() error) (string, string, error) {
	t.Helper()
	origStdout, origStderr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stdout, os.Stderr = outW, errW

	runErr := fn()

	_ = outW.Close()
	_ = errW.Close()
	os.Stdout, os.Stderr = origStdout, origStderr

	outBytes, _ := io.ReadAll(outR)
	errBytes, _ := io.ReadAll(errR)
	return string(outBytes), string(errBytes), runErr
}

func TestDB_Version_NoDB(t *testing.T) {
	failOnOpenDatabase(t)

	stdout, _, err := captureOutput(t, func() error {
		return run(context.Background(), []string{"version"})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(stdout) == "" {
		t.Fatalf("expected non-empty version output, got %q", stdout)
	}
}

func TestDB_InitConfig_NoDB(t *testing.T) {
	failOnOpenDatabase(t)

	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "server.json")
	dataDir := filepath.Join(tmp, "data")

	_, _, err := captureOutput(t, func() error {
		return run(context.Background(), []string{
			"init-config",
			"--config", configPath,
			"--database-url", "postgres://example",
			"--base-url", "http://example",
			"--data-dir", dataDir,
		})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(configPath); statErr != nil {
		t.Fatalf("expected config written at %s: %v", configPath, statErr)
	}
}

func TestDB_JWTKey_NoSubcommand(t *testing.T) {
	failOnOpenDatabase(t)

	_, stderr, err := captureOutput(t, func() error {
		return run(context.Background(), []string{"jwt-key"})
	})
	if !errors.Is(err, ff.ErrHelp) {
		t.Fatalf("expected ff.ErrHelp, got %v", err)
	}
	if !strings.Contains(stderr, "jwt-key") {
		t.Fatalf("expected jwt-key usage in stderr, got %q", stderr)
	}
}

func TestDB_JWTKey_UnknownSubcommand(t *testing.T) {
	failOnOpenDatabase(t)

	_, _, err := captureOutput(t, func() error {
		return run(context.Background(), []string{"jwt-key", "wat"})
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `unknown jwt-key command "wat"`) {
		t.Fatalf("expected 'unknown jwt-key command \"wat\"' in error, got %q", err.Error())
	}
}

func TestDB_UnknownCommand(t *testing.T) {
	failOnOpenDatabase(t)

	_, _, err := captureOutput(t, func() error {
		return run(context.Background(), []string{"frobnicate"})
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `unknown command "frobnicate"`) {
		t.Fatalf("expected 'unknown command \"frobnicate\"' in error, got %q", err.Error())
	}
}

func TestDB_Help(t *testing.T) {
	failOnOpenDatabase(t)

	cases := [][]string{
		{"help"},
		{"--help"},
	}
	for _, args := range cases {
		args := args
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, stderr, err := captureOutput(t, func() error {
				return run(context.Background(), args)
			})
			if !errors.Is(err, ff.ErrHelp) {
				t.Fatalf("expected ff.ErrHelp, got %v", err)
			}
			// Root usage mentions each top-level subcommand.
			for _, want := range []string{"init-config", "seed-admin", "jwt-key", "version"} {
				if !strings.Contains(stderr, want) {
					t.Fatalf("expected root usage to mention %q; got %q", want, stderr)
				}
			}
		})
	}
}

func TestDB_LeafHelp(t *testing.T) {
	failOnOpenDatabase(t)

	_, stderr, err := captureOutput(t, func() error {
		return run(context.Background(), []string{"seed-admin", "--help"})
	})
	if !errors.Is(err, ff.ErrHelp) {
		t.Fatalf("expected ff.ErrHelp, got %v", err)
	}
	if !strings.Contains(stderr, "seed-admin") {
		t.Fatalf("expected seed-admin usage in stderr, got %q", stderr)
	}
}
