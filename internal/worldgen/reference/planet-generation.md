# Planet Generation Rules

This document specifies the rules for generating planets within a star system
in Far Horizons. It is intended as a reference for coding agents implementing
planet generation in Go.

For the overall galaxy creation process, see [galaxy-generation.md](galaxy-generation.md).

## Conventions

Arithmetic uses Go types as declared on the `Planet` struct:
`Diameter`, `TemperatureClass`, and `PressureClass` are `int`;
`Density`, `Gravity`, and `MiningDifficulty` are `float64`.
Integer division truncates toward zero.

`roll(1, n)` returns a uniformly distributed random integer
in the range `[1, n]` (inclusive on both ends).

`d100(1)` is shorthand for `roll(1, 100)`.

`d4(n)` is shorthand for the sum of `n` calls to `roll(1, 4)`.

When the document says "repeat until condition," it means a loop that
re-rolls every iteration until the condition is satisfied.

Constants, enumerations, and type definitions are in
[galaxy-generation.md](galaxy-generation.md).

## Inputs

```
star            *Star    // the parent star (provides star.NumPlanets)
orbit           int      // 1-based orbit index
previousPlanet  *Planet  // the planet in the preceding orbit (nil for orbit 1)
```

The function returns a single `*Planet`.

## Planet Type

```go
type Planet struct {
    Diameter         int                    // thousands of km
    Density          float64                // earth ≈ 5.5
    Gravity          float64                // in G's; earth = 1.0
    TemperatureClass int                    // 1..30 (3..7 for gas giants)
    PressureClass    int                    // 0..29

    Special struct {
        NotSpecial      bool
        IdealHomePlanet bool
        IdealColony     bool
        RadioactiveHell bool
    }

    Gases            map[AtmosphericGas]int // gas → percent (0–100)

    MiningDifficulty float64
}
```

## Atmospheric Gas Enumeration

| Value | Constant | Gas             |
|------:|----------|-----------------|
|     0 | GasNone  | (none)          |
|     1 | GasH2    | Hydrogen        |
|     2 | GasCH4   | Methane         |
|     3 | GasHe    | Helium          |
|     4 | GasNH3   | Ammonia         |
|     5 | GasN2    | Nitrogen        |
|     6 | GasCO2   | Carbon dioxide  |
|     7 | GasO2    | Oxygen          |
|     8 | GasHCl   | Hydrogen chloride |
|     9 | GasCl2   | Chlorine        |
|    10 | GasF2    | Fluorine        |
|    11 | GasH2O   | Water vapor     |
|    12 | GasSO2   | Sulfur dioxide  |
|    13 | GasH2S   | Hydrogen sulfide |

## Reference Tables

Seed values are based on Earth's solar system. Index 0 is unused.
Index 5 represents the asteroid belt (fantasy values).

```
seed_diameter    = [_, 5, 12, 13,  7, 20, 143, 121, 51, 49]  // thousands of km
seed_temp_class  = [_, 29, 27, 11,  9,  8,   6,   5,  5,  3]
```

These tables are indexed by `seedIndex` (computed per-planet below).

## Algorithm

Planets are generated sequentially from orbit 1 to `star.NumPlanets`.
Earlier orbits are closer to the star; later orbits are farther away.

### Per-Planet Generation

For each `orbit` from 1 to `star.NumPlanets`:

#### Step 1 — Compute Seed Index

The seed index selects a starting diameter and temperature class from the
reference tables.

```
if star.NumPlanets <= 3:
    seedIndex = 2 * orbit + 1
else:
    seedIndex = (9 * orbit) / star.NumPlanets
```

The `NumPlanets <= 3` case nudges values toward the Earth-like zone.

#### Step 2 — Starting Values

```
dia = seed_diameter[seedIndex]
tc  = seed_temp_class[seedIndex]
gases = empty map[AtmosphericGas]int
```

#### Step 3 — Randomize Diameter

```
die_size = dia / 4
if die_size < 2:
    die_size = 2

repeat 4 times:
    r = roll(1, die_size)
    if d100(1) > 50:
        dia = dia + r
    else:
        dia = dia - r

// enforce minimum diameter of 3 (3,000 km)
while dia < 3:
    dia += roll(1, 4)

diameter = dia
```

> The maximum possible diameter is approximately 283 (283,000 km).

#### Step 4 — Determine Gas Giant Status

```
gas_giant = (diameter > 40)
```

Planets with diameter > 40,000 km are gas giants.

#### Step 5 — Compute Density

Density is a `float64` (Earth ≈ 5.50).

```
if gas_giant:
    // range: 0.60 to 1.70
    density = float64(58 + roll(1, 56) + roll(1, 56)) / 100
else:
    // range: 3.70 to 5.70
    density = float64(368 + roll(1, 101) + roll(1, 101)) / 100
```

#### Step 6 — Compute Gravity

Gravity is a `float64` (Earth = 1.0).

```
gravity = density * float64(diameter) / 72
```

> The divisor 72 is calibrated so that Earth's values (density≈5.50,
> diameter=13) yield gravity≈1.0.

#### Step 7 — Randomize Temperature Class

```
die_size = tc / 4
if die_size < 2:
    die_size = 2

n_rolls = roll(1, 3) + roll(1, 3) + roll(1, 3)

repeat n_rolls times:
    r = roll(1, die_size)
    if d100(1) > 50:
        tc = tc + r
    else:
        tc = tc - r
```

Clamp by planet category:

```
if gas_giant:
    while tc < 3:
        tc += roll(1, 2)
    while tc > 7:
        tc -= roll(1, 2)
else:
    while tc < 1:
        tc += roll(1, 2)
    while tc > 30:
        tc -= roll(1, 2)
```

#### Step 8 — Warm Small Systems

If the system has fewer than 4 planets and this orbit is 1 or 2, ensure
the planet is not too cold:

```
if star.NumPlanets < 4 AND orbit < 3:
    while tc < 12:
        tc += roll(1, 4)
```

#### Step 9 — Enforce Temperature Ordering

Planets farther from the star must not be warmer than planets closer to it.
The comparison is against the previous planet's final temperature class:

```
if previousPlanet != nil AND previousPlanet.TemperatureClass < tc:
    tc = previousPlanet.TemperatureClass
```

#### Step 10 — Compute Pressure Class

Pressure class starts from gravity (a float64), converted to an integer
via `math.Floor`:

```
pc = int(math.Floor(gravity * 10))
die_size = pc / 4
if die_size < 2:
    die_size = 2

n_rolls = roll(1, 3) + roll(1, 3) + roll(1, 3)

repeat n_rolls times:
    r = roll(1, die_size)
    if d100(1) > 50:
        pc = pc + r
    else:
        pc = pc - r
```

Clamp by planet category:

```
if gas_giant:
    while pc < 11:
        pc += roll(1, 3)
    while pc > 29:
        pc -= roll(1, 3)
else:
    while pc < 0:
        pc += roll(1, 3)
    while pc > 12:
        pc -= roll(1, 3)
```

#### Step 11 — Generate Atmosphere

If `pressure_class == 0`, the planet has no atmosphere. The `Gases` map
remains empty.

Otherwise:

##### 11a — Determine Gas Window

The temperature class is mapped to a range of gas indices via a switch:

```
n = (100 * tc) / 225

switch:
    case n <= 1:  min_gas = 1,  max_gas = 5
    case n == 2:  min_gas = 2,  max_gas = 6
    case n == 3:  min_gas = 3,  max_gas = 7
    case n == 4:  min_gas = 4,  max_gas = 8
    case n == 5:  min_gas = 5,  max_gas = 9
    case n == 6:  min_gas = 6,  max_gas = 10
    case n == 7:  min_gas = 7,  max_gas = 11
    case n == 8:  min_gas = 8,  max_gas = 12
    case n >= 9:  min_gas = 9,  max_gas = 13
```

Each case covers a window of 5 consecutive gas types from the atmospheric
gas enumeration.

##### 11b — Select Gases

```
num_gases_wanted = d4(2) / 2
first_gas_found = 0   // tracks which gas was added first
```

Repeat the following until at least one gas has been added to the map:

For each gas index `i` from `min_gas` to `max_gas`:

1. If `len(gases) == num_gases_wanted`, stop iterating.
2. If `i == GasHe` (Helium, value 3):
   - Skip if `tc > 5` (too hot for helium).
   - Skip if `roll(1, 3) >= 2` (2-in-3 chance of skipping).
   - Otherwise: add Helium with quantity = `roll(1, 20)`.
3. If `i != GasHe`:
   - Skip if `roll(1, 3) >= 3` (1-in-3 chance of skipping).
   - Otherwise: add gas `i`.
     - If `i == GasO2`, quantity = `roll(1, 50)`.
     - Else, quantity = `roll(1, 100)`.
4. When a gas is added:
    ```
    gases[i] = quantity
    if this is the first gas added:
        first_gas_found = i
    ```

> The outer "repeat until `len(gases) > 0`" ensures at least one gas
> is always selected. If the inner loop finishes without finding any gas,
> restart the inner loop from `min_gas`.

##### 11c — Normalize to Percentages

```
total_quantity = sum of all values in gases map

percent_unallocated = 100
for each gas in gases:
    gases[gas] = (100 * gases[gas]) / total_quantity
    percent_unallocated -= gases[gas]

// give any rounding remainder to the first gas found
gases[first_gas_found] += percent_unallocated
```

#### Step 12 — Compute Mining Difficulty

Mining difficulty is a `float64`.

```
mining_dif = 0
repeat until mining_dif >= 40 AND mining_dif <= 500:
    mining_dif = float64(
        (roll(1,3) + roll(1,3) - roll(1,4))
        * roll(1, diameter)
        + roll(1, 30) + roll(1, 30)
    )

// apply fudge factor
mining_dif = mining_dif * 11 / 5
```

> Note: the base formula sums **two** `roll(1,3)` and subtracts one `roll(1,4)`,
> yielding a multiplier range of −2 to +5 before scaling by diameter.

## Finalization

The function returns the completed `Planet` struct directly. All fields are
set during generation:

```
planet.Diameter          = diameter          // int
planet.Density           = density           // float64
planet.Gravity           = gravity           // float64
planet.TemperatureClass  = temperature_class // int
planet.PressureClass     = pressure_class    // int
planet.Gases             = gases             // map[AtmosphericGas]int
planet.MiningDifficulty  = mining_difficulty // float64
planet.Special           = (zero-value struct; all bools false)
```
