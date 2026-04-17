# Project Burndown

Open issues to address. Newer items appear at the top; mark `[DONE]` when
closed.

---

## Ultrareview — 2026-04-17

Findings from remote ultrareview of `main` (46 files, +3607/-298).

### BD-1. `EnsureBootstrapAdmin` accepts whitespace-only password [DONE]

**Severity:** normal (invalidated on review)

**File:** `internal/service/user_service.go:386-392`

The ultrareview flagged that the skip-check uses `TrimSpace(password) == ""`
while the must-all-be-set check uses un-trimmed `password == ""`, letting
`"   "` or `"  validpass  "` fall through to `CreateUser`/`UpdateUser` and
be bcrypt-hashed verbatim.

**Why this is not a bug:** `validatePassword`
(`user_service.go:584-594`) rejects both cases explicitly as a
user-safety feature against copy-paste whitespace:

```go
if len(strings.TrimSpace(password)) < 8 { return ErrInvalidPassword }
if normalized := strings.TrimSpace(password); normalized != password {
    return ErrInvalidPassword
}
```

`"   "` fails the length check; `"  validpass  "` fails the trim-equality
check. The bootstrap path still aborts loudly via
`CreateUser`/`UpdateUser` — no silent lockout. The ultrareview's claim
that `"  validpass  "` would pass `validatePassword` was wrong.

No action required.

---

### BD-2. `BaseURL` silently defaults to non-routable `http://drynn.test:8989` [DONE]

**Severity:** normal

**Files:**
- `internal/config/config.go:96-100` (LoadPath seed)
- `internal/config/config.go:133` (WritePath default)
- `cmd/db/main.go` — `runInitConfig` has no `-base-url` flag

The PR moved invitation/reset-link base URL from per-request derivation
(removed `requestBaseURL` helper) into static `Config.BaseURL`. Both load
and write paths fall back to `http://drynn.test:8989` (RFC 6761 reserved
`.test` TLD) when neither `base_url` in `server.json` nor `DRYNN_BASE_URL`
is set. `DatabaseURL` errors when empty; `BaseURL` does not. Invitation
emails (`invitation_service.go:119`), password-reset emails
(`password_reset_service.go:95`), and admin console invitation rows
(`admin.go:201`) all ship dead links in production.

**Suggestion:** Reject empty `BaseURL` at `Config` load (mirror the
`DatabaseURL` check at `config.go:108-110`). Remove the `"http://drynn.test:8989"`
seed and `defaultString` fallback. Add `-base-url` flag to `init-config` and
require it at write time.

---

### BD-3. `empire_control_player_fk` `ON DELETE SET NULL` violates NOT NULL [DONE]

**Severity:** normal

**Files:**
- `db/migrations/20260417040157_add_game_schema.sql:56-69`
- `db/schema.sql:290` (mirror)

The composite FK
`(game_id, player_id) REFERENCES players(game_id, id) ON DELETE SET NULL`
has no column list, so Postgres nulls *both* referencing columns. But
`empire_control.game_id` is `NOT NULL`, so any direct `DELETE FROM players`
aborts with a not-null constraint violation. The cascade-from-`games` path
happens to work because `empires → empire_control` cascades first, but GM
cleanup of resigned/eliminated players, test teardown, and future admin
utilities will fail.

**Suggestion:** Use PG15+ column-list syntax
`ON DELETE SET NULL (player_id)`, or reference just `players.id`
(bigserial is globally unique), or switch to `ON DELETE RESTRICT`. Mirror
the fix in both the migration and `db/schema.sql`.

---

### BD-4. Cookie rename missed four literals in `handler_test.go`

**Severity:** normal

**File:** `internal/server/handler_test.go` lines 126, 264, 290, 426

The rename `hobo_access → drynn_access` in `internal/auth/jwt.go:16-17`
missed hard-coded strings in the server tests. `authCookie()` fabricates a
cookie name the server no longer honors; three assertions look for a
Set-Cookie name the server no longer emits. `go test ./internal/server/...`
is red on this branch.

**Suggestion:** Replace the four literals with `"drynn_access"`, or expose
a test helper that references the unexported `accessCookieName` constant so
future renames cannot drift.

---

### BD-5. Email templates loaded from CWD break non-repo-root deployments

**Severity:** normal

**File:** `internal/email/templates.go:11-22`

`loadTemplates()` uses `template.ParseFS(os.DirFS("."), "web/templates/emails/*.gohtml")`,
which resolves relative to the process CWD at runtime — not the binary.
systemd units without `WorkingDirectory=`, docker with a different
`WORKDIR`, and `go install` all break with `template: pattern matches no
files` on first email send. Worse, the error is latched in
`sync.Once`/`emailTemplatesErr` for the process lifetime, so even fixing
the CWD requires a restart. Every invitation, password-reset, and
access-request email is affected.

**Suggestion:** Use `//go:embed web/templates/emails/*.gohtml` (as
`sitefs.go:11-12` already does for site assets) so the failure becomes
build-time rather than silent runtime. At minimum, call `loadTemplates()`
at server startup so the crash happens before traffic is accepted.

---

### BD-6. `ListUsersWithRoles` user ordering depends on role names

**Severity:** nit

**Files:**
- `db/queries/users.sql:80-86`
- `internal/service/user_service.go:182-201` (consumer)

`ORDER BY u.created_at DESC, r.name` combined with first-encounter dedup
via `seen[row.ID]` means that when two users share `created_at`, the
user-level order is determined by whichever user's alphabetically-first
role name sorts earliest. Renaming a role on user B can flip the relative
order of A and B in the admin list. Same-`created_at` is realistic for
fixtures, seed scripts, or any single-statement multi-row `INSERT` (PG
`NOW()` is `transaction_timestamp()`).

**Suggestion:** One-line SQL change:
`ORDER BY u.created_at DESC, u.id, r.name`. No Go-side change required;
regenerate sqlc output.

---
