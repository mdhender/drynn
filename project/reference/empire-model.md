# Empire Model Reference

**STATUS: DRAFT — do not implement.** This document has not yet been reconciled with the current drynn architecture. Known issues are tracked in `project/reconciliation-notes.md`. Coding agents must not build schema, store, or engine code against this spec until the DRAFT marker is removed.

Technical reference for the empire side of the drynn game model: the Empire itself, the Player entity that represents a human seat in a game, the Agent entity that represents an engine strategy, and the `empire_control` bridge that records who currently operates each empire.

Application-level authorization (`role`) is out of scope for this document. See `project/explanation/roles-membership-and-status.md` for the distinction between application roles and game participation. Vacation — an out-of-game player concept — is explained in `project/explanation/vacation-mode.md`. Empire-owned assets are defined in `world-model.md` and (forthcoming) `units-model.md`.

## Scope

This document defines:

- `Empire` — a faction in a game.
- `Player` — an account's seat in a specific game.
- `Agent` — a shared, versioned engine strategy that can operate a seat.
- `empire_control` — the bridge that records which player and which agent (if any) currently operate each empire, and whether the agent assignment was made by the GM.
- `Empire System Name` — per-empire names for star systems.
- `Empire Planet Name` — per-empire names for planets.
- `Empire Jump Point Knowledge` — per-empire knowledge of jump points.
- `Vessel` — ships and colonies merged. The order-receiving entity; holds inventory.
- `Vessel Inventory` — the Units held by each Vessel, per Tech Level.
- `Population Group` — people on a Vessel, keyed by Group Type.
- `Training Queue` — population being trained from one Group Type to another.
- `Mining Group`, `Farming Group`, `Factory Group` — order-established production groups on a Vessel, prioritized by Group Number.

It does not define the Game entity (see `game-model.md`), account-level authorization, or type definitions for ships, buildings, resources, and population groups (see `units-model.md`). Pre-empire world entities (star systems, jump routes, planets, deposits, farmland) are defined in `world-model.md`. Every `Game ID` in this document FKs to the `Game` entity in `game-model.md`.

## Data Shape

```
Game              1 — N  Empire
Game              1 — N  Player
Game              1 — N  Vessel
Player            N — 1  Account
Empire            1 — 1  empire_control
empire_control    N — 1  Player   (Player ID nullable)
empire_control    N — 1  Agent    (Agent ID nullable)
Empire            1 — N  Vessel
Vessel            N — 1  Vessel Type         (global catalog)
Vessel            1 — N  Vessel Inventory
Vessel Inventory  N — 1  Unit                (global catalog)
Vessel            1 — N  Population Group
Vessel            1 — N  Training Queue
Vessel            1 — N  Mining Group
Vessel            1 — N  Farming Group
Vessel            1 — N  Factory Group
Mining Group      N — 1  Natural Resource
Farming Group     N — 1  Natural Resource
```

- A game has many empires and many players.
- A player is always backed by an account; there are no agent-backed Player rows.
- Each empire has exactly one `empire_control` row; both sides of the bridge may be NULL.
- Agents are assigned to empires directly via `empire_control.Agent ID`, not through Player rows.
- Agents are intentionally shared: one Agent record may be assigned to empires across many games simultaneously.
- Who submits orders for an empire is derived from `empire_control`:
  - agent if `Agent ID IS NOT NULL`, otherwise
  - player (via the account behind the referenced Player row) if `Player ID IS NOT NULL`, otherwise
  - no one — the empire is uncontrolled for the turn. Uncontrolled empires are mechanically processed for production and attrition; no orders are created.

## Empire

A faction in a game. Empires persist for the lifetime of a game.

| Field   | Type   | Description                     |
|---------|--------|---------------------------------|
| ID      | int64  | Internal identifier             |
| Game ID | int64  | The game this empire belongs to |
| Name    | string | Empire name                     |

Constraints:

- **PRIMARY KEY (ID)**; **UNIQUE (Game ID, ID)** — parent-key shape for composite FKs per A1.
- **UNIQUE (Game ID, lower(Name))** — case-insensitive per-game uniqueness. Normalization pipeline specified in `name-normalization.md`.

Notes:

- Empire-owned assets (colonies, ships, infrastructure, inventory, jump point knowledge) FK to `Empire.ID` and are defined in `world-model.md` and `units-model.md`. Control changes on `empire_control` never move assets between empires; assets stay with the Empire row.
- There is no status column on Empire. An empire with no controller (both `empire_control.Player ID` and `Agent ID` NULL) creates no orders and is mechanically processed for production and attrition. Over time its units may migrate to other empires via game mechanics; that migration does not delete the Empire record.
- Empires may also lose assets to other empires (seizure) or to independence; those mechanics are out of scope for this document.

## Player

An account's seat in a specific game. Every Player is account-backed; agents are not modeled as Players.

| Field      | Type                                      | Description                                                 |
|------------|-------------------------------------------|-------------------------------------------------------------|
| ID         | int64                                     | Internal identifier                                         |
| Game ID    | int64                                     | The game this player belongs to                             |
| Account ID | int64                                     | The account behind this seat (not nullable)                 |
| Is GM      | bool                                      | When true, this is a game-master seat with no empire        |
| Status     | enum (`active`, `resigned`, `eliminated`) | Current lifecycle state of the seat                         |

Constraints:

- **UNIQUE (Game ID, Account ID)** — across *all* Player rows, not just active ones. An account holds at most one Player record in a game for the lifetime of that game. This preserves the rejoin-block, prevents an account from being both GM and regular player in the same game, and lets a new GM be appointed (as a fresh account) after a prior GM resigns.
- **No `empire_control` row may reference a Player where `Is GM = true`.** GM seats do not control empires. Enforced at application level.

Notes:

- `resigned` — the seat holder left the game. Covers both account-initiated resignation (for regular players) and admin-initiated booting (for GMs). Row preserved for rejoin-block and history.
- `eliminated` — the Empire this player controlled was destroyed by game mechanics. Row preserved for history. Never applies to GM seats.
- Player rows have no `Empire ID` field. The relationship runs through `empire_control`.

## Agent

A shared, versioned engine strategy that can operate an empire seat.

| Field   | Type   | Description                             |
|---------|--------|-----------------------------------------|
| ID      | int64  | Internal identifier                     |
| Name    | string | Agent name (e.g., "Vacation Agent")     |
| Version | string | Version tag (e.g., `v1`, `2026-04-a`)   |

Constraints:

- **PRIMARY KEY (ID)**.
- **UNIQUE (lower(Name))** — case-insensitive global uniqueness. Agents are shared across games; their names must be unambiguous in admin and player-facing selection UIs. Normalization pipeline specified in `name-normalization.md`.

Notes:

- Agent rows are immutable once created. A new version of an agent is added as a new row, not by mutating an existing row. This prevents accidentally changing the behavior of every running empire by editing a shared record.
- Agents have no game scope; one Agent record can be assigned to empires across many games.
- Existing `empire_control.Agent ID` pointers are never auto-updated to a newer Agent version. Changing an empire's agent (version or identity) is always an explicit GM action.

## Empire Control

The bridge table that records who currently operates each empire.

| Field     | Type             | Description                                            |
|-----------|------------------|--------------------------------------------------------|
| Empire ID | int64            | Primary key; one row per Empire                        |
| Player ID | int64 (nullable) | The player authorized to submit orders for this empire |
| Agent ID  | int64 (nullable) | The agent currently submitting orders for this empire  |
| GM Set    | bool             | True if the current `Agent ID` was set by the GM       |

Constraints:

- **PRIMARY KEY (Empire ID)** — exactly one control row per empire.
- **UNIQUE (Player ID) WHERE Player ID IS NOT NULL** — a player controls at most one empire. Combined with `UNIQUE (Game ID, Account ID)` on Player, this enforces one empire per account per game.
- **CHECK: Agent ID IS NULL ⇒ GM Set = false.** The GM-set flag is only meaningful while an agent is assigned; clearing the agent also clears the flag.
- Cross-table invariant: when `Player ID` is set, its Player's `Game ID` must match the Empire's `Game ID`. Enforced at application level unless a composite FK is added.

State interpretations:

| Player ID | Agent ID | GM Set | Meaning                                                                           |
|-----------|----------|--------|-----------------------------------------------------------------------------------|
| set       | null     | false  | Player controls the empire; the account uploads orders.                           |
| set       | set      | false  | Vacation (self-service). Agent submits orders; player can clear the agent.        |
| set       | set      | true   | GM-assigned agent over an account-held seat. Agent submits; player cannot clear.  |
| null      | set      | true   | Post-resignation or post-elimination agent-only control (GM-assigned).            |
| null      | null     | false  | Uncontrolled. Newly created, between controllers, or effectively eliminated empire. |

### Authz derived from empire_control

- **Who submits orders** — agent if `Agent ID IS NOT NULL`; else player (via the account behind `Player ID`) if `Player ID IS NOT NULL`; else no one.
- **Who reads reports and history** — the account behind `Player ID` when that Player's `Status = 'active'`, regardless of `Agent ID`. GMs always see empire data by virtue of their GM Player row, not via `empire_control`.
- **Who can modify `Agent ID`** — the GM always. The account behind `Player ID` only when `GM Set = false`. This lets players self-service vacation while preserving the GM's ability to lock an assignment after a resignation.

## Per-Empire World Views

An empire's view of the game world — its names for star systems, its knowledge of jump points — is scoped to the empire and stored in per-empire tables. These reference world entities defined in `world-model.md` but are not shared between empires.

### Empire System Name

A per-empire name for a star system. Systems start unnamed; an empire may assign its own name, which is visible only to that empire.

| Field     | Type   | Description                        |
|-----------|--------|------------------------------------|
| Empire ID | int64  | The empire naming the system       |
| System ID | int64  | The star system being named        |
| Name      | string | The per-empire name for the system |

Constraints:

- **PRIMARY KEY (Empire ID, System ID)** — one name per empire per system.
- **UNIQUE (Empire ID, lower(Name))** — case-insensitive per-empire uniqueness. `Empire ID` is game-scoped, so this is implicitly per-game. Normalization pipeline specified in `name-normalization.md`.

Notes:

- System names are not shared between empires; each empire sees only its own names. System coordinates (X, Y) are game-level and identical across empires.
- Not all empires name every system; a missing record means "no per-empire name."
- `System ID` FKs to `Star Systems` in `world-model.md`.

### Empire Planet Name

A per-empire name for a planet. Planets start unnamed; an empire may assign its own name, which is visible only to that empire.

| Field     | Type   | Description                        |
|-----------|--------|------------------------------------|
| Empire ID | int64  | The empire naming the planet       |
| Planet ID | int64  | The planet being named             |
| Name      | string | The per-empire name for the planet |

Constraints:

- **PRIMARY KEY (Empire ID, Planet ID)** — one name per empire per planet.
- **UNIQUE (Empire ID, lower(Name))** — case-insensitive per-empire uniqueness. `Empire ID` is game-scoped, so this is implicitly per-game. Normalization pipeline specified in `name-normalization.md`.

Notes:

- Planet names are not shared between empires; each empire sees only its own names.
- Not all empires name every planet; a missing record means "no per-empire name."
- `Planet ID` FKs to `Planets` in `world-model.md`.

### Empire Jump Point Knowledge

Per-empire knowledge of jump points. Each record tracks what an empire knows about a single jump point observed from a specific system.

| Field             | Type   | Description                                         |
|-------------------|--------|-----------------------------------------------------|
| Game ID           | int64  | The game                                            |
| Empire ID         | int64  | The empire                                          |
| Route ID          | int64  | The jump route this knowledge applies to            |
| System ID         | int64  | The system the empire observes this jump point from |
| Detected          | bool   | Whether the empire knows this jump point exists     |
| Range Band        | string | Traversal difficulty category (nullable)            |
| Destination Known | bool   | Whether the empire knows where this route leads     |

Constraints:

- **PRIMARY KEY (Empire ID, Route ID, System ID)** — one knowledge record per empire, per jump point, per observing system.
- `Game ID` is carried on the row for query convenience and must match the Game of the referenced Empire and Route.

Notes:

- Knowledge is per-empire, per-jump-point, per-system.
- Each field is independent; an empire may detect a jump point without knowing its range band or destination.
- `Route ID` FKs to `Jump Routes` in `world-model.md`.
- Schema only in Sprint 4; see [Jump Point Knowledge and Orders](jump-point-knowledge-and-orders.md) for discovery rules and order mechanics.

## Vessel

A Vessel is an order-receiving entity owned by an empire. Ships and colonies are both Vessels — the `Vessel Type Code` distinguishes behavior; the engine interprets.

| Field               | Type                                      | Description                                                                                                                  |
|---------------------|-------------------------------------------|------------------------------------------------------------------------------------------------------------------------------|
| ID                  | int64                                     | Internal identifier                                                                                                          |
| Game ID             | int64                                     | The game this vessel belongs to                                                                                              |
| Empire ID           | int64                                     | The empire that owns this vessel                                                                                             |
| Vessel Type Code    | string                                    | FK to `Vessel Type` in `units-model.md` (e.g., `scout`, `transport`, `surface-colony`, `enclosed-colony`, `orbital-colony`)  |
| Name                | string                                    | Vessel name                                                                                                                  |
| Status              | enum (`active`, `abandoned`, `destroyed`) | Current lifecycle state                                                                                                      |
| Tech Level          | int (0..10)                               | Technology level at construction; alpha starts at 1                                                                          |
| Planet ID           | int64 (nullable)                          | Set if anchored to a planet (colonies; ships landed on a planet)                                                             |
| System ID           | int64 (nullable)                          | Set if present in a system without a planetary anchor (ships in transit or in system-level orbit)                            |
| Docked At Vessel ID | int64 (nullable)                          | Set if docked at another vessel (ships at colonies, ships at ships)                                                          |

Constraints:

- **PRIMARY KEY (ID)**; **UNIQUE (Game ID, ID)** — parent-key shape for composite FKs per A1.
- **FOREIGN KEY (Game ID, Empire ID) REFERENCES empires(Game ID, ID)**.
- **FOREIGN KEY (Vessel Type Code) REFERENCES vessel_types(Code)** — global catalog in `units-model.md`.
- **FOREIGN KEY (Game ID, Planet ID) REFERENCES planets(Game ID, ID)** — when `Planet ID IS NOT NULL`.
- **FOREIGN KEY (Game ID, System ID) REFERENCES star_systems(Game ID, ID)** — when `System ID IS NOT NULL`.
- **FOREIGN KEY (Game ID, Docked At Vessel ID) REFERENCES vessels(Game ID, ID)** — when `Docked At Vessel ID IS NOT NULL`.
- **CHECK (Tech Level BETWEEN 0 AND 10)**.
- **CHECK** exactly one of `Planet ID`, `System ID`, `Docked At Vessel ID` is non-null.
- **UNIQUE (Game ID, Empire ID, lower(Name))** — case-insensitive per-empire uniqueness. Normalization pipeline specified in `name-normalization.md`.

Notes:

- Orders in drynn are issued to Vessels; no other entity receives orders directly.
- `Status = active` — in play, accepting orders.
- `Status = abandoned` — no longer controlled but preserved for history.
- `Status = destroyed` — removed from play by game mechanics but preserved for history.
- Engine enforces location compatibility per `Vessel Type Code`: colonies anchor to planets; ships typically set `System ID` or dock at another vessel.
- `Tech Level` on the Vessel row is the vessel's own construction tech level and is fixed at creation. Units held in inventory carry their own Tech Level per row (see `Vessel Inventory`).

## Vessel Inventory

Records the Units held by a Vessel, at a specific Tech Level per Unit.

| Field      | Type        | Description                                                                           |
|------------|-------------|---------------------------------------------------------------------------------------|
| Game ID    | int64       | The game this inventory row belongs to                                                |
| Vessel ID  | int64       | The vessel holding this inventory                                                     |
| Unit Code  | string      | FK to `Unit` in `units-model.md`                                                      |
| Tech Level | int (0..10) | Tech level of the Units in this row                                                   |
| Quantity   | int         | Total count                                                                           |
| Active     | int         | Number currently operational; semantics deferred to production-phase pass             |
| Cargo      | int         | Reserved; semantics deferred to production-phase pass                                 |
| Mass       | int         | Reserved; semantics deferred to production-phase pass                                 |
| Volume     | int         | Reserved; semantics deferred to production-phase pass                                 |

Constraints:

- **PRIMARY KEY (Game ID, Vessel ID, Unit Code, Tech Level)** — one row per Unit-at-Tech-Level on a given Vessel.
- **FOREIGN KEY (Game ID, Vessel ID) REFERENCES vessels(Game ID, ID)** — per A1.
- **FOREIGN KEY (Unit Code) REFERENCES units(Code)** — global catalog in `units-model.md`.
- **CHECK (Tech Level BETWEEN 0 AND 10)**.
- **CHECK (Quantity >= 0)**, **CHECK (Active >= 0)**, **CHECK (Active <= Quantity)**.

Notes:

- A vessel with 5 mines at tech level 1 and 3 mines at tech level 2 has two rows: `(…, unit='mine', tech=1, qty=5)` and `(…, unit='mine', tech=2, qty=3)`.
- Full semantics of `Active`, `Cargo`, `Mass`, `Volume` are deferred to the production-phase modeling pass. Fields are reserved so the schema carries them from day one.
- `Mining Group`, `Farming Group`, and `Factory Group` (below) reference this table via `(Vessel ID, Unit Code, Tech Level)`.

## Population Group

The people residing on a Vessel, categorized by training level.

| Field      | Type                                                        | Description                              |
|------------|-------------------------------------------------------------|------------------------------------------|
| Game ID    | int64                                                       | The game this population row belongs to  |
| Vessel ID  | int64                                                       | The vessel hosting this population       |
| Group Type | enum (`untrained`, `worker`, `manager`, `soldier`, `pilot`) | Training category                        |
| Count      | int                                                         | Number of individuals in this group      |

Constraints:

- **PRIMARY KEY (Game ID, Vessel ID, Group Type)** — one row per Group Type per Vessel.
- **FOREIGN KEY (Game ID, Vessel ID) REFERENCES vessels(Game ID, ID)** — per A1.
- **CHECK (Count >= 0)**.

Notes:

- Population is not modeled as a Unit because it cannot be manufactured. Training transforms one Group Type into another via `Training Queue`.
- `untrained` — raw population, no specialisation.
- `worker` — staffs mines, farms, and factories.
- `manager` — supervises factory production (steady-state target: 1 manager per 10 factory workers).
- `soldier` — ground combat and defence.
- `pilot` — crews ships.
- Population lives on any Vessel (colony or ship); engine enforces type-specific operational limits.

## Training Queue

Population currently being trained from one Group Type to another.

| Field           | Type   | Description                                                       |
|-----------------|--------|-------------------------------------------------------------------|
| ID              | int64  | Internal identifier                                               |
| Game ID         | int64  | The game this training row belongs to                             |
| Vessel ID       | int64  | The vessel where training occurs (engine requires a colony type)  |
| From Group Type | string | Source Population Group Type                                      |
| To Group Type   | string | Target Population Group Type                                      |
| Count           | int    | Number of individuals in training                                 |
| Start Turn      | int    | Turn training began                                               |
| Completion Turn | int    | Turn training completes                                           |

Constraints:

- **PRIMARY KEY (ID)**; **UNIQUE (Game ID, ID)** — parent-key shape.
- **FOREIGN KEY (Game ID, Vessel ID) REFERENCES vessels(Game ID, ID)** — per A1.
- **CHECK (Count >= 0)**.
- **CHECK (Completion Turn >= Start Turn)**.
- `From Group Type` and `To Group Type` must be valid Population Group Type values; engine-enforced.

### Training Durations

Base durations (may be adjusted by tech level or engine rules):

| Target Group Type | Duration |
|-------------------|----------|
| `worker`          | 1 turn   |
| `manager`         | 4 turns  |
| `soldier`         | 4 turns  |
| `pilot`           | 4 turns  |

Notes:

- Training occurs only at Vessels of a colony type; engine enforces.
- On the Completion Turn, trained individuals move from the queue into the target `Population Group` row.

## Mining Group

An order-established assignment of mine Units on a Vessel to a non-farmland Natural Resource. Group Number captures priority for labor and materials allocation.

| Field       | Type        | Description                                                            |
|-------------|-------------|------------------------------------------------------------------------|
| Game ID     | int64       | The game this group belongs to                                         |
| Vessel ID   | int64       | The vessel whose mines are assigned                                    |
| Resource ID | int64       | The Natural Resource being mined (`Resource Type != farmland`)         |
| Group No    | int         | Group priority; lower number = higher priority                         |
| Unit Code   | string      | The Unit type (e.g., `mine`); FK to Unit catalog                       |
| Tech Level  | int (0..10) | Tech level of the Units in this group row                              |
| Quantity    | int         | Number of mine Units committed at this Unit × Tech Level               |

Constraints:

- **PRIMARY KEY (Game ID, Vessel ID, Resource ID, Group No, Unit Code, Tech Level)**.
- **FOREIGN KEY (Game ID, Vessel ID) REFERENCES vessels(Game ID, ID)** — per A1.
- **FOREIGN KEY (Game ID, Resource ID) REFERENCES natural_resources(Game ID, ID)** — per A1.
- **FOREIGN KEY (Unit Code) REFERENCES units(Code)** — global catalog in `units-model.md`.
- **FOREIGN KEY (Game ID, Vessel ID, Unit Code, Tech Level) REFERENCES vessel_inventory(Game ID, Vessel ID, Unit Code, Tech Level)** — committed Units must exist in the vessel's inventory.
- **CHECK (Tech Level BETWEEN 0 AND 10)**, **CHECK (Quantity > 0)**.

Notes:

- Multiple rows sharing `(Vessel ID, Resource ID, Group No)` with different `Tech Level` represent a mixed-tech group.
- Engine enforces: `Unit Code` is a mine-variant Unit; the referenced Natural Resource has `Resource Type != farmland`; the Natural Resource's `Planet ID` matches the Vessel's `Planet ID` at creation time.
- The same-planet invariant holds structurally, not by declarative constraint. The reasoning — surface-colony Vessel Type restriction, surface-colony immobility, and mandatory planet placement at creation — is documented in `project/explanation/extractor-restrictions.md`.
- Mines remain in `Vessel Inventory`; this table records only the active assignment. Unassigned mines stay in inventory without a group row.

## Farming Group

An order-established assignment of farm Units on a Vessel to a farmland Natural Resource. Same structural shape as Mining Group.

| Field       | Type        | Description                                                 |
|-------------|-------------|-------------------------------------------------------------|
| Game ID     | int64       | The game this group belongs to                              |
| Vessel ID   | int64       | The vessel whose farms are assigned                         |
| Resource ID | int64       | The Natural Resource being farmed (`Resource Type = farmland`) |
| Group No    | int         | Group priority                                              |
| Unit Code   | string      | The Unit type (e.g., `farm`); FK to Unit catalog            |
| Tech Level  | int (0..10) | Tech level of the Units in this group row                   |
| Quantity    | int         | Number of farm Units committed                              |

Constraints:

- **PRIMARY KEY (Game ID, Vessel ID, Resource ID, Group No, Unit Code, Tech Level)**.
- **FOREIGN KEY (Game ID, Vessel ID) REFERENCES vessels(Game ID, ID)**.
- **FOREIGN KEY (Game ID, Resource ID) REFERENCES natural_resources(Game ID, ID)**.
- **FOREIGN KEY (Unit Code) REFERENCES units(Code)**.
- **FOREIGN KEY (Game ID, Vessel ID, Unit Code, Tech Level) REFERENCES vessel_inventory(Game ID, Vessel ID, Unit Code, Tech Level)**.
- **CHECK (Tech Level BETWEEN 0 AND 10)**, **CHECK (Quantity > 0)**.

Notes:

- Engine enforces: `Unit Code` is a farm-variant Unit; the referenced Natural Resource has `Resource Type = farmland`; the Natural Resource's `Planet ID` matches the Vessel's `Planet ID` at creation time.
- The same-planet invariant holds structurally, as with Mining Group. See `project/explanation/extractor-restrictions.md` for the reasoning.

## Factory Group

An order-established group of factory Units on a Vessel. No Natural Resource reference; factories consume Units, not natural resources.

| Field      | Type        | Description                                                 |
|------------|-------------|-------------------------------------------------------------|
| Game ID    | int64       | The game this group belongs to                              |
| Vessel ID  | int64       | The vessel whose factories are assigned                     |
| Group No   | int         | Group priority                                              |
| Unit Code  | string      | The Unit type (e.g., `factory`); FK to Unit catalog         |
| Tech Level | int (0..10) | Tech level of the Units in this group row                   |
| Quantity   | int         | Number of factory Units committed                           |

Constraints:

- **PRIMARY KEY (Game ID, Vessel ID, Group No, Unit Code, Tech Level)**.
- **FOREIGN KEY (Game ID, Vessel ID) REFERENCES vessels(Game ID, ID)**.
- **FOREIGN KEY (Unit Code) REFERENCES units(Code)**.
- **FOREIGN KEY (Game ID, Vessel ID, Unit Code, Tech Level) REFERENCES vessel_inventory(Game ID, Vessel ID, Unit Code, Tech Level)**.
- **CHECK (Tech Level BETWEEN 0 AND 10)**, **CHECK (Quantity > 0)**.

Notes:

- Engine enforces: `Unit Code` is a factory-variant Unit.
- The Unit a factory group is currently producing (its per-turn order) is not a schema field — it is order state, handled in the production-phase modeling pass.

## Lifecycle Transitions

Each transition lists only the fields it changes.

### Game Setup

- Create Empire rows.
- Create one `empire_control` row per Empire with `Player ID = NULL`, `Agent ID = NULL`, `GM Set = false`.
- Create the GM Player row(s) with `Is GM = true`, `Status = 'active'`.
- For each joining account, create a Player row (`Is GM = false`, `Status = 'active'`) and assign it by setting `empire_control.Player ID` for one empire.

### Vacation (self-service)

Permitted only when `GM Set = false`. See `project/explanation/vacation-mode.md` for the player-facing flow.

- `empire_control.Agent ID` := chosen agent. `Player ID` remains set. `GM Set` remains `false`.
- Player row unchanged.

### Return from Vacation (self-service)

Permitted only when `GM Set = false`.

- `empire_control.Agent ID` := NULL.
- `empire_control.GM Set` := `false` (already false; stays false per the CHECK).
- Player row unchanged.

### Resignation (account quits)

- Player row: `Status` := `resigned`.
- `empire_control.Player ID` := NULL. `empire_control.Agent ID` stays as-is; `GM Set` stays as-is.
- If the GM then assigns an agent: `empire_control.Agent ID` := chosen agent, `GM Set` := `true`. (Assignment is a separate GM action, not part of the resignation itself.)

### Re-humanizing (new human takes an agent-controlled empire)

- GM creates a new Player row for the new account (`Is GM = false`, `Status = 'active'`). The INSERT is rejected by `UNIQUE (Game ID, Account ID)` if that account has any prior Player row in this game — this is the rejoin-block enforcement.
- `empire_control.Agent ID` := NULL, `GM Set` := `false`, `Player ID` := new Player ID.

### Elimination (empire destroyed by game mechanics)

- `empire_control.Player ID` := NULL, `Agent ID` := NULL, `GM Set` := `false`.
- The controlling Player: `Status` := `eliminated`.
- No replacement Player row is created.
- The Empire row persists with no controller. Subsequent turns apply production and attrition mechanically; unit migration proceeds per game rules.

### GM Resignation or Booting

- GM Player: `Status` := `resigned`. All other fields left intact.
- No `empire_control` changes (GM seats don't control empires).
- A new GM may be added by creating a fresh Player row with `Is GM = true, Status = 'active'` for a new account. The resigned GM's account cannot take any seat in this game (`UNIQUE (Game ID, Account ID)` blocks).

### Admin-driven Seat Transfer

Not supported in any environment. Reassigning a seat from one account to another — production, dev, or test — is explicitly disallowed. It would bypass the rejoin-block and break the fairness invariant that an account may hold at most one Player row per game for the lifetime of that game.

## Open Questions

All prior open questions have been resolved. Removal of the DRAFT marker is gated on Phase 2 / Phase 3 work tracked in `project/reconciliation-notes.md` (cross-doc conflicts and alignment with `db/schema.sql`).
