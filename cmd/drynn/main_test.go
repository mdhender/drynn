package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v4"
)

// tempSessionHome redirects os.UserConfigDir() (used by sessionPath) to a
// temp directory for this test by overriding HOME and XDG_CONFIG_HOME.
func tempSessionHome(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
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

func preseedSession(t *testing.T, s sessionData) {
	t.Helper()
	if err := saveSession(s); err != nil {
		t.Fatalf("preseed session: %v", err)
	}
}

func TestDrynn_Login_SavesSession(t *testing.T) {
	tempSessionHome(t)

	var receivedBody []byte
	var receivedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"atok","refresh_token":"rtok"}`))
	}))
	t.Cleanup(srv.Close)

	_, _, err := captureOutput(t, func() error {
		return run([]string{
			"login",
			"--email", "user@example.com",
			"--password", "secret",
			"--server", srv.URL,
		})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedPath != "/api/v1/login" {
		t.Fatalf("expected /api/v1/login, got %q", receivedPath)
	}
	var got map[string]string
	if err := json.Unmarshal(receivedBody, &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got["email"] != "user@example.com" || got["password"] != "secret" {
		t.Fatalf("unexpected body: %+v", got)
	}

	s, err := loadSession()
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if s.ServerURL != srv.URL {
		t.Fatalf("expected server_url=%s, got %s", srv.URL, s.ServerURL)
	}
	if s.AccessToken != "atok" || s.RefreshToken != "rtok" {
		t.Fatalf("expected tokens atok/rtok, got %+v", s)
	}
}

func TestDrynn_Login_ConfigWithExistingSession(t *testing.T) {
	tempSessionHome(t)
	preseedSession(t, sessionData{
		ServerURL:   "http://old.example",
		AccessToken: "existing",
	})

	var requestSeen bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestSeen = true
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "server.json")
	if err := os.WriteFile(cfgPath, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, err := captureOutput(t, func() error {
		return run([]string{
			"login",
			"--email", "u@x.com",
			"--password", "p",
			"--server", srv.URL,
			"--config", cfgPath,
		})
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	want := "existing session found; run 'drynn logout' before using --config"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected %q in error, got %q", want, err.Error())
	}
	if requestSeen {
		t.Fatalf("expected no HTTP request to be made")
	}
}

func TestDrynn_Logout_ClearsTokens(t *testing.T) {
	tempSessionHome(t)
	preseedSession(t, sessionData{
		ServerURL:    "http://example",
		AccessToken:  "a",
		RefreshToken: "r",
	})

	_, _, err := captureOutput(t, func() error {
		return run([]string{"logout"})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s, err := loadSession()
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if s.AccessToken != "" || s.RefreshToken != "" {
		t.Fatalf("expected empty tokens, got %+v", s)
	}
	if s.ServerURL != "http://example" {
		t.Fatalf("expected server_url preserved, got %q", s.ServerURL)
	}
}

func TestDrynn_Health_ServerURL(t *testing.T) {
	tempSessionHome(t)

	var seenPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","version":"x"}`))
	}))
	t.Cleanup(srv.Close)

	stdout, _, err := captureOutput(t, func() error {
		return run([]string{"health", "--server", srv.URL})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seenPath != "/api/v1/health" {
		t.Fatalf("expected /api/v1/health, got %q", seenPath)
	}
	if !strings.Contains(stdout, "status=ok") {
		t.Fatalf("expected status=ok in output, got %q", stdout)
	}
}

func TestDrynn_Version_NoSideEffects(t *testing.T) {
	tempSessionHome(t)

	stdout, _, err := captureOutput(t, func() error {
		return run([]string{"version"})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(stdout) == "" {
		t.Fatalf("expected non-empty version output")
	}

	path, err := sessionPath()
	if err != nil {
		t.Fatalf("session path: %v", err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected no session file at %s", path)
	}
}

func TestDrynn_UnknownCommand(t *testing.T) {
	tempSessionHome(t)

	_, _, err := captureOutput(t, func() error {
		return run([]string{"frobnicate"})
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("expected 'unknown command' in error, got %q", err.Error())
	}
}

func TestDrynn_Help(t *testing.T) {
	tempSessionHome(t)

	cases := [][]string{
		{"help"},
		{"--help"},
	}
	for _, args := range cases {
		args := args
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, stderr, err := captureOutput(t, func() error {
				return run(args)
			})
			if !errors.Is(err, ff.ErrHelp) {
				t.Fatalf("expected ff.ErrHelp, got %v", err)
			}
			for _, want := range []string{"login", "logout", "health", "version"} {
				if !strings.Contains(stderr, want) {
					t.Fatalf("expected root usage to mention %q; got %q", want, stderr)
				}
			}
		})
	}
}
