# AGENTS.md — internal/worldgen

## Overview

Pure in-memory world generation. Produces a `*Galaxy` (systems on an axial hex map, stars, planets with atmospheres and mining difficulty) and home-system `*HomeSystemTemplate` values. No database, no I/O beyond optional HTML/JSON rendering.

The package is "generator-first": it runs before any adapter exists. Adapters and persistence live outside this package and are the responsibility of their callers.

## Types are for the generator's convenience

All types in this package — `Galaxy`, `System`, `Star`, `Planet`, `HomeSystemTemplate`, `TemplatePlanet`, `TemplateGas`, etc. — exist to make the generator itself readable and testable. They are **not** a public schema and they do not mirror any database table.

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

- `design/home-system-template-design.md` — working spec for template generation (`GenerateHomeSystemTemplate`, `TemplatePlanet`, ALSN, viability window). Treat as authoritative.
- `design/mining-difficulty.md` — working spec for mining difficulty formulas.
- `reference/*.md` — inherited from a prior engine, kept for historical context. **Treat `design/` as authoritative where they disagree.** In particular, the inherited reference doc describes flat `homesystem{n}.dat` files; templates will live in the database.

Do not implement Phase 2 (system selection), Phase 3 (template application), or Phase 4 (race/empire init) from the reference doc here. Those wait until after the model is updated and an adapter exists.

## Accepted defects — do not propose fixes

- Random-walk clamping loops in `generator.go` (`rollStar`, `rollPlanet`) and `templates.go` (`generateHomeSystemTemplateAttempt`) burn an unpredictable number of PRNG draws before settling. This was reviewed and accepted; attempt counts stay within ~5k in practice and this code is infrequent.
- The gas-selection retry loop (`rollNonEarthAtmosphere`, `rollPlanet`) can spin when every candidate in the window is randomly skipped. Also accepted.

If you see warnings in `burndown.md` about these, they describe the known state, not a backlog item.

## Determinism

- Seed the generator with `prng.NewFromSeed(s1, s2)` and pass via `WithPRNG`. Do not read `math/rand` or `crypto/rand` inside this package.
- `Generate` assigns sequential `System.ID` and `Star.ID` values so that callers have a stable sort key — use these whenever you need to iterate over stars deterministically (see `GenerateHomeSystemTemplateUntilViable`).
- Any map-backed field (e.g. `Planet.Gases`) must be sorted before emitting to stable output. The JSON DTO layer already does this; replicate the pattern if you add another renderer.

## Testing notes

- Pure functions only — no DB, no network, no fixtures. Unit tests can run without Docker or Postgres.
- The private `generateHomeSystemTemplateAttempt(rng, *Star) (*HomeSystemTemplate, int)` always returns a template plus its score (never nil), which is the seam for template-generation tests: feed a deterministic PRNG and a star with a known planet count, assert on the returned struct.
- `MarshalSimulationJSON` produces byte-stable output given identical input, which makes it the natural target for golden file tests.
