# Home System Templates Reference

This document is the drynn-native specification for the home system
template subsystem: what templates are, how they are generated, how
they are applied to a star system, and what the supporting data
structures look like. It is intended as a reference for coding agents
working on the drynn `internal/worldgen` package.

Companion documents:

- [planet-generation.md](planet-generation.md) — planet generation details.
- [home-system-generation.md](home-system-generation.md) — the full home
  system creation lifecycle, including species initialization.
- [lsn-determination.md](lsn-determination.md) — LSN variants.

This document supersedes the older engine-inherited descriptions
wherever they disagree. Where the older docs use integer scaled units
(e.g. gravity × 100), this document uses drynn's native representation.

## What Is a Home System Template?

Every player race needs a home star system — a set of planets where
one planet is especially hospitable. Rather than generating planets
from scratch each time, the engine pre-builds a small library of
**templates**: one planet set per possible planet count (3 through 9).
When a race is created, the engine picks the template matching the
target star's planet count, applies small random tweaks so no two home
systems are identical, then overwrites the star's planets with the
result.

This gives every race a roughly equal start while still feeling unique.

### Lifecycle at a Glance

1. **Game setup** — The game master runs template generation once. It
   produces seven templates, for planet counts 3, 4, 5, 6, 7, 8, and 9.
   The caller persists the returned slice(s) however it wants (in
   memory, on disk, in the database — out of scope for this document).

2. **Race creation** — Each time a new race is added, the engine
   selects a star system, loads the template matching its planet count,
   perturbs the template, and writes the modified data into the
   system's `[]*Planet`.

3. **Gameplay** — Templates are not used again after race creation.
   The planet data lives in the database from that point on.

## Scope and Non-Goals

- **Scope:** deterministic, side-effect-free generation of a
  `HomeSystemTemplate`; deterministic application of a template to
  an existing slice of `*Planet`.
- **Not in scope:** persistence format, database schema, race (species)
  initialization, home-system selection (which star becomes a home
  system). The caller owns all of those.
- **Not in scope:** changes to galaxy-wide planet generation. The
  `rollPlanet` path in `generator.go` is unchanged by this document.

## Units and Conventions

`rng.Roll(low, high)` returns a uniformly distributed integer in
`[low, high]` (inclusive on both ends). See `internal/prng/prng.go`.

Integer fields use integer arithmetic with truncating division.

drynn's native units, used throughout this document:

| Field              | Type      | Unit                         |
|--------------------|-----------|------------------------------|
| `Diameter`         | `int`     | thousands of km              |
| `Density`          | `float64` | earth ≈ 5.5 (generator only) |
| `Gravity`          | `float64` | earth = 1.0 G                |
| `TemperatureClass` | `int`     | 1..30 (3..7 for gas giants)  |
| `PressureClass`    | `int`     | 0..29                        |
| `MiningDifficulty` | `float64` | earth ≈ 210 (scaled × 100)   |

> `MiningDifficulty` is the one field that is still stored in the old
> engine's ×100 scaled form. The current `rollPlanet` generator emits
> values in that scale (see `generator.go`) and the viability formula
> below depends on that scale. This doc matches the existing generator
> rather than rescaling it.

"Repeat until condition" means a loop that regenerates from scratch on
every iteration until the condition is met.

## Constants

### Gas Identifiers

Drynn's `AtmosphericGas` constants (see `planets.go`):

| Constant  | Value | Gas               |
|-----------|-------|-------------------|
| `GasNone` | 0     | no gas            |
| `GasH2`   | 1     | Hydrogen          |
| `GasCH4`  | 2     | Methane           |
| `GasHe`   | 3     | Helium            |
| `GasNH3`  | 4     | Ammonia           |
| `GasN2`   | 5     | Nitrogen          |
| `GasCO2`  | 6     | Carbon Dioxide    |
| `GasO2`   | 7     | Oxygen            |
| `GasHCl`  | 8     | Hydrogen Chloride |
| `GasCl2`  | 9     | Chlorine          |
| `GasF2`   | 10    | Fluorine          |
| `GasH2O`  | 11    | Steam             |
| `GasSO2`  | 12    | Sulfur Dioxide    |
| `GasH2S`  | 13    | Hydrogen Sulfide  |

### Reference Tables

Seed values loosely modelled on Earth's solar system. Index 0 is
unused. Index 5 represents the asteroid belt (fantasy values).

```go
seedDiameter         = [_, 5, 12, 13,  7, 20, 143, 121, 51, 49]   // thousands of km
seedTemperatureClass = [_, 29, 27, 11,  9,  8,   6,   5,  5,  3]
```

These match the tables already used in `rollPlanet`.

### Viability Window

A template is accepted only if its viability score is **strictly**
between 53 and 57 (exclusive on both ends). The narrow window keeps
starting systems balanced — neither too harsh nor too generous.

## Data Types

These types live in the `worldgen` package. `TemplatePlanet` is a
distinct type from `Planet`: it carries only the fields needed to
stamp a home world onto an existing `*Planet`. The `Special` value
(`IdealHomePlanet` in particular) is a **template-time** concept and
belongs on `TemplatePlanet`, not on `Planet`.

### TemplatePlanetSpecial

```go
type TemplatePlanetSpecial int

const (
    TemplateNotSpecial       TemplatePlanetSpecial = 0
    TemplateIdealHomePlanet  TemplatePlanetSpecial = 1
    TemplateIdealColony      TemplatePlanetSpecial = 2
    TemplateRadioactiveHell  TemplatePlanetSpecial = 3
)
```

Template generation only ever emits `TemplateNotSpecial` or
`TemplateIdealHomePlanet`.

### TemplatePlanet

```go
type TemplatePlanet struct {
    Diameter         int                         // thousands of km; minimum 3
    Gravity          float64                     // earth = 1.0
    TemperatureClass int                         // 1..30
    PressureClass    int                         // 0..29
    MiningDifficulty float64                     // scaled × 100; earth ≈ 210
    Gases            map[AtmosphericGas]int      // gas → percent (0..100)
    Special          TemplatePlanetSpecial       // 0 or 1 from generation
}
```

`Gases` has the same shape as `Planet.Gases`. Percentages sum to 100
for planets with an atmosphere; the map is empty for vacuum worlds
(`PressureClass == 0`).

`Density` is not stored. It is an intermediate used during generation
to compute `Gravity`, and nothing in template application needs it.

### HomeSystemTemplate

```go
type HomeSystemTemplate struct {
    NumPlanets     int                // 3..9
    Planets        []*TemplatePlanet  // length == NumPlanets, in orbit order (inner to outer)
    ViabilityScore int                // the accepted score (54, 55, or 56)
}
```

The caller persists this however it likes; no on-disk format is
prescribed by this package.

## Home-Planet Propagation on Apply

`TemplatePlanet.Special` holds the template-time designation. When a
template is applied to a star's `[]*Planet`, the applier propagates
`TemplateIdealHomePlanet` into the target `*Planet`.

> **Open refactor:** at time of writing `Planet` still carries the old
> `Special` struct (see `planets.go`). The intended direction is to
> move `Special` off `Planet` and replace it with a simple home-planet
> marker (e.g. `Planet.IsHomePlanet bool`). Until that refactor lands,
> the applier should write through to
> `Planet.Special.IdealHomePlanet`. A caller that has moved to the
> new representation can update the applier in one place.

`System.HomeSystem` is the system-level flag; the caller sets it after
a successful apply.

## Function 1 — Generate a Home System Template

```go
func GenerateHomeSystemTemplate(rng *prng.PRNG, numPlanets int) *HomeSystemTemplate
```

`numPlanets` must be in `[3, 9]`.

**Returns** a `*HomeSystemTemplate` if the generated system passes the
viability check, or `nil` if it does not. No side effects, no global
state. All randomness is drawn from `rng`.

Callers are responsible for retrying on `nil` until they get a viable
template (a thin wrapper that loops is fine).

### Algorithm Overview

Generate planets one at a time, innermost orbit first. Planets closer
to the star are hotter; planets farther away are cooler. Exactly one
planet becomes the earth-like candidate (the future home world). Once
all planets are generated, a viability check decides whether the
system is balanced enough.

### Per-Planet Generation

Process `orbit` from 1 to `numPlanets`. Each planet is generated
independently except that temperature must not rise as orbit
increases.

#### 1. Starting Values

```
if numPlanets <= 3:
    baseValue = 2*orbit + 1
else:
    baseValue = (9 * orbit) / numPlanets

dia = seedDiameter[baseValue]
tc  = seedTemperatureClass[baseValue]
```

Example: in a 5-planet system, orbit 3 gives `baseValue = 5`, so
`dia = 20` and `tc = 8`.

#### 2. Randomize Diameter

```
dieSize = max(dia / 4, 2)

repeat 4 times:
    r = rng.Roll(1, dieSize)
    if rng.Roll(1, 100) > 50:
        dia += r
    else:
        dia -= r

while dia < 3:
    dia += rng.Roll(1, 4)
```

Minimum diameter is 3 (3,000 km). Practical maximum is about 283.

#### 3. Gas Giant Check

```
isGasGiant = (dia > 40)
```

#### 4. Density and Gravity

```
if isGasGiant:
    density = (58 + rng.Roll(1, 56) + rng.Roll(1, 56)) / 100.0   // 0.60..1.70
else:
    density = (368 + rng.Roll(1, 101) + rng.Roll(1, 101)) / 100.0  // 3.70..5.70

gravity = density * float64(dia) / 72.0
```

The divisor 72 is calibrated so earth (density ≈ 5.5, diameter 13)
yields `Gravity` ≈ 1.0.

#### 5. Randomize Temperature Class

```
dieSize = max(tc / 4, 2)
nRolls  = rng.Roll(1, 3) + rng.Roll(1, 3) + rng.Roll(1, 3)

repeat nRolls times:
    r = rng.Roll(1, dieSize)
    if rng.Roll(1, 100) > 50:
        tc += r
    else:
        tc -= r
```

Clamp by category. Clamping uses small random nudges rather than hard
assignment to avoid clustering at the boundary:

```
if isGasGiant:
    while tc < 3:  tc += rng.Roll(1, 2)
    while tc > 7:  tc -= rng.Roll(1, 2)
else:
    while tc < 1:  tc += rng.Roll(1, 3)
    while tc > 30: tc -= rng.Roll(1, 3)
```

#### 6. Warm Small Systems

For systems with fewer than 4 planets, the first two orbits must not
be too cold:

```
if numPlanets < 4 AND orbit <= 2:
    while tc < 12:
        tc += rng.Roll(1, 4)
```

#### 7. Temperature Ordering

A planet may not be warmer than the planet one orbit closer in:

```
if orbit > 1 AND previousPlanet.TemperatureClass < tc:
    tc = previousPlanet.TemperatureClass
```

#### 8. Earth-Like Override

Fires **once** — on the first planet (in orbit order) whose
temperature class is ≤ 11. It replaces all previously computed values
for that planet:

```
diameter         = 11 + rng.Roll(1, 3)                                       // 12..14
gravity          = (93 + rng.Roll(1, 11) + rng.Roll(1, 11) + rng.Roll(1, 5)) / 100.0  // 0.97..1.20
temperatureClass = 9 + rng.Roll(1, 3)                                        // 10..12
pressureClass    = 8 + rng.Roll(1, 3)                                        // 9..11
miningDifficulty = 208.0 + float64(rng.Roll(1, 11) + rng.Roll(1, 11))        // 210..230
special          = TemplateIdealHomePlanet
```

Build the atmosphere (fill `Gases` directly — map insertion order is
not significant because shifts happen at apply time, not here):

```
totalPercent = 0

if rng.Roll(1, 3) == 1:                 // 1-in-3 chance of ammonia
    p := rng.Roll(1, 30)
    gases[GasNH3] = p
    totalPercent += p

if rng.Roll(1, 3) == 1:                 // 1-in-3 chance of carbon dioxide
    p := rng.Roll(1, 30)
    gases[GasCO2] = p
    totalPercent += p

// oxygen: 11..30%
p := rng.Roll(1, 20) + 10
gases[GasO2] = p
totalPercent += p

// nitrogen takes the remainder
gases[GasN2] = 100 - totalPercent
```

After the override, **skip** the remaining per-planet steps
(pressure class, atmosphere, mining difficulty) for this planet and
proceed to the next orbit. Mark the override as consumed so it does
not fire again.

**Why the override?** Without it, naturally earth-like planets are
vanishingly rare. The override guarantees one breathable, moderate-
gravity, moderate-mining candidate per template.

#### 9. Pressure Class (non-earth-like only)

```
pc = int(gravity * 10)   // drynn units: gravity is in G; pc seed uses the integer part
dieSize = max(pc / 4, 2)
nRolls  = rng.Roll(1, 3) + rng.Roll(1, 3) + rng.Roll(1, 3)

repeat nRolls times:
    r = rng.Roll(1, dieSize)
    if rng.Roll(1, 100) > 50:
        pc += r
    else:
        pc -= r
```

Clamp:

```
if isGasGiant:
    while pc < 11: pc += rng.Roll(1, 3)
    while pc > 29: pc -= rng.Roll(1, 3)
else:
    while pc < 0:  pc += rng.Roll(1, 3)
    while pc > 12: pc -= rng.Roll(1, 3)
```

Force vacuum (`pc = 0`) when the planet cannot retain an atmosphere:

```
if gravity < 0.1:        // too small to hold an atmosphere
    pc = 0
else if tc < 2 OR tc > 27:  // too extreme for an atmosphere
    pc = 0
```

> The old engine used `gravity < 10` against scaled gravity; in drynn
> units (Earth = 1.0) that threshold is `0.1`.

#### 10. Atmosphere (non-earth-like only)

If `pc == 0`, leave `Gases` empty and skip to mining difficulty.

Otherwise choose a window of five candidate gases based on temperature:

```
firstGas = clamp((100 * tc) / 225, 1, 9)
```

Select gases:

```
numWanted = (rng.Roll(1, 4) + rng.Roll(1, 4)) / 2
totalQty  = 0

repeat until len(gases) > 0:
    for g := firstGas; g <= firstGas+4; g++:
        if len(gases) == numWanted:
            break

        if g == GasHe:
            if rng.Roll(1, 3) > 1:   continue    // 2-in-3 skip
            if tc > 5:               continue    // too hot
            qty := rng.Roll(1, 20)
        else:
            if rng.Roll(1, 3) == 3:  continue    // 1-in-3 skip
            if g == GasO2:
                qty := rng.Roll(1, 50)           // oxygen is self-limiting
            else:
                qty := rng.Roll(1, 100)

        gases[AtmosphericGas(g)] = qty
        totalQty += qty
```

The outer retry loop ensures at least one gas is always chosen. If
every candidate in the window is skipped, try the whole window again.

Normalize quantities to integer percentages. Because `gases` is a map,
Go's iteration order is randomized and **must not** be used as the
source of logic. Copy the keys into a slice and shuffle it with the
PRNG, then drive normalization from the shuffled slice — that is the
only way to get deterministic output from a map-backed atmosphere.

```
order := make([]AtmosphericGas, 0, len(gases))
for g := range gases {
    order = append(order, g)
}
rng.Shuffle(len(order), func(i, j int) {
    order[i], order[j] = order[j], order[i]
})

remainder := 100
for _, g := range order {
    gases[g] = (100 * gases[g]) / totalQty
    remainder -= gases[g]
}

// remainder target is the first entry of the shuffled slice
gases[order[0]] += remainder
```

Every downstream step that needs to walk `gases` in order must reuse
this same shuffle-first pattern. Do not iterate a map anywhere that
feeds logic (randomization targets, remainder recipients, slot
selection, etc.).

#### 11. Mining Difficulty (non-earth-like only)

Home-system templates use the "easier" mining formula. Output is in
drynn's scaled-×100 float units (matches `rollPlanet`).

```
md = 0.0
repeat until md >= 30.0 AND md <= 1000.0:
    md = float64(
        (rng.Roll(1, 3) + rng.Roll(1, 3) + rng.Roll(1, 3) - rng.Roll(1, 4))
        * rng.Roll(1, dia)
        + rng.Roll(1, 20) + rng.Roll(1, 20))
```

No fudge factor (unlike the standard formula used in galaxy creation,
which multiplies by 11/5).

### Assembly

After per-planet generation, construct `TemplatePlanet` values from
the working values (copy `Diameter`, `Gravity`, `TemperatureClass`,
`PressureClass`, `MiningDifficulty`, `Gases`, `Special`).

### Viability Check

Exactly one planet must have `Special == TemplateIdealHomePlanet`.
If none does (no planet ever reached `tc ≤ 11` during generation),
return `nil`.

Otherwise compute the score across all planets:

```
homePlanet = the planet with Special == TemplateIdealHomePlanet
score := 0

for each p in planets:
    lsn := approximateLSN(p, homePlanet)
    score += 20000 / ((3 + lsn) * (50 + int(p.MiningDifficulty)))
```

All arithmetic in this loop is integer. `int(p.MiningDifficulty)`
truncates; values land in the same scale the viability constants
(50, 20000) were calibrated against.

**Accept the template only if `score > 53 AND score < 57`**
(i.e. score ∈ {54, 55, 56}). Otherwise return `nil`.

### Return

```go
return &HomeSystemTemplate{
    NumPlanets:     numPlanets,
    Planets:        planets,
    ViabilityScore: score,
}
```

## Approximate LSN

Used only during template generation. Assumes the candidate species
breathes oxygen and treats any gas not on the home planet as poison.
Each class of difference costs 2 points (not 3, as the full LSN does).

```go
func approximateLSN(candidate, home *TemplatePlanet) int {
    lsn := 0
    lsn += 2 * abs(candidate.TemperatureClass - home.TemperatureClass)
    lsn += 2 * abs(candidate.PressureClass    - home.PressureClass)

    lsn += 2  // assume oxygen is absent; the loop below removes the penalty if present

    for g := range candidate.Gases {
        if g == GasO2 {
            lsn -= 2
        }
        if _, ok := home.Gases[g]; !ok {
            lsn += 2
        }
    }
    return lsn
}
```

### Worked Example

Home planet: `{N2: 70, O2: 20, CO2: 10}`, tc 11, pc 10.
Candidate: `{N2: 85, HCl: 15}`, tc 6, pc 3.

```
2*|6 - 11|   = 10
2*|3 - 10|   = 14
+ 2          =  2    (assume no O2)
+ 0                   (N2 is on home; not poison; no O2 bonus)
+ 2                   (HCl is NOT on home; poison)
lsn = 28
```

Score contribution (mining difficulty 150):
`20000 / ((3 + 28) * (50 + 150)) = 20000 / 6200 = 3`.

The home planet (lsn = 0, md ≈ 220) contributes
`20000 / (3 * 270) ≈ 24`. The home planet always dominates; other
planets are small bonuses that the viability window tunes.

## Function 2 — Apply a Template to a System

```go
func ApplyHomeSystemTemplate(rng *prng.PRNG, template *HomeSystemTemplate, planets []*Planet) error
```

Mutates `planets` in place. Returns an error if preconditions fail.

### Precondition

```go
if len(template.Planets) != len(planets) {
    return fmt.Errorf("template has %d planets but system has %d",
        len(template.Planets), len(planets))
}
```

### Algorithm

For each planet index `i`:

1. Copy `template.Planets[i]` into a local working value (do not
   mutate the template itself — it is reused for other home systems).
2. Perturb the working value.
3. Write the perturbed fields onto `planets[i]`.

#### 2a. Temperature Class

```
if tp.TemperatureClass > 12:
    tp.TemperatureClass -= rng.Roll(1, 3) - 1    // -0, -1, or -2 (net cools)
else if tp.TemperatureClass > 0:
    tp.TemperatureClass += rng.Roll(1, 3) - 1    // +0, +1, or +2 (net warms)
```

`Roll(1, 3) - 1` yields 0, 1, or 2 — not symmetric around zero.

#### 2b. Pressure Class

Same shape as temperature:

```
if tp.PressureClass > 12:
    tp.PressureClass -= rng.Roll(1, 3) - 1
else if tp.PressureClass > 0:
    tp.PressureClass += rng.Roll(1, 3) - 1
```

#### 2c. Atmosphere Shift

The template stores `Gases` as a map, which has no ordering. To apply
the "slot 1 / slot 2" shift deterministically:

1. Copy `tp.Gases` keys into a slice `order`.
2. Shuffle `order` using `rng.Shuffle`. Because `rng` is deterministic,
   the result is reproducible.
3. If `len(order) >= 3`, perform the shift on the second and third
   entries (`order[1]` and `order[2]`):

```
if len(order) >= 3:
    shift := rng.Roll(1, 25) + 10      // 11..35 percentage points

    g1 := order[1]
    g2 := order[2]

    if tp.Gases[g2] > 50:
        tp.Gases[g1] += shift
        tp.Gases[g2] -= shift
    else if tp.Gases[g1] > 50:
        tp.Gases[g1] -= shift
        tp.Gases[g2] += shift
```

If `len(order) < 3`, no shift is applied. The check on "> 50" ensures
the donor has enough mass to give; if neither does, the shift is
skipped to avoid negative percentages.

> Implementation note: the iteration of `rng.Shuffle` over `order` is
> the determinism boundary. Do not iterate the map for logic — only
> use the shuffled slice.

#### 2d. Diameter

```
if tp.Diameter > 12:
    tp.Diameter -= rng.Roll(1, 3) - 1
else if tp.Diameter > 0:
    tp.Diameter += rng.Roll(1, 3) - 1
```

#### 2e. Gravity

Gravity is unscaled (earth = 1.0). Old-engine `Roll(1, 10)` translates
to a nudge of 0.01..0.10 G.

```
nudge := float64(rng.Roll(1, 10)) / 100.0

if tp.Gravity > 1.0:
    tp.Gravity -= nudge
else if tp.Gravity > 0:
    tp.Gravity += nudge
```

#### 2f. Mining Difficulty

Mining difficulty stays in scaled-×100 units (Earth ≈ 210), matching
the generator:

```
if tp.MiningDifficulty > 100.0:
    tp.MiningDifficulty -= float64(rng.Roll(1, 10))
else if tp.MiningDifficulty > 0:
    tp.MiningDifficulty += float64(rng.Roll(1, 10))
```

#### 3. Copy Into the Planet

After perturbation, write onto `planets[i]`:

```go
p := planets[i]
p.Diameter         = tp.Diameter
p.Gravity          = tp.Gravity
p.TemperatureClass = tp.TemperatureClass
p.PressureClass    = tp.PressureClass
p.MiningDifficulty = tp.MiningDifficulty
p.Gases            = tp.Gases            // clone first; see below

// Propagate the home-planet marker. Until the Planet.Special refactor
// lands, write to p.Special.IdealHomePlanet:
p.Special.NotSpecial      = (tp.Special == TemplateNotSpecial)
p.Special.IdealHomePlanet = (tp.Special == TemplateIdealHomePlanet)
p.Special.IdealColony     = false
p.Special.RadioactiveHell = false
```

**Always clone `Gases`.** The working `tp` is a local copy of the
template, but `tp.Gases` still points at the template's map. Build a
fresh `map[AtmosphericGas]int` for the target planet so subsequent
applies of the same template are not affected.

`Density` is not written by this function. Either leave it as the
galaxy-creation value (cheap and harmless — it is not used in
gameplay) or recompute it from `Diameter` and `Gravity` if a caller
wants consistency (`density = 72 * gravity / diameter`).

Fields that are **not** touched by this function: anything owned by
the containing `Star` or `System` (orbit index, parent star, system
flags). Those are the caller's responsibility. In particular, setting
`System.HomeSystem = true` is not done here.

### Return

```go
return nil
```

## Important Invariants

1. **Templates are generated once per game.** The call is idempotent
   — regenerating overwrites prior output. The caller should persist
   results before relying on them.

2. **All home systems of the same planet count share one template.**
   Differentiation comes only from the randomization in
   `ApplyHomeSystemTemplate`.

3. **The viability window is narrow** — scores 54, 55, and 56 only.
   Many random planet sets are rejected; the generator caller may
   iterate tens to hundreds of times per template.

4. **Templates carry no star/system metadata.** No parent star, no
   orbit index. Those belong to the target `Star` and are not
   overwritten by template application.

5. **Exactly one planet has `TemplateIdealHomePlanet`** in a
   successful template. That designation propagates onto the target
   `Planet` during apply and is later used by race-initialization code
   to locate the home world.

6. **Determinism requires a single `*prng.PRNG`.** All rolls in
   both `GenerateHomeSystemTemplate` and `ApplyHomeSystemTemplate`
   must come from the supplied rng, including `rng.Shuffle` used in
   the atmosphere shift.

## Function Summary

| Function                     | Input                                              | Output                  | Side effects         |
|------------------------------|----------------------------------------------------|-------------------------|----------------------|
| `GenerateHomeSystemTemplate` | `*prng.PRNG`, `numPlanets int`                     | `*HomeSystemTemplate`   | None (nil on failure)|
| `ApplyHomeSystemTemplate`    | `*prng.PRNG`, `*HomeSystemTemplate`, `[]*Planet`   | `error`                 | Mutates `planets`    |
| `approximateLSN`             | `candidate`, `home` (both `*TemplatePlanet`)       | `int`                   | None                 |

## Relationship to Other Subsystems

```
Galaxy Creation (rollStar → rollPlanet)
  └─ creates star systems with random planets (earth-like OFF)

Template Creation (this document)
  └─ produces 7 HomeSystemTemplates (one per planet count 3..9)
     Caller persists however it likes.

Race Creation (future)
  ├─ selects a star system for the new race
  ├─ calls ApplyHomeSystemTemplate on the system's []*Planet
  ├─ sets System.HomeSystem = true
  └─ initializes race gas tolerances, tech levels, home colony
     (see home-system-generation.md for the surrounding lifecycle)
```

## Addendum A — The Viability Window as a Difficulty Knob

The viability window — the range of scores
`GenerateHomeSystemTemplate` will accept — controls **how useful the
non-home planets are for early expansion**. It is a game-wide setting:
all races created under the same window get systems drawn from the
same difficulty tier.

### Why the Score Is Returned

`HomeSystemTemplate.ViabilityScore` carries the accepted score (54,
55, or 56). Persist it alongside the template so later analytics can
correlate player experience with template difficulty.

### Anatomy of the Score

```
score = Σ 20000 / ((3 + LSN(planet, home)) * (50 + int(planet.MiningDifficulty)))
```

The home planet (LSN = 0, md ≈ 210–230) always contributes roughly
23–25 points. The remaining points come from the neighbors. Tuning
the window really tunes "how much expansion value do the neighbors
provide."

| Window  | Neighbor contribution | Early-game feel                       |
|---------|-----------------------|---------------------------------------|
| 46–50   | ~22–26 points         | Harsh — neighbors are hostile/barren  |
| 54–56   | ~30–32 points         | Balanced — the original design        |
| 60–64   | ~36–40 points         | Generous — inviting neighbors         |

### Shifting Bounds

- Raising the lower bound puts a **floor on expansion quality**: no
  race will start in a system where every neighbor needs heavy
  life-support investment.
- Lowering the upper bound puts a **ceiling on starting advantage**:
  no race can luck into a cluster of easy, oxygen-rich neighbors.
- Moving both bounds together uniformly eases or tightens the game
  while preserving the width of the band.

### Window Width vs. Generation Cost

| Window width | Typical attempts per template |
|--------------|-------------------------------|
| 10 points    | ~10–50                        |
| 3 points     | ~100–500                      |
| 1 point      | ~500–5,000                    |

Cost is paid once at setup, so even thousands of attempts complete
in under a second on modern hardware.

### Recommendation

Expose the accepted minimum and maximum as game configuration
parameters, inclusive on both ends — e.g. `viability_min = 54`,
`viability_max = 56` reproduces the original accepted set
{54, 55, 56}. The `score > 53 AND score < 57` check in
`GenerateHomeSystemTemplate` becomes
`score >= viability_min AND score <= viability_max`. Keep
`ViabilityScore` stored alongside each template so it can be
correlated with player satisfaction later.

## Addendum B — Unscale `MiningDifficulty` (Future Work)

`MiningDifficulty` is the last worldgen field still carrying the old
C engine's ×100 integer scaling (stored as `float64` but valued
around 210 for earth, 30–1000 for generated planets, etc.). Every
other continuous field — `Gravity`, `Density` — already lives in
true units. The scaling was preserved here only because the existing
`rollPlanet` generator and the viability formula's constants (`50`,
`20000`) were calibrated against it, and changing one without the
others would silently shift the balance.

A future pass should move `MiningDifficulty` onto the same footing
as the rest. The touch points are all in `internal/worldgen`:

- `rollPlanet` in `generator.go` — divide the emitted value by 100
  (and drop the `* 11 / 5` fudge factor or re-express it against the
  unscaled range).
- `GenerateHomeSystemTemplate` — earth-like override range
  `210..230` → `2.10..2.30`; easier-mining range `[30, 1000]` →
  `[0.30, 10.00]`.
- Viability formula — retune constants so the score still lands in
  the original 54–56 band with unscaled inputs. One workable rescale
  is `score += 200 / ((3 + lsn) * (0.5 + md))`, but the exact choice
  should be validated by regenerating templates and comparing score
  distributions against the current output.
- `ApplyHomeSystemTemplate` — thresholds (`> 100.0` → `> 1.0`) and
  nudge magnitudes (`Roll(1, 10)` → `Roll(1, 10) / 100.0`) rescale
  the same way as `Gravity` did in this document.
- Any persisted templates from before the rescale must be
  regenerated; there is no safe in-place migration for a calibration
  change of this size.

Until that work lands, treat the ×100 scaling as a known quirk and
keep it confined to `MiningDifficulty` in this document.
