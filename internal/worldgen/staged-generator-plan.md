# Staged Generator Plan

Burndown for the redesign of worldgen to emit stages:

1. `HomeStarTemplates` (ephemeral-candidate driven)
2. Cluster placement (hex disk, merge hard-capped at 5 stars)
3. Stars + planets per system
4. Deposits per planet

Design was hammered out in session on 2026-04-22. The authoritative
drynn-native specification lives under `reference/`; this file is the
execution punch-list, not the spec.

Companion documents:

- [`reference/home-system-templates.md`](reference/home-system-templates.md) — home-star template spec (stage 1).
- [`reference/cluster-generation.md`](reference/cluster-generation.md) — cluster placement spec (stage 2).
- [`burndown.md`](burndown.md) — orthogonal known-defect log. Random-increment clamps (items 8, 9) stay by design and are **not** blocked by this plan.

## Status legend

- `[ ]` pending
- `[~]` in progress
- `[x]` done

---

## 1. Vocabulary renames

Mechanical find-replace passes. Done before the stage-1 implementation so
the staged code lands under the final names.

- [x] `Galaxy` type → `Cluster`. File `galaxy.go` → `cluster.go`. Callers in `internal/worldgen/**` (non-cartesian) and `cmd/drynn/**` updated; `internal/worldgen/cartesian` retains its separate `Galaxy` type (out of scope).
- [x] `HomeSystemTemplate` → `HomeStarTemplate`, including `GenerateHomeSystemTemplate*` and `HomeSystemTemplateUnavailableHTML`.
- [x] Drop `HomeStarTemplate.SourceStarID` and the JSON field `source_star_id`. Ephemeral stage-1 candidates have no persistent source star.
- [x] `SimulationOutcome.Galaxy` → `.Cluster`. No golden files to regenerate.
- [x] Sweep worldgen docs for "galaxy" references in prose; `reference/galaxy-generation.md` renamed to `reference/cluster-generation.md`. Inherited DRAFT docs under `design/` retain their "galaxy" prose as historical content. Project-level docs under `project/` are out of scope for this pass.

## 2. Stage-1 design documentation

- [x] Update `reference/home-system-templates.md`:
  - Added **Vocabulary** note near the top.
  - Added **Stage-1 Driver** section describing the shared candidate stream, 10k cap, per-slot metadata, NULL slot semantics, viability window as a GM knob, match-required rule, and own PRNG substream.
  - Reference doc now uses `HomeStarTemplate` throughout (rename landed).
- [x] Update `design/home-system-template-design.md` DRAFT banner to point at this plan; note that per-planet generation (steps 1a–1k) remains authoritative and is not changed by the stage-1 work.
- [ ] Update `reference/home-system-generation.md` Phase 1 to match the new driver (candidate stream, not a per-count "repeat until viable" loop).

## 3. Generator / Cluster shape refactor

- [x] Split the `Generator`'s single PRNG into per-stage substreams via `prng.PRNG.Split()`. `Generate` splits the master twice (templates, cluster); `GenerateCluster` internally splits again into placement + stars. Sibling-stream isolation is unit-tested in `internal/prng/prng_test.go` and verified end-to-end (changing the viability window under a fixed master seed does not perturb cluster systems/stars/planets).
- [x] Add `Cluster.HomeStarTemplates` — slice of length 10 indexed by planet count (3..9). Each slot is a `HomeStarTemplateOutcome` with `Attempts`, `BestScore`, and (possibly nil) `Template`. `AcceptedSeed` deferred — not load-bearing yet.
- [x] Flatten `Cluster` into parallel `Systems`, `Stars`, `Planets` slices with `SystemID`/`StarID` references. `rollStar` returns `(*Star, []*Planet)`; `Generate` stamps IDs and appends. Viewers/JSON/CLI precompute `map[ParentID][]Child` for efficient per-parent access.
- [x] Split public entry points to reflect the staged API. Per the agreed design (2026-04-22), templates are **not** an input to cluster generation — they're produced in stage 1 and attached to the cluster as a stored library for later empire-assignment use.
  - `GenerateHomeStarTemplates(rng, window, maxRolls) []*HomeStarTemplateOutcome` — public.
  - `GenerateCluster(rng, ClusterOptions) (*Cluster, error)` — systems, stars, planets.
  - `GenerateDeposits(rng, cluster)` — deferred (stage-4 design pending).
- [x] Retain a single `Generate(options...)` convenience wrapper for tests and simple CLI invocations. Internally it splits the master PRNG into stage substreams and runs stages in order, attaching templates to the cluster.

## 4. Stage-1 implementation (single shared-candidate loop)

- [x] Replace `GenerateHomeStarTemplateUntilViable` with a driver that:
  - Uses its own PRNG substream (currently shares the cluster generator's PRNG; substream plumbing pending).
  - Rolls a candidate star via existing `rollStar` (unchanged).
  - If the candidate's planet count N ∈ [3, 9] and slot N is empty, runs `generateHomeStarTemplateAttempt` on it. On viable score, fills slot N.
  - If slot N is already filled (or N is out of range), discards the candidate and continues.
  - Terminates when either all 7 slots are filled or the candidate-roll count reaches 10,000.
- [x] Record per-slot metadata: `Attempts`, `BestScore`. `AcceptedSeed` deferred until substreams land.
- [x] Emit NULL slots (no template) for budget-exhausted counts; the GM review UI decides whether to accept or re-seed.
- [x] Drive viability window from the config/option passed in (default `(53, 57)` preserves current behavior). Single window applied across all planet counts.

## 5. Stage-2 cleanup

- [x] In `placement.go`, replace the `placements[idx].Stars = rng.Roll(2, 5)` re-roll with a hard cap: merge is `min(Stars+1, 5)`. No more Roll(2, 5) dampener.

## 6. CLI update

- [ ] `cmd/drynn simulate` walks the staged entry points in order, accepting defaults for each. (Partial: already consumes `cluster.HomeStarTemplates`.)
- [x] JSON state output (`jsonstate.go`) reflects the flat cluster shape: three top-level slices (`systems`, `stars`, `planets`) with `system_id`/`star_id` references, plus `home_star_templates` with per-slot metadata.

## 7. Stage-4 deposits

- [x] Design locked 2026-04-22. See [`design/natural-resource-deposits.md`](design/natural-resource-deposits.md).
- [x] `Planet.Kind` field added (`KindRocky`, `KindGasGiant`; `KindAsteroidBelt` reserved). Stamped in `rollPlanet` and re-stamped inside the template earth-like override. Helper `planetKindFromDiameter` centralizes the rule.
- [x] Inline `Diameter > 40` checks migrated to `Kind == KindGasGiant` at `templates.go:174` and `generator.go:244`. JSON state now surfaces `kind` on planet and template-planet docs.
- [x] Quantity-range distribution: **triangular** (sum of two equal dice × step). Log-uniform and hybrid variants documented as playtest fallbacks. See design doc § Quantity Distribution.
- [x] Implemented `GenerateDeposits(rng, cluster)` per the design doc in `deposits.go`. Added `Deposit` struct and `Resource` enum (`Fuel`/`Gold`/`Metal`/`NonMetal`) with `String()`/`Label()`/`UnitCode()`. `Cluster.Deposits` and `Cluster.DepositsForPlanet(id)` helper in `cluster.go`. Wired as third stage substream in `Generate(...)`; JSON state output surfaces deposits on the cluster doc. Smoke-tested: 10-system run produced 613 deposits across 44 planets with correct resource distribution (gold ~1%, no gold on gas giants), IDs follow `PlanetID*100+N`, quantity/yield ranges match the dice tables.

---

## Out-of-worldgen follow-ups

These are not part of worldgen stage execution but must be understood when
the empire-assignment layer is built:

- Template-to-star binding happens at empire assignment (DB-side). The
  empire-assignment code deletes the target star's existing planets +
  deposits and writes the template's planet set + fresh deposits in one
  transaction.
- Candidate eligibility for empire assignment is computed at assign time,
  not stored as a flag during cluster generation:
  - Star has 3..9 planets.
  - Non-NULL template exists for that planet count (match required).
  - System has no existing home system within N hexes (N is an
    empire-assignment parameter).

## Notes

- The random-increment clamp loops in `rollStar`/`rollPlanet` stay by
  design (see `burndown.md` items 8, 9). The 10k cap on stage 1 is a
  cap on candidate rolls, not on RNG calls.
- Once a slot is filled, further candidates with that planet count are
  discarded without an attempt — this keeps the budget focused on slots
  that still need filling.
