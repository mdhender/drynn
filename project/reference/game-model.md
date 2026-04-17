# Game Model Reference

**STATUS: DRAFT.** Reconciled with current architecture and translated into `db/schema.sql` (migration `20260417040157_add_game_schema.sql`). Game settings (turn cadence, generation parameters) and archival policy remain open — see §Open Questions. Coding agents may build against the defined fields.

Technical reference for the `Game` entity. A game is the top-level container that every other model entity scopes to. Empires, players, star systems, planets, jump routes, and their children all carry a `Game ID` that FKs here.

## Scope

This document defines:

- `Game` — the top-level container for a single drynn match.

It does not define game setup procedure, turn-processing mechanics, scoring, or archival.

## Game

| Field        | Type                                    | Description                                                     |
|--------------|-----------------------------------------|-----------------------------------------------------------------|
| ID           | int64                                   | Internal identifier                                             |
| Name         | string                                  | Human-readable title (e.g., "Orion Nebula 2026")                |
| Status       | enum (`setup`, `active`, `completed`)   | Game lifecycle state                                            |
| Current Turn | int                                     | Turn counter; starts at `0`, increments at the end of each turn |

Notes:

- `Status = 'setup'` — the galaxy has been generated and empires may have been assigned, but no turn has been processed yet.
- `Status = 'active'` — the game is in play; turns are being processed.
- `Status = 'completed'` — the game has ended. No further turns are processed; records are preserved for history.
- Every `Game ID` reference across `world-model.md`, `empire-model.md`, `units-model.md`, and `world-generation.md` FKs to this entity.

## Game ID Invariant

Every game-scoped entity in the drynn model carries a `Game ID` column that FKs to `Game.ID`. This includes: Empire, Player (see `empire-model.md`), Star System, Jump Route, Planet, Deposit, Farmland (see `world-model.md`), and every empire-owned instance (see `empire-model.md` once the Phase 2 hoist completes).

The `Agent` entity is intentionally global and has no `Game ID`; agents are shared across games.

### Composite foreign keys

Child entities reference their parents with composite FKs that include `Game ID`, preventing cross-game references at the DB level:

```
FOREIGN KEY (game_id, parent_id) REFERENCES parent(game_id, id)
```

For this to work, parents carry a `UNIQUE (game_id, id)` index in addition to the primary key on `id`. One extra index per parent table.

Rationale: game is a hard tenant boundary in drynn. Every handler, report, and turn-processing step runs in-context; cross-game queries do not exist. Denormalizing `Game ID` makes every query a straight `WHERE game_id = $1` lookup, keeps tenant isolation explicit and enforceable at the DB level, and leaves the door open for per-game partitioning, archival, and row-level security without schema churn.

## Open Questions

- **Game settings / cadence.** Turn cadence (real-time interval between turns, or manual advance), starting-empire parameters, number of systems, and home-system placement rules are not defined here. Decide whether they belong on `Game` directly, on a separate `Game Settings` entity, or in external configuration.
- **Archival.** Whether `completed` games remain in the live database indefinitely or move to an archive is undecided.
