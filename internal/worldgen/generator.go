// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

import (
	"fmt"
	"math"

	"github.com/mdhender/drynn/internal/prng"
	hexmap "github.com/mdhender/drynn/internal/worldgen/hexes"
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

	hexSystems, err := hexmap.NewGenerator(g.r).Generate(g.desiredRadius, g.desiredNumSystems, g.minimumDistance, g.merge)
	if err != nil {
		return nil, fmt.Errorf("hex gen: %w", err)
	}

	galaxy := &Galaxy{
		Radius:  g.desiredRadius,
		Systems: make([]*System, 0, len(hexSystems)),
	}
	for _, hs := range hexSystems {
		sys := &System{
			Hex:   hs.Hex,
			Stars: make([]*Star, 0, hs.Stars),
		}
		for i := 0; i < hs.Stars; i++ {
			sys.Stars = append(sys.Stars, g.rollStar())
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
		star.kind = starDwarf
	case 2: // 10% degenerate
		star.kind = starDegenerate
	case 3: // 10% giant
		star.kind = starGiant
	default: // 70% main sequence
		star.kind = starMainSequence
	}

	// determine star color randomly
	switch g.r.Roll(1, 7) {
	case 1:
		star.color = colorBlue
	case 2:
		star.color = colorBlueWhite
	case 3:
		star.color = colorWhite
	case 4:
		star.color = colorYellowWhite
	case 5:
		star.color = colorYellow
	case 6:
		star.color = colorOrange
	case 7:
		star.color = colorRed
	default:
		panic("assert(max(starColor) == colorRed)")
	}

	// determine star size randomly
	star.size = g.r.D10(1) - 1

	// determine number of planets orbiting this star
	star.numPlanets = -2 // default to a negative number for some reason or another
	var sizeOfDie int    // used to generate number of planets; set by color of star
	switch star.color {
	case colorBlue:
		sizeOfDie = 8
	case colorBlueWhite:
		sizeOfDie = 7
	case colorWhite:
		sizeOfDie = 6
	case colorYellowWhite:
		sizeOfDie = 5
	case colorYellow:
		sizeOfDie = 4
	case colorOrange:
		sizeOfDie = 3
	case colorRed:
		sizeOfDie = 2
	default:
		panic(fmt.Sprintf("assert(star.color != %d)", star.color))
	}
	var numberOfRolls int // used to generate number of planets; set by type of star
	switch star.kind {
	case starDwarf:
		numberOfRolls = 1
	case starDegenerate, starMainSequence:
		numberOfRolls = 2
	case starGiant:
		numberOfRolls = 3
	default:
		panic(fmt.Sprintf("assert(star.kind != %d)", star.kind))
	}
	for i := 1; i <= numberOfRolls; i++ {
		star.numPlanets += g.r.Roll(1, sizeOfDie)
	}
	for star.numPlanets < 1 { // bump up to a minimum of 1
		star.numPlanets += g.r.Roll(1, 2)
	}
	for star.numPlanets > 9 { // bump down to a maximum of 9
		star.numPlanets -= g.r.Roll(1, 3)
	}
	seedDiameter := []int{0, 5, 12, 13, 7, 20, 143, 121, 51, 49} // thousands of km
	seedTemperatureClass := []int{0, 29, 27, 11, 9, 8, 6, 5, 5, 3}
	var previousPlanet *Planet
	for orbit := 1; orbit <= star.numPlanets; orbit++ {
		// determine seed; bias towards "earth" zones
		var seedIndex int
		if star.numPlanets <= 3 {
			seedIndex = 2*orbit + 1
		} else {
			seedIndex = 9 * orbit / star.numPlanets
		}
		// use seed to initialize starting values
		p := &Planet{
			Diameter:         seedDiameter[seedIndex],
			TemperatureClass: seedTemperatureClass[seedIndex],
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
		for p.Diameter < 3 { // bump up to a minimum of 3
			p.Diameter += g.r.D4(1)
		}
		// are we a gas giant?
		isGasGiant := p.Diameter > 40

		// compute density
		if isGasGiant { // range 0.6 to 1.7
			p.Density = float64(58+g.r.Roll(1, 56)+g.r.Roll(1, 56)) / 100
		} else { // range 3.7 to 5.7
			p.Density = float64(368 + g.r.Roll(1, 101) + g.r.Roll(1, 101))
		}

		// compute gravity (the divisor 72 is calibrated so that Earth's values (density=5.50, diameter=13) yield gravity ~= 1.0)
		p.Gravity = p.Density * float64(p.Diameter) / 72

		// randomize temperature class
		dieSize = p.TemperatureClass / 4
		if dieSize < 2 {
			dieSize = 2
		}
		numberOfRolls = g.r.Roll(1, 3) + g.r.Roll(1, 3) + g.r.Roll(1, 3)
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
		for p.TemperatureClass < minTC { // bump up
			p.TemperatureClass += g.r.Roll(1, 2)
		}
		for p.TemperatureClass > maxTC { // bump down
			p.TemperatureClass -= g.r.Roll(1, 2)
		}

		// warm small systems
		if star.numPlanets < 4 && orbit < 3 {
			for p.TemperatureClass < 12 {
				p.TemperatureClass += g.r.Roll(1, 4)
			}
		}

		// enforce temperature ordering.
		// Planets farther from the star must not be warmer than planets closer to it. The comparison is against the previous planet's final (stored) temperature class.
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
		for p.PressureClass < minPC { // bump up
			p.PressureClass += g.r.Roll(1, 3)
		}
		for p.PressureClass > maxPC { // bump down
			p.PressureClass -= g.r.Roll(1, 3)
		}

		// randomize atmosphere
		var numberOfGasesWanted int
		var minGasIndex, maxGasIndex AtmosphericGas
		if p.PressureClass > 0 {
			numberOfGasesWanted = g.r.D4(2) / 2
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
		}
		var firstGasFound AtmosphericGas
		for len(p.Gases) == 0 {
			for i := minGasIndex; i <= maxGasIndex; i++ {
				if len(p.Gases) == numberOfGasesWanted {
					break
				}
				if i == GasHe {
					if p.TemperatureClass > 5 { // too hot for helium
						continue
					}
					if g.r.Roll(1, 3) > 1 { // skip the gas for some reason
						continue
					}
					if firstGasFound == 0 {
						firstGasFound = i
					}
					p.Gases[i] = g.r.Roll(1, 20)
					continue
				}
				if g.r.Roll(1, 3) == 3 { // skip the gas for some reason
					continue
				}
				if firstGasFound == 0 {
					firstGasFound = i
				}
				if i == GasO2 {
					p.Gases[i] = g.r.Roll(1, 50)
				} else {
					p.Gases[i] = g.r.Roll(1, 100)
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

		star.Planets = append(star.Planets, g.rollPlanet(p, false, false))
		previousPlanet = p
	}

	return star
}

func (g *Generator) rollPlanet(p *Planet, earthLike, makeMiningEasier bool) *Planet {
	panic("!")
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
