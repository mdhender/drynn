# Extractor Restrictions

**STATUS: DRAFT.** This document explains a design invariant that spans entities in `project/reference/empire-model.md`, `project/reference/world-model.md`, and `project/reference/units-model.md`. The invariant is stable and the surrounding model is reconciled. See `project/reconciliation-notes.md` for the resolved B8 entry.

This document explains the rules that govern where and how empires can extract natural resources in `drynn`, and the design reasoning behind those rules.

It is aimed at human developers working on the engine, the schema, or the order validator. The goal is not to prescribe implementation steps, but to explain why the data model is shaped the way it is and why the schema does not carry constraints that you might expect it to.

## What An Extractor Is

An *extractor* is a Unit on a Vessel that produces a resource from a Natural Resource row. There are two families in alpha:

- **Mines** — `mine`-variant Units, committed to a non-farmland Natural Resource (`ore`, `energy`, `gold`, or `materials`). Output is the corresponding Unit type: `metals`, `fuel`, `gold`, `non-metals`.
- **Farms** — `farm`-variant Units, committed to a `farmland` Natural Resource. Output is `food`.

The commitment is an engine-level assignment modeled in `empire-model.md` as a `Mining Group` or `Farming Group`. The group row names a Vessel, a Natural Resource, a Unit Code and Tech Level, and a priority (Group Number). The Units themselves stay in `Vessel Inventory`; the group row records only the active assignment.

Factories (`factory`-variant Units on a `Factory Group`) are not extractors. They consume other Units via recipes and do not reference Natural Resources, so the restrictions below do not apply to them.

## The Three Restrictions

Three rules, all enforced by the game engine rather than by the schema:

1. **Only surface colonies can host extractors.** An order to create a Mining Group or Farming Group is rejected unless the target Vessel's `Vessel Type` is `surface-colony`. Ships, enclosed colonies, and orbital colonies cannot operate mines or farms in alpha.

2. **Surface colonies cannot move off-planet.** A surface colony Vessel is created with a `Planet ID` and that `Planet ID` never changes. Move orders that target a surface-colony vessel are rejected. If a surface colony is abandoned or destroyed, it is removed from the game, not relocated.

3. **Surface colonies must be built on the surface of a planet.** The order that creates a surface colony requires a `Planet ID` on an existing Planet row. A surface colony cannot be created in deep space, in orbit without an associated planet, or inside a ship.

These three rules form a chain. Together they mean: every Mining Group and every Farming Group references a Vessel that is on a planet and has always been on that planet.

## Why The Restrictions Exist

There are two reasons, and they reinforce each other.

### The Game-Mechanics Reason

Mines and farms interact with the physical substrate of a planet. A mine drills into ore-bearing rock; a farm grows food in soil under local sunlight. A ship in orbit has access to neither. An enclosed or orbital colony has life support but not the geology or biosphere of the world below. Restricting extraction to surface colonies makes the thematic commitment legible: if you want to exploit a planet's resources, you have to live on it.

The prohibition on moving surface colonies is a consequence of the same thinking. A surface colony is a settlement, not a vehicle. It has no engines; it is built in place; it does not relocate.

### The Data-Model Reason

A Mining Group row points at one Vessel and one Natural Resource. The Natural Resource has a `Planet ID`. The Vessel, when it is a surface colony, also has a `Planet ID`. For the mining relationship to make physical sense, those two `Planet ID` values must be equal.

The obvious question is: how does the schema enforce that?

Answer: it doesn't need to. The three restrictions above already make a bad state unreachable.

- Rule 1 guarantees the Vessel is a surface colony.
- Rule 3 guarantees the surface colony has a `Planet ID`.
- Rule 2 guarantees that `Planet ID` does not change after the Mining Group is created.
- The order validator that creates the Mining Group checks, at creation time, that the Natural Resource's `Planet ID` matches the Vessel's `Planet ID`.

Every one of these is an engine rule. Given all four, there is no sequence of legal orders that can produce a Mining Group row whose Vessel and Natural Resource disagree on `Planet ID`. The invariant holds structurally.

## Why The Schema Does Not Enforce It

A reader looking at `empire-model.md` will notice that Mining Group has no `Planet ID` field, no composite foreign key connecting Vessel's planet to Natural Resource's planet, and no CHECK constraint or trigger enforcing the match. This is intentional.

The alternatives each carry a cost:

- **Denormalize `Planet ID` onto Mining Group** and extend the composite foreign keys to both Vessel and Natural Resource. This works, but it adds a redundant column and obligates every future change to Vessel or Natural Resource placement to also update every referencing Mining Group row. Because surface colonies never move, that sync work would never actually happen — the column would be dead weight.

- **Add a trigger.** Triggers are a last resort for cross-row invariants. They are harder to reason about than a declarative constraint, easier to drift from the model documentation, and in this case they would be checking a property that the order validator has already checked.

- **Rely on the engine rules alone.** This is what the model does. The restrictions above make the invariant unreachable-to-violate, so the schema does not need to express it.

The rule of thumb: declarative constraints exist to catch bugs that the application layer might introduce. When the application layer structurally cannot introduce the bug — because a chain of upstream rules forecloses it — the constraint is redundant and its maintenance cost is pure drag.

## What This Means For Engine Code

Three places in the engine carry the weight that the schema does not:

1. **The order validator for "build Mining Group" and "build Farming Group."** It must reject any order whose target Vessel is not a surface colony, and it must verify that the Natural Resource's `Planet ID` matches the Vessel's `Planet ID` at creation time.

2. **The vessel-movement handler.** It must reject any move order that targets a surface-colony Vessel. This is also where the immobility of enclosed and orbital colonies in alpha is enforced; only ships move in the alpha rule set.

3. **The vessel-creation handler for surface colonies.** It must require a `Planet ID` on an existing, in-game Planet row. A `NULL` or cross-game `Planet ID` is a bug.

If any of these three checks is missed, a bad state becomes reachable. Tests should cover each of them directly, at the order-handling layer, rather than relying on downstream schema errors to surface the problem.

## When The Restrictions Change

These restrictions are alpha rules. If drynn later introduces mobile mining platforms, orbital extractors, or colony-ship variants that can land and relaunch, the chain above breaks and the schema will need to carry the invariant explicitly — most likely by denormalizing `Planet ID` onto Mining Group and Farming Group and extending the composite foreign keys.

Until then, the restrictions hold and the schema stays lean.

## Related Documents

- `project/reference/empire-model.md` — Mining Group, Farming Group, and Vessel entity specs. The Notes under each group entity cite this document for the structural reasoning.
- `project/reference/world-model.md` — Natural Resource entity, including the `Resource Type` enum that gates mine vs. farm eligibility.
- `project/reference/units-model.md` — Vessel Type catalog, including the `surface-colony` code that gates the restrictions in this document.
- `project/reconciliation-notes.md` — resolved B8 entry recording the decision to close this as a structural invariant rather than a schema constraint.
