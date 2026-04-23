package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mdhender/drynn/internal/prng"
	"github.com/mdhender/drynn/internal/worldgen"
)

type simulateOpts struct {
	radius      int
	systems     int
	minDistance int
	merge       bool
	seed1       uint64
	seed2       uint64
	randomSeeds bool
	outDir      string
	jsonPath    string
	coords      bool
	planets     bool
	deposits    bool
}

// runSimulate simulates the GM's interactive workflow by walking the
// staged worldgen entry points in order (templates → cluster → deposits).
// Each stage prints a brief summary before the next begins, mirroring
// what a future interactive GM review would see between stages.
// Writes HTML reports for the cluster and for each of the seven template
// slots; when --json is set, the full run state is also written to a
// deterministic JSON file suitable for golden file tests.
func runSimulate(opts simulateOpts) error {
	seed1, seed2 := opts.seed1, opts.seed2
	if opts.randomSeeds {
		seed1, seed2 = cryptoRandSeeds()
	}
	fmt.Printf("seeds: %d %d\n", seed1, seed2)

	if err := os.MkdirAll(opts.outDir, 0o755); err != nil {
		return fmt.Errorf("create out dir: %w", err)
	}

	// Master PRNG split into per-stage substreams. Split order is fixed
	// so that tweaking one stage's inputs does not shift another stage's
	// output under the same seed.
	rng := prng.NewFromSeed(seed1, seed2)
	rngTemplates := rng.Split()
	rngCluster := rng.Split()
	rngDeposits := rng.Split()

	// Stage 1: home-star templates.
	fmt.Println("stage 1: home-star templates")
	templates := worldgen.GenerateHomeStarTemplates(rngTemplates,
		worldgen.DefaultViabilityWindow, worldgen.DefaultMaxCandidateRolls)
	for n := 3; n <= 9; n++ {
		outcome := templates[n]
		if outcome.Template == nil {
			fmt.Printf("  n=%d: no viable template (%d attempts, best score %d)\n",
				n, outcome.Attempts, outcome.BestScore)
		} else {
			fmt.Printf("  n=%d: viable (score %d, %d attempts)\n",
				n, outcome.Template.ViabilityScore, outcome.Attempts)
		}
	}

	// Stages 2+3: hex placement, stars, and planets.
	fmt.Println("stage 2+3: cluster placement + stars + planets")
	cluster, err := worldgen.GenerateCluster(rngCluster, worldgen.ClusterOptions{
		Radius:          opts.radius,
		NumSystems:      opts.systems,
		MinimumDistance: opts.minDistance,
		Merge:           opts.merge,
	})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		if cluster == nil {
			return err
		}
	}
	cluster.HomeStarTemplates = templates
	fmt.Printf("  %d systems, %d stars, %d planets in radius-%d cluster\n",
		len(cluster.Systems), cluster.TotalStars(), cluster.TotalPlanets(), cluster.Radius)

	// Stage 4: deposits.
	fmt.Println("stage 4: deposits")
	worldgen.GenerateDeposits(rngDeposits, cluster)
	fmt.Printf("  %d deposits across %d planets\n", cluster.TotalDeposits(), cluster.TotalPlanets())

	clusterPath := filepath.Join(opts.outDir, "cluster.html")
	showPlanets := opts.planets || opts.deposits
	if err := os.WriteFile(clusterPath, cluster.ToHTML(0, opts.coords, showPlanets, opts.deposits), 0o644); err != nil {
		return fmt.Errorf("write cluster.html: %w", err)
	}
	fmt.Printf("wrote %s\n", clusterPath)

	for n := 3; n <= 9; n++ {
		outcome := cluster.HomeStarTemplates[n]
		path := filepath.Join(opts.outDir, fmt.Sprintf("home-system-%d.html", n))
		var html []byte
		if outcome.Template == nil {
			html = worldgen.HomeStarTemplateUnavailableHTML(n, outcome.Attempts, outcome.BestScore)
		} else {
			html = outcome.Template.ToHTML()
		}
		if err := os.WriteFile(path, html, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		fmt.Printf("wrote %s\n", path)
	}

	if opts.jsonPath != "" {
		body, err := worldgen.MarshalSimulationJSON(worldgen.SimulationOutcome{
			Seed1:   seed1,
			Seed2:   seed2,
			Cluster: cluster,
		})
		if err != nil {
			return fmt.Errorf("marshal state json: %w", err)
		}
		if err := os.WriteFile(opts.jsonPath, body, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", opts.jsonPath, err)
		}
		fmt.Printf("wrote %s\n", opts.jsonPath)
	}

	return nil
}
