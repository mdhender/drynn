# Galaxy Generation Rules

This document specifies the rules for creating a galaxy using hex-based
2D axial coordinates. It is intended as a reference for coding agents
implementing galaxy creation in Go.

Planet generation is detailed in [planet-generation.md](planet-generation.md).

## Conventions

All arithmetic is **integer arithmetic** unless stated otherwise.
Integer division truncates toward zero.
Density, gravity, and mining difficulty are **float64** values.

`roll(low, high)` returns a uniformly distributed random integer
in the range `[low, high]` (inclusive on both ends).

When the document says "repeat until condition," it means a loop that
re-rolls every iteration until the condition is satisfied.

## Spatial Model

The galaxy uses a **2D hex grid** with **axial coordinates** (`Q`, `R`).
Stars are placed on hexes within a hex disk of a given radius centered
at the origin `(0, 0)`.

A hex disk of radius `r` contains all hexes `(Q, R)` where the hex
distance from `(0, 0)` is ≤ `r`. The total number of hexes (capacity)
in a disk of radius `r` is:

```
capacity(r) = 1 + 3 * r * (r + 1)
```

Hex distance between two hexes `(Q₁, R₁)` and `(Q₂, R₂)` is:

```
distance = max(|Q₁ - Q₂|, |R₁ - R₂|, |(Q₁ + R₁) - (Q₂ + R₂)|)
```

This is the standard cube-distance formula for axial hex coordinates.

## Enumerations

### Star Type

| Name          | Value | Probability |
|---------------|-------|-------------|
| DWARF         | 1     | 10%         |
| DEGENERATE    | 2     | 10%         |
| MAIN_SEQUENCE | 3     | 70%         |
| GIANT         | 4     | 10%         |

### Star Color

| Name         | Value |
|--------------|-------|
| BLUE         | 1     |
| BLUE_WHITE   | 2     |
| WHITE        | 3     |
| YELLOW_WHITE | 4     |
| YELLOW       | 5     |
| ORANGE       | 6     |
| RED          | 7     |

### Atmospheric Gas

| Name | Value | Description        |
|------|-------|--------------------|
| NONE | 0     | No gas / empty     |
| H2   | 1     | Hydrogen           |
| CH4  | 2     | Methane            |
| HE   | 3     | Helium             |
| NH3  | 4     | Ammonia            |
| N2   | 5     | Nitrogen           |
| CO2  | 6     | Carbon Dioxide     |
| O2   | 7     | Oxygen             |
| HCL  | 8     | Hydrogen Chloride  |
| CL2  | 9     | Chlorine           |
| F2   | 10    | Fluorine           |
| H2O  | 11    | Steam              |
| SO2  | 12    | Sulfur Dioxide     |
| H2S  | 13    | Hydrogen Sulfide   |

### Planet Special

The special designation is a **struct of bools**, not an integer enum.
At most one field is true at a time.

| Field Name       | Meaning                |
|------------------|------------------------|
| NotSpecial       | Not special            |
| IdealHomePlanet  | Ideal home planet      |
| IdealColony      | Ideal colony planet    |
| RadioactiveHell  | Radioactive hell-hole  |

## Type Definitions

### Galaxy

```
Galaxy {
    Radius   int        // hex disk radius
    Systems  []*System  // all star systems in the galaxy
}
```

### System

A system occupies a single hex and may contain **multiple stars**
(created by the merge step of the placement algorithm).

```
System {
    Hex        Axial    // axial hex coordinates (Q, R)
    Stars      []*Star  // one or more stars in this system
    HomeSystem bool     // true if this is a designated home system
}
```

### Star

Each star within a system has its own set of orbiting planets.

```
Star {
    Kind       StarType   // DWARF | DEGENERATE | MAIN_SEQUENCE | GIANT
    Color      StarColor  // BLUE through RED
    Size       int        // 0 through 9 inclusive
    NumPlanets int        // 1 through 9 inclusive
    Planets    []*Planet  // the planets orbiting this star
}
```

### Planet

```
Planet {
    Diameter         int              // in thousands of kilometers
    Density          float64          // g/cm³ (Earth ≈ 5.5)
    Gravity          float64          // surface gravity in G's (Earth = 1.0)
    TemperatureClass int              // 1–30 (3–7 for gas giants)
    PressureClass    int              // 0–29
    Special          struct {         // struct of bools
        NotSpecial      bool
        IdealHomePlanet bool
        IdealColony     bool
        RadioactiveHell bool
    }
    Gases            map[Gas]int      // atmospheric gas → percentage
    MiningDifficulty float64          // mining difficulty
}
```

## Inputs

Galaxy creation is configured via functional options with the following
defaults:

| Option                  | Default | Description                                        |
|-------------------------|---------|----------------------------------------------------|
| `desiredNumSystems`     | 100     | Number of star systems to place.                   |
| `desiredRadius`         | 15      | Radius of the hex disk.                            |
| `minimumDistance`        | 2       | Minimum hex distance between systems.              |
| `merge`                 | true    | If true, nearby systems merge (multi-star).        |
| `prng`                  | seeded  | Pseudo-random number generator (default seed 10,10).|

### Option Functions

```go
WithDesiredNumberOfSystems(n int)
WithDesiredRadius(r int)
WithMinimumDistance(d int)
WithMerge(merge bool)
WithPRNG(r *prng.PRNG)
```

## Galaxy Creation Algorithm

### Step 1 — Hex Placement

Place systems onto the hex disk. The procedure `placeHexSystems` takes
`desiredRadius`, `desiredNumSystems`, `minimumDistance`, and `merge` as
inputs.

#### 1a — Enumerate Hex Candidates

Generate all hexes in a disk of radius `desiredRadius` using
`hexes.Disk(r)`. This produces `1 + 3 * r * (r + 1)` candidate hexes.

#### 1b — Shuffle

Shuffle the candidate hex list once using Fisher-Yates with the PRNG.

#### 1c — Consume Candidates

Iterate through the shuffled list. Maintain a list of placed systems
(each tracking its hex and star count, initially 1). For each candidate
hex:

```
for each candidate in shuffled_hexes:
    nearest = find placed system with smallest hex distance to candidate
    dist = hex_distance(candidate, nearest)

    if dist < minimumDistance:
        if merge:
            nearest.star_count += 1
            if nearest.star_count > 5:
                nearest.star_count = roll(2, 5)
        else:
            discard candidate
    else:
        create new placement at candidate hex with star_count = 1

    if number of placements == desiredNumSystems:
        stop
```

If all candidates are exhausted before reaching `desiredNumSystems`,
return an error.

### Step 2 — Generate Stars and Planets

For each placement from Step 1, create a `System` at the placement's hex.
Generate `star_count` stars for that system. Each star is generated by
`rollStar()`.

#### 2a — Determine Star Type

```
star_type = roll(1, 10)
if star_type == 1:
    star_type = DWARF
else if star_type == 2:
    star_type = DEGENERATE
else if star_type == 3:
    star_type = GIANT
else:
    star_type = MAIN_SEQUENCE
```

This gives MAIN_SEQUENCE a 70% probability (rolls 4–10).

#### 2b — Determine Star Color

```
star_color = roll(1, 7)
```

Uniformly distributed across all seven colors.

#### 2c — Determine Star Size

```
star_size = roll(1, 10) - 1       // 0 through 9
```

#### 2d — Determine Number of Planets

The number of planets depends on star color and star type.

Compute the die size based on color (bluer stars → bigger die):

```
d = RED + 2 - star_color          // i.e., 9 - star_color
```

| Color        | star_color | Die size (d) |
|--------------|------------|--------------|
| BLUE         | 1          | 8            |
| BLUE_WHITE   | 2          | 7            |
| WHITE        | 3          | 6            |
| YELLOW_WHITE | 4          | 5            |
| YELLOW       | 5          | 4            |
| ORANGE       | 6          | 3            |
| RED          | 7          | 2            |

Determine the number of rolls based on star type:

| Star Type     | Value | Rolls |
|---------------|-------|-------|
| DWARF         | 1     | 1     |
| DEGENERATE    | 2     | 2     |
| MAIN_SEQUENCE | 3     | 2     |
| GIANT         | 4     | 3     |

The mapping is:

```
num_rolls = star_type
if num_rolls > 2:
    num_rolls -= 1
```

Roll for the planet count:

```
num_planets = -2
for i in 1..num_rolls:
    num_planets += roll(1, d)
```

Clamp the result:

```
while num_planets < 1:
    num_planets += roll(1, 2)
while num_planets > 9:
    num_planets -= roll(1, 3)
```

#### 2e — Generate Planets

For each orbit `1..num_planets`, call the planet generation procedure
(see [planet-generation.md](planet-generation.md)).

### Step 3 — Output

The result of galaxy creation is a single `Galaxy` record containing:

1. **Radius** — the hex disk radius.
2. **Systems** — an array of `System` records, each containing one or
   more `Star` records, each containing its `Planet` records.

## Entry Point

```go
galaxy, err := worldgen.Generate(
    worldgen.WithDesiredNumberOfSystems(100),
    worldgen.WithDesiredRadius(15),
    worldgen.WithMinimumDistance(2),
    worldgen.WithMerge(true),
    worldgen.WithPRNG(myPRNG),
)
```

## Boundary Note

Galaxy creation does **not** assign home systems to races. Home system
assignment occurs in a separate step that is outside the scope of this
document.
