// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

import (
	"encoding/json"
	"sort"
)

// SimulationOutcome bundles everything a simulation run produces: the
// seeds that drove the PRNG, the generated galaxy, and one entry per
// requested home-system template size. It is the canonical state that
// the CLI's --json flag serializes.
type SimulationOutcome struct {
	Seed1     uint64
	Seed2     uint64
	Galaxy    *Galaxy
	Templates []TemplateOutcome
}

// TemplateOutcome is the result of one template-generation request.
// Template is nil when no candidate star with the given planet count
// produced a viable template; CandidateCount is the number of stars
// that were tried.
type TemplateOutcome struct {
	NumPlanets     int
	CandidateCount int
	Template       *HomeSystemTemplate
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
	Seed1     uint64        `json:"seed1"`
	Seed2     uint64        `json:"seed2"`
	Galaxy    galaxyDoc     `json:"galaxy"`
	Templates []templateDoc `json:"templates"`
}

type galaxyDoc struct {
	Radius  int         `json:"radius"`
	Systems []systemDoc `json:"systems"`
}

type systemDoc struct {
	ID    int       `json:"id"`
	Q     int       `json:"q"`
	R     int       `json:"r"`
	Stars []starDoc `json:"stars"`
}

type starDoc struct {
	ID         int         `json:"id"`
	Kind       string      `json:"kind"`
	Color      string      `json:"color"`
	Size       int         `json:"size"`
	NumPlanets int         `json:"num_planets"`
	Planets    []planetDoc `json:"planets"`
}

type planetDoc struct {
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

type templateDoc struct {
	NumPlanets     int                 `json:"num_planets"`
	CandidateCount int                 `json:"candidate_count"`
	Viable         bool                `json:"viable"`
	ViabilityScore int                 `json:"viability_score,omitempty"`
	SourceStarID   int                 `json:"source_star_id,omitempty"`
	Planets        []templatePlanetDoc `json:"planets,omitempty"`
}

type templatePlanetDoc struct {
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
		Seed1:     s.Seed1,
		Seed2:     s.Seed2,
		Templates: make([]templateDoc, 0, len(s.Templates)),
	}
	if s.Galaxy != nil {
		doc.Galaxy = buildGalaxyDoc(s.Galaxy)
	}
	for _, outcome := range s.Templates {
		doc.Templates = append(doc.Templates, buildTemplateDoc(outcome))
	}
	return doc
}

func buildGalaxyDoc(g *Galaxy) galaxyDoc {
	out := galaxyDoc{
		Radius:  g.Radius,
		Systems: make([]systemDoc, 0, len(g.Systems)),
	}
	for _, sys := range g.Systems {
		sd := systemDoc{
			ID:    sys.ID,
			Q:     sys.Hex.Q,
			R:     sys.Hex.R,
			Stars: make([]starDoc, 0, len(sys.Stars)),
		}
		for _, star := range sys.Stars {
			stard := starDoc{
				ID:         star.ID,
				Kind:       star.Kind.String(),
				Color:      star.Color.String(),
				Size:       star.Size,
				NumPlanets: star.NumPlanets,
				Planets:    make([]planetDoc, 0, len(star.Planets)),
			}
			for _, p := range star.Planets {
				stard.Planets = append(stard.Planets, planetDoc{
					Diameter:         p.Diameter,
					Density:          p.Density,
					Gravity:          p.Gravity,
					TemperatureClass: p.TemperatureClass,
					PressureClass:    p.PressureClass,
					Atmosphere:       sortGasMap(p.Gases),
					MiningDifficulty: p.MiningDifficulty,
				})
			}
			sd.Stars = append(sd.Stars, stard)
		}
		out.Systems = append(out.Systems, sd)
	}
	return out
}

func buildTemplateDoc(o TemplateOutcome) templateDoc {
	td := templateDoc{
		NumPlanets:     o.NumPlanets,
		CandidateCount: o.CandidateCount,
	}
	if o.Template == nil {
		return td
	}
	td.Viable = true
	td.ViabilityScore = o.Template.ViabilityScore
	td.SourceStarID = o.Template.SourceStarID
	td.Planets = make([]templatePlanetDoc, 0, len(o.Template.Planets))
	for _, p := range o.Template.Planets {
		td.Planets = append(td.Planets, templatePlanetDoc{
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
