# Roles and Game Participation

**STATUS: DRAFT.** The `Player` model referenced here is defined in `project/reference/empire-model.md`. The application-role content reflects the current implementation; the game-participation content is forward-looking until the schema lands. See `project/reconciliation-notes.md` for status. The filename (`roles-membership-and-status.md`) is legacy from the prior engine vocabulary and is retained to avoid breaking cross-references.

This document explains the distinction between application roles and game participation in `drynn`.

It is aimed at human developers. The goal is not to prescribe implementation steps, but to explain the language we use and the design reasons behind it.

## Why This Distinction Matters

`drynn` has at least two different scopes of authorization and identity:

- access to the application as a whole
- participation inside a specific game

If we use the same word for both, the code and documentation become ambiguous quickly. A person can be an authenticated user of the product without being a participant in any particular game. Likewise, a participant in a game can change state inside that game without changing their application-wide permissions.

That is why we reserve `role` for the application scope, and use the `Player` entity for the game scope.

## Application Roles

Application roles answer the question: what can this account do in the product overall?

The stored roles are:

- `admin`
- `user`

`admin` is intentionally god-like. It is allowed to do everything.

`user` is the default role assigned to accounts. A user can enter the lobby, view available games, join games when allowed, and manage most of their own profile data.

There is also a synthetic `guest` role. It is not stored, is not assigned to any account, and cannot be granted. It is a sentinel value used for unauthenticated sessions so role checks can be written without nil guards against a missing viewer. A `guest` viewer has no account behind it and cannot enter the lobby, join games, or interact with other accounts.

These roles belong to the account or session, not to a game.

## Game Participation: Player

Game participation is modeled as a `Player` entity — an account's seat in a specific game. See `project/reference/empire-model.md` for the full spec.

Key points:

- Every `Player` is backed by an `Account` and carries a `Status` (`active`, `resigned`, `eliminated`).
- `Is GM` distinguishes a game-master seat (no empire) from a regular player seat (controls one empire).
- A `UNIQUE (Game ID, Account ID)` constraint across all `Player` rows — active *and* terminal — means an account holds at most one seat per game across the entire lifetime of that game. This blocks rejoining after resignation and prevents an account from being both GM and regular player in the same game.

Agent control — when the engine is operating an empire instead of a human — is **not** a status on `Player`. It lives on the `empire_control` bridge row for the empire: `Player ID`, `Agent ID`, and a `GM Set` flag together record who is currently operating the seat and how. A player's seat can be temporarily operated by an agent (vacation) without ever leaving `active` status, and a resigned player's empire can be taken over by an agent without touching the resigned row.

## Why `Is GM` Is a Flag (Not a Separate Entity Type)

Collapsing GM and regular player into a single `Player` entity with an `Is GM` flag keeps per-game participation uniform — same rejoin-block rule, same lifecycle transitions, same account linkage. The flag distinguishes authority without splitting the schema.

Similarly, agent control is not a status value on `Player`. If it were, transitioning a seat to and from agent control would ping-pong the player's status and mix "who holds the seat" with "who is currently operating it." Keeping control on a separate bridge row preserves that distinction cleanly: player identity and seat history stay on `Player`; current operator state lives on `empire_control`.

## Why This Helps Future Maintenance

This vocabulary keeps the account model clear:

- application concerns stay at the application level (`role`, stored in `user_roles`).
- game-participation concerns stay at the game level (`Player`, `empire_control`).

It also keeps authorization logic easier to read:

- app checks ask about `role`.
- game-level permission checks ask about `Is GM` and `Status` on the relevant `Player`.
- control checks (who submits orders this turn, who can clear an agent) ask about `empire_control`.

That separation should reduce both schema churn and policy confusion as the server, CLI services, and eventually the game engine grow more capable.

## The Practical Language We Want

Going forward, the preferred terms are:

- `role` for application-wide authorization (`admin`, `user`; plus the synthetic `guest` sentinel).
- `Player` for an account's per-game seat.
- `empire_control` for who currently operates a given empire.

When we say "player" in casual conversation, we usually mean "an account with role `user` that has a `Player` row in some game." That shorthand is fine in discussion; in code and documentation, prefer the precise form.
