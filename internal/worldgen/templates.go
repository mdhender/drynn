// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

import (
	"sort"

	"github.com/mdhender/drynn/internal/prng"
)

// TemplatePlanet captures the physical properties of one planet in a
// home-system template. Units follow design/home-system-template-design.md:
// gravity and mining difficulty are integers scaled by 100 (gravity 100 ==
// Earth gravity; mining difficulty 250 == MD 2.50). These units differ
// from worldgen.Planet, which stores gravity/density as float64 and applies
// an 11/5 fudge to mining difficulty; do not mix the two.
type TemplatePlanet struct {
	Diameter         int
	Gravity          int
	TemperatureClass int
	PressureClass    int
	MiningDifficulty int
	Atmosphere       []TemplateGas
	Special          int
}

// TemplateGas is one entry in a TemplatePlanet atmosphere.
type TemplateGas struct {
	Gas     AtmosphericGas
	Percent int
}

// HomeSystemTemplate is the output of one template-generation attempt.
// It is populated whether or not the attempt passes the viability gate;
// the caller inspects ViabilityScore to decide acceptance.
type HomeSystemTemplate struct {
	NumPlanets     int
	Planets        []TemplatePlanet
	ViabilityScore int
	SourceStarID   int
}

// GenerateHomeSystemTemplate picks a star from galaxy that has numPlanets
// planets, then returns the first viable home-system template produced by
// GenerateHomeSystemTemplateUntilViable. Returns nil if no matching star
// yields a viable template.
func GenerateHomeSystemTemplate(r *prng.PRNG, galaxy *Galaxy, numPlanets int) *HomeSystemTemplate {
	return GenerateHomeSystemTemplateUntilViable(r, galaxy, numPlanets)
}

// GenerateHomeSystemTemplateUntilViable collects every star in galaxy with
// exactly numPlanets planets, sorts them by Star.ID for determinism, and
// runs one template-generation attempt per star using the shared PRNG.
// It returns the first template whose viability score falls in the
// exclusive window (53, 57), or nil if the star slice is exhausted first.
//
// The star slice also bounds the attempt count — there is no separate cap.
func GenerateHomeSystemTemplateUntilViable(r *prng.PRNG, galaxy *Galaxy, numPlanets int) *HomeSystemTemplate {
	candidates := collectStarsWithPlanetCount(galaxy, numPlanets)
	for _, star := range candidates {
		template, score := generateHomeSystemTemplateAttempt(r, star)
		if score > 53 && score < 57 {
			return template
		}
	}
	return nil
}

func collectStarsWithPlanetCount(galaxy *Galaxy, numPlanets int) []*Star {
	var stars []*Star
	for _, sys := range galaxy.Systems {
		for _, star := range sys.Stars {
			if len(star.Planets) == numPlanets {
				stars = append(stars, star)
			}
		}
	}
	sort.Slice(stars, func(i, j int) bool { return stars[i].ID < stars[j].ID })
	return stars
}

// startDiameter and startTempClass seed planet generation. Index 0 is
// unused; index 5 represents the asteroid belt (fantasy values).
var startDiameter = [10]int{0, 5, 12, 13, 7, 20, 143, 121, 51, 49}
var startTempClass = [10]int{0, 29, 27, 11, 9, 8, 6, 5, 5, 3}

// generateHomeSystemTemplateAttempt runs one template-generation attempt
// for a star with len(star.Planets) planets. It always returns a non-nil
// template and its computed viability score; acceptance is the caller's
// job. See internal/worldgen/design/home-system-template-design.md.
func generateHomeSystemTemplateAttempt(r *prng.PRNG, star *Star) (*HomeSystemTemplate, int) {
	numPlanets := len(star.Planets)
	template := &HomeSystemTemplate{
		NumPlanets:   numPlanets,
		Planets:      make([]TemplatePlanet, 0, numPlanets),
		SourceStarID: star.ID,
	}

	earthLikeConsumed := false
	previousTC := 0
	hasPrevious := false

	for orbit := 1; orbit <= numPlanets; orbit++ {
		var tp TemplatePlanet

		// 1a. Starting values
		var baseValue int
		if numPlanets <= 3 {
			baseValue = 2*orbit + 1
		} else {
			baseValue = (9 * orbit) / numPlanets
		}
		if baseValue < 1 {
			baseValue = 1
		} else if baseValue >= len(startDiameter) {
			baseValue = len(startDiameter) - 1
		}
		tp.Diameter = startDiameter[baseValue]
		tp.TemperatureClass = startTempClass[baseValue]

		// 1b. Randomize diameter
		dieSize := tp.Diameter / 4
		if dieSize < 2 {
			dieSize = 2
		}
		for i := 0; i < 4; i++ {
			delta := r.Roll(1, dieSize)
			if r.Roll(1, 100) > 50 {
				tp.Diameter += delta
			} else {
				tp.Diameter -= delta
			}
		}
		for tp.Diameter < 3 {
			tp.Diameter += r.Roll(1, 4)
		}

		// 1c. Gas-giant test
		isGasGiant := tp.Diameter > 40

		// 1d. Density and gravity (both × 100)
		var density int
		if isGasGiant {
			density = 58 + r.Roll(1, 56) + r.Roll(1, 56)
		} else {
			density = 368 + r.Roll(1, 101) + r.Roll(1, 101)
		}
		tp.Gravity = (density * tp.Diameter) / 72

		// 1e. Randomize temperature class
		dieSize = tp.TemperatureClass / 4
		if dieSize < 2 {
			dieSize = 2
		}
		nRolls := r.Roll(1, 3) + r.Roll(1, 3) + r.Roll(1, 3)
		for i := 0; i < nRolls; i++ {
			delta := r.Roll(1, dieSize)
			if r.Roll(1, 100) > 50 {
				tp.TemperatureClass += delta
			} else {
				tp.TemperatureClass -= delta
			}
		}
		if isGasGiant {
			for tp.TemperatureClass < 3 {
				tp.TemperatureClass += r.Roll(1, 2)
			}
			for tp.TemperatureClass > 7 {
				tp.TemperatureClass -= r.Roll(1, 2)
			}
		} else {
			for tp.TemperatureClass < 1 {
				tp.TemperatureClass += r.Roll(1, 3)
			}
			for tp.TemperatureClass > 30 {
				tp.TemperatureClass -= r.Roll(1, 3)
			}
		}

		// 1f. Warm small systems
		if numPlanets < 4 && orbit <= 2 {
			for tp.TemperatureClass < 12 {
				tp.TemperatureClass += r.Roll(1, 4)
			}
		}

		// 1g. Enforce temperature ordering (non-increasing outward)
		if hasPrevious && previousTC < tp.TemperatureClass {
			tp.TemperatureClass = previousTC
		}

		// 1h. Earth-like override — fires once, on the first planet (in
		// orbit order) whose TC <= 11. Replaces all prior values for this
		// planet and skips steps 1i–1k.
		if !earthLikeConsumed && tp.TemperatureClass <= 11 {
			tp.Diameter = 11 + r.Roll(1, 3)
			tp.Gravity = 93 + r.Roll(1, 11) + r.Roll(1, 11) + r.Roll(1, 5)
			tp.TemperatureClass = 9 + r.Roll(1, 3)
			tp.PressureClass = 8 + r.Roll(1, 3)
			tp.MiningDifficulty = 208 + r.Roll(1, 11) + r.Roll(1, 11)
			tp.Special = 1
			tp.Atmosphere = buildEarthLikeAtmosphere(r)
			earthLikeConsumed = true
		} else {
			// 1i. Pressure class (non-earth-like)
			tp.PressureClass = tp.Gravity / 10
			dieSize = tp.PressureClass / 4
			if dieSize < 2 {
				dieSize = 2
			}
			nRolls = r.Roll(1, 3) + r.Roll(1, 3) + r.Roll(1, 3)
			for i := 0; i < nRolls; i++ {
				delta := r.Roll(1, dieSize)
				if r.Roll(1, 100) > 50 {
					tp.PressureClass += delta
				} else {
					tp.PressureClass -= delta
				}
			}
			if isGasGiant {
				for tp.PressureClass < 11 {
					tp.PressureClass += r.Roll(1, 3)
				}
				for tp.PressureClass > 29 {
					tp.PressureClass -= r.Roll(1, 3)
				}
			} else {
				for tp.PressureClass < 0 {
					tp.PressureClass += r.Roll(1, 3)
				}
				for tp.PressureClass > 12 {
					tp.PressureClass -= r.Roll(1, 3)
				}
			}
			if tp.Gravity < 10 || tp.TemperatureClass < 2 || tp.TemperatureClass > 27 {
				tp.PressureClass = 0
			}

			// 1j. Atmosphere (non-earth-like) — vacuum worlds stay empty
			if tp.PressureClass > 0 {
				tp.Atmosphere = rollNonEarthAtmosphere(r, tp.TemperatureClass)
			}

			// 1k. Mining difficulty (non-earth-like), easier formula with
			// no 11/5 fudge factor. Accept values in [30, 1000].
			for {
				val := (r.Roll(1, 3) + r.Roll(1, 3) + r.Roll(1, 3) - r.Roll(1, 4)) *
					r.Roll(1, tp.Diameter)
				val += r.Roll(1, 20) + r.Roll(1, 20)
				if val >= 30 && val <= 1000 {
					tp.MiningDifficulty = val
					break
				}
			}
		}

		template.Planets = append(template.Planets, tp)
		previousTC = tp.TemperatureClass
		hasPrevious = true
	}

	// 2. Viability check
	score := computeViabilityScore(template.Planets)
	template.ViabilityScore = score
	return template, score
}

// buildEarthLikeAtmosphere follows the earth-like recipe: optional NH3
// (1-in-3), a reserved N2 placeholder filled last, optional CO2 (1-in-3),
// O2 at 11–30%, with N2 taking whatever percentage is left over.
func buildEarthLikeAtmosphere(r *prng.PRNG) []TemplateGas {
	var gases []TemplateGas
	total := 0

	if r.Roll(1, 3) == 1 {
		pct := r.Roll(1, 30)
		gases = append(gases, TemplateGas{Gas: GasNH3, Percent: pct})
		total += pct
	}

	nitroIndex := len(gases)
	gases = append(gases, TemplateGas{Gas: GasN2, Percent: 0})

	if r.Roll(1, 3) == 1 {
		pct := r.Roll(1, 30)
		gases = append(gases, TemplateGas{Gas: GasCO2, Percent: pct})
		total += pct
	}

	o2Pct := r.Roll(1, 20) + 10
	gases = append(gases, TemplateGas{Gas: GasO2, Percent: o2Pct})
	total += o2Pct

	gases[nitroIndex].Percent = 100 - total
	return gases
}

// rollNonEarthAtmosphere picks up to four gases from a five-wide window
// centered on the planet's temperature class, rolls quantities per the
// per-gas skip-and-magnitude rules, and normalizes to percentages.
func rollNonEarthAtmosphere(r *prng.PRNG, tc int) []TemplateGas {
	firstGas := (100 * tc) / 225
	if firstGas < 1 {
		firstGas = 1
	} else if firstGas > 9 {
		firstGas = 9
	}

	numWanted := (r.Roll(1, 4) + r.Roll(1, 4)) / 2

	var picked []TemplateGas
	var quantities []int
	totalQty := 0

	for len(picked) == 0 {
		for gasID := firstGas; gasID <= firstGas+4; gasID++ {
			if len(picked) == numWanted {
				break
			}
			g := AtmosphericGas(gasID)
			var qty int
			if g == GasHe {
				if r.Roll(1, 3) > 1 {
					continue
				}
				if tc > 5 {
					continue
				}
				qty = r.Roll(1, 20)
			} else {
				if r.Roll(1, 3) == 3 {
					continue
				}
				if g == GasO2 {
					qty = r.Roll(1, 50)
				} else {
					qty = r.Roll(1, 100)
				}
			}
			picked = append(picked, TemplateGas{Gas: g, Percent: qty})
			quantities = append(quantities, qty)
			totalQty += qty
		}
	}

	totalPct := 0
	for i := range picked {
		pct := (100 * quantities[i]) / totalQty
		picked[i].Percent = pct
		totalPct += pct
	}
	picked[0].Percent += 100 - totalPct
	return picked
}

// computeViabilityScore returns 0 if the system has no planet with
// Special == 1 (no earth-like candidate). Otherwise it sums the per-planet
// contribution 20000 / ((3 + ALSN) * (50 + miningDifficulty)) across
// every planet, with the home planet compared against itself.
func computeViabilityScore(planets []TemplatePlanet) int {
	var home *TemplatePlanet
	for i := range planets {
		if planets[i].Special == 1 {
			home = &planets[i]
			break
		}
	}
	if home == nil {
		return 0
	}
	score := 0
	for i := range planets {
		lsn := approximateLSN(planets[i], *home)
		score += 20000 / ((3 + lsn) * (50 + planets[i].MiningDifficulty))
	}
	return score
}

// approximateLSN is the template-time life-support estimate. It differs
// from the runtime LSN used during empire planet scans: it assumes the
// species breathes oxygen, and treats any gas absent from the home
// planet as poison.
func approximateLSN(candidate, home TemplatePlanet) int {
	lsn := 0
	lsn += 2 * absInt(candidate.TemperatureClass-home.TemperatureClass)
	lsn += 2 * absInt(candidate.PressureClass-home.PressureClass)
	lsn += 2 // start by assuming no oxygen

	homeGases := make(map[AtmosphericGas]bool, len(home.Atmosphere))
	for _, g := range home.Atmosphere {
		homeGases[g.Gas] = true
	}
	for _, g := range candidate.Atmosphere {
		if g.Gas == GasO2 {
			lsn -= 2
		}
		if !homeGases[g.Gas] {
			lsn += 2
		}
	}
	return lsn
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
