package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mdhender/drynn/internal/config"
)

type sessionData struct {
	ServerURL    string `json:"server_url"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func sessionPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	return filepath.Join(dir, "drynn", "drynn.json"), nil
}

func loadSession() (sessionData, error) {
	path, err := sessionPath()
	if err != nil {
		return sessionData{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return sessionData{}, nil
		}
		return sessionData{}, fmt.Errorf("read session: %w", err)
	}

	var s sessionData
	if err := json.Unmarshal(data, &s); err != nil {
		return sessionData{}, fmt.Errorf("decode session: %w", err)
	}
	return s, nil
}

func saveSession(s sessionData) error {
	path, err := sessionPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	payload, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	payload = append(payload, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return fmt.Errorf("write session: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace session: %w", err)
	}
	return nil
}

func clearTokens() error {
	s, err := loadSession()
	if err != nil {
		return err
	}
	s.AccessToken = ""
	s.RefreshToken = ""
	return saveSession(s)
}

// resolveServerURL returns the server URL using the precedence:
// flag > env (DRYNN_SERVER_URL) > config file base_url > existing session.
func resolveServerURL(serverFlag, configPath string, existing sessionData) (string, error) {
	if serverFlag != "" {
		return strings.TrimRight(serverFlag, "/"), nil
	}

	if v := os.Getenv("DRYNN_SERVER_URL"); v != "" {
		return strings.TrimRight(v, "/"), nil
	}

	if configPath != "" {
		cfg, err := config.LoadPath(configPath)
		if err != nil {
			return "", fmt.Errorf("load config: %w", err)
		}
		if cfg.BaseURL != "" {
			return strings.TrimRight(cfg.BaseURL, "/"), nil
		}
	}

	if existing.ServerURL != "" {
		return existing.ServerURL, nil
	}

	return "", fmt.Errorf("server URL is required: use --server, DRYNN_SERVER_URL, or --config")
}
