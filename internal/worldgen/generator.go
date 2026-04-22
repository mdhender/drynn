// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

import (
	"fmt"
	"math"

	"github.com/mdhender/drynn/internal/prng"
)

func Generate(options ...Option) (*Galaxy, error) {
	g := &Generator{
		desiredNumSystems: 100,
		desiredRadius:     15,
		minimumDistance:   2,
		merge:             true,
		r:                 prng.NewFromSeed(10, 10),
	}
	for _, opt := range options {
		if err := opt(g); err != nil {
			return nil, err
		}
	}

	placements, err := placeHexSystems(g.r, g.desiredRadius, g.desiredNumSystems, g.minimumDistance, g.merge)
	if err != nil {
		return nil, fmt.Errorf("hex gen: %w", err)
	}

	galaxy := &Galaxy{
		Radius:  g.desiredRadius,
		Systems: make([]*System, 0, len(placements)),
	}
	nextSystemID, nextStarID := 1, 1
	for _, p := range placements {
		sys := &System{
			ID:    nextSystemID,
			Hex:   p.Hex,
			Stars: make([]*Star, 0, p.Stars),
		}
		nextSystemID++
		for i := 0; i < p.Stars; i++ {
			star := g.rollStar()
			star.ID = nextStarID
			nextStarID++
			sys.Stars = append(sys.Stars, star)
		}
		galaxy.Systems = append(galaxy.Systems, sys)
	}
	return galaxy, nil
}

func (g *Generator) rollStar() *Star {
	star := &Star{}

	// determine star type randomly
	switch g.r.Roll(1, 10) {
	case 1: // 10% dwarf
		star.Kind = StarDwarf
	case 2: // 10% degenerate
		star.Kind = StarDegenerate
	case 3: // 10% giant
		star.Kind = StarGiant
	default: // 70% main sequence
		star.Kind = StarMainSequence
	}

	// determine star color randomly
	switch g.r.Roll(1, 7) {
	case 1:
		star.Color = ColorBlue
	case 2:
		star.Color = ColorBlueWhite
	case 3:
		star.Color = ColorWhite
	case 4:
		star.Color = ColorYellowWhite
	case 5:
		star.Color = ColorYellow
	case 6:
		star.Color = ColorOrange
	case 7:
		star.Color = ColorRed
	default:
		panic("assert(max(StarColor) == ColorRed)")
	}

	// determine star size randomly
	star.Size = g.r.D10(1) - 1

	// determine number of planets orbiting this star.
	// The -2 is a bias to offset the multiple die rolls so the average
	// planet count lands in a reasonable range (roughly 1–9).
	star.NumPlanets = -2
	var sizeOfDie int // used to generate number of planets; set by color of star
	switch star.Color {
	case ColorBlue:
		sizeOfDie = 8
	case ColorBlueWhite:
		sizeOfDie = 7
	case ColorWhite:
		sizeOfDie = 6
	case ColorYellowWhite:
		sizeOfDie = 5
	case ColorYellow:
		sizeOfDie = 4
	case ColorOrange:
		sizeOfDie = 3
	case ColorRed:
		sizeOfDie = 2
	default:
		panic(fmt.Sprintf("assert(star.Color != %d)", star.Color))
	}
	var numberOfRolls int // used to generate number of planets; set by type of star
	switch star.Kind {
	case StarDwarf:
		numberOfRolls = 1
	case StarDegenerate, StarMainSequence:
		numberOfRolls = 2
	case StarGiant:
		numberOfRolls = 3
	default:
		panic(fmt.Sprintf("assert(star.Kind != %d)", star.Kind))
	}
	for i := 1; i <= numberOfRolls; i++ {
		star.NumPlanets += g.r.Roll(1, sizeOfDie)
	}
	// KNOWN DEFECT: these clamping loops use random increments, burning an
	// unpredictable number of RNG calls. This makes reproducibility fragile
	// if roll ranges change. Needs a design review to replace with bounded
	// clamping.
	for star.NumPlanets < 1 { // bump up to a minimum of 1
		star.NumPlanets += g.r.Roll(1, 2)
	}
	for star.NumPlanets > 9 { // bump down to a maximum of 9
		star.NumPlanets -= g.r.Roll(1, 3)
	}

	var previousPlanet *Planet
	for orbit := 1; orbit <= star.NumPlanets; orbit++ {
		p := g.rollPlanet(star, orbit, previousPlanet)
		star.Planets = append(star.Planets, p)
		previousPlanet = p
	}

	return star
}

// rollPlanet generates a single planet at the given orbit around star.
// previousPlanet is used to enforce temperature ordering (may be nil for orbit 1).
func (g *Generator) rollPlanet(star *Star, orbit int, previousPlanet *Planet) *Planet {
	seedDiameter := []int{0, 5, 12, 13, 7, 20, 143, 121, 51, 49} // thousands of km
	seedTemperatureClass := []int{0, 29, 27, 11, 9, 8, 6, 5, 5, 3}

	// determine seed index; bias towards "earth" zones
	var seedIndex int
	if star.NumPlanets <= 3 {
		seedIndex = 2*orbit + 1
	} else {
		seedIndex = 9 * orbit / star.NumPlanets
	}

	p := &Planet{
		Diameter:         seedDiameter[seedIndex],
		TemperatureClass: seedTemperatureClass[seedIndex],
		Gases:            make(map[AtmosphericGas]int),
	}

	// randomize diameter
	dieSize := p.Diameter / 4
	if dieSize < 2 {
		dieSize = 2
	}
	for n := 4; n > 0; n-- {
		delta := g.r.Roll(1, dieSize)
		if g.r.D100(1) > 50 {
			p.Diameter += delta
		} else {
			p.Diameter -= delta
		}
	}
	// KNOWN DEFECT: clamping loop uses random increments. See rollStar.
	for p.Diameter < 3 { // bump up to a minimum of 3
		p.Diameter += g.r.D4(1)
	}

	isGasGiant := p.Diameter > 40

	// compute density
	if isGasGiant { // range 0.6 to 1.7
		p.Density = float64(58+g.r.Roll(1, 56)+g.r.Roll(1, 56)) / 100
	} else { // range 3.7 to 5.7
		p.Density = float64(368+g.r.Roll(1, 101)+g.r.Roll(1, 101)) / 100
	}

	// compute gravity (the divisor 72 is calibrated so that Earth's values
	// (density=5.50, diameter=13) yield gravity ~= 1.0)
	p.Gravity = p.Density * float64(p.Diameter) / 72

	// randomize temperature class
	dieSize = p.TemperatureClass / 4
	if dieSize < 2 {
		dieSize = 2
	}
	numberOfRolls := g.r.Roll(1, 3) + g.r.Roll(1, 3) + g.r.Roll(1, 3)
	for ; numberOfRolls > 0; numberOfRolls-- {
		delta := g.r.Roll(1, dieSize)
		if g.r.D100(1) > 50 {
			p.TemperatureClass += delta
		} else {
			p.TemperatureClass -= delta
		}
	}
	// clamp by planet category
	minTC, maxTC := 1, 30
	if isGasGiant {
		minTC, maxTC = 3, 7
	}
	// KNOWN DEFECT: clamping loop uses random increments. See rollStar.
	for p.TemperatureClass < minTC {
		p.TemperatureClass += g.r.Roll(1, 2)
	}
	for p.TemperatureClass > maxTC {
		p.TemperatureClass -= g.r.Roll(1, 2)
	}

	// warm small systems
	if star.NumPlanets < 4 && orbit < 3 {
		// KNOWN DEFECT: clamping loop uses random increments. See rollStar.
		for p.TemperatureClass < 12 {
			p.TemperatureClass += g.r.Roll(1, 4)
		}
	}

	// enforce temperature ordering.
	// Planets farther from the star must not be warmer than planets closer
	// to it. The comparison is against the previous planet's final
	// (stored) temperature class.
	if previousPlanet != nil && previousPlanet.TemperatureClass < p.TemperatureClass {
		p.TemperatureClass = previousPlanet.TemperatureClass
	}

	// randomize pressure class
	p.PressureClass = int(math.Floor(p.Gravity * 10))
	dieSize = p.PressureClass / 4
	if dieSize < 2 {
		dieSize = 2
	}
	numberOfRolls = g.r.Roll(1, 3) + g.r.Roll(1, 3) + g.r.Roll(1, 3)
	for ; numberOfRolls > 0; numberOfRolls-- {
		delta := g.r.Roll(1, dieSize)
		if g.r.D100(1) > 50 {
			p.PressureClass += delta
		} else {
			p.PressureClass -= delta
		}
	}
	// clamp by planet category
	minPC, maxPC := 0, 12
	if isGasGiant {
		minPC, maxPC = 11, 29
	}
	// KNOWN DEFECT: clamping loop uses random increments. See rollStar.
	for p.PressureClass < minPC {
		p.PressureClass += g.r.Roll(1, 3)
	}
	for p.PressureClass > maxPC {
		p.PressureClass -= g.r.Roll(1, 3)
	}

	// randomize atmosphere. a pressure class of 0 means a vacuum, so skip
	// gas selection and leave p.Gases empty.
	if p.PressureClass > 0 {
		numberOfGasesWanted := g.r.D4(2) / 2
		var minGasIndex, maxGasIndex AtmosphericGas
		switch n := 100 * p.TemperatureClass / 225; {
		case n <= 1:
			minGasIndex, maxGasIndex = 1, 5
		case n == 2:
			minGasIndex, maxGasIndex = 2, 6
		case n == 3:
			minGasIndex, maxGasIndex = 3, 7
		case n == 4:
			minGasIndex, maxGasIndex = 4, 8
		case n == 5:
			minGasIndex, maxGasIndex = 5, 9
		case n == 6:
			minGasIndex, maxGasIndex = 6, 10
		case n == 7:
			minGasIndex, maxGasIndex = 7, 11
		case n == 8:
			minGasIndex, maxGasIndex = 8, 12
		case n >= 9:
			minGasIndex, maxGasIndex = 9, 13
		}

		// KNOWN DEFECT: this outer retry loop re-attempts the entire gas
		// window when every candidate is randomly skipped. With a narrow
		// window (5 gases, each ~2/3 chance of skip) this can burn many
		// iterations. Needs a design review.
		var firstGasFound AtmosphericGas
		for len(p.Gases) == 0 {
			for i := minGasIndex; i <= maxGasIndex; i++ {
				if len(p.Gases) == numberOfGasesWanted {
					break
				}
				skipChance, maxQty := 3, 100
				if i == GasHe {
					if p.TemperatureClass > 5 { // too hot for helium
						continue
					}
					skipChance, maxQty = 2, 20
				} else if i == GasO2 {
					maxQty = 50
				}
				if g.r.Roll(1, 3) >= skipChance { // skip the gas for some reason
					continue
				}
				p.Gases[i] = g.r.Roll(1, maxQty)
				if len(p.Gases) == 1 {
					// excess will be allocated to the first gas found
					firstGasFound = i
				}
			}
		}
		// normalize gas to percentages
		totalQuantity := 0
		for _, quantity := range p.Gases {
			totalQuantity += quantity
		}
		percentUnallocated := 100
		for gas, quantity := range p.Gases {
			p.Gases[gas] = (100 * quantity) / totalQuantity
			percentUnallocated -= p.Gases[gas]
		}
		// allocate any remaining amount to the first gas
		p.Gases[firstGasFound] += percentUnallocated
	}

	// randomize mining difficulty
	p.MiningDifficulty = 0
	for {
		p.MiningDifficulty = float64((g.r.Roll(1, 3)+g.r.Roll(1, 3)-g.r.Roll(1, 4))*g.r.Roll(1, p.Diameter) + g.r.Roll(1, 30) + g.r.Roll(1, 30))
		if p.MiningDifficulty >= 40 && p.MiningDifficulty <= 500 {
			break
		}
	}
	// apply fudge factor
	p.MiningDifficulty = p.MiningDifficulty * 11 / 5

	return p
}

type Option func(*Generator) error

func WithDesiredNumberOfSystems(n int) Option {
	return func(g *Generator) error {
		g.desiredNumSystems = n
		return nil
	}
}

func WithDesiredRadius(r int) Option {
	return func(g *Generator) error {
		g.desiredRadius = r
		return nil
	}
}

func WithMinimumDistance(d int) Option {
	return func(g *Generator) error {
		g.minimumDistance = d
		return nil
	}
}

func WithMerge(merge bool) Option {
	return func(g *Generator) error {
		g.merge = merge
		return nil
	}
}

func WithPRNG(r *prng.PRNG) Option {
	return func(g *Generator) error {
		g.r = r
		return nil
	}
}

type Generator struct {
	desiredNumSystems int
	desiredRadius     int
	minimumDistance   int
	merge             bool
	r                 *prng.PRNG
}
