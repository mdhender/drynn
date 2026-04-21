# Home System Generation Rules

> **Status: Not yet implemented.** The rules below describe planned behavior
> for a future implementation. The active `worldgen` package does not yet
> include home system template creation, system selection, template
> application, or species initialization.

This document specifies the planned rules for generating a home system for
a new species in Far Horizons. It covers template creation, system selection,
template application with randomization, and species initialization
(gas tolerances, tech levels, mining/manufacturing bases). It is intended
as a design reference for future implementation.

For planet generation details, see [planet-generation.md](planet-generation.md).
For LSN calculations, see [lsn-determination.md](lsn-determination.md).

## Conventions

All arithmetic is **integer arithmetic** unless stated otherwise.
Integer division truncates toward zero.

`roll(low, high)` returns a uniformly distributed random integer
in the range `[low, high]` (inclusive on both ends).

When the document says "repeat until condition," it means a loop that
re-rolls every iteration until the condition is satisfied.

Constants, enumerations, and type definitions are in
[galaxy-generation.md](galaxy-generation.md) unless defined below.

## Constants

```
HP_AVAILABLE_POP = 1500   // initial population units on home planet
```

### Named Planet Status Flags

| Name        | Description                  |
|-------------|------------------------------|
| HOME_PLANET | Planet is a species' home    |
| POPULATED   | Planet has population        |

### Tech Level Indices

| Name | Index | Description    |
|------|-------|----------------|
| MI   | 0     | Mining         |
| MA   | 1     | Manufacturing  |
| ML   | 2     | Military       |
| GV   | 3     | Gravitics      |
| LS   | 4     | Life Support   |
| BI   | 5     | Biology        |

## Type Definitions

### Species

```
Species {
    x, y, z           int        // coordinates of home system
    pn                int        // orbit number of home planet
    required_gas      int        // gas id the species must breathe
    required_gas_min  int        // minimum acceptable percentage
    required_gas_max  int        // maximum acceptable percentage
    neutral_gas       [6]int     // gases harmless to the species
    poison_gas        [6]int     // gases toxic to the species
    tech_level        [6]int     // current tech levels (indexed by MI..BI)
    num_namplas       int        // number of named planets
}
```

### Named Planet (Colony)

```
NamedPlanet {
    name              string     // planet name (5–31 characters)
    x, y, z, pn      int        // coordinates and orbit
    status            set of StatusFlag   // e.g., {HOME_PLANET, POPULATED}
    planet_index      int        // index into global planets array
    mi_base           int        // mining base × 10
    ma_base           int        // manufacturing base × 10
    pop_units         int        // available population units
    shipyards         int        // number of shipyards
}
```

## Overview

Home system generation is a multi-phase process:

1. **Template creation** — Generate a set of home system planet templates
   (one per planet count 3–9), each containing planets with an earth-like
   candidate. This is done once at game setup.
2. **System selection** — When a species is created, find a suitable star
   system to become its home system.
3. **Template application** — Replace the selected system's planets with
   a template, applying minor random variations.
4. **Species initialization** — Set the species' gas tolerances, tech
   levels, and home colony properties based on the home planet.

## Phase 1 — Template Creation

For each planet count `n` from 3 to 9 inclusive, generate a home system
template file.

### Algorithm

```
for n = 3 to 9:
    repeat:
        generate_planets(n, earth_like=true, makeMiningEasier=true)
    until viability check passes
    save template as "homesystem{n}.dat"
```

Planet generation with `earth_like=true` and `makeMiningEasier=true`
follows the algorithm in [planet-creation.md](planet-creation.md). The
viability check is also specified there (see "Home System Viability Check").

The "repeat until viability check passes" loop means that planets are
regenerated from scratch until the system produces a `special == IDEAL_HOME_PLANET` planet
**and** the viability score falls in the range `(53, 57)` exclusive.

## Phase 2 — System Selection

When creating a new species, a star system must be selected and converted
into a home system.

### Step 1 — Find Existing Home Systems

Scan all star systems for planets with `special == IDEAL_HOME_PLANET`
that are not already claimed by another species. A system is "claimed" if
any existing species has its home coordinates matching the system.

If unclaimed candidate systems exist, randomly choose one.

### Step 2 — Find a New Candidate (If Needed)

If no unclaimed home systems exist, find a new candidate system using
these criteria:

1. The system must have **at least 3 planets**.
2. The system must **not** already be a home system.
3. The system must **not** have a wormhole.
4. The system must **not** have an existing home system within the
   minimum separation radius.

The minimum separation radius defaults to 10 parsecs. The distance check
uses squared Euclidean distance:

```
dx = star.x - other.x
dy = star.y - other.y
dz = star.z - other.z
if dx² + dy² + dz² <= radius²:
    too close (reject)
```

If multiple systems meet the criteria, one is chosen at random (the
candidate list is shuffled using a Fisher-Yates shuffle and the first
qualifying system is returned).

If no systems meet all criteria, species creation fails.

### Step 3 — Convert to Home System

Once a candidate system is selected, apply a home system template to it
(see Phase 3 below), then mark `star.home_system = true`.

## Phase 3 — Template Application

Load the template file matching the system's planet count
(`homesystem{num_planets}.dat`). Apply minor random modifications to
each planet in the template, then copy the modified template data into
the system's planets.

### Randomization Rules

For each planet in the template:

#### Temperature Class

```
if planet.temperature_class > 12:
    planet.temperature_class -= roll(1, 3) - 1    // change by -1, 0, or +1 (net: decrease)
else if planet.temperature_class > 0:
    planet.temperature_class += roll(1, 3) - 1    // change by -1, 0, or +1 (net: increase)
```

#### Pressure Class

```
if planet.pressure_class > 12:
    planet.pressure_class -= roll(1, 3) - 1
else if planet.pressure_class > 0:
    planet.pressure_class += roll(1, 3) - 1
```

#### Atmosphere

If the planet has a gas in slot 2 (the third slot, 0-indexed):

```
adjustment = roll(1, 25) + 10    // 11–35

if gas_percent[2] > 50:
    gas_percent[1] += adjustment
    gas_percent[2] -= adjustment
else if gas_percent[1] > 50:
    gas_percent[1] -= adjustment
    gas_percent[2] += adjustment
```

This shifts percentage between the second and third gas slots.

#### Diameter

```
if planet.diameter > 12:
    planet.diameter -= roll(1, 3) - 1
else if planet.diameter > 0:
    planet.diameter += roll(1, 3) - 1
```

#### Gravity

```
if planet.gravity > 100:
    planet.gravity -= roll(1, 10)
else if planet.gravity > 0:
    planet.gravity += roll(1, 10)
```

#### Mining Difficulty

```
if planet.mining_difficulty > 100:
    planet.mining_difficulty -= roll(1, 10)
else if planet.mining_difficulty > 0:
    planet.mining_difficulty += roll(1, 10)
```

### Copying

After randomization, copy from the template into the system's planet
data for each planet:

- `temperature_class`
- `pressure_class`
- `special`
- `gas[0..3]` and `gas_percent[0..3]`
- `diameter`
- `gravity`
- `mining_difficulty`
- `econ_efficiency`
- `md_increase`

Set `star.home_system = true`.

## Phase 4 — Species Initialization

After the home system is prepared, initialize the species data and its
home colony.

### Step 1 — Tech Levels

Tech levels are set from player input:

```
species.tech_level[MI] = 10       // fixed
species.tech_level[MA] = 10       // fixed
species.tech_level[ML] = input.ml // player-chosen
species.tech_level[GV] = input.gv // player-chosen
species.tech_level[LS] = input.ls // player-chosen
species.tech_level[BI] = input.bi // player-chosen
```

**Constraint:** The four player-chosen tech levels must sum to ≤ 15:

```
input.ml + input.gv + input.ls + input.bi <= 15
```

### Step 2 — Required Gas

The required gas is always oxygen:

```
species.required_gas = O2
```

Compute the acceptable percentage range from the home planet's oxygen
percentage:

```
for each gas slot i in 0..3:
    if home_planet.gas[i] == O2:
        o2_percent = home_planet.gas_percent[i]

species.required_gas_min = o2_percent / 2
if species.required_gas_min < 1:
    species.required_gas_min = 1

species.required_gas_max = 2 * o2_percent
if species.required_gas_max < 20:
    species.required_gas_max += 20
else if species.required_gas_max > 100:
    species.required_gas_max = 100
```

### Step 3 — Neutral and Poison Gases

Build a set of "good gases" (gases that are either required or neutral
to the species).

#### 3a — Start with Home Planet Gases

```
good_gas = array of 14 booleans, all false   // indexed 0..13
num_neutral = 0

for each gas slot i in 0..3:
    if home_planet.gas[i] > 0:
        good_gas[home_planet.gas[i]] = true
        num_neutral++
```

#### 3b — Always Include Noble and Common Gases

Helium and water are always neutral:

```
if NOT good_gas[HE]:
    good_gas[HE] = true
    num_neutral++

if NOT good_gas[H2O]:
    good_gas[H2O] = true
    num_neutral++
```

#### 3c — Fill to Seven Neutral Gases

Add random gases until there are exactly 7 good gases total (one of
which is O2, the required gas):

```
while num_neutral < 7:
    g = roll(1, 13)
    if NOT good_gas[g]:
        good_gas[g] = true
        num_neutral++
```

#### 3d — Assign Neutral Gases to Species

The 6 neutral gas slots are filled with all good gases **except** O2
(which is the required gas, not a neutral gas), in ascending order of
gas id:

```
slot = 0
for gas_id = 1 to 13:
    if good_gas[gas_id] AND gas_id != O2:
        species.neutral_gas[slot] = gas_id
        slot++
```

#### 3e — Assign Poison Gases to Species

The 6 poison gas slots are filled with all gases that are **not** good,
in ascending order of gas id:

```
slot = 0
for gas_id = 1 to 13:
    if NOT good_gas[gas_id]:
        species.poison_gas[slot] = gas_id
        slot++
```

> **Note:** There are 13 possible gases. 7 are good (1 required + 6 neutral).
> The remaining 6 are poison. This exactly fills both the `neutral_gas[6]`
> and `poison_gas[6]` arrays.

### Step 4 — Home Colony

Create a single named planet (colony) record for the home planet:

```
colony.name         = input.homeworld_name
colony.x            = star.x
colony.y            = star.y
colony.z            = star.z
colony.pn           = home_planet.orbit
colony.planet_index = home_planet.index
colony.status       = {HOME_PLANET, POPULATED}
colony.pop_units    = HP_AVAILABLE_POP       // 1500
colony.shipyards    = 1
```

All other colony fields are initialized to zero.

### Step 5 — Mining and Manufacturing Bases

The initial economic capacity is derived from the MI and MA tech levels:

```
base = species.tech_level[MI] + species.tech_level[MA]
base = 25 * base + roll(1, base) + roll(1, base) + roll(1, base)
```

Mining base (scaled × 10):

```
colony.mi_base = (home_planet.mining_difficulty * base)
                 / (10 * species.tech_level[MI])
```

Manufacturing base (scaled × 10):

```
colony.ma_base = (10 * base) / species.tech_level[MA]
```

The initial raw material production is:

```
raw_materials = (10 * species.tech_level[MI] * colony.mi_base)
                / home_planet.mining_difficulty
```

The initial production capacity is:

```
production_capacity = (species.tech_level[MA] * colony.ma_base) / 10
```

> Both formulas are provided for verification. The stored values are
> `mi_base` and `ma_base`; production is derived at runtime.

### Step 6 — Species Record

```
species.x            = star.x
species.y            = star.y
species.z            = star.z
species.pn           = home_planet.orbit
species.num_namplas   = 1
```

### Step 7 — Mark System as Visited

Add the species to the home star's visited set:

```
star.visited_by.add(species_number)
```

## Boundary Notes

- Home system generation assumes that galaxy creation and home system
  template creation have already been completed.
- The `fix_gases` function (used elsewhere for atmosphere adjustment
  during gameplay) is **not** called during home system generation.
- The home planet always has `econ_efficiency = 100` at runtime (set
  during production, not during generation).
