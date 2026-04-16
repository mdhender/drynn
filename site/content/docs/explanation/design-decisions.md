---
title: Design decisions
weight: 30
---

This document explains the reasoning behind Drynn's major design
choices. Each section covers what was chosen, what alternatives
existed, and why this path was taken.

## Invite-only registration

Users cannot register without a valid invitation code from an admin.
There is no open signup page.

Competitions need to control who participates. Open signup would
require CAPTCHAs, email verification flows, and abuse mitigation —
complexity that serves no purpose when an admin already knows who
should have access. Invitations are simpler: the admin decides, the
user registers, and the app trusts the admin's judgment.

## Database-backed JWT signing keys

Signing keys live in the `jwt_signing_keys` table, not in config
files or environment variables. The server loads them from the
database on each request.

Static secrets are hard to rotate. Changing an environment variable
requires restarting the server, and every restart invalidates all
existing tokens at once. Database-backed keys allow rotation without
downtime: create a new key, retire the old one with a grace period,
and existing tokens remain valid until the grace period expires.
Each key has a state (`active`, `retired`, `revoked`) and
timestamps, providing an audit trail.

## Roles loaded per-request, not embedded in JWT

JWT tokens contain only the user ID, token type, and expiry. Roles
are loaded from the database on every authenticated request by the
`loadCurrentViewer` middleware.

Embedding roles in the token would create a consistency gap: an
admin removes a user's `admin` role, but the old token still grants
admin access until it expires. Loading per-request means role
changes take effect immediately. The cost is one database query per
authenticated request, which is negligible for the small user base
a competition typically has.

## sqlc and Atlas instead of an ORM

Database access uses sqlc (code generation from SQL) and Atlas
(schema-based migrations) rather than an ORM like GORM or Ent.

ORMs hide what the database is doing. They generate SQL you cannot
easily inspect, introduce N+1 query patterns, and push you toward
framework conventions. sqlc takes the opposite approach: you write
the SQL, sqlc generates type-safe Go functions. The queries in
`db/queries/` are readable, copy-pasteable into `psql`, and
generate zero runtime overhead.

Atlas works similarly: you define the desired schema in
`db/schema.sql`, and Atlas diffs it against the current migration
state to generate safe migration SQL. The schema file is the source
of truth, not a chain of migration files.

## Echo v5

Echo v5 uses a pointer receiver on `Context` (`*echo.Context`),
which means middleware can set values on the context and have them
visible downstream. Earlier versions used a value receiver, which
caused subtle bugs where middleware wrote to a copy that handlers
never saw.

Echo was chosen over Gin, Chi, or the standard library because it
provides just enough structure — routing, middleware, rendering —
without forcing opinions about ORMs, validation, or caching.

## Server-side rendering, not a JavaScript framework

Pages are rendered by Go templates on the server. There is no
React, Vue, or Svelte build step.

A full-stack JavaScript framework would introduce a separate build
toolchain (Node, npm, bundlers), duplicate business logic between
client and server, and add deployment complexity. Server-side
rendering avoids all of this: HTML is generated fresh per request,
forms work without JavaScript, and template changes deploy
instantly. The architecture is prepared for progressive enhancement
with HTMX or Alpine.js if interactive features are needed later,
but the baseline is functional without any client-side JavaScript.

## Config file with environment overrides

Configuration is loaded from a JSON file first, then environment
variables are applied on top. Environment variables always win.

Environment-only configuration requires every deployment to set
every variable explicitly. A config file provides shipped defaults
(15-minute access tokens, 7-day refresh tokens, insecure cookies
for development) so that most deployments only need to override
`DATABASE_URL`. The two-tier approach also keeps config auditable:
the file can be committed alongside the deployment, and environment
variables handle per-environment secrets.

## Guest sentinel viewer

When no user is signed in, `auth.CurrentViewer()` returns a
sentinel `Viewer` with `Roles: []string{"guest"}` instead of `nil`.

This eliminates nil checks throughout handlers and templates.
Code like `viewer.HasRole("admin")` safely returns `false` for
guests. Templates can reference `.CurrentUser.Handle` without
conditional guards. The sentinel is never stored in the database —
it exists purely to simplify control flow. Each request gets a
fresh copy, so there is no shared mutable state.

## Database constraints as defense in depth

Every validation rule enforced in the service layer is also
enforced by a database constraint: handle format, email uniqueness,
role foreign keys, one-active-key-per-type.

The service layer catches most invalid input before it reaches the
database. But if a bug slips through, or if someone modifies data
directly via SQL, the database rejects it. The constraints are
redundant by design — they exist to catch what the application
layer misses.
