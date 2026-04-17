package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mdhender/drynn/internal/email"
)

const (
	configVersion = 1
	configPathEnv = "DRYNN_CONFIG_PATH"
)

type Config struct {
	ConfigPath           string
	AppAddr              string
	DatabaseURL          string
	DataDir              string
	JWTAccessTTL         time.Duration
	JWTRefreshTTL        time.Duration
	CookieSecure         bool
	BaseURL              string
	Mailgun              email.MailgunConfig
	RequestAccessEnabled bool
	AdminContactEmail    string
}

type InitOptions struct {
	AppAddr              string
	DatabaseURL          string
	DataDir              string
	JWTAccessTTL         time.Duration
	JWTRefreshTTL        time.Duration
	CookieSecure         bool
	BaseURL              string
	Mailgun              email.MailgunConfig
	RequestAccessEnabled bool
	AdminContactEmail    string
	Force                bool
}

type fileConfig struct {
	Version              int               `json:"version"`
	AppAddr              string            `json:"app_addr"`
	DatabaseURL          string            `json:"database_url"`
	DataDir              string            `json:"data_dir"`
	JWTAccessTTL         string            `json:"jwt_access_ttl"`
	JWTRefreshTTL        string            `json:"jwt_refresh_ttl"`
	CookieSecure         bool              `json:"cookie_secure"`
	BaseURL              string            `json:"base_url"`
	Mailgun              fileMailgunConfig `json:"mailgun"`
	RequestAccessEnabled bool              `json:"request_access_enabled"`
	AdminContactEmail    string            `json:"admin_contact_email"`
}

type fileMailgunConfig struct {
	APIKey        string `json:"api_key"`
	SendingDomain string `json:"sending_domain"`
	FromAddress   string `json:"from_address"`
	FromName      string `json:"from_name"`
}

func DefaultPath() string {
	if value := strings.TrimSpace(os.Getenv(configPathEnv)); value != "" {
		return value
	}

	return filepath.Join("data", "var", "drynn", "server.json")
}

func DefaultDataDir() string {
	return filepath.Join("data", "var", "drynn", "data")
}

func Load() (Config, error) {
	return LoadPath(DefaultPath())
}

func LoadPath(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultPath()
	}

	cfg := Config{
		ConfigPath:    path,
		AppAddr:       ":8080",
		DataDir:       DefaultDataDir(),
		JWTAccessTTL:  15 * time.Minute,
		JWTRefreshTTL: 7 * 24 * time.Hour,
		CookieSecure:  false,
	}

	if err := mergeFileConfig(&cfg, path); err != nil {
		return Config{}, err
	}

	applyEnvOverrides(&cfg)

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required in %s or the environment", path)
	}
	if cfg.BaseURL == "" {
		return Config{}, fmt.Errorf("base_url is required in %s or DRYNN_BASE_URL in the environment", path)
	}

	return cfg, nil
}

func WritePath(path string, options InitOptions) (Config, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultPath()
	}

	databaseURL := strings.TrimSpace(options.DatabaseURL)
	if databaseURL == "" {
		return Config{}, fmt.Errorf("database URL is required")
	}

	baseURL := strings.TrimSpace(options.BaseURL)
	if baseURL == "" {
		return Config{}, fmt.Errorf("base URL is required")
	}

	cfg := fileConfig{
		Version:       configVersion,
		AppAddr:       defaultString(options.AppAddr, ":8080"),
		DatabaseURL:   databaseURL,
		DataDir:       defaultString(options.DataDir, DefaultDataDir()),
		JWTAccessTTL:  defaultDuration(options.JWTAccessTTL, 15*time.Minute).String(),
		JWTRefreshTTL: defaultDuration(options.JWTRefreshTTL, 7*24*time.Hour).String(),
		CookieSecure:  options.CookieSecure,
		BaseURL:       baseURL,
		Mailgun: fileMailgunConfig{
			APIKey:        strings.TrimSpace(options.Mailgun.APIKey),
			SendingDomain: strings.TrimSpace(options.Mailgun.SendingDomain),
			FromAddress:   strings.TrimSpace(options.Mailgun.FromAddress),
			FromName:      strings.TrimSpace(options.Mailgun.FromName),
		},
		RequestAccessEnabled: options.RequestAccessEnabled,
		AdminContactEmail:    strings.TrimSpace(options.AdminContactEmail),
	}

	if !options.Force {
		if _, err := os.Stat(path); err == nil {
			return Config{}, fmt.Errorf("config file already exists at %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("stat config file: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Config{}, fmt.Errorf("create config directory: %w", err)
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("create data directory: %w", err)
	}

	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return Config{}, fmt.Errorf("marshal config file: %w", err)
	}
	payload = append(payload, '\n')

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, payload, 0o600); err != nil {
		return Config{}, fmt.Errorf("write config file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return Config{}, fmt.Errorf("replace config file: %w", err)
	}

	return LoadPath(path)
}

func mergeFileConfig(cfg *Config, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("stat config file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("config path %s is a directory", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	var fileCfg fileConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fileCfg); err != nil {
		return fmt.Errorf("decode config file: %w", err)
	}
	if fileCfg.Version != 0 && fileCfg.Version != configVersion {
		return fmt.Errorf("unsupported config version %d", fileCfg.Version)
	}

	if fileCfg.AppAddr != "" {
		cfg.AppAddr = fileCfg.AppAddr
	}
	if fileCfg.DatabaseURL != "" {
		cfg.DatabaseURL = fileCfg.DatabaseURL
	}
	if fileCfg.DataDir != "" {
		cfg.DataDir = fileCfg.DataDir
	}
	if fileCfg.JWTAccessTTL != "" {
		duration, err := time.ParseDuration(fileCfg.JWTAccessTTL)
		if err != nil {
			return fmt.Errorf("parse jwt_access_ttl: %w", err)
		}
		cfg.JWTAccessTTL = duration
	}
	if fileCfg.JWTRefreshTTL != "" {
		duration, err := time.ParseDuration(fileCfg.JWTRefreshTTL)
		if err != nil {
			return fmt.Errorf("parse jwt_refresh_ttl: %w", err)
		}
		cfg.JWTRefreshTTL = duration
	}
	cfg.CookieSecure = fileCfg.CookieSecure
	if fileCfg.BaseURL != "" {
		cfg.BaseURL = fileCfg.BaseURL
	}

	if fileCfg.Mailgun.APIKey != "" {
		cfg.Mailgun.APIKey = fileCfg.Mailgun.APIKey
	}
	if fileCfg.Mailgun.SendingDomain != "" {
		cfg.Mailgun.SendingDomain = fileCfg.Mailgun.SendingDomain
	}
	if fileCfg.Mailgun.FromAddress != "" {
		cfg.Mailgun.FromAddress = fileCfg.Mailgun.FromAddress
	}
	if fileCfg.Mailgun.FromName != "" {
		cfg.Mailgun.FromName = fileCfg.Mailgun.FromName
	}

	cfg.RequestAccessEnabled = fileCfg.RequestAccessEnabled
	if fileCfg.AdminContactEmail != "" {
		cfg.AdminContactEmail = fileCfg.AdminContactEmail
	}

	return nil
}

func applyEnvOverrides(cfg *Config) {
	cfg.AppAddr = envOrDefault("DRYNN_APP_ADDR", cfg.AppAddr)
	cfg.DatabaseURL = envOrDefault("DRYNN_DATABASE_URL", cfg.DatabaseURL)
	cfg.DataDir = envOrDefault("DRYNN_DATA_DIR", cfg.DataDir)
	cfg.JWTAccessTTL = durationOrDefault("DRYNN_JWT_ACCESS_TTL", cfg.JWTAccessTTL)
	cfg.JWTRefreshTTL = durationOrDefault("DRYNN_JWT_REFRESH_TTL", cfg.JWTRefreshTTL)
	cfg.CookieSecure = boolOrDefault("DRYNN_COOKIE_SECURE", cfg.CookieSecure)
	cfg.BaseURL = envOrDefault("DRYNN_BASE_URL", cfg.BaseURL)
	cfg.Mailgun.APIKey = envOrDefault("DRYNN_MAILGUN_API_KEY", cfg.Mailgun.APIKey)
	cfg.Mailgun.SendingDomain = envOrDefault("DRYNN_MAILGUN_SENDING_DOMAIN", cfg.Mailgun.SendingDomain)
	cfg.Mailgun.FromAddress = envOrDefault("DRYNN_MAILGUN_FROM_ADDRESS", cfg.Mailgun.FromAddress)
	cfg.Mailgun.FromName = envOrDefault("DRYNN_MAILGUN_FROM_NAME", cfg.Mailgun.FromName)
	cfg.RequestAccessEnabled = boolOrDefault("DRYNN_REQUEST_ACCESS_ENABLED", cfg.RequestAccessEnabled)
	cfg.AdminContactEmail = envOrDefault("DRYNN_ADMIN_CONTACT_EMAIL", cfg.AdminContactEmail)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func boolOrDefault(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}

	return strings.TrimSpace(value)
}

func defaultDuration(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}

	return value
}

func durationOrDefault(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}
