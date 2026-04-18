package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/peterbourgon/ff/v4"

	drynn "github.com/mdhender/drynn"
	"github.com/mdhender/drynn/internal/config"
	"github.com/mdhender/drynn/internal/worldgen"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

type drynnRuntime struct {
	session   sessionData
	serverURL string
}

func main() {
	log.SetFlags(0)

	env := os.Getenv("DRYNN_ENV")
	if env == "" {
		env = "development"
	}
	if err := config.LoadDotfiles(env); err != nil {
		log.Fatalf("dotenv: %v\n", err)
	}

	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, ff.ErrHelp) {
			return
		}
		log.Fatal(err)
	}
}

func run(args []string) error {
	// Shared parent flag set: --server and --config inherited by leaf commands
	// that talk to the server.
	serverFlags := ff.NewFlagSet("server-common")
	server := serverFlags.StringLong("server", "", "server URL (e.g. http://localhost:8080)")
	configPath := serverFlags.StringLong("config", "", "path to the server config file")

	// Shared runtime state populated by pre-run for commands that need
	// a resolved session and server URL. Closures capture &rt.
	var rt drynnRuntime

	// login
	loginFlags := ff.NewFlagSet("login").SetParent(serverFlags)
	loginEmail := loginFlags.StringLong("email", "", "account email address")
	loginPassword := loginFlags.StringLong("password", "", "account password")
	loginCmd := &ff.Command{
		Name:      "login",
		Usage:     "login [flags]",
		ShortHelp: "authenticate with the server",
		Flags:     loginFlags,
		Exec: func(ctx context.Context, args []string) error {
			if *loginEmail == "" || *loginPassword == "" {
				return fmt.Errorf("email and password are required")
			}

			endpoint, err := url.JoinPath(rt.serverURL, "/api/v1/login")
			if err != nil {
				return fmt.Errorf("build login URL: %w", err)
			}

			body, err := json.Marshal(map[string]string{
				"email":    *loginEmail,
				"password": *loginPassword,
			})
			if err != nil {
				return fmt.Errorf("encode request: %w", err)
			}

			resp, err := httpClient.Post(endpoint, "application/json", bytes.NewReader(body))
			if err != nil {
				return fmt.Errorf("login request: %w", err)
			}
			defer resp.Body.Close()

			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("read response: %w", err)
			}

			if resp.StatusCode != http.StatusOK {
				var apiErr struct {
					Error string `json:"error"`
				}
				if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error != "" {
					return fmt.Errorf("login failed: %s", apiErr.Error)
				}
				return fmt.Errorf("login failed: %s", resp.Status)
			}

			var result struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
			}
			if err := json.Unmarshal(respBody, &result); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			if err := saveSession(sessionData{
				ServerURL:    rt.serverURL,
				AccessToken:  result.AccessToken,
				RefreshToken: result.RefreshToken,
			}); err != nil {
				return err
			}

			fmt.Println("logged in")
			return nil
		},
	}

	// logout (no shared parent — doesn't need --server/--config)
	logoutFlags := ff.NewFlagSet("logout")
	logoutCmd := &ff.Command{
		Name:      "logout",
		Usage:     "logout",
		ShortHelp: "clear the current session",
		Flags:     logoutFlags,
		Exec: func(ctx context.Context, args []string) error {
			if err := clearTokens(); err != nil {
				return err
			}
			fmt.Println("logged out")
			return nil
		},
	}

	// health
	healthFlags := ff.NewFlagSet("health").SetParent(serverFlags)
	healthCmd := &ff.Command{
		Name:      "health",
		Usage:     "health [flags]",
		ShortHelp: "check server health",
		Flags:     healthFlags,
		Exec: func(ctx context.Context, args []string) error {
			endpoint, err := url.JoinPath(rt.serverURL, "/api/v1/health")
			if err != nil {
				return fmt.Errorf("build health URL: %w", err)
			}

			resp, err := httpClient.Get(endpoint)
			if err != nil {
				return fmt.Errorf("health request: %w", err)
			}
			defer resp.Body.Close()

			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("read response: %w", err)
			}

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("health check failed: %s", resp.Status)
			}

			var result struct {
				Status  string `json:"status"`
				Version string `json:"version"`
			}
			if err := json.Unmarshal(respBody, &result); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			fmt.Printf("status=%s version=%s\n", result.Status, result.Version)
			return nil
		},
	}

	// version (no shared parent — standalone)
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

	// test-points (no shared parent — standalone diagnostic)
	testPointsFlags := ff.NewFlagSet("test-points")
	testPointsGen := testPointsFlags.StringLong("generator", "uniformSphere", "point generator: naive, naiveDisk, uniformDisk, uniformSphere")
	testPointsNumber := testPointsFlags.IntLong("count", 100, "number of points")
	testPointsOut := testPointsFlags.StringLong("out", ".", "output directory for the generated HTML")
	testPointsCmd := &ff.Command{
		Name:      "test-points",
		Usage:     "test-points [flags]",
		ShortHelp: "render a worldgen point generator to an HTML/SVG preview",
		Flags:     testPointsFlags,
		Exec: func(ctx context.Context, args []string) error {
			switch *testPointsGen {
			case "naive", "naiveDisk", "uniformDisk", "uniformSphere":
			default:
				return fmt.Errorf("unknown generator %q (want naive, naiveDisk, uniformDisk, or uniformSphere)", *testPointsGen)
			}
			return worldgen.TestPointsGenerator(*testPointsNumber, *testPointsGen, *testPointsOut)
		},
	}

	// game (parent command; dispatches to create/list/show/update/delete)
	gameFlags := ff.NewFlagSet("game").SetParent(serverFlags)

	gameCreateFlags := ff.NewFlagSet("create").SetParent(gameFlags)
	gameCreateFile := gameCreateFlags.StringLong("file", "", "path to game config JSON file")
	gameCreateCmd := &ff.Command{
		Name:      "create",
		Usage:     "create --file <path> [flags]",
		ShortHelp: "create a new game from a config file",
		Flags:     gameCreateFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runGameCreate(ctx, *gameCreateFile, &rt)
		},
	}

	gameListFlags := ff.NewFlagSet("list").SetParent(gameFlags)
	gameListCmd := &ff.Command{
		Name:      "list",
		Usage:     "list [flags]",
		ShortHelp: "list all games",
		Flags:     gameListFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runGameList(ctx, &rt)
		},
	}

	gameShowFlags := ff.NewFlagSet("show").SetParent(gameFlags)
	gameShowID := gameShowFlags.StringLong("id", "", "game id")
	gameShowCmd := &ff.Command{
		Name:      "show",
		Usage:     "show --id <id> [flags]",
		ShortHelp: "show details of a game",
		Flags:     gameShowFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runGameShow(ctx, *gameShowID, &rt)
		},
	}

	gameUpdateFlags := ff.NewFlagSet("update").SetParent(gameFlags)
	gameUpdateID := gameUpdateFlags.StringLong("id", "", "game id")
	gameUpdateCmd := &ff.Command{
		Name:      "update",
		Usage:     "update --id <id> [flags]",
		ShortHelp: "update a game (not yet implemented)",
		Flags:     gameUpdateFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runGameUpdate(ctx, *gameUpdateID, &rt)
		},
	}

	gameDeleteFlags := ff.NewFlagSet("delete").SetParent(gameFlags)
	gameDeleteID := gameDeleteFlags.StringLong("id", "", "game id")
	gameDeleteCmd := &ff.Command{
		Name:      "delete",
		Usage:     "delete --id <id> [flags]",
		ShortHelp: "delete a game",
		Flags:     gameDeleteFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runGameDelete(ctx, *gameDeleteID, &rt)
		},
	}

	gameCmd := &ff.Command{
		Name:      "game",
		Usage:     "game <command> [flags]",
		ShortHelp: "manage games (create, list, show, update, delete)",
		Flags:     gameFlags,
		Subcommands: []*ff.Command{
			gameCreateCmd,
			gameListCmd,
			gameShowCmd,
			gameUpdateCmd,
			gameDeleteCmd,
		},
		Exec: func(ctx context.Context, args []string) error {
			gameUsage()
			if len(args) == 0 || args[0] == "help" {
				return ff.ErrHelp
			}
			return fmt.Errorf("unknown game command %q", args[0])
		},
	}

	// root (dispatches to subcommands; fallback prints usage or unknown-command error)
	rootFlags := ff.NewFlagSet("drynn")
	rootCmd := &ff.Command{
		Name:  "drynn",
		Usage: "drynn <command> [flags]",
		Flags: rootFlags,
		Subcommands: []*ff.Command{
			loginCmd,
			logoutCmd,
			healthCmd,
			versionCmd,
			testPointsCmd,
			gameCmd,
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
		default:
			printLeafUsage(selected)
		}
		return ff.ErrHelp
	}
	if parseErr != nil {
		return parseErr
	}

	// Pre-run: load session and resolve server URL for commands that need them.
	switch selected {
	case loginCmd:
		session, err := loadSession()
		if err != nil {
			return err
		}
		if *configPath != "" && session.AccessToken != "" {
			return fmt.Errorf("existing session found; run 'drynn logout' before using --config")
		}
		serverURL, err := resolveServerURL(*server, *configPath, session)
		if err != nil {
			return err
		}
		rt.session = session
		rt.serverURL = serverURL
	case healthCmd, gameCmd, gameCreateCmd, gameListCmd, gameShowCmd, gameUpdateCmd, gameDeleteCmd:
		session, err := loadSession()
		if err != nil {
			return err
		}
		serverURL, err := resolveServerURL(*server, *configPath, session)
		if err != nil {
			return err
		}
		rt.session = session
		rt.serverURL = serverURL
	}

	return rootCmd.Run(context.Background())
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s <command> [flags]\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "commands:")
	fmt.Fprintln(os.Stderr, "  login        authenticate with the server")
	fmt.Fprintln(os.Stderr, "  logout       clear the current session")
	fmt.Fprintln(os.Stderr, "  health       check server health")
	fmt.Fprintln(os.Stderr, "  version      print the build version")
	fmt.Fprintln(os.Stderr, "  test-points  render a worldgen point generator to an HTML/SVG preview")
	fmt.Fprintln(os.Stderr, "  game         manage games (create, list, show, update, delete)")
}

func gameUsage() {
	fmt.Fprintf(os.Stderr, "usage: %s game <command> [flags]\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "commands:")
	fmt.Fprintln(os.Stderr, "  create    create a new game from a config file")
	fmt.Fprintln(os.Stderr, "  list      list all games")
	fmt.Fprintln(os.Stderr, "  show      show details of a game")
	fmt.Fprintln(os.Stderr, "  update    update a game (not yet implemented)")
	fmt.Fprintln(os.Stderr, "  delete    delete a game")
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
