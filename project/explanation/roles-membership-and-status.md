# Roles, Membership, and Status

**STATUS: DRAFT — do not implement.** This document was copied from an earlier version of the game engine and has not yet been reconciled with the current drynn architecture. Known schema and cross-document issues are tracked in `project/reconciliation-notes.md`. Coding agents must not build schema, store, or engine code against this spec until the DRAFT marker is removed.

This document explains the distinction between application roles and game membership in `drynn`.

It is aimed at human developers. The goal is not to prescribe implementation steps, but to explain the language we want to use and the design reasons behind it.

## Why This Distinction Matters

`drynn` has at least two different scopes of authorization and identity:

- access to the application as a whole
- participation inside a specific game

If we use the same word for both, the code and documentation become ambiguous very quickly. A person can be an authenticated user of the product without being a participant in any particular game. Likewise, a participant in a game can change state inside that game without changing their application-wide permissions.

That is why we want to reserve `role` for the application scope, and use `membership_type` and `membership_status` for the game scope.

## Application Roles

Application roles answer the question: what can this account do in the product overall?

The stored roles are:

- `admin`
- `user`

`admin` is intentionally god-like. It is allowed to do everything.

`user` is the default role assigned to accounts. A user can enter the lobby, view available games, join games when allowed, and manage most of their own profile data.

There is also a synthetic `guest` role. It is not stored, is not assigned to any account, and cannot be granted. It is a sentinel value used for unauthenticated sessions so role checks can be written without nil guards against a missing viewer. A `guest` viewer has no account behind it and cannot enter the lobby, join games, or interact with other accounts.

These roles belong to the account or session, not to a game.

## Game Membership Types

Game membership answers a different question: what is this account's relationship to this particular game?

The current membership types are:

- `gm`
- `player`

`gm` is the game-level authority. It can perform nearly all game management actions, such as updating game settings, adding or removing players, running turns, generating reports, and viewing player data.

`player` is a participant seat in the game. When we say "player" in ordinary conversation, we often mean an account with the application role `user` that has been added to a game as a player.

That shorthand is fine in casual discussion, but in the codebase it is better to stay precise: an account has an application role, and a game membership has a membership type.

## Membership Status

Membership status answers another question: what is the current operating state of this membership?

The current statuses are:

- `human`
- `agent`
- `resigned`
- `eliminated`

This is separate from membership type on purpose.

For example, a `player` membership can move from `human` to `agent` without ceasing to be a player membership. That is useful because we do not want to delete a player's historical participation when they leave or when the game engine has to take over control of their seat.

`agent` means the engine is currently controlling the seat.

`resigned` means the participant has left the game, but the membership record remains as part of the game's history and structure.

`eliminated` means the participant is still part of the game's history, but is no longer active in play.

In other words, membership type tells us what the seat is, while membership status tells us what condition that seat is in.

## Why `agent` Is A Status Instead of a Type

The most important design decision here is treating `agent` as status rather than type.

If `agent` were a peer to `gm` and `player`, we would be mixing two different ideas:

- authority or function within the game
- current control mode or lifecycle state

That would make it harder to reason about game history, permissions, reporting, and ownership. A seat that becomes engine-controlled is still fundamentally the same player seat. What changed is not what kind of membership it is, but how it is currently being operated.

Modeling `agent` as status preserves that distinction.

## Why This Helps Future Maintenance

This vocabulary should make the application easier to extend.

It keeps the account model clear:

- application concerns stay at the application level
- game concerns stay at the game level

It also keeps authorization logic easier to read:

- app checks can ask about `role`
- game checks can ask about `membership_type`
- lifecycle or control checks can ask about `membership_status`

That separation should reduce both schema churn and policy confusion as the server, CLI services, and eventually the game engine grow more capable.

## The Practical Language We Want

Going forward, the preferred terms are:

- `role` for application-wide authorization
- `membership_type` for game participation kind
- `membership_status` for current game-participation state

When we say "player" in code or documentation, we should be careful about which meaning we intend:

- a product user with role `user`
- a game membership with type `player`

Both usages are understandable in context, but the second is the one that matters for schema and service design.
