# Name Normalization Reference

**STATUS: DRAFT.** Reconciled with the current model. The alpha character whitelist is frozen for the alpha release; a Unicode-aware whitelist is a possible post-alpha change. See `project/reconciliation-notes.md` for status.

This document specifies how player-settable display names are normalized in `drynn` before they are stored, and how the schema enforces uniqueness on those names. It is the shared reference cited by each entity that carries a `Name` column.

## Scope

The rules in this document apply to every player-settable `Name` field in the model:

| Entity                | Field  | Uniqueness scope                         | Documented in               |
|-----------------------|--------|------------------------------------------|-----------------------------|
| `Empire`              | `Name` | per `(Game ID)`                          | `empire-model.md` §Empire   |
| `Agent`               | `Name` | global                                   | `empire-model.md` §Agent    |
| `Empire System Name`  | `Name` | per `(Game ID, Empire ID)`               | `empire-model.md`           |
| `Empire Planet Name`  | `Name` | per `(Game ID, Empire ID)`               | `empire-model.md`           |
| `Vessel`              | `Name` | per `(Game ID, Empire ID)`               | `empire-model.md` §Vessel   |

`Player.Account` is out of scope. It is an FK to the account record, whose handle uniqueness is enforced by the account layer (see CLAUDE.md for handle rules).

## Normalization Pipeline

The order processor runs this pipeline against every player-supplied `Name` value before writing it to the datastore. The pipeline runs even when the input arrives in quoted form (quotes are not a bypass).

1. **Character whitelist.** Only ASCII letters, ASCII digits, and a small punctuation set are allowed (full set defined in *Character Whitelist* below). Characters outside the set cause the order to be rejected with a diagnostic; the input is not silently stripped.
2. **Whitespace normalization.** Any whitespace character (tab, newline, vertical tab, non-breaking space, etc.) is converted to a single ordinary space (U+0020).
3. **Trim.** Leading and trailing spaces are removed.
4. **Collapse.** Internal runs of two or more spaces are collapsed to a single space.
5. **Empty check.** If the result is empty, the order is rejected.

The surviving string is what gets stored. Case is preserved exactly as submitted.

## Character Whitelist

The alpha whitelist is intentionally narrow — ASCII only, the character set a 1978-era system would accept:

| Category      | Characters   | Code points                  |
|---------------|--------------|------------------------------|
| ASCII letters | `A–Z`, `a–z` | U+0041–U+005A, U+0061–U+007A |
| ASCII digits  | `0–9`        | U+0030–U+0039                |
| Space         | ` `          | U+0020                       |
| Period        | `.`          | U+002E                       |
| Hyphen-minus  | `-`          | U+002D                       |
| Number sign   | `#`          | U+0023                       |
| Apostrophe    | `'`          | U+0027                       |

Regex form: `[A-Za-z0-9 .\-#']`.

Deliberately excluded for alpha: Unicode letters outside ASCII (including accented Latin such as `é`, `ñ`, `ü`), typographic ("smart") quotes (U+2018, U+2019), em-dash and en-dash (U+2014, U+2013), underscore, and all other ASCII punctuation. A name submitted with any of these is rejected at the order processor, not silently normalized or substituted.

Non-ASCII input may be reconsidered after alpha. The cost of widening the set is a Unicode-category test and a decision about canonical normalization form (NFC is the usual choice); neither is infrastructure the alpha needs.

## Case Handling

Names are stored with the submitted case intact. Comparison — both in the order processor and in the schema — is **case-insensitive**.

The schema enforces case-insensitive uniqueness by placing the UNIQUE constraint over `lower(Name)` rather than over `Name` directly. The idiomatic shape:

```sql
UNIQUE INDEX ... ON table (Game ID, Empire ID, lower(Name))
```

A `citext` column would also work but is not required; functional indexes on `lower(Name)` keep the schema portable across Postgres installations without the `citext` extension.

The order processor lowercases both the submitted name and any names it is comparing against during order resolution. It never mutates the stored value to lowercase.

## Uniqueness Constraints

Each entity carries the uniqueness constraint appropriate to its scope. The constraint sits on `lower(Name)` plus the enclosing scope columns:

- `Empire`: `UNIQUE (Game ID, lower(Name))`.
- `Agent`: `UNIQUE (lower(Name))`.
- `Empire System Name`: `UNIQUE (Empire ID, lower(Name))`. `Empire ID` is game-scoped, so this is implicitly per-game.
- `Empire Planet Name`: `UNIQUE (Empire ID, lower(Name))`. Same game-scope reasoning.
- `Vessel`: `UNIQUE (Game ID, Empire ID, lower(Name))`.

Each entity's own Constraints section in `empire-model.md` states the rule; this document is the single source of truth for the normalization pipeline those constraints assume.

## Cross-Table Ambiguity In Orders

A single empire may have a Vessel, an Empire System Name, and an Empire Planet Name that share the same normalized name — the schema only enforces uniqueness within each table. Cross-table collisions are resolved at order-parse time:

- If an order's name argument resolves to exactly one target across Vessel / Empire System Name / Empire Planet Name, the order proceeds.
- If it resolves to more than one, the order is rejected as ambiguous. The player must disambiguate by order form, by renaming one of the targets, or by providing a more specific reference.

No schema constraint covers cross-table uniqueness. It is an order-parser concern and lives outside this document.

## Why Case-Insensitive

Two empires named `Terran Federation` and `terran federation` are confusing to humans even if distinct to the database. Allowing players to submit near-duplicate names would let one account shadow another by mimicry. The GM is ultimately responsible for policing bad-faith naming; the schema constraint provides a first line of defense so the GM does not have to catch everything manually.

## Related Documents

- `project/reference/empire-model.md` — entity specs for Empire, Agent, Empire System Name, Empire Planet Name, and Vessel. Each entity's Constraints section cites this document.
- `project/reconciliation-notes.md` — resolved F2 entry, and the open character-whitelist question that gates this document's DRAFT marker.
