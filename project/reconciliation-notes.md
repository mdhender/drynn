# World Model / World Generation Reconciliation Notes

Working notes for reconciling the drynn documentation set (`world-model.md`, `empire-model.md`, `units-model.md`, `world-generation.md`, `game-model.md`) against the current architecture. The reference docs were copied from a prior game engine; this tracks what needs to change before any of them is treated as a spec.

## Resolved (DRAFT-banner sweep, 2026-04-17)

- `game-model.md`, `world-model.md`, `empire-model.md`, `units-model.md` banners softened from "DRAFT — do not implement" to reconciled-with-schema form. Each banner now cites the migration file and states what (if anything) remains open for that doc.
- Dependent banners on `project/explanation/extractor-restrictions.md` and `project/explanation/roles-membership-and-status.md` updated to reflect that the surrounding model is reconciled and the schema has landed.
- `empire-model.md` §Open Questions note updated: all prior questions are closed; the DRAFT banner now tracks engine-code completeness, not unresolved model issues.
- Banners retained on `vacation-mode.md` (self-service flow is planned, not implemented) and on open-question docs (`units-model.md` still has production-phase questions; `name-normalization.md` is frozen for alpha).

## Resolved (Phase 3 — schema alignment, 2026-04-17)

- Settled model translated into `db/schema.sql`. 23 new tables: `games`, global catalogs (`vessel_types`, `units`, `unit_recipes`), world (`star_systems`, `jump_routes`, `planets`, `home_worlds`, `natural_resources`), empire layer (`agents`, `empires`, `players`, `empire_control`), per-empire views (`empire_system_names`, `empire_planet_names`, `empire_jump_point_knowledge`), vessels (`vessels`, `vessel_inventory`), and empire instance entities (`population_groups`, `training_queue`, `mining_groups`, `farming_groups`, `factory_groups`). Migration generated via `atlas migrate diff add_game_schema --env local` at `db/migrations/20260417040157_add_game_schema.sql`. `atlas migrate lint` passes with only PG110 (column padding) warnings, which are cosmetic.
- Conventions applied: BIGINT PKs via `BIGSERIAL` (UUID kept for `players.account_id` which FKs `users.id`); CHECK constraints for enums (matching existing `jwt_signing_keys.state` pattern, no Postgres `ENUM` types); composite FKs carrying `game_id` per A1 prevent cross-game references; functional `UNIQUE (lower(name))` indexes enforce case-insensitive uniqueness per `name-normalization.md`.
- Minor model doc alignments made during the schema pass:
  - `Player.Account ID` typed as `uuid` (FK to `users.id`) rather than `int64`.
  - `Empire System Name` and `Empire Planet Name` now carry `Game ID` per A1, enabling composite FKs to both empire and system/planet and preventing cross-game references.
- Sprint-scoped language removed from `empire-model.md` (the one remaining "Schema only in Sprint 4" reference on `Empire Jump Point Knowledge` is gone; schema exists now).
- **Deferred from this pass:** sqlc queries (to land alongside engine code per file), RLS policies (no session-role infrastructure in place), world-generation logic (Go code, not schema), seed data, DRAFT-banner softening sweep on the model docs.

## Resolved (F2 closure, 2026-04-16)

- **F2** (name uniqueness) closed across every player-settable `Name` field:
  - `Empire.Name` — `UNIQUE (Game ID, lower(Name))`.
  - `Agent.Name` — `UNIQUE (lower(Name))` (global, since Agent is a global catalog).
  - `Empire System Name.Name` — `UNIQUE (Empire ID, lower(Name))`.
  - `Empire Planet Name.Name` — `UNIQUE (Empire ID, lower(Name))`.
  - `Vessel.Name` — `UNIQUE (Game ID, Empire ID, lower(Name))`.
  Every rule is case-insensitive via a functional index over `lower(Name)`. Names are stored with submitted case intact; comparison is case-insensitive in both the schema and the order processor. Normalization (character whitelist, whitespace → space, trim, collapse, non-empty) is the order processor's responsibility at input time; the schema constraint backstops it. Full spec in new document `project/reference/name-normalization.md`. Cross-table ambiguity (e.g., a Vessel and a System sharing a name within one empire) is an order-parser concern, not a schema constraint.
- **Whitelist closed for alpha:** `[A-Za-z0-9 .\-#']` — ASCII letters, digits, space, period, hyphen-minus, number sign, apostrophe. Intentionally 1978-era ASCII. Unicode letters and typographic punctuation are rejected, not normalized. Widening the set post-alpha is feasible but not on the alpha path. Full spec in `project/reference/name-normalization.md` §Character Whitelist.
- `Player.Account` parked: account handle uniqueness is enforced at the account layer (see CLAUDE.md handle rules); no new rule in the game-scoped model.

## Resolved (B8 closure, 2026-04-16)

- **B8** (extractor same-planet invariant) closed on structural grounds, not via declarative constraint. The invariant `Mining/Farming Group → Natural Resource.Planet ID = Vessel.Planet ID` holds because:
  1. Only surface-colony Vessel Types accept "build mining group" / "build farming group" orders (engine rule).
  2. Surface colonies are planet-bound and cannot move off-planet (engine rule).
  3. Surface colonies must be built on the surface of a planet (engine rule).
  Together these guarantee the Vessel associated with any Mining/Farming Group is immobile and planet-bound, so a row can never drift into a state where the planets disagree. No denormalized `Planet ID`, composite FK extension, or trigger is required. The full reasoning (game-mechanics rationale, data-model rationale, schema trade-offs, and engine-code responsibilities) is captured in `project/explanation/extractor-restrictions.md`; the Mining Group and Farming Group Notes in `empire-model.md` cite that document so the question does not get re-opened.

## Resolved (standalone-items pass, 2026-04-16)

- **A2** — `Game` entity defined in new `project/reference/game-model.md` with fields `ID`, `Name`, `Status`, `Current Turn`.
- **B3** — `Empire Jump Point Knowledge` PK stated as `(Empire ID, Route ID, System ID)`; entity moved to `empire-model.md` under "Per-Empire World Views."
- **B7** — Jump Route invariants: canonical ordering (`System A ID < System B ID`) plus `UNIQUE (Game ID, System A ID, System B ID)` prevent self-loops and reverse-duplicate rows.
- **C2** — Deposit resource types expanded from "ore or fuel" to `ore`, `fuel`, `gold`, `materials`.
- **E1** — Per-empire system naming: new `Empire System Name` entity added to `empire-model.md`.
- **E2** — Game-level turn/clock: `Current Turn` field on `Game`.
- **Placement cluster** — Empire Jump Point Knowledge and Empire System Name live in `empire-model.md`. Resource-type constants split: deposit types (`ore`, `fuel`, `gold`, `materials`) in `world-model.md`; mine-output types (`metals`, `fuel`, `gold`, `non-metals`), `food`, and `rstu` in `units-model.md`. Deposit-to-output mapping chart will live with the Mine entity during the units hoist.
- **New field** — `Last Turn Used` added to Jump Route.
- **Doc scope boundary** — world-model = pre-empire physical world only; post-empire entities go to empire-model (control + per-empire views) or units-model.
- **Units/empire split (revision)** — `units-model.md` holds *type definitions only* (ship classes, building templates, unit-output constants, population types, factory recipes, deposit-to-output mapping). Empire-owned *instance* entities (Colony, Ship, Mine, Farm, Factory, Population Group, Training Queue, Inventory) go to `empire-model.md` during the Phase 2 hoist. This supersedes the earlier "empire-owned technical assets" framing.
- **A1 (Game ID denormalization)** — every game-scoped entity carries a `Game ID` that FKs to `game-model.md`. Composite FKs prevent cross-game references. `Agent` is global. Details in `game-model.md` §Game ID Invariant.

## Resolved (Phase 2 — world-generation rewrite, 2026-04-16)

- `project/reference/world-generation.md` rewritten against the settled model:
  - Alpha Constants table updated to model field names (`Capacity`, `Base Extraction`, `Yield Percent`, `Reserves`, `Is Infinite`). Added per-entity rows for farmland, standard mineable, and homeworld mineable.
  - Farmland Algorithm (renamed "Farmland Generation") creates `26 - LSN` Natural Resource rows per eligible planet with `Resource Type = farmland`, `Capacity = 1`, `Yield Percent = max(1, 15 - orbit)`, `Is Infinite = true`. Net farm output `= 15 - orbit` food per turn, matching prior alpha behavior.
  - Deposit Algorithm (renamed "Mineable Natural Resource Generation") updated: `Capacity = 1,000 * orbit`, `Yield Percent = 10`, `Reserves = LSN * 1,000,000`, `Is Infinite = false`, `Base Extraction = 100` (alpha default).
  - Homeworld rule absorbed into `Is Infinite = true` — the engine no longer needs a `Home World` flag check during extraction. `Home World` is inserted as a row in the Home World table rather than a flag on Planet.
  - `fuel` natural-resource references renamed to `energy` throughout.
  - `Game ID` scoping made explicit in Scope and Notes for Implementers; cross-references `game-model.md` §Game ID Invariant.
  - LSN wrap rules retained. Note clarifies that the generation `0..99` range is intentionally narrower than the model `0..100` range (planets are always strictly better than vacuum).
  - Jump Route Rule adds `Last Turn Used = NULL` at generation.
- Phase 2 bullets closed: "Deposit field naming" and "Generation never mentions Game ID scoping."
- DRAFT banner on `world-generation.md` softened — content is reconciled; only alpha `Base Extraction` defaults are flagged as provisional.

## Resolved (Phase 1.5 G5 — roles/players explanation, 2026-04-16)

- `project/explanation/roles-membership-and-status.md` rewritten against the Player model. Obsolete sections removed: `Game Membership Types`, `Membership Status`, `Why agent Is A Status Instead of a Type`, and the old `Practical Language We Want`. New content describes `Player`, the `Is GM` flag, and `empire_control` as the control bridge.
- Title updated to "Roles and Game Participation." Filename kept (`roles-membership-and-status.md`) to preserve cross-references; a note in the DRAFT banner flags the legacy filename.
- DRAFT banner softened: application-role content is current; game-participation content remains forward-looking until the schema lands.

## Resolved (units hoist — Sub-sweep 3, 2026-04-16)

- **`Population Group`** hoisted to `empire-model.md`: PK `(Game ID, Vessel ID, Group Type)`. `Group Type` is an inline enum (`untrained`, `worker`, `manager`, `soldier`, `pilot`). Previous Colony-ID / Ship-ID XOR replaced by single Vessel ID.
- **`Training Queue`** hoisted to `empire-model.md`: PK `(ID)` + `UNIQUE (Game ID, ID)`. Vessel ID replaces Colony ID. Training durations retained in an inline subsection. Engine enforces training-on-colony-type-vessels.
- **`Mining Group`, `Farming Group`, `Factory Group`** added to `empire-model.md` per the Group-No priority model. Composite PKs per your spec; FK into `Vessel Inventory` via `(Vessel ID, Unit Code, Tech Level)` so committed Units must exist on the vessel.
- **Removed from `world-model.md`:** `Population Groups`, `Training Queue`, `Mines`, `Farms`, `Factories`, `Inventory`. Each section replaced by a pointer. Type Constants trimmed: `Population Group Types`, `Unit Types`, `Ship Types` removed (superseded by empire-model enum, units-model Unit catalog, and units-model Vessel Type catalog respectively). `world-model.md` now holds only pre-empire world entities plus pointers.
- **Empire-model scope and data shape** updated to include all five new entities.
- **Phase 1 closures in this pass:**
  - **B1** (Population Group no ID) — composite PK `(Game ID, Vessel ID, Group Type)`.
  - **B2** (Inventory no ID) — Inventory entity removed; `Vessel Inventory` (Sub-sweep 2) carries a composite PK.
  - **B5** (Population / Inventory XOR) — XOR gone; both now key on Vessel ID directly.
  - **C1** (lookup discipline) — fully closed. `Vessel Type Code` and `Unit Code` are FKs to global catalogs; `Group Type` is an inline enum. Stringly-typed `Resource Type` on old Inventory is gone.
- **Still open:** none from Sub-sweep 3. F2 closed under the 2026-04-16 F2 pass; see top of file.

## Resolved (units hoist — Sub-sweep 2, 2026-04-16)

- **`Vessel` entity** added to `empire-model.md`: fields `(ID, Game ID, Empire ID, Vessel Type Code, Name, Status, Tech Level, Planet ID, System ID, Docked At Vessel ID)`. PK `(ID)` + `UNIQUE (Game ID, ID)` parent-key shape. Composite FKs to Empire / Planet / Star System / self per A1. `Vessel Type Code` FKs to the global catalog in `units-model.md`. CHECK: exactly one location field non-null; Tech Level ∈ [0, 10].
- **`Vessel Inventory` entity** added: PK `(Game ID, Vessel ID, Unit Code, Tech Level)`. Attributes `Quantity`, `Active`, `Cargo`, `Mass`, `Volume` reserved; semantics deferred to production-phase pass.
- **Colonies and Ships sections removed** from `world-model.md`; replaced by a pointer to the new `Vessel` entity.
- **Empire-model scope and data shape** updated to include Vessel and Vessel Inventory.
- **Phase 1 closes in this pass:**
  - **B4** (Ship `Colony ID`/`System ID` XOR) — replaced by Vessel's three-way location XOR CHECK.
  - **E3** (colony lifecycle) — `Vessel.Status` covers both colonies and ships.
  - **F1** (Movement Points naming) — moved to `Vessel Type.Movement Points` in `units-model.md` (baseline per class, not per instance).
- **B6 obsolete**: "One colony per empire per planet" no longer applies — under Vessel, an empire can have many vessels on the same planet (colonies of different types plus landed ships). If "one *surface colony* per empire per planet" is still desired, that's an engine rule, not a schema constraint.

Remaining cleanup of `world-model.md` (pending Sub-sweep 3): `Mines`, `Farms`, `Factories`, `Population Groups`, `Training Queue`, `Inventory` sections still reference `Colony ID` / `Ship ID` and are scheduled for removal or restructure (Mines/Farms/Factories → Unit catalog; Population + Training → hoist to `empire-model.md`; Inventory → superseded by `Vessel Inventory`).

## Resolved (units hoist — Sub-sweep 1, 2026-04-16)

- **Unit catalog structure** defined in `units-model.md`: `Code` (PK), `Display Name`, optional `Category`, `Source` (enum `mined`/`farmed`/`factory`). Catalog is global (no Game ID), matching the `Agent` pattern.
- **Unit Recipe** table defined: `(Unit Code, Input Code)` PK, `Quantity` attribute. Applies only to Units with `Source = factory`.
- **Vessel Type** catalog defined: `Code` (PK), `Display Name`, `Category` (`ship`/`colony`), `Movement Points`, `Cargo Capacity`. Global.
- **Deposit-to-output mapping** documented in `units-model.md`: `ore → metals`, `energy → fuel`, `gold → gold`, `materials → non-metals`, `farmland → food`.
- **Natural Resource `fuel` → `energy` rename** applied in `world-model.md`. Unit `fuel` unchanged. Mining `energy` produces `fuel`.
- **Core concept shifts recorded** for downstream sub-sweeps:
  - Ships and colonies will merge into a single `Vessel` entity (Sub-sweep 3).
  - Mines/Farms/Factories are Unit catalog entries, not separate instance entities. They materialize as rows in `Vessel Inventory`.
  - Population is *not* modeled as Units (not manufactured); `Population Group` remains a separate entity in `empire-model.md`.
  - Tech Level (0..10, starting at 1) is a per-instance attribute on `Vessel` and `Vessel Inventory`; not a catalog field.

## Resolved (systems-and-planets sweep, 2026-04-16)

- **Star System `Home System` bool** added. Immutable. Constraints: `UNIQUE (Game ID, ID)`, `UNIQUE (Game ID, X, Y)` — one system per coordinate per game (no multi-star systems).
- **Planet fields** added: `Game ID` (per A1), `Planet Type` enum (`rocky`, `gas giant`, `asteroid belt`; alpha generates only `rocky`), `LSN int` with `CHECK (0–100)`. LSN expanded to "Life Support Number" — 0 = no life support required, 100 = space-equivalent.
- **Home World as a side table** in `world-model.md` (PK `(Game ID, Planet ID)`), not a bool on Planet. Designation is immutable after generation.
- **Planet `Name` removed** from `world-model.md`; per-empire names moved to new `Empire Planet Name` in `empire-model.md`, mirroring `Empire System Name`.
- **Planet Types** Type Constant added to `world-model.md`.
- **LSN range note**: model allows `0..100` because `100` represents space-equivalent — an LSN value no planet reaches. Generation produces planet LSNs in `0..99` because rocky planets are always strictly better than vacuum. Both are intentionally correct; no reconciliation needed on this point.
- **Phase 2 "Generation references LSN, Planet Type, Home System, Home World"** closed at the model level.

## Resolved (natural-resources sweep, 2026-04-16)

- **Farmland merged into Deposit**, renamed `Natural Resource`. `Resource Type` enum now includes `farmland` alongside `ore`, `fuel`, `gold`, `materials`. Many rows per planet (like deposits). Engine gates extractor type (mine vs farm) by `Resource Type`.
- **New `Is Infinite` bool** on Natural Resource replaces the `Reserves = 0 = infinite` sentinel (closes D1). Homeworld resources use `Is Infinite = true`; the engine no longer needs a `Home World` special case during extraction.
- **Mines and Farms** updated: `Deposit ID` / `Farmland ID` renamed to `Resource ID`, both now FK to Natural Resource. Eligibility gated by `Resource Type`.
- **C1 partially resolved** — `Resource Type` is a declarative enum. `Population Group Type`, `Ship Type`, and `Inventory.Resource Type` remain open, pending the units hoist.
- **C2 re-resolved** — Natural-resource `Resource Type` set is `ore`, `fuel`, `gold`, `materials`, `farmland`.
- **Phase 2 "Farmland cardinality"** closed at the model level. Many natural-resource rows per planet, any number of which may be `farmland`.
- **Phase 2 "Homeworld immutable deposit"** closed at the model level. `Is Infinite = true` replaces the special case.

## Scope discipline

Entity sweeps from here forward close Phase 1 schema issues only. Corresponding rewrites of `world-generation.md` — which still uses names like `quantity`, `yield`, `farmland count`, `farmland yield`, and carries the `Home World` immutable-deposit special case — are deferred to a later Phase 2.5 pass after the model is settled.

## Phase 1 — Schema issues in `world-model.md`

These are data-model problems internal to `world-model.md`, independent of generation.

### A. Game scoping

- **A1. [RESOLVED]** Decision: denormalize `Game ID` on every game-scoped entity. Reasoning: game is a hard tenant boundary, every query is in-game scoped, simpler tenant isolation, RLS compatibility, partitioning/archival flexibility. Composite FKs (`FOREIGN KEY (game_id, parent_id) REFERENCES parent(game_id, id)`) prevent cross-game references at the DB level. The `Agent` entity is explicitly global (no Game ID) because agents are shared across games. Details in `game-model.md` §Game ID Invariant.
- **A2.** The `Game` entity itself is never defined in this doc. Everything references `Game ID` but there's no table for it. Either add it here or cite the doc that owns it.

### B. Missing constraints and keys

- **B1.** `Population Groups` has no `ID` field. Intended composite PK is presumably `(Colony ID | Ship ID, Group Type)`. Spell it out.
- **B2.** `Inventory` has no `ID` field. Same issue — composite PK on `(Colony ID | Ship ID, Resource Type)`. Spell it out.
- **B3.** `Empire Jump Point Knowledge` has no `ID` field. Composite PK likely `(Empire ID, Route ID, System ID)`. Spell it out.
- **B4.** `Ships.Colony ID` / `Ships.System ID` XOR constraint ("Exactly one must be set") is prose-only. Needs a CHECK constraint note.
- **B5.** `Population Groups` and `Inventory` have the same XOR on `Colony ID` / `Ship ID`. Same fix.
- **B6.** "One colony per empire per planet" needs a unique constraint on `(Empire ID, Planet ID)`.
- **B7.** Jump Route endpoint rules aren't stated: prevent self-loops (`A == B`) and prevent duplicate routes `(A,B)` / `(B,A)`. Canonical ordering (e.g., `A < B`) with a unique constraint.
- **B8. [RESOLVED]** Closed on structural grounds during the 2026-04-16 B8 pass. The invariant holds because only surface-colony Vessel Types accept mine/farm-group orders and surface colonies are immobile and planet-bound. No declarative constraint needed; see `project/explanation/extractor-restrictions.md` for the full reasoning.

### C. Lookup table / enum discipline

- **C1.** `Deposits.Resource Type`, `Inventory.Resource Type`, `Population Groups.Group Type`, `Ships.Ship Type` are typed `string` but the doc says resource types live in a `units` lookup table. Decide: FK to lookup tables, or DB CHECK constraints, or PostgreSQL enums. Apply consistently.
- **C2.** `Deposits.Resource Type` prose says "`ore` or `fuel`" only. Confirm that's the full set for deposits (farmland produces food, factories produce RSTUs, materials are seeded — so yes, ore + fuel is the full deposit set).

### D. Sentinel values

- **D1.** `Deposits.Reserves = 0` meaning "infinite" is a magic value. Easy to miscompute when net output is subtracted. Options: nullable `Reserves` (NULL = infinite), separate `is_infinite bool`, or keep sentinel but document the subtraction rule explicitly.

### E. Missing entities

- **E1.** Per-empire system naming: the doc says "Systems start unnamed. Players name their own systems; names are per-player." No table defined for that mapping. Add `Empire System Name` (Empire ID, System ID, Name) or equivalent.
- **E2.** Game-level turn/clock: `Training Queue` uses `Start Turn` and `Completion Turn`, but no entity tracks the game's current turn. Add it here or cite where it lives.
- **E3.** Colony lifecycle: no field or state for a destroyed/abandoned colony. Decide if colonies are deleted, soft-deleted, or carry a status field.

### F. Field naming consistency

- **F1.** Ship `Movement Points` field name — check against whatever the movement/orders doc uses. Don't drift.
- **F2. [RESOLVED]** Closed during the 2026-04-16 F2 pass. Every player-settable `Name` field now carries a case-insensitive UNIQUE constraint at the appropriate scope (per-game for Empire; per-empire for Vessel, Empire System Name, Empire Planet Name; global for Agent). Normalization pipeline specified in the new `project/reference/name-normalization.md`; see top of file for full resolution entry.

## Phase 1.5 — Roles, players, and empire model

Parallel to Phase 1. Application-role vocabulary lives in `project/explanation/roles-membership-and-status.md`. Game-level control vocabulary was originally framed as `membership_type`/`membership_status` but is now modeled as a single `Player` entity in `project/reference/empire-model.md`.

- **G1. [RESOLVED]** `roles-membership-and-status.md` described `guest` as a stored downgrade role for disabled accounts. Current implementation: `guest` is a synthetic sentinel viewer for unauthenticated sessions. User prefers the current implementation; doc prose updated to match. No schema change.
- **G2. [RESOLVED]** All rounds of open questions in `empire-model.md` are closed. Final shape: control lives on the `empire_control` bridge with nullable `Player ID`, nullable `Agent ID`, and a `GM Set` boolean that gates player self-service over the agent column. Empire has no status column — inactive empires are just uncontrolled and are mechanically processed for production/attrition. Agents are shared across games and immutable once created; new versions add new rows. Auto-updates never happen; only the GM changes agent assignments (and only the GM can when `GM Set = true`). Admin-driven seat transfer is disallowed in all environments. GM lifecycle uses `resigned` status with the record otherwise intact; a fresh account may be added as a new GM afterward. Vacation is an out-of-game concept documented in `project/explanation/vacation-mode.md`.
- **G3. [RESOLVED]** The Empire entity has been hoisted to `empire-model.md` together with Player, Agent, and a new `empire_control` bridge. `world-model.md` §Empires is now a pointer. Control lives on `empire_control`, not on Empire; `Membership ID` no longer exists as a field anywhere.
- **G4. [RESOLVED]** The controlling entity is `Player`, documented in `empire-model.md`. No separate Membership section is needed in `world-model.md`.
- **G5. [RESOLVED]** `roles-membership-and-status.md` rewritten against the Player model. Obsolete membership sections removed; new content describes `Player`, `Is GM`, and `empire_control` as the control bridge. Title updated to "Roles and Game Participation"; filename kept to preserve cross-references.
- **G6.** Control-shape shift: prior rounds of this doc assumed an "agent Player row" pattern. The final design puts control on an `empire_control` bridge with nullable `Player ID` and `Agent ID`. Vacation, resignation, and re-humanizing are all bridge-row edits; no `superseded` status was needed. Any earlier notes or scratch in this file or `empire-model.md` referencing `Controlled By` on Player, `Agent ID` on Player, or `superseded` are obsolete.

## Phase 2 — Cross-document conflicts (deferred)

Tracked for later; don't start until Phase 1 is resolved.

- **[RESOLVED at model level]** Farmland cardinality: Farmland is now a `Resource Type` value in the Natural Resource entity; a planet has many rows including any number of farmland rows. Generation doc rewrite deferred.
- **[RESOLVED]** Deposit field naming: generation rewrite uses model vocabulary (`Capacity`, `Base Extraction`, `Yield Percent`, `Reserves`, `Is Infinite`). `Base Extraction = 100` alpha default set explicitly.
- **[RESOLVED]** Generation references `LSN`, `Planet Type`, `Home System`, `Home World` — all now on the model. The `0..100` model range vs `0..99` generation range is intentional (planets are always strictly better than vacuum) and does not require reconciliation.
- **[RESOLVED]** Generation `Game ID` scoping: Scope and Notes for Implementers now state all generated rows carry the current `Game ID`, with cross-reference to `game-model.md` §Game ID Invariant.
- **[RESOLVED at model level]** Homeworld immutable deposit: replaced by `Is Infinite = true` on the Natural Resource row. Generation doc rewrite deferred.
- **[RESOLVED]** Units hoist complete across Sub-sweeps 1–3. Instance rows moved to `empire-model.md` (as `Vessel`, `Vessel Inventory`, `Population Group`, `Training Queue`, `Mining/Farming/Factory Group`); type definitions consolidated into `units-model.md` (as `Unit`, `Unit Recipe`, `Vessel Type`).

## Phase 3 — Architecture alignment

- **[RESOLVED]** Model translated into `db/schema.sql`; migration `20260417040157_add_game_schema.sql` generated and lints clean. See the 2026-04-17 Phase 3 entry at the top of this file for the full rundown.
- **[RESOLVED]** Sprint-scoped language ("Schema only in Sprint 4") removed from `empire-model.md`. The prior "Sprint 5/6" references in inherited docs were already gone by the time this pass ran.
