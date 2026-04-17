# World Model Reference

**STATUS: DRAFT.** Reconciled with current architecture and translated into `db/schema.sql` (migration `20260417040157_add_game_schema.sql`). Coding agents may build against this spec.

Technical reference for the pre-empire world in drynn: the galaxy generated during setup, before any empire is placed. Star systems, jump routes, planets, deposits, and farmland are defined here. Empire-owned entities (colonies, ships, population, inventory, control, per-empire knowledge) are defined in `empire-model.md` and `units-model.md`. The `Game` entity that scopes every row is defined in `game-model.md`; every `Game ID` in this document FKs to it.

## Empires

The Empire entity, together with Player, Agent, and the `empire_control` bridge, is defined in `empire-model.md`. Empire-owned asset types (colonies, ships, infrastructure, inventory, jump point knowledge) are defined in the sections below and FK to `Empire.ID`.

## Galaxy Structure

### Star Systems

A star system is a node in the galaxy graph. Systems are generated per-game during the Setup phase.

| Field       | Type  | Description                                                                        |
|-------------|-------|------------------------------------------------------------------------------------|
| ID          | int64 | Internal identifier, never shown to players                                        |
| Game ID     | int64 | The game this system belongs to                                                    |
| X           | int   | Grid coordinate, assigned at creation, immutable                                   |
| Y           | int   | Grid coordinate, assigned at creation, immutable                                   |
| Home System | bool  | When true, this system hosts an empire's home world. Set at generation, immutable. |

Constraints:

- **UNIQUE (Game ID, ID)** — parent-key for composite FKs from Planets and Jump Routes.
- **UNIQUE (Game ID, X, Y)** — one system per coordinate per game. drynn does not model multi-star systems.

Notes:

- Coordinates are assigned once during galaxy generation and never change.
- Players reference systems as `System (X,Y) "Name"`, where the name is the per-empire name recorded in `Empire System Name` (see `empire-model.md`).
- The galaxy contains 19 star systems connected by jump routes.

### Jump Routes

A jump route is a bidirectional edge between two star systems.

| Field          | Type           | Description                                                               |
|----------------|----------------|---------------------------------------------------------------------------|
| ID             | int64          | Internal identifier                                                       |
| Game ID        | int64          | The game this route belongs to                                            |
| System A ID    | int64          | One endpoint of the route (canonical order: `System A ID < System B ID`)  |
| System B ID    | int64          | Other endpoint of the route                                               |
| Cost           | int            | Movement point cost to traverse                                           |
| Last Turn Used | int (nullable) | Turn number of the most recent traversal; NULL if never used              |

Constraints:

- **CHECK: System A ID < System B ID** — canonical endpoint ordering. Prevents self-loops (`A == B`) and duplicate-in-reverse rows (`(A,B)` and `(B,A)`).
- **UNIQUE (Game ID, System A ID, System B ID)** — one jump route per game per unordered pair of systems. Combined with the ordering CHECK, this is a total natural key.

Notes:

- All jump routes are bidirectional. If a ship can travel A to B, it can travel B to A.
- Cost is symmetric in both directions.
- See [Jump Point Knowledge and Orders](jump-point-knowledge-and-orders.md) for reconnaissance and discovery rules.

*`Empire Jump Point Knowledge` has moved to `empire-model.md` (§Per-Empire World Views).*

## Planetary Bodies

### Planets

A planet belongs to a star system. Planets hold natural resources and may host colonies.

| Field       | Type                                         | Description                                                                                                          |
|-------------|----------------------------------------------|----------------------------------------------------------------------------------------------------------------------|
| ID          | int64                                        | Internal identifier                                                                                                  |
| Game ID     | int64                                        | The game this planet belongs to                                                                                      |
| System ID   | int64                                        | The star system this planet orbits                                                                                   |
| Orbit       | int                                          | Orbital position within the system                                                                                   |
| Planet Type | enum (`rocky`, `gas giant`, `asteroid belt`) | Physical type (see Planet Types)                                                                                     |
| LSN         | int (0–100)                                  | Life Support Number. 0 = benign (no life support required); 100 = space-equivalent (vacuum, radiation, temperature). |

Constraints:

- **UNIQUE (Game ID, ID)** — parent-key for composite FKs from Natural Resources.
- **FOREIGN KEY (Game ID, System ID) REFERENCES star_systems(Game ID, ID)** — per A1.
- **UNIQUE (Game ID, System ID, Orbit)** — one planet per orbit per system.
- **CHECK (LSN BETWEEN 0 AND 100)**.

Notes:

- A system may contain multiple planets.
- Player-facing planet names are per-empire; see `Empire Planet Name` in `empire-model.md`.
- Habitability is a gameplay rule: a planet is habitable without enclosed life-support colonies when `LSN < 25`. At higher LSN, colonies require life-support infrastructure.
- Home-world designation is recorded in the separate `Home World` table below, not as a flag on Planet.

### Home World

Designates a planet as an empire's starting home world. Home worlds have distinct generation rules (see `world-generation.md`) and are the natural seat of an empire's initial assets.

| Field     | Type  | Description                           |
|-----------|-------|---------------------------------------|
| Game ID   | int64 | The game this home world belongs to   |
| Planet ID | int64 | The planet designated as a home world |

Constraints:

- **PRIMARY KEY (Game ID, Planet ID)** — one row per home world per game.
- **FOREIGN KEY (Game ID, Planet ID) REFERENCES planets(Game ID, ID)** — per A1.

Notes:

- Home-world designation is set at generation and is immutable for the lifetime of the game.
- The planet's star system is expected to have `Home System = true`; this is an application-level invariant.
- A home world's natural resources are created with `Is Infinite = true` (see Natural Resources), so mining and farming do not deplete them.
- The mapping from home world to owning empire lives at the empire level (see `empire-model.md`).

### Natural Resources

A natural resource is an extractable site on a planet. Extractors (Mines and Farms, see `units-model.md`) are assigned to natural resources to produce unit-output types. The engine enforces which extractor type may be assigned based on `Resource Type`: Mines to non-farmland resources, Farms to farmland resources.

| Field           | Type                                                  | Description                                                                    |
|-----------------|-------------------------------------------------------|--------------------------------------------------------------------------------|
| ID              | int64                                                 | Internal identifier                                                            |
| Game ID         | int64                                                 | The game this resource belongs to                                              |
| Planet ID       | int64                                                 | The planet this resource is on                                                 |
| Resource Type   | enum (`ore`, `energy`, `gold`, `materials`, `farmland`) | Resource type (see Resource Types)                                             |
| Capacity        | int                                                   | Maximum number of extractors assignable to this resource                       |
| Base Extraction | int                                                   | Gross units produced per staffed extractor per turn                            |
| Yield Percent   | int                                                   | Percentage of gross that becomes usable output (1–100)                         |
| Reserves        | int                                                   | Total extractable units remaining (meaningful only when `Is Infinite = false`) |
| Is Infinite     | bool                                                  | When true, the resource is never depleted and `Reserves` is ignored            |

Constraints:

- **UNIQUE (Game ID, ID)** — parent-key shape for composite FK from extractors.
- **FOREIGN KEY (Game ID, Planet ID) REFERENCES planets(Game ID, ID)** — per A1.

Notes:

- A planet has many natural resource rows, one per specific site. A planet's farmland rows are not special-cased — they are natural resources with `Resource Type = farmland`.
- `Resource Type` gates extractor eligibility: Farms may only be assigned to rows with `Resource Type = farmland`; Mines only to rows with any other type. Engine enforces.
- Production values (`Capacity`, `Base Extraction`, `Yield Percent`, `Is Infinite`) are set at creation and do not change. `Reserves` decrements each turn only when `Is Infinite = false`.
- Gross per staffed extractor per turn = `Base Extraction`. Usable output = `Base Extraction × Yield Percent / 100`. `Reserves` decrements by usable output, not gross.
- `Is Infinite = true` replaces the earlier `Reserves = 0 = infinite` sentinel (closes D1). Homeworld resources are created with `Is Infinite = true`; the engine no longer needs a special-case `Home World` check during extraction.
- Output unit type is derived from `Resource Type` via the resource-to-output mapping defined in `units-model.md`.

## Colonies and Ships

Colonies and Ships have been merged into a single `Vessel` entity defined in `empire-model.md`. See `empire-model.md` §Vessel and §Vessel Inventory.

## Population

Population Groups and Training Queue have been hoisted to `empire-model.md`, keyed on `Vessel ID`. See `empire-model.md` §Population Group and §Training Queue.

## Infrastructure

Mines, Farms, and Factories are Unit catalog entries (see `units-model.md` §Unit) that materialize as rows in `Vessel Inventory` (see `empire-model.md`). Extractor assignment is captured by `Mining Group`, `Farming Group`, and `Factory Group` in `empire-model.md`.

## Inventory

Inventory on colonies and ships is superseded by `Vessel Inventory` defined in `empire-model.md`.

## Type Constants

### Resource Types

Resource types that exist in natural resources. Extractors (Mines for non-farmland, Farms for farmland) convert natural resources into unit-output types defined in `units-model.md`; the full resource-to-output mapping is documented there.

- `ore` — mined into `metals`
- `energy` — mined into `fuel`
- `gold` — mined into `gold`
- `materials` — mined into `non-metals`
- `farmland` — farmed into `food`

`rstu` is not a natural-resource type; it is a unit-output type produced by factories and is defined in `units-model.md`.

### Planet Types

Physical type of a planet. Alpha generation produces only `rocky` planets; the other types are declared here so the enum is stable for future generation rules.

- `rocky` — solid surface, typical of habitable worlds.
- `gas giant` — massive gas world with no solid surface.
- `asteroid belt` — a field of rocky debris rather than a single body.

*`Population Group Types` moved to `empire-model.md` §Population Group. `Unit Types` superseded by the Unit catalog in `units-model.md`. `Ship Types` superseded by the Vessel Type catalog in `units-model.md`.*
