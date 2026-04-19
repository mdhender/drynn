package cartesian

const (
	standardNumberOfSpecies             = 25
	standardNumberOfStarSystems float64 = 90
	standardGalacticRadius      float64 = 20 // parsecs

	minSpecies = 1
	maxSpecies = 250

	minStars = 12
	maxStars = 1000

	minRadius = 6.0
	maxRadius = 50.0

	maxDiameter = 2 * maxRadius // 100
	maxPlanets  = 9 * maxStars  // 9000

	hpAvailablePop = 1500
)

type atmosphericGas int

const (
	gasNone atmosphericGas = 0
	gasH2   atmosphericGas = 1
	gasCH4  atmosphericGas = 2
	gasHe   atmosphericGas = 3
	gasNH3  atmosphericGas = 4
	gasN2   atmosphericGas = 5
	gasCO2  atmosphericGas = 6
	gasO2   atmosphericGas = 7
	gasHCl  atmosphericGas = 8
	gasCl2  atmosphericGas = 9
	gasF2   atmosphericGas = 10
	gasH2O  atmosphericGas = 11
	gasSO2  atmosphericGas = 12
	gasH2S  atmosphericGas = 13

	numGases = 13
)

type planetSpecial int

const (
	planetNotSpecial      planetSpecial = 0
	planetIdealHomePlanet planetSpecial = 1
	planetIdealColony     planetSpecial = 2
	planetRadioactiveHell planetSpecial = 3
)

type planet struct {
	temperatureClass int // 1..30 (3..7 for gas giants)
	pressureClass    int // 0..29

	special struct {
		notSpecial      bool
		idealHomePlanet bool
		idealColony     bool
		radioactiveHell bool
	}

	// gases is a map[atmosphericGas]percent, where percent is scaled to 0...100
	gases map[atmosphericGas]int

	diameter int // thousands of km

	gravity          int // scaled, × 100 (Earth = 100)
	miningDifficulty int // scaled, × 100
}

// species is the combined home-system actor produced by the home-system generator.
//
// invariants:
// 1. All gases are in one of the three groups
// 2. No gas is in multiple groups
type species struct {
	// requiredGases is a map[atmosphericGas]percent, where percent is scaled to 0...100
	requiredGases map[atmosphericGas]int

	neutralGases map[atmosphericGas]bool
	poisonGases  map[atmosphericGas]bool
}
