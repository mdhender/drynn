# Units Model Reference

**STATUS: DRAFT — do not implement.** This document defines the drynn type catalog (Unit, Unit Recipe, Vessel Type). It is reconciled with the current architecture but has not been stabilized against production-phase modeling; full Unit enumeration and Vessel Type attribute set are still open. Known issues are tracked in `project/reconciliation-notes.md`. Coding agents must not build schema, store, or engine code against this spec until the DRAFT marker is removed.

Technical reference for the drynn type catalog: `Unit`, `Unit Recipe`, and `Vessel Type`. This document does not hold instance rows. Empire-owned instances (Vessel, Vessel Inventory, Mining/Farming/Factory Groups, Population Group, Training Queue) live in `empire-model.md`. Natural-world entities (star systems, jump routes, planets, natural resources) live in `world-model.md`. The `Game` entity lives in `game-model.md`.

## Scope

This document defines the drynn type catalog. It does not hold instance rows.

Type definitions:

- **`Unit`** — catalog of inventory-item types (~30 in alpha): natural-resource-derived goods (metals, fuel, gold, non-metals, food), manufactured goods (rstu, cngd, …), and infrastructure (mine, farm, factory, …).
- **`Unit Recipe`** — input requirements for Units with `Source = factory`.
- **`Vessel Type`** — catalog of vessel types (ships and colonies) with per-type attributes. Alpha types: `scout`, `transport`, `surface-colony`, `enclosed-colony`, `orbital-colony`.

Out of scope:

- Natural-world entities (star systems, jump routes, planets, natural resources) — see `world-model.md`.
- Empire-owned instances (Empire, Player, Agent, Empire Control, per-empire world views, Vessel, Vessel Inventory, Mining Group, Farming Group, Factory Group, Population Group, Training Queue) — see `empire-model.md`.
- Natural-resource types (`ore`, `energy`, `gold`, `materials`, `farmland`) — see `world-model.md`. These are in-ground deposit types; the Units they produce (`metals`, `fuel`, `gold`, `non-metals`, `food`) are defined in this document.
- Population types — population is not modeled as Units because it cannot be manufactured; it lives on `Population Group` in `empire-model.md`.
- Tech Level — a per-instance attribute (0..10) tracked on `Vessel` and `Vessel Inventory` in `empire-model.md`, not a catalog field.
- The `Game` entity — see `game-model.md`.

## Unit

A catalog of the things empires can own as inventory. Includes natural-resource-derived goods (metals, fuel, gold, non-metals, food), manufactured goods (rstu, cngd, …), and infrastructure (mine, farm, factory, …). Roughly 30 entries in alpha; full enumeration is deferred to production-phase modeling.

| Field        | Type                                | Description                                                                                                         |
|--------------|-------------------------------------|---------------------------------------------------------------------------------------------------------------------|
| Code         | string                              | Unique identifier. Primary key.                                                                                     |
| Display Name | string                              | Human-readable name.                                                                                                |
| Category     | string (nullable)                   | Optional UI grouping (resource, manufactured, infrastructure, …). For player convenience; the engine does not rely on it. |
| Source       | enum (`mined`, `farmed`, `factory`) | How the Unit comes into existence.                                                                                  |

Constraints:

- **PRIMARY KEY (Code)**. Codes are globally unique across all games.

Notes:

- The Unit catalog is global across games (no `Game ID`), matching the `Agent` pattern in `empire-model.md`.
- `Source = mined` — produced by mines assigned to non-farmland natural resources.
- `Source = farmed` — produced by farms assigned to farmland natural resources.
- `Source = factory` — produced by factories; inputs given in `Unit Recipe` below.
- Tech Level is a per-instance attribute on `Vessel Inventory` (see `empire-model.md`), not a catalog field.
- Alpha Unit codes include `metals`, `fuel`, `gold`, `non-metals`, `food`, `rstu`, `cngd`, `mine`, `farm`, `factory`. Full set (~30) is deferred.
- Population is not modeled as Units (it is not manufactured); see `Population Group` in `empire-model.md`.

### Deposit-to-Output Mapping

Mined and farmed Units derive from specific natural-resource types:

| Natural Resource Type | Source   | Unit Code    |
|-----------------------|----------|--------------|
| `ore`                 | `mined`  | `metals`     |
| `energy`              | `mined`  | `fuel`       |
| `gold`                | `mined`  | `gold`       |
| `materials`           | `mined`  | `non-metals` |
| `farmland`            | `farmed` | `food`       |

This mapping is an engine-level rule, not a stored table. The Natural Resource `Resource Type` (see `world-model.md`) determines which Unit an extractor produces.

## Unit Recipe

Recipe rows for Units whose `Source = factory`. One row per input.

| Field      | Type   | Description                                |
|------------|--------|--------------------------------------------|
| Unit Code  | string | FK to `Unit.Code`. The Unit being produced. |
| Input Code | string | FK to `Unit.Code`. An input Unit required. |
| Quantity   | int    | Units of input required per Unit produced. |

Constraints:

- **PRIMARY KEY (Unit Code, Input Code)**.
- **FOREIGN KEY (Unit Code) REFERENCES Unit(Code)**. Must reference a Unit with `Source = factory`; enforced at application level.
- **FOREIGN KEY (Input Code) REFERENCES Unit(Code)**.

Notes:

- Factory operational overhead (fuel consumed per turn just to run the factory) is not a recipe input. It is a factory-operation attribute handled by the production-phase model.
- Tech level scaling of recipe inputs and outputs is a game-engine rule tied to the factory's tech level; nothing is stored on the recipe row for it.

## Vessel Type

A catalog of vessel types. Vessels are ships and colonies — the order-receiving entities in drynn.

| Field           | Type                    | Description                                             |
|-----------------|-------------------------|---------------------------------------------------------|
| Code            | string                  | Unique identifier. Primary key.                         |
| Display Name    | string                  | Human-readable name.                                    |
| Category        | enum (`ship`, `colony`) | Broad type grouping.                                    |
| Movement Points | int                     | Baseline movement per turn (0 for colonies).            |
| Cargo Capacity  | int                     | Baseline cargo capacity; semantics TBD.                 |

Constraints:

- **PRIMARY KEY (Code)**. Codes are globally unique across all games.

Notes:

- The Vessel Type catalog is global across games.
- Alpha codes: `scout`, `transport` (ships); `surface-colony`, `enclosed-colony`, `orbital-colony` (colonies).
- Tech Level is tracked per-vessel on the `Vessel` entity in `empire-model.md`.
- Additional per-type attributes (armament, life support, docking capacity, construction cost, …) will be added during production-phase modeling. The current field set is a starting shape, not the final one.

## Open Questions

Tracked here so they are not lost during reconciliation. These block removal of the DRAFT marker.

- **Full Unit catalog enumeration.** Alpha is said to have ~30 Unit types; only ~10 are sketched. Full enumeration is deferred to production-phase modeling.
- **Full Vessel Type attribute set.** Starting fields are `Code`, `Display Name`, `Category`, `Movement Points`, `Cargo Capacity`. Additional per-type attributes (armament, life support, docking capacity, construction cost, …) will be added during production-phase modeling.
- **Factory operational overhead.** The existing prose says factories consume 1 fuel per turn just to operate. This is separate from Unit Recipe inputs. Where it lives (Vessel Type attribute? a Factory-operation row?) is TBD.
- **Recipe tech-level scaling.** Factories operate at a tech level (0..10). How tech level modifies recipe inputs and outputs is a game-engine rule; whether any of it needs to be stored is open.

## Resolved Placement Decisions

- **Units/empire split (2026-04-16):** this document holds type definitions only. Empire-owned instance entities (Vessel, Vessel Inventory, Mining Group, Farming Group, Factory Group, Population Group, Training Queue) live in `empire-model.md`.
- **Ships and colonies merged into `Vessel` (2026-04-16):** one order-receiving entity. `Vessel Type` in this document catalogs the ship and colony variants.
- **Mines/Farms/Factories are Units (2026-04-16):** catalog entries with `Source = factory`, materialized as rows in `Vessel Inventory`. Not separate entities.
- **Population is not Units (2026-04-16):** population cannot be manufactured; `Population Group` stays a separate entity in `empire-model.md`.
- **Empire Jump Point Knowledge** lives in `empire-model.md` under "Per-Empire World Views."
- **Resource types split:** natural-resource types (`ore`, `energy`, `gold`, `materials`, `farmland`) in `world-model.md` on the `Natural Resource` entity; Unit catalog (`metals`, `fuel`, `gold`, `non-metals`, `food`, `rstu`, `cngd`, `mine`, `farm`, `factory`, …) in this document.
- **`fuel` name clash resolved (2026-04-16):** Natural Resource type renamed `fuel` → `energy`. Unit code `fuel` stays. Mining `energy` produces `fuel` units.
