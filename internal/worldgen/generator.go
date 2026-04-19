// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

import (
	"fmt"

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

	return star
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
