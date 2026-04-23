// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

import "github.com/mdhender/drynn/internal/prng"

// Resource is the kind of material a deposit produces. See
// internal/worldgen/design/natural-resource-deposits.md § Resources.
type Resource int

const (
	Fuel     Resource = 1
	Gold     Resource = 2
	Metal    Resource = 3
	NonMetal Resource = 4
)

// String returns the internal lowercase identifier for the resource,
// matching the StarType / PlanetKind convention (suitable for JSON and
// DB representation).
func (r Resource) String() string {
	switch r {
	case Fuel:
		return "fuel"
	case Gold:
		return "gold"
	case Metal:
		return "metal"
	case NonMetal:
		return "non-metal"
	}
	return "unknown"
}

// Label returns the player-facing display label (reports, UI).
func (r Resource) Label() string {
	switch r {
	case Fuel:
		return "Fuel"
	case Gold:
		return "Gold"
	case Metal:
		return "Metals"
	case NonMetal:
		return "Non-Metals"
	}
	return "Unknown"
}

// UnitCode returns the short production-unit code used in game output.
func (r Resource) UnitCode() string {
	switch r {
	case Fuel:
		return "FUEL"
	case Gold:
		return "GOLD"
	case Metal:
		return "METS"
	case NonMetal:
		return "NMTS"
	}
	return "????"
}

// Deposit is a per-planet mineral deposit stamped by GenerateDeposits.
// Quantity is denominated in the deposit's unit code (Resource.UnitCode).
// YieldPct is the fraction of extracted raw turned into refined output.
// MiningDifficulty is seeded from the owning Planet and mutates
// independently during play as the deposit depletes; the generator
// never touches it after the initial stamp.
//
// ID follows PlanetID*100 + N where N is the 1-based deposit index on
// the planet (1..40). The gap between planets prevents players from
// inferring deposit counts on unexplored worlds.
type Deposit struct {
	ID               int
	PlanetID         int
	Resource         Resource
	Quantity         int
	YieldPct         float64
	MiningDifficulty float64
}

// GenerateDeposits is the stage-4 worldgen entry point. It walks the
// cluster's planets and appends per-planet deposits to cluster.Deposits
// using the dice tables in design/natural-resource-deposits.md.
func GenerateDeposits(rng *prng.PRNG, cluster *Cluster) {
	if cluster == nil {
		return
	}
	for _, p := range cluster.Planets {
		n := rollNumDeposits(rng, p.Kind)
		for i := 1; i <= n; i++ {
			cluster.Deposits = append(cluster.Deposits, rollDeposit(rng, p, i))
		}
	}
}

func rollNumDeposits(rng *prng.PRNG, kind PlanetKind) int {
	switch kind {
	case KindAsteroidBelt:
		return rng.D4(10) // 10d4 → 10..40
	case KindGasGiant:
		return rng.D12(3) // 3d12 → 3..36
	case KindRocky:
		n := rng.D8(3) - 2 // 3d8-2 → 1..22
		if n < 1 {
			n = 1
		}
		return n
	}
	return 0
}

func rollDeposit(rng *prng.PRNG, p *Planet, n int) *Deposit {
	d := &Deposit{
		ID:               p.ID*100 + n,
		PlanetID:         p.ID,
		MiningDifficulty: p.MiningDifficulty,
	}
	roll := rng.Roll(1, 100)
	switch p.Kind {
	case KindAsteroidBelt:
		rollAsteroidBeltDeposit(rng, d, roll)
	case KindGasGiant:
		rollGasGiantDeposit(rng, d, roll)
	case KindRocky:
		rollRockyDeposit(rng, d, roll, p.MiningDifficulty >= 500)
	}
	return d
}

// triangularQuantity rolls a triangular-distributed quantity as
// step × (dSides + dSides). Range is [2×step, 2×sides×step] with the
// peak at (sides+1)×step.
func triangularQuantity(rng *prng.PRNG, step, sides int) int {
	return step * (rng.Roll(1, sides) + rng.Roll(1, sides))
}

func rollAsteroidBeltDeposit(rng *prng.PRNG, d *Deposit, roll int) {
	switch {
	case roll == 1:
		d.Resource = Gold
		d.Quantity = triangularQuantity(rng, 50_000, 50) // 100K..5M
		d.YieldPct = float64(rng.Roll(1, 3)) / 100
	case roll <= 10:
		d.Resource = Fuel
		d.Quantity = triangularQuantity(rng, 500_000, 99) // 1M..99M
		d.YieldPct = float64(rng.D6(3)-2) / 100
	default:
		d.Resource = Metal
		d.Quantity = triangularQuantity(rng, 500_000, 99)
		d.YieldPct = float64(rng.D10(3)-2) / 100
	}
}

func rollGasGiantDeposit(rng *prng.PRNG, d *Deposit, roll int) {
	switch {
	case roll <= 15:
		d.Resource = Fuel
		d.Quantity = triangularQuantity(rng, 500_000, 99)
		d.YieldPct = float64(rng.D4(10)-2) / 100
	case roll <= 40:
		d.Resource = Metal
		d.Quantity = triangularQuantity(rng, 500_000, 99)
		d.YieldPct = float64(rng.D6(10)) / 100
	default:
		d.Resource = NonMetal
		d.Quantity = triangularQuantity(rng, 500_000, 99)
		d.YieldPct = float64(rng.D6(10)) / 100
	}
}

func rollRockyDeposit(rng *prng.PRNG, d *Deposit, roll int, hardMD bool) {
	switch {
	case roll == 1:
		d.Resource = Gold
		d.Quantity = triangularQuantity(rng, 50_000, 10) // 100K..1M
		if hardMD {
			d.YieldPct = float64(rng.Roll(1, 3)) / 100
		} else {
			d.YieldPct = float64(rng.D4(3)-3) / 100
		}
	case roll <= 15:
		d.Resource = Fuel
		d.Quantity = triangularQuantity(rng, 500_000, 99)
		if hardMD {
			d.YieldPct = float64(rng.D4(10)-2) / 100
		} else {
			d.YieldPct = float64(rng.D8(10)) / 100
		}
	case roll <= 45:
		d.Resource = Metal
		d.Quantity = triangularQuantity(rng, 500_000, 99)
		if hardMD {
			d.YieldPct = float64(rng.D6(10)) / 100
		} else {
			d.YieldPct = float64(rng.D8(10)) / 100
		}
	default:
		d.Resource = NonMetal
		d.Quantity = triangularQuantity(rng, 500_000, 99)
		if hardMD {
			d.YieldPct = float64(rng.D6(10)) / 100
		} else {
			d.YieldPct = float64(rng.D8(10)) / 100
		}
	}
}
