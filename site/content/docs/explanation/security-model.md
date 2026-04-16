---
title: Security model
weight: 40
---

Drynn's security is built on multiple reinforcing layers:
short-lived tokens, per-request authorization, input validation at
two levels, and browser-native protections against common web
attacks.

## Token architecture

Authentication uses a two-token approach:

- **Access token** (default 15-minute TTL) — carried as the
  `hobo_access` HTTP-only cookie. Short-lived to minimize exposure
  if compromised.
- **Refresh token** (default 7-day TTL) — carried as the
  `hobo_refresh` HTTP-only cookie. Used to silently issue new
  access tokens without requiring the user to sign in again.

Both tokens are HMAC-SHA256 JWTs signed with database-backed
keys. The JWT header includes a `kid` (key ID) that identifies
which signing key was used, allowing the server to look up the
correct key for verification even during key rotation.

Tokens contain only the user ID (`sub`), token type, and expiry.
No roles, no email, no mutable state. This keeps the token small
and avoids stale claims.

## Cookie security

Auth cookies are set with three protective flags:

- **HttpOnly** — JavaScript cannot read the cookies, preventing
  XSS attacks from exfiltrating tokens.
- **SameSite=Lax** — The browser sends cookies only on same-site
  requests and top-level navigations, blocking most cross-site
  request forgery.
- **Secure** — When `cookie_secure` is `true` (production),
  cookies are only sent over HTTPS, preventing interception.

## CSRF protection

State-changing requests (POST, PUT, DELETE, PATCH) are protected
by the FetchMetadata middleware, which wraps Go 1.25's
`http.CrossOriginProtection`. This checks the `Sec-Fetch-Site`
header and, for browsers that omit it, falls back to comparing the
`Origin` and `Host` headers. Cross-origin state-changing requests
are rejected.

Traditional CSRF tokens are unnecessary here because HTTP-only
cookies cannot be read by cross-origin JavaScript and SameSite=Lax
prevents the browser from attaching credentials to cross-site form
submissions.

## Password hashing

Passwords are hashed with bcrypt via `auth.HashPassword()` and
verified with `auth.ComparePassword()`. Bcrypt is intentionally
slow (tunable via cost factor), making brute-force attacks
expensive. It includes a per-hash salt automatically, preventing
rainbow table attacks. Comparison is constant-time to resist timing
attacks.

The minimum password length is 8 characters.

## Signing key lifecycle

Each token type (`access` and `refresh`) has exactly one active
signing key at any time, enforced by a unique partial index in the
database. Keys go through a lifecycle:

1. **Active** — used for both signing new tokens and verifying
   existing ones.
2. **Retired** — no longer used for signing, but still accepted for
   verification until a `verify_until` deadline passes. This grace
   period (default: the token's TTL) allows existing tokens to
   expire naturally during rotation.
3. **Revoked** — neither signing nor verification. The key remains
   in the database for auditing.

Rotation is atomic: creating a new key and retiring the old one
happen in a single database transaction. Setting the grace period
to zero immediately invalidates all tokens signed by the old key,
useful after a suspected compromise.

## Rate limiting

Authentication endpoints are rate-limited per IP address:

- `POST /signin`
- `POST /forgot-password`
- `POST /request-access`
- `POST /reset-password`

The limiter allows approximately 10 requests per minute with a
burst of 5 using Go's `x/time/rate` token bucket. Exceeding the
limit returns HTTP 429. Rate limiter buckets are garbage-collected
on a 10-minute schedule to prevent memory growth from IP churn.

This slows credential-stuffing attacks while leaving legitimate
users (who type at human speed) unaffected.

## Role-based access control

Roles (`user`, `admin`, `tester`) are stored in the `user_roles`
junction table and loaded from the database on every authenticated
request. They are never cached in tokens.

Enforcement happens at two levels:

- **Middleware** — the `requireRole` middleware guards entire route
  groups. The `/app/admin/*` group requires the `admin` role;
  requests without it receive 403 Forbidden before the handler is
  ever invoked.
- **Service layer** — business rules like "the last admin cannot
  be deactivated" are enforced in service methods, independent of
  the HTTP layer.

Because roles are loaded per-request, changes take effect
immediately. Revoking a user's `admin` role blocks their next
request, not their next sign-in.

## Input validation: two layers

Validation runs in both the service layer and the database:

**Service layer** (`internal/service/`):
- Handles are lowercased, trimmed, and checked against
  `^[a-z0-9_]+$` (3–32 characters).
- Emails are lowercased, trimmed, and parsed with
  `mail.ParseAddress`.
- Passwords must be at least 8 characters.

**Database constraints** (`db/schema.sql`):
- `CHECK (handle = lower(handle))` and
  `CHECK (handle ~ '^[a-z0-9_]+$')` — reject malformed handles.
- `UNIQUE` on `handle` and `email` — prevent duplicates even under
  race conditions.
- Foreign keys on `user_roles` — prevent orphaned role assignments.
- Partial unique index on `jwt_signing_keys` — enforces one active
  key per token type.

Database constraints are deliberately redundant with service
validation. If a service bug allows invalid data through, or if
someone modifies data via raw SQL, the database rejects it.

## Guest viewer pattern

Unauthenticated requests receive a sentinel `Viewer` with
`Roles: []string{"guest"}` instead of `nil`. This ensures that
`viewer.HasRole("admin")` returns `false` (not a nil-pointer panic)
and that templates can reference `.CurrentUser` fields without
conditional guards. The guest viewer is never persisted — it exists
only as a control-flow convenience.

## Defense in depth

No single layer is trusted to catch everything:

| Threat | Layer 1 | Layer 2 |
|--------|---------|---------|
| Stolen token | Short TTL (15 min) | Key rotation invalidates all tokens |
| XSS | HttpOnly cookies | CSP headers (when configured) |
| CSRF | SameSite=Lax cookies | FetchMetadata middleware |
| Brute force | Rate limiting | Bcrypt cost factor |
| Invalid input | Service validation | Database constraints |
| Privilege escalation | Middleware role check | Service-layer invariants |

Each pair is independently sufficient against its threat. Both
failing simultaneously is the scenario that leads to a breach — and
that is significantly less likely than either failing alone.
