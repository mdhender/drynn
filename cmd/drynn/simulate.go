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

// runSimulate simulates the GM's interactive workflow: generate a galaxy,
// write galaxy.html, then produce a home-system template for each planet
// count 3..9 and write home-system-{N}.html. When --json is set, the full
// run state is also written to a deterministic JSON file suitable for
// golden file tests. The same PRNG stream is threaded through galaxy and
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

	galaxy, err := worldgen.Generate(
		worldgen.WithDesiredRadius(opts.radius),
		worldgen.WithDesiredNumberOfSystems(opts.systems),
		worldgen.WithMinimumDistance(opts.minDistance),
		worldgen.WithMerge(opts.merge),
		worldgen.WithPRNG(rng),
	)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		if galaxy == nil {
			return err
		}
	}

	fmt.Printf("generated %d systems (%d stars, %d planets) in radius-%d galaxy\n",
		len(galaxy.Systems),
		worldgen.TotalStars(galaxy.Systems),
		totalPlanets(galaxy),
		galaxy.Radius,
	)

	galaxyPath := filepath.Join(opts.outDir, "galaxy.html")
	if err := os.WriteFile(galaxyPath, galaxy.ToHTML(0, false, true), 0o644); err != nil {
		return fmt.Errorf("write galaxy.html: %w", err)
	}
	fmt.Printf("wrote %s\n", galaxyPath)

	outcomes := make([]worldgen.TemplateOutcome, 0, 7)
	for n := 3; n <= 9; n++ {
		candidates := starsWithPlanetCount(galaxy, n)
		template := worldgen.GenerateHomeSystemTemplate(rng, galaxy, n)
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
			Galaxy:    galaxy,
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

func starsWithPlanetCount(g *worldgen.Galaxy, n int) []*worldgen.Star {
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

func totalPlanets(g *worldgen.Galaxy) int {
	total := 0
	for _, sys := range g.Systems {
		for _, star := range sys.Stars {
			total += len(star.Planets)
		}
	}
	return total
}
