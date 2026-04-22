# Proposal: New World Model

Maps `worldgen.Galaxy` to the database schema and resolves gaps between the generator output and the current game tables.

## Context

The world generator (`internal/worldgen`) produces a `Galaxy` containing systems on a hexagonal (axial) grid. Each system contains multiple stars, each star owns 1–10 planets with rich physical attributes. The current schema has `star_systems` and `planets` but no `stars` table, and `planets` lacks columns for the detailed attributes the generator produces.

## Phase 1 — Schema Changes

### 1a. Add `stars` table

```sql
stars (
    id BIGSERIAL PK,
    game_id BIGINT NOT NULL,
    system_id BIGINT NOT NULL,
    ordinal INT NOT NULL,       -- position within system (1-based)
    kind TEXT NOT NULL,          -- 'dwarf', 'degenerate', 'main-sequence', 'giant'
    color TEXT NOT NULL,         -- 'blue', 'blue-white', 'white', 'yellow-white', 'yellow', 'orange', 'red'
    size INT NOT NULL            -- 0..9
)
```

- FK: `(game_id, system_id) → star_systems(game_id, id) ON DELETE CASCADE`
- Unique: `(game_id, id)` for composite FK use by children.
- Unique: `(game_id, system_id, ordinal)` to prevent duplicate ordinals.

### 1b. Re-parent `planets` from systems to stars

- Replace `planets.system_id` with `star_id`.
- Composite FK: `(game_id, star_id) → stars(game_id, id) ON DELETE CASCADE`.
- Unique constraint becomes `(game_id, star_id, orbit)`.

### 1c. Add `planet_details` table

Mutable attributes (terraforming can change these):

```sql
planet_details (
    game_id BIGINT NOT NULL,
    planet_id BIGINT NOT NULL,
    diameter INT NOT NULL,              -- thousands of km
    density NUMERIC NOT NULL,           -- earth ~= 5.5
    gravity NUMERIC NOT NULL,           -- in G's; earth = 1.0
    temperature_class INT NOT NULL,     -- 1..30 (3..7 for gas giants)
    pressure_class INT NOT NULL,        -- 0..29
    mining_difficulty NUMERIC NOT NULL,
    PRIMARY KEY (game_id, planet_id)
)
```

FK: `(game_id, planet_id) → planets(game_id, id) ON DELETE CASCADE`.

### 1d. Add `planet_atmospheres` table

```sql
planet_atmospheres (
    game_id BIGINT NOT NULL,
    planet_id BIGINT NOT NULL,
    gas TEXT NOT NULL,       -- 'H2', 'CH4', 'He', 'NH3', 'N2', 'CO2', 'O2', 'HCl', 'Cl2', 'F2', 'H2O', 'SO2', 'H2S'
    percent INT NOT NULL,   -- 0..100
    PRIMARY KEY (game_id, planet_id, gas)
)
```

FK: `(game_id, planet_id) → planets(game_id, id) ON DELETE CASCADE`.

Terraforming a planet updates `planet_details` and may also change `planet_atmospheres` rows.

### 1e. Remove `lsn` from `planets`

LSN is not a planet attribute. It is a computed value that bridges species (race?) and `planet_details`. Every species evaluates the same planet differently based on their biological requirements. If a planet is terraformed, `planet_details` change, and the LSN must be recomputed for **all** species.

> **Open question:** Species or Race? The naming convention needs to be decided before we build the Empire layer.

LSN computation will be implemented when we work on species/empires.

### 1f. Leave `jump_routes` in place

Open design question — jump routes may be replaced by a mechanic that uses hexagonal distance between systems. No changes for now.

### 1g. Leave `home_worlds` / `is_home_system` in place

These are populated during Empire setup, not by worldgen seeding. When we're ready for Empires, the generator will mutate star/planet data for the designated home system.

### 1h. `Planet.Special` and `System.HomeSystem` are not persisted

These are generator-internal flags used during world creation. They do not map to database columns.

## Phase 2 — Add Natural Resource Generation to Worldgen

- Extend the planet generator to produce resource deposits (ore, energy, gold, materials, farmland).
- Use `MiningDifficulty` as an input to resource capacity/yield rolls.
- Output maps directly to `natural_resources` rows during seeding.

## Phase 3 — Write the Seeder

A function (in `internal/service/` or a `cmd/db` subcommand) that:

1. Creates a `games` row.
2. Calls `worldgen.Generate(...)`.
3. Inserts `star_systems` — `Hex.Q → x`, `Hex.R → y`, `is_home_system = false`.
4. Inserts `stars` per system — `Kind`, `Color`, `Size`, `ordinal` (1-based index within system).
5. Inserts `planets` per star — orbit index → `orbit`, derived `planet_type` (`Diameter > 40 → 'gas giant'`, else `'rocky'`).
6. Inserts `planet_details` — diameter, density, gravity, temperature/pressure class, mining difficulty.
7. Inserts `planet_atmospheres` — gas composition from `Planet.Gases`.
8. Inserts `natural_resources` per planet (from Phase 2 output).

### Mapping summary

| Worldgen field            | DB column                                          |
|---------------------------|----------------------------------------------------|
| `System.Hex.Q`            | `star_systems.x`                                   |
| `System.Hex.R`            | `star_systems.y`                                   |
| `Star.Kind.String()`      | `stars.kind`                                       |
| `Star.Color.String()`     | `stars.color`                                      |
| `Star.Size`               | `stars.size`                                       |
| star index (1-based)      | `stars.ordinal`                                    |
| orbit index (1-based)     | `planets.orbit`                                    |
| `Diameter > 40`           | `planets.planet_type = 'gas giant'` else `'rocky'` |
| `Planet.Diameter`         | `planet_details.diameter`                          |
| `Planet.Density`          | `planet_details.density`                           |
| `Planet.Gravity`          | `planet_details.gravity`                           |
| `Planet.TemperatureClass` | `planet_details.temperature_class`                 |
| `Planet.PressureClass`    | `planet_details.pressure_class`                    |
| `Planet.MiningDifficulty` | `planet_details.mining_difficulty`                 |
| `Planet.Gases[gas]`       | `planet_atmospheres.gas`, `.percent`               |

## Phase 4 — Galaxy Viewer

Hook the existing `RenderDiskSVG` into the web app as a game-scoped page. The SVG renderer already produces an embeddable `<svg>` fragment suitable for a `.gohtml` template.

### Routes

| Route                         | Handler        | Auth                | Content                    |
|-------------------------------|----------------|---------------------|----------------------------|
| `/app/admin/games/:id/galaxy` | `AdminHandler` | admin               | Map + system/planet report |
| `/app/games/:id/galaxy`       | `AppHandler`   | active player or GM | Map only (no reports)      |

The GM sees the same full view as admin (map + reports). Active players in the game see the system map only.

### Implementation

1. Register `"admin/galaxy"` and `"app/galaxy"` templates in `render.go`, using the `app.gohtml` layout.
2. Add `ShowGalaxy` handler to `AdminHandler` — loads star systems + stars + planets from DB, builds `[]viewerSystem` for `RenderDiskSVG`, passes the SVG as `template.HTML` plus planet report data into the template.
3. Add `ShowGalaxy` handler to `AppHandler` — same SVG rendering, but no planet/system report. Must verify the player is active in the game.
4. GM check: if the player has `is_gm = true` for the game, render the full admin view (map + reports) even on the `/app/` route.

### Viewer code

The `worldgen` viewer files (`viewer.go`, `hexviewer.go`) stay as-is. `RenderDiskSVG` is the reusable entry point. The standalone `ToHTML` / `RenderDiskHTML` methods remain available for dev/debug use but are not used by the web app.

## Deferred (Empire work)

- `asteroid belt` planet type
- LSN computation (species/race-dependent, derived from `planet_details` + `planet_atmospheres`)
- `home_worlds` / `is_home_system` assignment and star/planet mutation
- Jump routes redesign
