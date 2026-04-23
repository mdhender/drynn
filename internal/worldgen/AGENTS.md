# AGENTS.md — internal/worldgen

## Overview

Pure in-memory world generation. Produces a `*Cluster` containing systems on an axial hex map, stars, planets (with atmospheres, mining difficulty, and planet kind), per-planet deposits, and a home-star-template library. No database, no I/O beyond optional HTML/JSON rendering.

The package runs four stages via a single `Generate(options...)` convenience wrapper, or callers can invoke stages independently:

1. `GenerateHomeStarTemplates` — stage-1 template library (7 slots for planet counts 3–9).
2. `GenerateCluster` — stage-2 hex placement + stage-3 star/planet rolling.
3. `GenerateDeposits` — stage-4 per-planet mineral deposits.

Each stage receives its own PRNG substream split from the master seed, so re-running one stage does not perturb another.

The package is "generator-first": it runs before any adapter exists. Adapters and persistence live outside this package and are the responsibility of their callers.

## Types are for the generator's convenience

All types in this package — `Cluster`, `System`, `Star`, `Planet`, `Deposit`, `HomeStarTemplate`, `HomeStarTemplateOutcome`, `TemplatePlanet`, `TemplateGas`, etc. — exist to make the generator itself readable and testable. They are **not** a public schema and they do not mirror any database table.

Agents are **empowered** to update type definitions when doing so makes the code cleaner, more idiomatic, or easier to test:

- Rename, add, or remove fields.
- Split a struct or merge two.
- Change a map-backed collection to a slice (or vice versa) when it improves determinism or ergonomics.
- Unexport fields that should have been private.

When a type change breaks something outside this package — the CLI (`cmd/drynn/simulate.go`), future adapters, HTML/JSON renderers, tests — the **agent making the change is responsible for updating every broken caller**. Do not leave the build red or the callers out of sync. If the change is too invasive to complete in the same pass, stop, surface the scope, and ask for direction before committing.

What *not* to do:

- Do not add fields to support a speculative future database column. Wait until the adapter is being written.
- Do not add JSON struct tags to these types. JSON output is produced by the DTO layer in `jsonstate.go`; tagging the real types bakes a persistence concern into a pure generator.
- Do not add backwards-compatibility shims (`Deprecated: use X`) when renaming. Rename the callers too and delete the old name in the same change.

## Authoritative specs

The **`reference/` directory** contains the drynn-native specifications. These are the authoritative source for how worldgen works:

- `reference/cluster-generation.md` — cluster placement, star/planet rolling, type definitions.
- `reference/home-system-templates.md` — stage-1 driver, template generation, viability window, template application (future).
- `reference/home-system-generation.md` — full home-system lifecycle (Phase 1 implemented, Phases 2–4 future).
- `reference/planet-generation.md` — per-planet generation algorithm.
- `reference/lsn-determination.md` — approximate LSN (implemented) and full LSN (future).

The **`design/` directory** contains historical working documents inherited from a prior engine. They carry DRAFT banners. Where `design/` and `reference/` disagree, **`reference/` wins**. Specific notes:

- `design/home-system-template-design.md` — the per-planet generation steps (1a–1k) and viability-window tuning (Addendum A) remain valid. The single-attempt "loop until viable" wrapper is superseded by the stage-1 driver.
- `design/mining-difficulty.md` — historical reference for mining difficulty formulas and gameplay impact.
- `design/natural-resource-deposits.md` — current deposit-generation design (locked 2026-04-22). Matches `deposits.go`.

Do not implement Phase 2 (system selection), Phase 3 (template application), or Phase 4 (race/empire init) from the reference docs here. Those wait until after the model is updated and an adapter exists.

## Key types

| Type | File | Description |
|------|------|-------------|
| `Cluster` | `cluster.go` | Top-level output: `Systems`, `Stars`, `Planets`, `Deposits` (flat slices), `HomeStarTemplates` |
| `System` | `systems.go` | Hex location with sequential `ID` |
| `Star` | `stars.go` | Type, color, size, `NumPlanets`; references `System.ID` via `SystemID` |
| `Planet` | `planets.go` | Physical attributes, `Kind` (`KindRocky`/`KindGasGiant`), atmosphere as `map[AtmosphericGas]int` |
| `Deposit` | `deposits.go` | Per-planet resource deposit (`Resource`, `Quantity`, `YieldPct`, `MiningDifficulty`) |
| `HomeStarTemplate` | `templates.go` | Planet set for one planet count (3–9) with viability score |
| `HomeStarTemplateOutcome` | `templates.go` | Stage-1 result per slot: template (may be nil), attempts, best score |
| `TemplatePlanet` | `templates.go` | Template planet with int gravity (×100), `[]TemplateGas` atmosphere |

## Accepted defects — do not propose fixes

- Random-walk clamping loops in `generator.go` (`rollStar`, `rollPlanet`) and `templates.go` (`generateHomeStarTemplateAttempt`) burn an unpredictable number of PRNG draws before settling. This was reviewed and accepted; attempt counts stay within ~5k in practice and this code is infrequent.
- The gas-selection retry loop (`rollNonEarthAtmosphere`, `rollPlanet`) can spin when every candidate in the window is randomly skipped. Also accepted.

If you see warnings in `burndown.md` about these, they describe the known state, not a backlog item.

## Determinism

- Seed the generator with `prng.NewFromSeed(s1, s2)` and pass via `WithPRNG`. Do not read `math/rand` or `crypto/rand` inside this package.
- `Generate` splits the master PRNG into per-stage substreams via `prng.PRNG.Split()`. Changing one stage's inputs does not shift another stage's output under the same master seed.
- `GenerateCluster` assigns sequential `System.ID`, `Star.ID`, and `Planet.ID` values. `GenerateDeposits` assigns `Deposit.ID = PlanetID*100 + N`.
- Any map-backed field (e.g. `Planet.Gases`) must be sorted before emitting to stable output. The JSON DTO layer already does this; replicate the pattern if you add another renderer.

## Testing notes

- Pure functions only — no DB, no network, no fixtures. Unit tests can run without Docker or Postgres.
- The private `generateHomeStarTemplateAttempt(rng, numPlanets int) (*HomeStarTemplate, int)` always returns a template plus its score (never nil), which is the seam for template-generation tests: feed a deterministic PRNG and a planet count, assert on the returned struct.
- `MarshalSimulationJSON` produces byte-stable output given identical input, which makes it the natural target for golden file tests.

## Ignored directories

- `aow/` and `cartesian/` are from a failed experiment. Ignore them.
