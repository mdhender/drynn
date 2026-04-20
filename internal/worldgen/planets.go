// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

type Planet struct {
	Diameter         int     // thousands of km
	Density          float64 // earth ~= 5.5
	Gravity          float64 // in G's; earth = 1.0
	TemperatureClass int     // 1..30 (3..7 for gas giants)
	PressureClass    int     // 0..29

	Special struct {
		NotSpecial      bool
		IdealHomePlanet bool
		IdealColony     bool
		RadioactiveHell bool
	}

	// gases is a map[AtmosphericGas]percent, where percent is scaled to 0...100
	Gases map[AtmosphericGas]int

	MiningDifficulty float64
}

type AtmosphericGas int

const (
	GasNone  AtmosphericGas = 0
	GasH2    AtmosphericGas = 1
	GasCH4   AtmosphericGas = 2
	GasHe    AtmosphericGas = 3
	GasNH3   AtmosphericGas = 4
	GasN2    AtmosphericGas = 5
	GasCO2   AtmosphericGas = 6
	GasO2    AtmosphericGas = 7
	GasHCl   AtmosphericGas = 8
	GasCl2   AtmosphericGas = 9
	GasF2    AtmosphericGas = 10
	GasH2O   AtmosphericGas = 11
	GasSO2   AtmosphericGas = 12
	GasH2S   AtmosphericGas = 13
	numGases                = 13
)
