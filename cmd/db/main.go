package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	drynn "github.com/mdhender/drynn"
	"github.com/mdhender/drynn/internal/auth"
	"github.com/mdhender/drynn/internal/config"
	"github.com/mdhender/drynn/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	testerEmailDomain          = "drynn.test"
	testerSentinelPasswordHash = "!tester_account_password_not_set!"
)

func main() {
	log.SetFlags(0)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "init-config":
		err = runInitConfig(os.Args[2:])
	case "seed-admin":
		err = runSeedAdmin(ctx, os.Args[2:])
	case "seed-testers":
		err = runSeedTesters(ctx, os.Args[2:])
	case "set-password":
		err = runSetPassword(ctx, os.Args[2:])
	case "jwt-key":
		err = runJWTKey(ctx, os.Args[2:])
	case "version":
		fmt.Println(drynn.Version().Core())
		return
	case "help", "-h", "--help":
		usage()
		return
	default:
		usage()
		log.Fatalf("unknown command %q", os.Args[1])
	}

	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Fatal(err)
	}
}

func runInitConfig(args []string) error {
	fs := flag.NewFlagSet("init-config", flag.ContinueOnError)
	configPath := fs.String("config", config.DefaultPath(), "path to the server config file")
	appAddr := fs.String("app-addr", ":8080", "server listen address")
	databaseURL := fs.String("database-url", "", "PostgreSQL connection string")
	dataDir := fs.String("data-dir", config.DefaultDataDir(), "path for server-managed data files")
	accessTTL := fs.Duration("jwt-access-ttl", 15*time.Minute, "access token lifetime")
	refreshTTL := fs.Duration("jwt-refresh-ttl", 7*24*time.Hour, "refresh token lifetime")
	cookieSecure := fs.Bool("cookie-secure", false, "set Secure on auth cookies")
	mailgunAPIKey := fs.String("mailgun-api-key", "", "Mailgun API key for outbound email")
	mailgunDomain := fs.String("mailgun-sending-domain", "", "Mailgun sending domain (e.g. mg.example.com)")
	mailgunFromAddr := fs.String("mailgun-from-address", "", "From address used on outbound email")
	mailgunFromName := fs.String("mailgun-from-name", "", "Optional display name for the From address")
	requestAccessEnabled := fs.Bool("request-access-enabled", false, "enable the public /request-access form (opt-in)")
	adminContactEmail := fs.String("admin-contact-email", "", "destination address for public access requests")
	force := fs.Bool("force", false, "overwrite an existing config file")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: %s init-config [flags]\n", os.Args[0])
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.WritePath(*configPath, config.InitOptions{
		AppAddr:       *appAddr,
		DatabaseURL:   *databaseURL,
		DataDir:       *dataDir,
		JWTAccessTTL:  *accessTTL,
		JWTRefreshTTL: *refreshTTL,
		CookieSecure:  *cookieSecure,
		Mailgun: config.MailgunConfig{
			APIKey:        *mailgunAPIKey,
			SendingDomain: *mailgunDomain,
			FromAddress:   *mailgunFromAddr,
			FromName:      *mailgunFromName,
		},
		RequestAccessEnabled: *requestAccessEnabled,
		AdminContactEmail:    *adminContactEmail,
		Force:                *force,
	})
	if err != nil {
		return err
	}

	fmt.Printf("wrote server config to %s\n", cfg.ConfigPath)
	fmt.Printf("data dir: %s\n", cfg.DataDir)
	return nil
}

func runSeedAdmin(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("seed-admin", flag.ContinueOnError)
	configPath := fs.String("config", config.DefaultPath(), "path to the server config file")
	handle := fs.String("handle", "", "admin handle")
	email := fs.String("email", "", "admin email")
	password := fs.String("password", "", "admin password")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: %s seed-admin [flags]\n", os.Args[0])
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *handle == "" || *email == "" || *password == "" {
		return fmt.Errorf("handle, email, and password are required")
	}

	_, pool, err := openDatabase(ctx, *configPath)
	if err != nil {
		return err
	}
	defer pool.Close()

	users := service.NewUserService(pool)
	if err := users.EnsureBootstrapAdmin(ctx, *handle, *email, *password); err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}

	fmt.Printf("ensured admin user %s <%s>\n", *handle, *email)
	return nil
}

func runSeedTesters(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("seed-testers", flag.ContinueOnError)
	configPath := fs.String("config", config.DefaultPath(), "path to the server config file")
	count := fs.Int("count", 0, "total number of tester accounts to ensure exist")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: %s seed-testers [flags]\n", os.Args[0])
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *count < 1 {
		return fmt.Errorf("count must be at least 1")
	}

	_, pool, err := openDatabase(ctx, *configPath)
	if err != nil {
		return err
	}
	defer pool.Close()

	users := service.NewUserService(pool)
	created, err := users.SeedTesters(ctx, *count, testerEmailDomain, testerSentinelPasswordHash)
	if err != nil {
		return fmt.Errorf("seed testers: %w", err)
	}

	if created == 0 {
		fmt.Printf("tester accounts already at or above %d; nothing to do\n", *count)
		return nil
	}

	fmt.Printf("created %d tester account(s); set passwords via the admin edit-user form before sign-in\n", created)
	return nil
}

func runSetPassword(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("set-password", flag.ContinueOnError)
	configPath := fs.String("config", config.DefaultPath(), "path to the server config file")
	email := fs.String("email", "", "user email address")
	password := fs.String("password", "", "new password")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: %s set-password [flags]\n", os.Args[0])
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *email == "" || *password == "" {
		return fmt.Errorf("email and password are required")
	}

	_, pool, err := openDatabase(ctx, *configPath)
	if err != nil {
		return err
	}
	defer pool.Close()

	users := service.NewUserService(pool)
	if err := users.SetPasswordByEmail(ctx, *email, *password); err != nil {
		return fmt.Errorf("set password: %w", err)
	}

	fmt.Printf("updated password for %s\n", *email)
	return nil
}

func runJWTKey(ctx context.Context, args []string) error {
	if len(args) == 0 {
		jwtKeyUsage()
		return flag.ErrHelp
	}

	switch args[0] {
	case "create":
		return runJWTKeyCreate(ctx, args[1:])
	case "expire":
		return runJWTKeyExpire(ctx, args[1:])
	case "delete":
		return runJWTKeyDelete(ctx, args[1:])
	case "help", "-h", "--help":
		jwtKeyUsage()
		return nil
	default:
		jwtKeyUsage()
		return fmt.Errorf("unknown jwt-key command %q", args[0])
	}
}

func runJWTKeyCreate(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("jwt-key create", flag.ContinueOnError)
	configPath := fs.String("config", config.DefaultPath(), "path to the server config file")
	tokenType := fs.String("type", auth.TokenTypeAccess, "token type: access or refresh")
	verifyOldFor := fs.Duration("verify-old-for", -1, "verification grace period for the previous active key; defaults to the token TTL")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: %s jwt-key create [flags]\n", os.Args[0])
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, pool, err := openDatabase(ctx, *configPath)
	if err != nil {
		return err
	}
	defer pool.Close()

	gracePeriod := *verifyOldFor
	if gracePeriod < 0 {
		gracePeriod, err = ttlForTokenType(cfg, *tokenType)
		if err != nil {
			return err
		}
	}

	store := auth.NewKeyStore(pool)
	created, retired, err := store.CreateSigningKey(ctx, *tokenType, gracePeriod)
	if err != nil {
		return fmt.Errorf("create jwt signing key: %w", err)
	}

	fmt.Printf("created %s key %s\n", created.TokenType, created.ID)
	if retired != nil && retired.VerifyUntil != nil {
		fmt.Printf("retired previous %s key %s; valid for verification until %s\n", retired.TokenType, retired.ID, retired.VerifyUntil.Format(time.RFC3339))
	}
	return nil
}

func runJWTKeyExpire(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("jwt-key expire", flag.ContinueOnError)
	configPath := fs.String("config", config.DefaultPath(), "path to the server config file")
	keyID := fs.String("id", "", "signing key UUID")
	verifyFor := fs.Duration("verify-for", 0, "continue allowing verification for this duration after expiring the key")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: %s jwt-key expire [flags]\n", os.Args[0])
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	parsedID, err := parseKeyID(*keyID)
	if err != nil {
		return err
	}

	_, pool, err := openDatabase(ctx, *configPath)
	if err != nil {
		return err
	}
	defer pool.Close()

	store := auth.NewKeyStore(pool)
	retired, err := store.ExpireSigningKey(ctx, parsedID, *verifyFor)
	if err != nil {
		return fmt.Errorf("expire jwt signing key: %w", err)
	}

	if retired.VerifyUntil == nil {
		fmt.Printf("expired key %s\n", retired.ID)
		return nil
	}

	fmt.Printf("expired key %s; verification allowed until %s\n", retired.ID, retired.VerifyUntil.Format(time.RFC3339))
	return nil
}

func runJWTKeyDelete(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("jwt-key delete", flag.ContinueOnError)
	configPath := fs.String("config", config.DefaultPath(), "path to the server config file")
	keyID := fs.String("id", "", "signing key UUID")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: %s jwt-key delete [flags]\n", os.Args[0])
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	parsedID, err := parseKeyID(*keyID)
	if err != nil {
		return err
	}

	_, pool, err := openDatabase(ctx, *configPath)
	if err != nil {
		return err
	}
	defer pool.Close()

	store := auth.NewKeyStore(pool)
	if err := store.DeleteSigningKey(ctx, parsedID); err != nil {
		return fmt.Errorf("delete jwt signing key: %w", err)
	}

	fmt.Printf("deleted key %s\n", parsedID)
	return nil
}

func openDatabase(ctx context.Context, configPath string) (config.Config, *pgxpool.Pool, error) {
	cfg, err := config.LoadPath(configPath)
	if err != nil {
		return config.Config{}, nil, err
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return config.Config{}, nil, fmt.Errorf("connect database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return config.Config{}, nil, fmt.Errorf("ping database: %w", err)
	}

	return cfg, pool, nil
}

func ttlForTokenType(cfg config.Config, tokenType string) (time.Duration, error) {
	switch strings.ToLower(strings.TrimSpace(tokenType)) {
	case auth.TokenTypeAccess:
		return cfg.JWTAccessTTL, nil
	case auth.TokenTypeRefresh:
		return cfg.JWTRefreshTTL, nil
	default:
		return 0, auth.ErrInvalidTokenType
	}
}

func parseKeyID(raw string) (uuid.UUID, error) {
	if raw == "" {
		return uuid.Nil, fmt.Errorf("key id is required")
	}

	parsedID, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse key id: %w", err)
	}

	return parsedID, nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s <command> [flags]\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "commands:")
	fmt.Fprintln(os.Stderr, "  init-config   write the persisted server config file")
	fmt.Fprintln(os.Stderr, "  seed-admin    create or update the bootstrap administrator")
	fmt.Fprintln(os.Stderr, "  seed-testers  ensure N tester accounts exist (sentinel password)")
	fmt.Fprintln(os.Stderr, "  set-password  set a user's password by email")
	fmt.Fprintln(os.Stderr, "  jwt-key       create, expire, or delete JWT signing keys")
	fmt.Fprintln(os.Stderr, "  version       print the build version")
}

func jwtKeyUsage() {
	fmt.Fprintf(os.Stderr, "usage: %s jwt-key <create|expire|delete> [flags]\n", os.Args[0])
}
