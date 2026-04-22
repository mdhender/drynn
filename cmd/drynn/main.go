package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v4"

	drynn "github.com/mdhender/drynn"
	"github.com/mdhender/drynn/internal/config"
	"github.com/mdhender/drynn/internal/prng"
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

	// test-hexmap (standalone diagnostic)
	testHexFlags := ff.NewFlagSet("test-hexmap")
	testHexRadius := testHexFlags.IntLong("radius", 15, "disk radius in hexes")
	testHexSystems := testHexFlags.IntLong("systems", 100, "number of star systems to place")
	testHexMinDist := testHexFlags.IntLong("min-distance", 0, "minimum distance between systems")
	testHexNoMerge := testHexFlags.BoolLong("no-merge", "discard placements that are too close instead of merging their stars (default is to merge)")
	testHexSeed1 := testHexFlags.UintLong("seed1", 20, "PRNG seed value 1")
	testHexSeed2 := testHexFlags.UintLong("seed2", 20, "PRNG seed value 2")
	testHexRandomSeeds := testHexFlags.BoolLong("use-random-seeds", "use random seeds instead of --seed1/--seed2")
	testHexCoords := testHexFlags.BoolLong("coords", "render axial coordinates in occupied hexes")
	testHexOut := testHexFlags.StringLong("out", ".", "output directory for the generated HTML")
	testHexCmd := &ff.Command{
		Name:      "test-hexmap",
		Usage:     "test-hexmap [flags]",
		ShortHelp: "generate a hex map with star systems and render to HTML",
		Flags:     testHexFlags,
		Exec: func(ctx context.Context, args []string) error {
			seed1, seed2 := uint64(*testHexSeed1), uint64(*testHexSeed2)
			if *testHexRandomSeeds {
				seed1, seed2 = cryptoRandSeeds()
			}
			fmt.Printf("seeds: %d %d\n", seed1, seed2)

			galaxy, err := worldgen.Generate(
				worldgen.WithDesiredRadius(*testHexRadius),
				worldgen.WithDesiredNumberOfSystems(*testHexSystems),
				worldgen.WithMinimumDistance(*testHexMinDist),
				worldgen.WithMerge(!*testHexNoMerge),
				worldgen.WithPRNG(prng.NewFromSeed(seed1, seed2)),
			)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "warning: %v\n", err)
				if galaxy == nil {
					return err
				}
			}
			fmt.Printf("placed %d systems (%d total stars, %d multi-star) in radius-%d disk\n",
				len(galaxy.Systems), worldgen.TotalStars(galaxy.Systems), worldgen.CountMultiStar(galaxy.Systems), *testHexRadius)

			// Star-count breakdown.
			var one, two, three, four, fivePlus int
			for _, s := range galaxy.Systems {
				switch len(s.Stars) {
				case 1:
					one++
				case 2:
					two++
				case 3:
					three++
				case 4:
					four++
				default:
					fivePlus++
				}
			}
			fmt.Printf("  1-star: %d   2-star: %d   3-star: %d   4-star: %d   5+-star: %d\n", one, two, three, four, fivePlus)

			// Pairwise distance statistics.
			if len(galaxy.Systems) >= 2 {
				var distances []int
				for i := 0; i < len(galaxy.Systems); i++ {
					for j := i + 1; j < len(galaxy.Systems); j++ {
						distances = append(distances, galaxy.Systems[i].Hex.Distance(galaxy.Systems[j].Hex))
					}
				}
				sort.Ints(distances)
				sum := 0
				for _, d := range distances {
					sum += d
				}
				mean := float64(sum) / float64(len(distances))
				var median float64
				n := len(distances)
				if n%2 == 1 {
					median = float64(distances[n/2])
				} else {
					median = float64(distances[n/2-1]+distances[n/2]) / 2.0
				}
				fmt.Printf("  distances (n=%d pairs): min=%d  max=%d  mean=%.1f  median=%.1f\n",
					len(distances), distances[0], distances[n-1], mean, median)

				// Nearest-neighbor distance statistics.
				nn := make([]int, len(galaxy.Systems))
				for i := range galaxy.Systems {
					nn[i] = math.MaxInt
					for j := range galaxy.Systems {
						if i == j {
							continue
						}
						if d := galaxy.Systems[i].Hex.Distance(galaxy.Systems[j].Hex); d < nn[i] {
							nn[i] = d
						}
					}
				}
				sort.Ints(nn)
				nnSum := 0
				for _, d := range nn {
					nnSum += d
				}
				nnMean := float64(nnSum) / float64(len(nn))
				var nnMedian float64
				nnN := len(nn)
				if nnN%2 == 1 {
					nnMedian = float64(nn[nnN/2])
				} else {
					nnMedian = float64(nn[nnN/2-1]+nn[nnN/2]) / 2.0
				}
				fmt.Printf("  nearest-neighbor: min=%d  max=%d  mean=%.1f  median=%.1f\n",
					nn[0], nn[nnN-1], nnMean, nnMedian)

				// Nearest-neighbor histogram.
				nnHist := make(map[int]int)
				for _, d := range nn {
					nnHist[d]++
				}
				var nnKeys []int
				for k := range nnHist {
					nnKeys = append(nnKeys, k)
				}
				sort.Ints(nnKeys)
				maxCount := 0
				for _, count := range nnHist {
					if count > maxCount {
						maxCount = count
					}
				}
				const barWidth = 40
				fmt.Println("  nearest-neighbor histogram:")
				for _, k := range nnKeys {
					count := nnHist[k]
					barLen := count * barWidth / maxCount
					if barLen == 0 && count > 0 {
						barLen = 1
					}
					fmt.Printf("    dist %2d: %s %d\n", k, strings.Repeat("█", barLen), count)
				}
			}

			html := galaxy.ToHTML(0, *testHexCoords, false)
			outPath := filepath.Join(*testHexOut, "hexmap.html")
			if err := os.WriteFile(outPath, html, 0o644); err != nil {
				return err
			}
			fmt.Printf("wrote %s\n", outPath)
			return nil
		},
	}

	// test-galaxy (standalone diagnostic)
	testGalaxyFlags := ff.NewFlagSet("test-galaxy")
	testGalaxyRadius := testGalaxyFlags.IntLong("radius", 15, "disk radius in hexes")
	testGalaxySystems := testGalaxyFlags.IntLong("systems", 100, "target number of star systems to place")
	testGalaxyMinDist := testGalaxyFlags.IntLong("min-distance", 0, "minimum distance between systems")
	testGalaxyNoMerge := testGalaxyFlags.BoolLong("no-merge", "discard placements that are too close instead of merging their stars (default is to merge)")
	testGalaxySeed1 := testGalaxyFlags.UintLong("seed1", 20, "PRNG seed value 1")
	testGalaxySeed2 := testGalaxyFlags.UintLong("seed2", 20, "PRNG seed value 2")
	testGalaxyRandomSeeds := testGalaxyFlags.BoolLong("use-random-seeds", "use random seeds instead of --seed1/--seed2")
	testGalaxyHTML := testGalaxyFlags.BoolLong("html", "write galaxy.html with the hex map")
	testGalaxyCoords := testGalaxyFlags.BoolLong("coords", "render axial coordinates in occupied hexes")
	testGalaxyPlanets := testGalaxyFlags.BoolLong("planets", "include a planet report in the generated HTML")
	testGalaxyPixel := testGalaxyFlags.Float64Long("pixel-size", 0, "hex pixel size (0 = auto-fit)")
	testGalaxyOut := testGalaxyFlags.StringLong("out", ".", "output directory for the generated HTML")
	testGalaxyCmd := &ff.Command{
		Name:      "test-galaxy",
		Usage:     "test-galaxy [flags]",
		ShortHelp: "generate a galaxy and optionally render it to HTML",
		Flags:     testGalaxyFlags,
		Exec: func(ctx context.Context, args []string) error {
			seed1, seed2 := uint64(*testGalaxySeed1), uint64(*testGalaxySeed2)
			if *testGalaxyRandomSeeds {
				seed1, seed2 = cryptoRandSeeds()
			}
			fmt.Printf("seeds: %d %d\n", seed1, seed2)

			galaxy, err := worldgen.Generate(
				worldgen.WithDesiredRadius(*testGalaxyRadius),
				worldgen.WithDesiredNumberOfSystems(*testGalaxySystems),
				worldgen.WithMinimumDistance(*testGalaxyMinDist),
				worldgen.WithMerge(!*testGalaxyNoMerge),
				worldgen.WithPRNG(prng.NewFromSeed(seed1, seed2)),
			)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "warning: %v\n", err)
				if galaxy == nil {
					return err
				}
			}

			totalStars, totalPlanets := 0, 0
			var one, two, three, four, fivePlus int
			for _, s := range galaxy.Systems {
				n := len(s.Stars)
				totalStars += n
				for _, star := range s.Stars {
					totalPlanets += len(star.Planets)
				}
				switch n {
				case 1:
					one++
				case 2:
					two++
				case 3:
					three++
				case 4:
					four++
				default:
					fivePlus++
				}
			}
			fmt.Printf("generated %d systems (%d stars, %d planets) in radius-%d galaxy\n",
				len(galaxy.Systems), totalStars, totalPlanets, galaxy.Radius)
			fmt.Printf("  1-star: %d   2-star: %d   3-star: %d   4-star: %d   5+-star: %d\n",
				one, two, three, four, fivePlus)

			if *testGalaxyHTML {
				html := galaxy.ToHTML(*testGalaxyPixel, *testGalaxyCoords, *testGalaxyPlanets)
				outPath := filepath.Join(*testGalaxyOut, "galaxy.html")
				if err := os.WriteFile(outPath, html, 0o644); err != nil {
					return err
				}
				fmt.Printf("wrote %s\n", outPath)
			}
			return nil
		},
	}

	// simulate (standalone; generates galaxy + home-system templates locally)
	simulateFlags := ff.NewFlagSet("simulate")
	simulateRadius := simulateFlags.IntLong("radius", 15, "disk radius in hexes")
	simulateSystems := simulateFlags.IntLong("systems", 100, "target number of star systems to place")
	simulateMinDist := simulateFlags.IntLong("min-distance", 0, "minimum distance between systems")
	simulateNoMerge := simulateFlags.BoolLong("no-merge", "discard placements that are too close instead of merging their stars (default is to merge)")
	simulateSeed1 := simulateFlags.UintLong("seed1", 20, "PRNG seed value 1")
	simulateSeed2 := simulateFlags.UintLong("seed2", 20, "PRNG seed value 2")
	simulateRandomSeeds := simulateFlags.BoolLong("use-random-seeds", "use random seeds instead of --seed1/--seed2")
	simulateOut := simulateFlags.StringLong("out", ".", "output directory for the generated HTML")
	simulateJSON := simulateFlags.StringLong("json", "", "also write deterministic run state to this JSON path")
	simulateCmd := &ff.Command{
		Name:      "simulate",
		Usage:     "simulate [flags]",
		ShortHelp: "generate a galaxy and home-system templates, writing HTML reports",
		Flags:     simulateFlags,
		Exec: func(ctx context.Context, args []string) error {
			return runSimulate(simulateOpts{
				radius:      *simulateRadius,
				systems:     *simulateSystems,
				minDistance: *simulateMinDist,
				merge:       !*simulateNoMerge,
				seed1:       uint64(*simulateSeed1),
				seed2:       uint64(*simulateSeed2),
				randomSeeds: *simulateRandomSeeds,
				outDir:      *simulateOut,
				jsonPath:    *simulateJSON,
			})
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
			testHexCmd,
			testGalaxyCmd,
			simulateCmd,
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
	fmt.Fprintln(os.Stderr, "  test-hexmap  generate a hex map with star systems and render to HTML")
	fmt.Fprintln(os.Stderr, "  test-galaxy  generate a galaxy and optionally render it to HTML")
	fmt.Fprintln(os.Stderr, "  simulate     generate a galaxy and home-system templates, writing HTML reports")
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

func cryptoRandSeeds() (uint64, uint64) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		log.Fatalf("crypto/rand: %v", err)
	}
	s1 := binary.LittleEndian.Uint64(buf[:8])
	s2 := binary.LittleEndian.Uint64(buf[8:])
	return s1, s2
}
