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

// runSimulate simulates the GM's interactive workflow: generate a cluster,
// write cluster.html, then produce a home-system template for each planet
// count 3..9 and write home-system-{N}.html. When --json is set, the full
// run state is also written to a deterministic JSON file suitable for
// golden file tests. The same PRNG stream is threaded through cluster and
// template generation so the full run is reproducible from the seeds.
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

	outcomes := make([]worldgen.TemplateOutcome, 0, 7)
	for n := 3; n <= 9; n++ {
		candidates := starsWithPlanetCount(cluster, n)
		template := worldgen.GenerateHomeSystemTemplate(rng, cluster, n)
		outcomes = append(outcomes, worldgen.TemplateOutcome{
			NumPlanets:     n,
			CandidateCount: len(candidates),
			Template:       template,
		})

		path := filepath.Join(opts.outDir, fmt.Sprintf("home-system-%d.html", n))
		var html []byte
		if template == nil {
			html = worldgen.HomeSystemTemplateUnavailableHTML(n, len(candidates))
			fmt.Printf("n=%d: no viable template (tried %d candidate stars)\n", n, len(candidates))
		} else {
			html = template.ToHTML()
			fmt.Printf("n=%d: viable template (source star #%d, score %d, tried from %d candidates)\n",
				n, template.SourceStarID, template.ViabilityScore, len(candidates))
		}
		if err := os.WriteFile(path, html, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		fmt.Printf("wrote %s\n", path)
	}

	if opts.jsonPath != "" {
		body, err := worldgen.MarshalSimulationJSON(worldgen.SimulationOutcome{
			Seed1:     seed1,
			Seed2:     seed2,
			Cluster:   cluster,
			Templates: outcomes,
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

func starsWithPlanetCount(g *worldgen.Cluster, n int) []*worldgen.Star {
	var out []*worldgen.Star
	for _, sys := range g.Systems {
		for _, star := range sys.Stars {
			if len(star.Planets) == n {
				out = append(out, star)
			}
		}
	}
	return out
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
