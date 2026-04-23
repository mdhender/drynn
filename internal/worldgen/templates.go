// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

import "github.com/mdhender/drynn/internal/prng"

// TemplatePlanet captures the physical properties of one planet in a
// home-system template. Units follow design/home-system-template-design.md:
// gravity and mining difficulty are integers scaled by 100 (gravity 100 ==
// Earth gravity; mining difficulty 250 == MD 2.50). These units differ
// from worldgen.Planet, which stores gravity/density as float64 and applies
// an 11/5 fudge to mining difficulty; do not mix the two.
type TemplatePlanet struct {
	Kind             PlanetKind
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

// HomeStarTemplate is the output of one template-generation attempt.
// It is populated whether or not the attempt passes the viability gate;
// the caller inspects ViabilityScore to decide acceptance.
type HomeStarTemplate struct {
	NumPlanets     int
	Planets        []TemplatePlanet
	ViabilityScore int
}

// ViabilityWindow is the acceptance band for a template's viability
// score. Scores strictly greater than Min and strictly less than Max
// are accepted. The bounds are exclusive on both ends.
type ViabilityWindow struct {
	Min, Max int
}

// Accepts reports whether score falls strictly between Min and Max.
func (w ViabilityWindow) Accepts(score int) bool {
	return score > w.Min && score < w.Max
}

// DefaultViabilityWindow matches the historical (53, 57) exclusive band:
// only scores of 54, 55, or 56 are accepted.
var DefaultViabilityWindow = ViabilityWindow{Min: 53, Max: 57}

// DefaultMaxCandidateRolls is the stage-1 runtime budget, counted in
// candidate-star rolls. Every candidate the driver rolls counts —
// including those discarded because the matching slot is already
// filled or the planet count is outside [3, 9].
const DefaultMaxCandidateRolls = 10_000

// HomeStarTemplateOutcome is the stage-1 result for one planet count.
// Template is nil when the candidate budget was exhausted before any
// viable template was accepted for this count. Attempts counts only
// calls into generateHomeStarTemplateAttempt (matching planet count);
// candidates skipped because the slot was already filled do not count.
// BestScore records the highest viability score seen during those
// attempts — useful when Template is nil to show the GM how close the
// driver came to producing a viable template.
type HomeStarTemplateOutcome struct {
	NumPlanets int
	Template   *HomeStarTemplate
	Attempts   int
	BestScore  int
}

// GenerateHomeStarTemplates runs the stage-1 driver described in
// reference/home-system-templates.md. It rolls ephemeral candidate
// stars via rollStar, attempts template generation whenever the
// candidate's planet count matches an empty slot in [3, 9], and
// terminates when all seven slots are filled or the candidate-roll
// count reaches maxCandidateRolls.
//
// If maxCandidateRolls is <= 0, DefaultMaxCandidateRolls is used.
// The returned slice always has length 10 with indexes 0..2 nil and
// 3..9 non-nil.
func GenerateHomeStarTemplates(rng *prng.PRNG, window ViabilityWindow, maxCandidateRolls int) []*HomeStarTemplateOutcome {
	if maxCandidateRolls <= 0 {
		maxCandidateRolls = DefaultMaxCandidateRolls
	}

	outcomes := make([]*HomeStarTemplateOutcome, 10)
	for n := 3; n <= 9; n++ {
		outcomes[n] = &HomeStarTemplateOutcome{NumPlanets: n}
	}

	filled := 0
	for rolls := 0; rolls < maxCandidateRolls && filled < 7; rolls++ {
		candidate, _ := rollStar(rng)
		n := candidate.NumPlanets
		if n < 3 || n > 9 {
			continue
		}
		slot := outcomes[n]
		if slot.Template != nil {
			continue
		}
		template, score := generateHomeStarTemplateAttempt(rng, n)
		slot.Attempts++
		if score > slot.BestScore {
			slot.BestScore = score
		}
		if window.Accepts(score) {
			slot.Template = template
			filled++
		}
	}
	return outcomes
}

// startDiameter and startTempClass seed planet generation. Index 0 is
// unused; index 5 represents the asteroid belt (fantasy values).
var startDiameter = [10]int{0, 5, 12, 13, 7, 20, 143, 121, 51, 49}
var startTempClass = [10]int{0, 29, 27, 11, 9, 8, 6, 5, 5, 3}

// generateHomeStarTemplateAttempt runs one template-generation attempt
// for numPlanets planets. It always returns a non-nil template and its
// computed viability score; acceptance is the caller's job. See
// internal/worldgen/design/home-system-template-design.md.
func generateHomeStarTemplateAttempt(r *prng.PRNG, numPlanets int) (*HomeStarTemplate, int) {
	template := &HomeStarTemplate{
		NumPlanets: numPlanets,
		Planets:    make([]TemplatePlanet, 0, numPlanets),
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
		tp.Kind = planetKindFromDiameter(tp.Diameter)
		isGasGiant := tp.Kind == KindGasGiant

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
			tp.Kind = planetKindFromDiameter(tp.Diameter)
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
