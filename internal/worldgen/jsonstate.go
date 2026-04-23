// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

import (
	"encoding/json"
	"sort"
)

// SimulationOutcome bundles everything a simulation run produces: the
// seeds that drove the PRNG and the generated cluster (which carries
// its own home-star-template library). It is the canonical state that
// the CLI's --json flag serializes.
type SimulationOutcome struct {
	Seed1   uint64
	Seed2   uint64
	Cluster *Cluster
}

// MarshalSimulationJSON returns a pretty-printed, deterministic JSON
// encoding of the simulation outcome. The output is suitable for golden
// file tests: all maps (Planet.Gases) are converted to percent-desc /
// gas-asc sorted slices, and all other data is emitted in its original
// generation order.
func MarshalSimulationJSON(s SimulationOutcome) ([]byte, error) {
	doc := buildStateDoc(s)
	return json.MarshalIndent(doc, "", "  ")
}

type stateDoc struct {
	Seed1   uint64     `json:"seed1"`
	Seed2   uint64     `json:"seed2"`
	Cluster clusterDoc `json:"cluster"`
}

type clusterDoc struct {
	Radius            int                   `json:"radius"`
	Systems           []systemDoc           `json:"systems"`
	Stars             []starDoc             `json:"stars"`
	Planets           []planetDoc           `json:"planets"`
	HomeStarTemplates []homeStarTemplateDoc `json:"home_star_templates,omitempty"`
}

type systemDoc struct {
	ID         int  `json:"id"`
	Q          int  `json:"q"`
	R          int  `json:"r"`
	HomeSystem bool `json:"home_system"`
}

type starDoc struct {
	ID         int    `json:"id"`
	SystemID   int    `json:"system_id"`
	Kind       string `json:"kind"`
	Color      string `json:"color"`
	Size       int    `json:"size"`
	NumPlanets int    `json:"num_planets"`
}

type planetDoc struct {
	ID               int      `json:"id"`
	StarID           int      `json:"star_id"`
	Orbit            int      `json:"orbit"`
	Kind             string   `json:"kind"`
	Diameter         int      `json:"diameter"`
	Density          float64  `json:"density"`
	Gravity          float64  `json:"gravity"`
	TemperatureClass int      `json:"temperature_class"`
	PressureClass    int      `json:"pressure_class"`
	Atmosphere       []gasDoc `json:"atmosphere"`
	MiningDifficulty float64  `json:"mining_difficulty"`
}

type gasDoc struct {
	Gas     string `json:"gas"`
	Percent int    `json:"percent"`
}

type homeStarTemplateDoc struct {
	NumPlanets     int                 `json:"num_planets"`
	Attempts       int                 `json:"attempts"`
	BestScore      int                 `json:"best_score"`
	Viable         bool                `json:"viable"`
	ViabilityScore int                 `json:"viability_score,omitempty"`
	Planets        []templatePlanetDoc `json:"planets,omitempty"`
}

type templatePlanetDoc struct {
	Kind             string   `json:"kind"`
	Diameter         int      `json:"diameter"`
	Gravity          int      `json:"gravity"`
	TemperatureClass int      `json:"temperature_class"`
	PressureClass    int      `json:"pressure_class"`
	MiningDifficulty int      `json:"mining_difficulty"`
	Atmosphere       []gasDoc `json:"atmosphere"`
	Special          int      `json:"special"`
}

func buildStateDoc(s SimulationOutcome) stateDoc {
	doc := stateDoc{
		Seed1: s.Seed1,
		Seed2: s.Seed2,
	}
	if s.Cluster != nil {
		doc.Cluster = buildClusterDoc(s.Cluster)
	}
	return doc
}

func buildClusterDoc(g *Cluster) clusterDoc {
	out := clusterDoc{
		Radius:  g.Radius,
		Systems: make([]systemDoc, 0, len(g.Systems)),
		Stars:   make([]starDoc, 0, len(g.Stars)),
		Planets: make([]planetDoc, 0, len(g.Planets)),
	}
	for n := 3; n <= 9; n++ {
		if n >= len(g.HomeStarTemplates) {
			break
		}
		outcome := g.HomeStarTemplates[n]
		if outcome == nil {
			continue
		}
		out.HomeStarTemplates = append(out.HomeStarTemplates, buildHomeStarTemplateDoc(outcome))
	}
	for _, sys := range g.Systems {
		out.Systems = append(out.Systems, systemDoc{
			ID:         sys.ID,
			Q:          sys.Hex.Q,
			R:          sys.Hex.R,
			HomeSystem: sys.HomeSystem,
		})
	}
	for _, star := range g.Stars {
		out.Stars = append(out.Stars, starDoc{
			ID:         star.ID,
			SystemID:   star.SystemID,
			Kind:       star.Kind.String(),
			Color:      star.Color.String(),
			Size:       star.Size,
			NumPlanets: star.NumPlanets,
		})
	}
	for _, p := range g.Planets {
		out.Planets = append(out.Planets, planetDoc{
			ID:               p.ID,
			StarID:           p.StarID,
			Orbit:            p.Orbit,
			Kind:             p.Kind.String(),
			Diameter:         p.Diameter,
			Density:          p.Density,
			Gravity:          p.Gravity,
			TemperatureClass: p.TemperatureClass,
			PressureClass:    p.PressureClass,
			Atmosphere:       sortGasMap(p.Gases),
			MiningDifficulty: p.MiningDifficulty,
		})
	}
	return out
}

func buildHomeStarTemplateDoc(o *HomeStarTemplateOutcome) homeStarTemplateDoc {
	td := homeStarTemplateDoc{
		NumPlanets: o.NumPlanets,
		Attempts:   o.Attempts,
		BestScore:  o.BestScore,
	}
	if o.Template == nil {
		return td
	}
	td.Viable = true
	td.ViabilityScore = o.Template.ViabilityScore
	td.Planets = make([]templatePlanetDoc, 0, len(o.Template.Planets))
	for _, p := range o.Template.Planets {
		td.Planets = append(td.Planets, templatePlanetDoc{
			Kind:             p.Kind.String(),
			Diameter:         p.Diameter,
			Gravity:          p.Gravity,
			TemperatureClass: p.TemperatureClass,
			PressureClass:    p.PressureClass,
			MiningDifficulty: p.MiningDifficulty,
			Atmosphere:       templateGasesToDoc(p.Atmosphere),
			Special:          p.Special,
		})
	}
	return td
}

// sortGasMap converts the map-backed atmosphere on a Planet into a
// deterministic slice. Ordering matches the HTML renderer: percent
// descending, then gas id ascending.
func sortGasMap(gases map[AtmosphericGas]int) []gasDoc {
	if len(gases) == 0 {
		return []gasDoc{}
	}
	keys := make([]AtmosphericGas, 0, len(gases))
	for k := range gases {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if gases[keys[i]] != gases[keys[j]] {
			return gases[keys[i]] > gases[keys[j]]
		}
		return keys[i] < keys[j]
	})
	out := make([]gasDoc, 0, len(keys))
	for _, k := range keys {
		out = append(out, gasDoc{Gas: k.String(), Percent: gases[k]})
	}
	return out
}

// templateGasesToDoc preserves the generator's original ordering — the
// slice is already deterministic, we just relabel gas ids as strings.
func templateGasesToDoc(atm []TemplateGas) []gasDoc {
	out := make([]gasDoc, 0, len(atm))
	for _, g := range atm {
		out = append(out, gasDoc{Gas: g.Gas.String(), Percent: g.Percent})
	}
	return out
}
