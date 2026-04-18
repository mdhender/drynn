package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peterbourgon/ff/v4"

	drynn "github.com/mdhender/drynn"
	"github.com/mdhender/drynn/internal/auth"
	"github.com/mdhender/drynn/internal/config"
	"github.com/mdhender/drynn/internal/email"
	"github.com/mdhender/drynn/internal/service"
)

const (
	testerEmailDomain          = "drynn.test"
	testerSentinelPasswordHash = "!tester_account_password_not_set!"
)

// openDatabaseFn is a test seam: tests replace it so commands can be exercised
// without a live PostgreSQL.
var openDatabaseFn = openDatabase

func main() {
	log.SetFlags(0)

	env := os.Getenv("DRYNN_ENV")
	if env == "" { // force a default to development
		env = "development"
	}
	if err := config.LoadDotfiles(env); err != nil {
		log.Fatalf("dotenv: %v\n", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	if err := run(ctx, args); err != nil {
		if errors.Is(err, ff.ErrHelp) {
			return
		}
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string) error {
	// Shared parent flag set: every leaf that takes --config inherits from here.
	dbCommonFlags := ff.NewFlagSet("db-common")
	configPath := dbCommonFlags.StringLong("config", config.DefaultPath(), "path to the server config file")

	// init-config
	initConfigFlags := ff.NewFlagSet("init-config").SetParent(dbCommonFlags)
	appAddr := initConfigFlags.StringLong("app-addr", ":8080", "server listen address")
	databaseURL := initConfigFlags.StringLong("database-url", "", "PostgreSQL connection string")
	dataDir := initConfigFlags.StringLong("data-dir", config.DefaultDataDir(), "path for server-managed data files")
	accessTTL := initConfigFlags.DurationLong("jwt-access-ttl", 15*time.Minute, "access token lifetime")
	refreshTTL := initConfigFlags.DurationLong("jwt-refresh-ttl", 7*24*time.Hour, "refresh token lifetime")
	cookieSecure := initConfigFlags.BoolLong("cookie-secure", "set Secure on auth cookies")
	baseURL := initConfigFlags.StringLong("base-url", "", "absolute base URL used for invitation and password-reset links (e.g. https://drynn.example.com)")
	mailgunAPIKey := initConfigFlags.StringLong("mailgun-api-key", "", "Mailgun API key for outbound email")
	mailgunDomain := initConfigFlags.StringLong("mailgun-sending-domain", "", "Mailgun sending domain (e.g. mg.example.com)")
	mailgunFromAddr := initConfigFlags.StringLong("mailgun-from-address", "", "From address used on outbound email")
	mailgunFromName := initConfigFlags.StringLong("mailgun-from-name", "", "Optional display name for the From address")
	requestAccessEnabled := initConfigFlags.BoolLong("request-access-enabled", "enable the public /request-access form (opt-in)")
	adminContactEmail := initConfigFlags.StringLong("admin-contact-email", "", "destination address for public access requests")
	force := initConfigFlags.BoolLong("force", "overwrite an existing config file")
	initConfigCmd := &ff.Command{
		Name:      "init-config",
		Usage:     "init-config [flags]",
		ShortHelp: "write the persisted server config file",
		Flags:     initConfigFlags,
		Exec: func(ctx context.Context, args []string) error {
			cfg, err := config.WritePath(*configPath, config.InitOptions{
				AppAddr:       *appAddr,
				DatabaseURL:   *databaseURL,
				DataDir:       *dataDir,
				JWTAccessTTL:  *accessTTL,
				JWTRefreshTTL: *refreshTTL,
				CookieSecure:  *cookieSecure,
				BaseURL:       *baseURL,
				Mailgun: email.MailgunConfig{
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
		},
	}

	// Shared DB context, populated by run() for DB-bound commands before
	// the selected command's Exec runs. Closures below capture these by address.
	var (
		dbCfg  config.Config
		dbPool *pgxpool.Pool
	)

	// seed-admin
	seedAdminFlags := ff.NewFlagSet("seed-admin").SetParent(dbCommonFlags)
	seedAdminHandle := seedAdminFlags.StringLong("handle", "", "admin handle")
	seedAdminEmail := seedAdminFlags.StringLong("email", "", "admin email")
	seedAdminPassword := seedAdminFlags.StringLong("password", "", "admin password")
	seedAdminCmd := &ff.Command{
		Name:      "seed-admin",
		Usage:     "seed-admin [flags]",
		ShortHelp: "create or update the bootstrap administrator",
		Flags:     seedAdminFlags,
		Exec: func(ctx context.Context, args []string) error {
			if *seedAdminHandle == "" || *seedAdminEmail == "" || *seedAdminPassword == "" {
				return fmt.Errorf("handle, email, and password are required")
			}
			users := service.NewUserService(dbPool)
			if err := users.EnsureBootstrapAdmin(ctx, *seedAdminHandle, *seedAdminEmail, *seedAdminPassword); err != nil {
				return fmt.Errorf("seed admin: %w", err)
			}
			fmt.Printf("ensured admin user %s <%s>\n", *seedAdminHandle, *seedAdminEmail)
			return nil
		},
	}

	// seed-testers
	seedTestersFlags := ff.NewFlagSet("seed-testers").SetParent(dbCommonFlags)
	seedTestersCount := seedTestersFlags.IntLong("count", 0, "total number of tester accounts to ensure exist")
	seedTestersCmd := &ff.Command{
		Name:      "seed-testers",
		Usage:     "seed-testers [flags]",
		ShortHelp: "ensure N tester accounts exist (sentinel password)",
		Flags:     seedTestersFlags,
		Exec: func(ctx context.Context, args []string) error {
			if *seedTestersCount < 1 {
				return fmt.Errorf("count must be at least 1")
			}
			users := service.NewUserService(dbPool)
			created, err := users.SeedTesters(ctx, *seedTestersCount, testerEmailDomain, testerSentinelPasswordHash)
			if err != nil {
				return fmt.Errorf("seed testers: %w", err)
			}
			if created == 0 {
				fmt.Printf("tester accounts already at or above %d; nothing to do\n", *seedTestersCount)
				return nil
			}
			fmt.Printf("created %d tester account(s); set passwords via the admin edit-user form before sign-in\n", created)
			return nil
		},
	}

	// set-password
	setPasswordFlags := ff.NewFlagSet("set-password").SetParent(dbCommonFlags)
	setPasswordEmail := setPasswordFlags.StringLong("email", "", "user email address")
	setPasswordValue := setPasswordFlags.StringLong("password", "", "new password")
	setPasswordCmd := &ff.Command{
		Name:      "set-password",
		Usage:     "set-password [flags]",
		ShortHelp: "set a user's password by email",
		Flags:     setPasswordFlags,
		Exec: func(ctx context.Context, args []string) error {
			if *setPasswordEmail == "" || *setPasswordValue == "" {
				return fmt.Errorf("email and password are required")
			}
			users := service.NewUserService(dbPool)
			if err := users.SetPasswordByEmail(ctx, *setPasswordEmail, *setPasswordValue); err != nil {
				return fmt.Errorf("set password: %w", err)
			}
			fmt.Printf("updated password for %s\n", *setPasswordEmail)
			return nil
		},
	}

	// jwt-key create
	jwtKeyCreateFlags := ff.NewFlagSet("create").SetParent(dbCommonFlags)
	jwtKeyCreateType := jwtKeyCreateFlags.StringLong("type", auth.TokenTypeAccess, "token type: access or refresh")
	jwtKeyCreateVerifyOldFor := jwtKeyCreateFlags.DurationLong("verify-old-for", -1, "verification grace period for the previous active key; defaults to the token TTL")
	jwtKeyCreateCmd := &ff.Command{
		Name:      "create",
		Usage:     "jwt-key create [flags]",
		ShortHelp: "create a new JWT signing key",
		Flags:     jwtKeyCreateFlags,
		Exec: func(ctx context.Context, args []string) error {
			gracePeriod := *jwtKeyCreateVerifyOldFor
			if gracePeriod < 0 {
				g, err := ttlForTokenType(dbCfg, *jwtKeyCreateType)
				if err != nil {
					return err
				}
				gracePeriod = g
			}
			store := auth.NewKeyStore(dbPool)
			created, retired, err := store.CreateSigningKey(ctx, *jwtKeyCreateType, gracePeriod)
			if err != nil {
				return fmt.Errorf("create jwt signing key: %w", err)
			}
			fmt.Printf("created %s key %s\n", created.TokenType, created.ID)
			if retired != nil && retired.VerifyUntil != nil {
				fmt.Printf("retired previous %s key %s; valid for verification until %s\n", retired.TokenType, retired.ID, retired.VerifyUntil.Format(time.RFC3339))
			}
			return nil
		},
	}

	// jwt-key expire
	jwtKeyExpireFlags := ff.NewFlagSet("expire").SetParent(dbCommonFlags)
	jwtKeyExpireID := jwtKeyExpireFlags.StringLong("id", "", "signing key UUID")
	jwtKeyExpireVerifyFor := jwtKeyExpireFlags.DurationLong("verify-for", 0, "continue allowing verification for this duration after expiring the key")
	jwtKeyExpireCmd := &ff.Command{
		Name:      "expire",
		Usage:     "jwt-key expire [flags]",
		ShortHelp: "retire a JWT signing key",
		Flags:     jwtKeyExpireFlags,
		Exec: func(ctx context.Context, args []string) error {
			parsedID, err := parseKeyID(*jwtKeyExpireID)
			if err != nil {
				return err
			}
			store := auth.NewKeyStore(dbPool)
			retired, err := store.ExpireSigningKey(ctx, parsedID, *jwtKeyExpireVerifyFor)
			if err != nil {
				return fmt.Errorf("expire jwt signing key: %w", err)
			}
			if retired.VerifyUntil == nil {
				fmt.Printf("expired key %s\n", retired.ID)
				return nil
			}
			fmt.Printf("expired key %s; verification allowed until %s\n", retired.ID, retired.VerifyUntil.Format(time.RFC3339))
			return nil
		},
	}

	// jwt-key delete
	jwtKeyDeleteFlags := ff.NewFlagSet("delete").SetParent(dbCommonFlags)
	jwtKeyDeleteID := jwtKeyDeleteFlags.StringLong("id", "", "signing key UUID")
	jwtKeyDeleteCmd := &ff.Command{
		Name:      "delete",
		Usage:     "jwt-key delete [flags]",
		ShortHelp: "delete a non-active JWT signing key",
		Flags:     jwtKeyDeleteFlags,
		Exec: func(ctx context.Context, args []string) error {
			parsedID, err := parseKeyID(*jwtKeyDeleteID)
			if err != nil {
				return err
			}
			store := auth.NewKeyStore(dbPool)
			if err := store.DeleteSigningKey(ctx, parsedID); err != nil {
				return fmt.Errorf("delete jwt signing key: %w", err)
			}
			fmt.Printf("deleted key %s\n", parsedID)
			return nil
		},
	}

	// jwt-key (parent — dispatches to create/expire/delete; fallback prints usage).
	jwtKeyFlags := ff.NewFlagSet("jwt-key")
	jwtKeyCmd := &ff.Command{
		Name:      "jwt-key",
		Usage:     "jwt-key <create|expire|delete> [flags]",
		ShortHelp: "create, expire, or delete JWT signing keys",
		Flags:     jwtKeyFlags,
		Subcommands: []*ff.Command{
			jwtKeyCreateCmd,
			jwtKeyExpireCmd,
			jwtKeyDeleteCmd,
		},
		Exec: func(ctx context.Context, args []string) error {
			jwtKeyUsage()
			if len(args) == 0 || args[0] == "help" {
				return ff.ErrHelp
			}
			return fmt.Errorf("unknown jwt-key command %q", args[0])
		},
	}

	// version (no shared parent — it doesn't need --config).
	versionFlags := ff.NewFlagSet("version")
	versionCmd := &ff.Command{
		Name:      "version",
		Usage:     "version",
		ShortHelp: "print the build version",
		Flags:     versionFlags,
		Exec: func(ctx context.Context, args []string) error {
			fmt.Println(drynn.Version().Core())
			return nil
		},
	}

	// root (no shared parent; fallback prints usage or unknown-command error).
	rootFlags := ff.NewFlagSet("db")
	rootCmd := &ff.Command{
		Name:  "db",
		Usage: "db <command> [flags]",
		Flags: rootFlags,
		Subcommands: []*ff.Command{
			initConfigCmd,
			seedAdminCmd,
			seedTestersCmd,
			setPasswordCmd,
			jwtKeyCmd,
			versionCmd,
		},
		Exec: func(ctx context.Context, args []string) error {
			usage()
			if len(args) == 0 || args[0] == "help" {
				return ff.ErrHelp
			}
			return fmt.Errorf("unknown command %q", args[0])
		},
	}

	parseErr := rootCmd.Parse(args)
	selected := rootCmd.GetSelected()

	if errors.Is(parseErr, ff.ErrHelp) {
		switch selected {
		case rootCmd, nil:
			usage()
		case jwtKeyCmd:
			jwtKeyUsage()
		default:
			printLeafUsage(selected)
		}
		return ff.ErrHelp
	}
	if parseErr != nil {
		return parseErr
	}

	// Open the database only for commands that need it.
	switch selected {
	case seedAdminCmd, seedTestersCmd, setPasswordCmd,
		jwtKeyCreateCmd, jwtKeyExpireCmd, jwtKeyDeleteCmd:
		cfg, pool, err := openDatabaseFn(ctx, *configPath)
		if err != nil {
			return err
		}
		defer pool.Close()
		dbCfg = cfg
		dbPool = pool
	}

	return rootCmd.Run(ctx)
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

func printLeafUsage(cmd *ff.Command) {
	fmt.Fprintf(os.Stderr, "usage: %s %s\n", os.Args[0], cmd.Usage)
	if cmd.Flags == nil {
		return
	}
	_ = cmd.Flags.WalkFlags(func(f ff.Flag) error {
		long, hasLong := f.GetLongName()
		if !hasLong {
			return nil
		}
		left := "--" + long
		if p := f.GetPlaceholder(); p != "" {
			left += " " + p
		}
		if def := f.GetDefault(); def == "" {
			fmt.Fprintf(os.Stderr, "  %s\t%s\n", left, f.GetUsage())
		} else {
			fmt.Fprintf(os.Stderr, "  %s\t%s (default %s)\n", left, f.GetUsage(), def)
		}
		return nil
	})
}
