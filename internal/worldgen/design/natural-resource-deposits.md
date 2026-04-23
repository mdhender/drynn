# Natural Resource Deposits Generation

Design for `GenerateDeposits(rng, cluster)` — stage 4 of the staged
generator. Produces per-planet mineral deposits that empires mine via
colony-assigned mining groups. Design locked 2026-04-22.

Proposed signature (matches staged-generator-plan.md §3):

```go
func GenerateDeposits(rng *prng.PRNG, cluster *Cluster)
```

Populates `cluster.Deposits` (flat slice with `PlanetID` references,
matching the `Systems` / `Stars` / `Planets` pattern).

## Dependencies

- Planets have been generated.
- `Planet.Kind` and `Planet.MiningDifficulty` are populated for every
  planet.

## Planet Kind

`Planet.Kind` is a new field classifying the planet for deposit
generation:

| Kind                | Constant           | Emitted today?                     |
|---------------------|--------------------|------------------------------------|
| Rocky (terrestrial) | `KindRocky`        | Yes                                |
| Gas Giant           | `KindGasGiant`     | Yes (diameter > 40)                |
| Asteroid Belt       | `KindAsteroidBelt` | No — pending planet generator work |

Until the planet generator emits asteroid belts, deposit generation
produces only rocky and gas-giant deposits. Asteroid-belt behavior is
specified below so the code can light up without re-design.

## Resources

Internal enum values and player-facing labels:

| Enum       | Label (reports, DB, UI) | Unit code |
|------------|-------------------------|-----------|
| `Fuel`     | Fuel                    | FUEL      |
| `Gold`     | Gold                    | GOLD      |
| `Metal`    | Metals                  | METS      |
| `NonMetal` | Non-Metals              | NMTS      |

Each deposit produces exactly one unit code, matching its resource:
fuel deposits produce FUEL, gold deposits produce GOLD, metal deposits
produce METS, non-metal deposits produce NMTS.

**Note (derived decision, 2026-04-22):** Earlier drafts mixed
"Metallics" / "Non-Metallics" (designer preference) with "Metals" /
"Non-Metals". Standardized on **Metals** / **Non-Metals** across
reports, DB tables, and UI for consistency. Override if the "Metallics"
flavor should win — one-line change.

## Number of Deposits per Planet

| Planet Kind         | Number of Deposits |
|---------------------|--------------------|
| Asteroid Belt       | 10d4     (10..40)  |
| Gas Giant           | 3d12     (3...36)  |
| Rocky (Terrestrial) | 3d8-2    (1...22)  |

The 40-deposit cap on asteroid belts is load-bearing — see *Deposit ID*
below.

## Deposit Type, Quantity, and Yield Pct

Rolled per deposit. `Quantity` is denominated in the deposit's resource
unit — FUEL for fuel deposits, GOLD for gold, METS for metal, NMTS for
non-metal. `YieldPct` is stored as a real number — the dice tables
below produce an integer `1..100` which is divided by 100 at storage
time (e.g. `3d10−2` → integer `1..28` → stored as `0.01..0.28`). See
*Yield Pct semantics* for how the engine uses it.

### Asteroid Belt Deposits

| Roll (1–100) | Resource | Quantity             | Yield Pct |
|--------------|----------|----------------------|-----------|
| 1            | Gold     | 100,000–5,000,000    | 1d3       |
| 2–10         | Fuel     | 1,000,000–99,000,000 | 3d6 − 2   |
| 11–100       | Metals   | 1,000,000–99,000,000 | 3d10 − 2  |

### Gas Giant Deposits

| Roll (1–100) | Resource   | Quantity             | Yield Pct |
|--------------|------------|----------------------|-----------|
| 1–15         | Fuel       | 1,000,000–99,000,000 | 10d4 − 2  |
| 16–40        | Metals     | 1,000,000–99,000,000 | 10d6      |
| 41–100       | Non-Metals | 1,000,000–99,000,000 | 10d6      |

### Rocky (Terrestrial) Deposits

Yield formulas branch on `Planet.MiningDifficulty`. The "hard" column
applies when `MiningDifficulty >= 500`; the "easy" column applies when
`MiningDifficulty < 500`.

**Threshold rationale (derived decision, 2026-04-22):** real
`Planet.MiningDifficulty` post-fudge lands in roughly `[88, 1100]`,
with earth-like planets clustering around `460..506`. The `>= 500`
breakpoint puts typical colony worlds at the boundary between the
generous and lean columns, replacing the old `ALSN < 25` / `ALSN >= 25`
split (ALSN is not available at deposit-generation time).

| Roll (1–100) | Resource   | Quantity             | Yield Pct (MD ≥ 500) | Yield Pct (MD < 500) |
|--------------|------------|----------------------|----------------------|----------------------|
| 1            | Gold       | 100,000–1,000,000    | 1d3                  | 3d4 − 3              |
| 2–15         | Fuel       | 1,000,000–99,000,000 | 10d4 − 2             | 10d8                 |
| 16–45        | Metals     | 1,000,000–99,000,000 | 10d6                 | 10d8                 |
| 46–100       | Non-Metals | 1,000,000–99,000,000 | 10d6                 | 10d8                 |

## Quantity Distribution

Deposit quantities follow a **triangular distribution** — rolled as the
sum of two equal dice times a step size, so range ends are rare and the
midpoint is most common. Concrete formulas per range:

| Range                  | Formula                    | Peak (midpoint) |
|------------------------|----------------------------|-----------------|
| 100,000 – 5,000,000    | `50,000 × (d50 + d50)`     | ~2.55M          |
| 100,000 – 1,000,000    | `50,000 × (d10 + d10)`     | ~550K           |
| 1,000,000 – 99,000,000 | `500,000 × (d99 + d99)`    | ~50M            |

### Design rationale

The original game strongly favored empires that explored widely and
fought hard to claim the best resource-producing systems. Quantity
variance between deposits is what makes "scouting for the rich system"
worth the cost — if every deposit were identical, exploration and
territorial conflict would have no economic payoff. Triangular keeps
most deposits in a familiar band while still producing notable
richer/poorer outliers.

### Fallback options (if playtesting shows issues)

If the spread feels too tight (planets feel interchangeable, scouting
has weak payoff), consider:

- **Log-uniform:** each order of magnitude is equally likely. Matches
  real geological distributions — most deposits small, rare giants.
  Creates strong "find the landmark" dynamics, but the median drops
  from ~50M to ~10M, so mining-group economics have to be retuned.
- **Hybrid (dice on the exponent):** e.g.
  `1,000,000 × 10^((3d3 − 3) / 3)` → 1M..100M, bell-shaped around 10M.
  Keeps familiar dice notation and an intuitive center while producing
  log-shaped tails. Fiddly to tune but flexible.

## Yield Pct Semantics

`YieldPct` is the fraction of extracted raw turned into refined output
in the deposit's unit code (FUEL, GOLD, METS, or NMTS). Stored as a
real number (e.g. `0.10`, not `10`).

Engine formula each turn a mining group works this deposit:

```
extracted = ...                           // determined by mining group
yield     = trunc(extracted * YieldPct)   // truncate toward zero
quantity -= yield
```

Example (metal deposit): `YieldPct = 0.10`, the mining group extracts
18 raw units → `yield = trunc(18 * 0.10) = trunc(1.8) = 1` METS
produced. The deposit's `Quantity` decreases by 1. Fuel, gold, and
non-metal deposits use the same formula, producing FUEL, GOLD, and
NMTS respectively.

## Deposit Struct

```go
type Deposit struct {
    ID               int       // PlanetID*100 + N (see Deposit ID)
    PlanetID         int
    Resource         Resource  // Fuel | Gold | Metal | NonMetal
    Quantity         int       // units remaining (FUEL, GOLD, METS, or NMTS per Resource)
    YieldPct         float64   // 0.01..1.00
    MiningDifficulty float64   // initial copy of Planet.MiningDifficulty
}
```

Each deposit carries its own `MiningDifficulty`, initialized from the
owning `Planet.MiningDifficulty` at generation time. The engine mutates
each deposit's value independently as its `Quantity` depletes (the dregs
get harder to mine). The generator never touches these values after the
initial stamp.

## Deposit ID

`ID = PlanetID * 100 + N`, where `N` is the 1-based index of this
deposit on its planet (`1..40`).

Rationale: leaves gaps between planets so players cannot infer the
deposit count of an unexplored world by inspecting the ID space. The
40-deposit cap on asteroid belts keeps the scheme collision-free.

If per-planet deposit counts ever exceed 99, this scheme must be
revisited.

## Post-Generation Mutation

The generator freezes only the initial values. The game engine owns:

- Decrementing `Quantity` each turn a mining group extracts from a
  deposit.
- Growing each deposit's `MiningDifficulty` as its `Quantity` depletes.
- Survey / visibility (which deposits an empire knows about).

None of this is a worldgen concern.
