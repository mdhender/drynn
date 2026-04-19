// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package cartesian

import (
	"sort"

	"github.com/mdhender/drynn/internal/prng"
)

type GalaxyGenerator interface {
	Generate(numberOfSystems int, radius float64, r *prng.PRNG, pg PointGenerator) *Galaxy
}

type Galaxy struct {
	Radius float64
	Stars  []*Star
}

type StandardGalaxyGenerator struct{}

func (gg *StandardGalaxyGenerator) Generate(desiredNumSystems int, desiredRadius float64, r *prng.PRNG, pg PointGenerator) *Galaxy {
	if desiredRadius <= minRadius {
		// compute the minimum volume needed and find the smallest radius whose cube is ≥ that volume:
		minVolume := float64(desiredNumSystems) * standardGalacticRadius * standardGalacticRadius * standardGalacticRadius / standardNumberOfStarSystems
		calculatedRadius := minRadius
		for calculatedRadius*calculatedRadius*calculatedRadius < minVolume {
			calculatedRadius++
		}
		desiredRadius = calculatedRadius
	}
	g := &Galaxy{
		Radius: desiredRadius,
		Stars:  make([]*Star, 0, desiredNumSystems),
	}

	// location grid
	locationGrid := make(map[Point]int)
	// always include the origin in the set of points
	locationGrid[Point{}] = 1

	for _, p := range pg.Generate(desiredNumSystems*100, r) {
		if len(locationGrid) >= desiredNumSystems {
			break
		}
		pq := p.quantize(g.Radius)
		locationGrid[pq] = 1
	}
	// warning: unlikely but possible that the location grid doesn't contain the requested number of systems

	// generate star data
	// first, make the set of points deterministic by sorting them
	points := make([]Point, 0, len(locationGrid))
	for p := range locationGrid {
		points = append(points, p)
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].Less(points[j])
	})
	// now it is safe to iterate over the list
	for _, p := range points {
		var star Star
		star.Point = p
		// determine star type
		switch starType(r.Roll(1, int(starGiant)+6)) {
		case starDwarf:
			star.kind = starDwarf
		case starDegenerate:
			star.kind = starDegenerate
		case starGiant:
			star.kind = starGiant
		default:
			star.kind = starMainSequence
		}
		// determine star color
		star.color = starColor(r.Roll(1, int(colorRed)))
		// determine star size
		star.size = r.D10(1) - 1

		// determine number of planets (1..9)
		sizeOfDie := 2 + int(colorRed-star.color) // bluer stars → bigger die
		numberOfRolls := int(star.kind)           // based on star type
		if numberOfRolls > 2 {
			numberOfRolls--
		}
		// roll for the planet count, clamping the result to 1..9
		numberOfPlanets := -2
		for i := 1; i <= numberOfRolls; i++ {
			numberOfPlanets += r.Roll(1, sizeOfDie)
		}
		for numberOfPlanets < 1 {
			numberOfPlanets += r.Roll(1, 2)
		}
		for numberOfPlanets > 9 {
			numberOfPlanets -= r.Roll(1, 3)
		}

		// generate planets

		// do not generate wormholes - not supported yet

		g.Stars = append(g.Stars, &star)
	}

	return g
}
