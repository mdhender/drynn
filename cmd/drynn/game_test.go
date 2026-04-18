package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGameCreate_PostsConfigFileVerbatimWithBearerToken(t *testing.T) {
	tempSessionHome(t)

	var (
		seenMethod string
		seenPath   string
		seenAuth   string
		seenCType  string
		seenBody   []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.Path
		seenAuth = r.Header.Get("Authorization")
		seenCType = r.Header.Get("Content-Type")
		var err error
		seenBody, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":42}`))
	}))
	t.Cleanup(srv.Close)

	preseedSession(t, sessionData{
		ServerURL:   srv.URL,
		AccessToken: "atok",
	})

	tmp := t.TempDir()
	cfgFile := filepath.Join(tmp, "game.json")
	cfgBody := []byte(`{"name":"Alpha","seed":123}`)
	if err := os.WriteFile(cfgFile, cfgBody, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	stdout, _, err := captureOutput(t, func() error {
		return run([]string{"game", "create", "--file", cfgFile})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if seenMethod != http.MethodPost {
		t.Fatalf("expected POST, got %q", seenMethod)
	}
	if seenPath != "/api/v1/games" {
		t.Fatalf("expected /api/v1/games, got %q", seenPath)
	}
	if seenAuth != "Bearer atok" {
		t.Fatalf("expected Bearer atok, got %q", seenAuth)
	}
	if seenCType != "application/json" {
		t.Fatalf("expected application/json, got %q", seenCType)
	}
	if !strings.Contains(string(seenBody), `"seed":123`) {
		t.Fatalf("expected body to contain \"seed\":123, got %q", string(seenBody))
	}
	if !strings.Contains(string(seenBody), `"name":"Alpha"`) {
		t.Fatalf("expected body to contain \"name\":\"Alpha\", got %q", string(seenBody))
	}
	if !strings.Contains(stdout, `"id":42`) {
		t.Fatalf("expected stdout to contain \"id\":42, got %q", stdout)
	}
}

func TestRunGameCreate_RequiresFile(t *testing.T) {
	tempSessionHome(t)
	preseedSession(t, sessionData{
		ServerURL:   "http://example",
		AccessToken: "atok",
	})

	_, _, err := captureOutput(t, func() error {
		return run([]string{"game", "create"})
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "--file") {
		t.Fatalf("expected error to mention --file, got %q", err.Error())
	}
}

func TestRunGameCreate_InvalidJSONFile(t *testing.T) {
	tempSessionHome(t)

	var requestSeen bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestSeen = true
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)

	preseedSession(t, sessionData{
		ServerURL:   srv.URL,
		AccessToken: "atok",
	})

	tmp := t.TempDir()
	cfgFile := filepath.Join(tmp, "bad.json")
	if err := os.WriteFile(cfgFile, []byte(`{bad json`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, err := captureOutput(t, func() error {
		return run([]string{"game", "create", "--file", cfgFile})
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "valid JSON") {
		t.Fatalf("expected error to mention valid JSON, got %q", err.Error())
	}
	if requestSeen {
		t.Fatalf("expected no HTTP request to be made")
	}
}

func TestRunGameCreate_RequiresLogin(t *testing.T) {
	tempSessionHome(t)
	preseedSession(t, sessionData{
		ServerURL:   "http://example",
		AccessToken: "",
	})

	tmp := t.TempDir()
	cfgFile := filepath.Join(tmp, "game.json")
	if err := os.WriteFile(cfgFile, []byte(`{"name":"Alpha"}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, err := captureOutput(t, func() error {
		return run([]string{"game", "create", "--file", cfgFile})
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not logged in") {
		t.Fatalf("expected error to contain 'not logged in', got %q", err.Error())
	}
}
