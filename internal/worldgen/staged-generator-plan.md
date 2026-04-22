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
- [`reference/galaxy-generation.md`](reference/galaxy-generation.md) — cluster placement spec (stage 2). Will be renamed to `cluster-generation.md` as part of the `Galaxy`→`Cluster` rename.
- [`burndown.md`](burndown.md) — orthogonal known-defect log. Random-increment clamps (items 8, 9) stay by design and are **not** blocked by this plan.

## Status legend

- `[ ]` pending
- `[~]` in progress
- `[x]` done

---

## 1. Vocabulary renames

Mechanical find-replace passes. Do these before the stage-1 implementation so
the staged code lands under the final names.

- [ ] `Galaxy` type → `Cluster`. File `galaxy.go` → `cluster.go`. Callers: `internal/worldgen/**`, `cmd/**`, viewers, JSON state, tests, design/reference docs.
- [ ] `HomeSystemTemplate` → `HomeStarTemplate`. Includes field-name companions (`templateDoc.source_star_id`, `HomeSystemTemplateUnavailableHTML`, etc.).
- [ ] Drop `HomeStarTemplate.SourceStarID` and the JSON field `source_star_id`. Ephemeral stage-1 candidates have no persistent source star.
- [ ] `SimulationOutcome.Galaxy` → `.Cluster`. Regenerate golden files if any.
- [ ] Sweep docs for "galaxy" references in prose; prefer "cluster." Exception: historical/code-citation references.

## 2. Stage-1 design documentation

- [~] Update `reference/home-system-templates.md`:
  - Add a **Vocabulary** note near the top.
  - Add a **Stage-1 Driver** section describing the shared candidate stream, 10k cap, per-slot metadata, NULL slot semantics, viability window as a GM knob, match-required rule, and own PRNG substream.
  - Flag the `HomeSystemTemplate`→`HomeStarTemplate` rename as pending.
- [ ] Update `design/home-system-template-design.md` DRAFT banner to point at this plan; note that per-planet generation (steps 1a–1k) remains authoritative and is not changed by the stage-1 work.
- [ ] Update `reference/home-system-generation.md` Phase 1 to match the new driver (candidate stream, not a per-count "repeat until viable" loop).

## 3. Generator / Cluster shape refactor

- [ ] Split the `Generator`'s single PRNG into one substream per stage: `rngTemplates`, `rngPlacement`, `rngStars`, `rngDeposits`. Each stage accepts its own seed; default substreams are derived from a master seed.
- [ ] Add `Cluster.HomeStarTemplates` — indexed by planet count (3..9). Each slot holds a richer outcome (template pointer, attempts made, best score seen, seed used) so GM review UIs can show something when a slot is NULL.
- [ ] Split public entry points to reflect the staged API:
  - `GenerateHomeStarTemplates(seed, window) ([10]*HomeStarTemplateOutcome, error)`
  - `GenerateCluster(seed, templates) (*Cluster, error)`
  - `GenerateDeposits(seed, cluster) error` (mutates cluster)
- [ ] Retain a single `Generate(options...)` convenience wrapper for tests and simple CLI invocations that walks all stages with default seeds.

## 4. Stage-1 implementation (single shared-candidate loop)

- [ ] Replace `GenerateHomeSystemTemplateUntilViable` with a driver that:
  - Uses its own PRNG substream.
  - Rolls a candidate star via existing `rollStar` (unchanged).
  - If the candidate's planet count N ∈ [3, 9] and slot N is empty, runs `generateHomeStarTemplateAttempt` on it. On viable score, fills slot N.
  - If slot N is already filled (or N is out of range), discards the candidate and continues.
  - Terminates when either all 7 slots are filled or the candidate-roll count reaches 10,000.
- [ ] Record per-slot metadata: attempts made, best score seen, seed used at slot acceptance.
- [ ] Emit NULL slots (no template) for budget-exhausted counts; the GM review UI decides whether to accept or re-seed.
- [ ] Drive viability window from the config/option passed in (default `(53, 57)` preserves current behavior). Single window applied across all planet counts.

## 5. Stage-2 cleanup

- [ ] In `placement.go`, replace the `placements[idx].Stars = rng.Roll(2, 5)` re-roll with a hard cap: merge is `min(Stars+1, 5)`. No more Roll(2, 5) dampener.

## 6. CLI update

- [ ] `cmd/drynn simulate` walks the staged entry points in order, accepting defaults for each.
- [ ] JSON state output (`jsonstate.go`) reflects the renamed types and per-slot metadata. Preserve determinism for golden-file tests.

## 7. Stage-4 deposits

- [ ] Separate design discussion is required before implementation. No work on deposits until then.

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
