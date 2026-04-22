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
}

// runSimulate simulates the GM's interactive workflow: generate a cluster
// (hex placement + stars + planets) and run the stage-1 home-star-template
// driver, writing HTML reports for the cluster and for each of the seven
// template slots. When --json is set, the full run state is also written
// to a deterministic JSON file suitable for golden file tests.
func runSimulate(opts simulateOpts) error {
	seed1, seed2 := opts.seed1, opts.seed2
	if opts.randomSeeds {
		seed1, seed2 = cryptoRandSeeds()
	}
	fmt.Printf("seeds: %d %d\n", seed1, seed2)

	if err := os.MkdirAll(opts.outDir, 0o755); err != nil {
		return fmt.Errorf("create out dir: %w", err)
	}

	rng := prng.NewFromSeed(seed1, seed2)

	cluster, err := worldgen.Generate(
		worldgen.WithDesiredRadius(opts.radius),
		worldgen.WithDesiredNumberOfSystems(opts.systems),
		worldgen.WithMinimumDistance(opts.minDistance),
		worldgen.WithMerge(opts.merge),
		worldgen.WithPRNG(rng),
	)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		if cluster == nil {
			return err
		}
	}

	fmt.Printf("generated %d systems (%d stars, %d planets) in radius-%d cluster\n",
		len(cluster.Systems),
		worldgen.TotalStars(cluster.Systems),
		totalPlanets(cluster),
		cluster.Radius,
	)

	clusterPath := filepath.Join(opts.outDir, "cluster.html")
	if err := os.WriteFile(clusterPath, cluster.ToHTML(0, false, true), 0o644); err != nil {
		return fmt.Errorf("write cluster.html: %w", err)
	}
	fmt.Printf("wrote %s\n", clusterPath)

	for n := 3; n <= 9; n++ {
		outcome := cluster.HomeStarTemplates[n]
		path := filepath.Join(opts.outDir, fmt.Sprintf("home-system-%d.html", n))
		var html []byte
		if outcome.Template == nil {
			html = worldgen.HomeStarTemplateUnavailableHTML(n, outcome.Attempts, outcome.BestScore)
			fmt.Printf("n=%d: no viable template (%d attempts, best score %d)\n",
				n, outcome.Attempts, outcome.BestScore)
		} else {
			html = outcome.Template.ToHTML()
			fmt.Printf("n=%d: viable template (score %d, %d attempts)\n",
				n, outcome.Template.ViabilityScore, outcome.Attempts)
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

func totalPlanets(g *worldgen.Cluster) int {
	total := 0
	for _, sys := range g.Systems {
		for _, star := range sys.Stars {
			total += len(star.Planets)
		}
	}
	return total
}
