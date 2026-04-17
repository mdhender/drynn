# Code Smells Review

Reviewed: 2026-04-16

---

## 1. Duplicate `MailgunConfig` structs [DONE]

**Files:**
- `internal/config/config.go` — `config.MailgunConfig`
- `internal/email/email.go` — `email.MailgunConfig`

Two identical structs with the same four fields (`APIKey`, `SendingDomain`,
`FromAddress`, `FromName`). Every call site that bridges the two packages
must copy field-by-field (see `cmd/email/main.go:106-111` and
`internal/server/server.go:40-45`).

**Suggestion:** Delete `config.MailgunConfig` and use `email.MailgunConfig`
everywhere, or define one canonical type and alias the other.

---

## 2. Duplicate "is Mailgun configured?" logic [DONE]

**Files:**
- `internal/config/config.go:40` — `config.MailgunConfig.Configured()` method
- `internal/service/mailgun.go:5` — `mailgunConfigured()` free function

Both check the same three fields. The service package's free function exists
because services accept `email.MailgunConfig`, not `config.MailgunConfig`.
This is a downstream symptom of smell #1.

**Suggestion:** Consolidate onto a single `Configured()` method on the
canonical `MailgunConfig` type.

---

## 3. Stale "Hobo" branding scattered across the codebase [DONE]

The project was renamed from **Hobo** to **Drynn**, but old references remain
in production-visible strings and internal constants:

| Location                                           | Example                                                            |
|----------------------------------------------------|--------------------------------------------------------------------|
| `internal/auth/jwt.go:16-17`                       | Cookie names `hobo_access`, `hobo_refresh`                         |
| `internal/config/config.go:17`                     | Env var `HOBO_CONFIG_PATH`                                         |
| `internal/handler/public.go:17`                    | Page title `"Hobo"`                                                |
| `internal/service/invitation_service.go:120-127`   | Email copy "join Hobo", subject "You're invited to Hobo"           |
| `internal/service/password_reset_service.go:90-97` | Email copy "your Hobo account", subject "Reset your Hobo password" |
| `internal/service/access_request_service.go:59`    | "access to Hobo"                                                   |
| `internal/server/server.go:15`                     | Import alias `hobomiddleware`                                      |
| `web/templates/layouts/public.gohtml:17`           | `<strong>Hobo</strong>`                                            |
| `web/components/sidebar.gohtml:7`                  | `<strong>Hobo</strong>`                                            |

**Suggestion:** Sweep all occurrences and update to "Drynn". Cookie names
are a breaking change (existing sessions will be lost), so plan a migration
window.

---

## 4. Env var prefix inconsistency [DONE]

`internal/config/config.go` uses **two different prefixes**:

- `HOBO_CONFIG_PATH` (line 17) — old name
- `DRYNN_APP_ADDR`, `DRYNN_DATABASE_URL`, etc. (lines 255-266) — new name

This forces operators to set env vars from both naming schemes.

**Suggestion:** Migrate `HOBO_CONFIG_PATH` → `DRYNN_CONFIG_PATH` (with
optional fallback during a deprecation period).

---

## 5. `ListInvitations` hydration duplicates `mapInvitationRow` [DONE]

**File:** `internal/service/invitation_service.go`

`ListInvitations` (lines 146-167) manually maps each
`sqlc.ListInvitationsRow` field-by-field instead of reusing
`mapInvitationRow`. The mapping logic for `UsedBy` / `UsedAt` nullable
fields is duplicated verbatim.

`mapInvitationRow` accepts `sqlc.Invitation` while `ListInvitations` works
with `sqlc.ListInvitationsRow` (which includes `InvitedByHandle`), so the
types don't unify directly — but the nullable-field mapping is identical
boilerplate.

**Suggestion:** Extract a shared helper or make `mapInvitationRow` generic
enough to cover both row types.

---

## 6. `signToken` computes `issuedAt` independently from `IssueTokens` [DONE]

**File:** `internal/auth/jwt.go`

`IssueTokens` (line 81) calls `time.Now().UTC()` to compute `now`, derives
`accessExpiry` and `refreshExpiry` from it, then passes those expiries into
`signToken`. But `signToken` (line 158) calls `time.Now().UTC()` _again_ for
`IssuedAt`. Under load the two calls can diverge by milliseconds, and in
tests the inconsistency makes assertions fragile.

**Suggestion:** Pass `now` into `signToken` so all timestamps in a token
pair are derived from a single clock read.

---

## 7. `isfile` has a redundant nil check [DONE]

**File:** `internal/config/dotenv.go:47-56`

```go
sb, err := os.Stat(path)
if err != nil {
    return false
} else if sb == nil {    // ← can never happen when err == nil
    return false
} else if sb.IsDir() || sb.Mode().IsDir() {
    return false
}
return sb.Mode().IsRegular()
```

`os.Stat` never returns `(nil, nil)`. The `sb == nil` branch is dead code.
Also, `sb.IsDir()` and `sb.Mode().IsDir()` are equivalent — only one is
needed.

**Suggestion:** Simplify to:
```go
sb, err := os.Stat(path)
return err == nil && sb.Mode().IsRegular()
```

---

## 8. `SiteFS` silently swallows the `fs.Sub` error [DONE]

**File:** `sitefs.go:14`

```go
sub, _ := fs.Sub(sitePublic, "web/sitepublic")
```

If the embedded directory is renamed or the embed directive changes, this
returns `nil` and every downstream `StaticFS` call will panic or serve 404s
with no diagnostic message.

The same pattern appears in `server.go:77-78`:
```go
docsFS, _ := fs.Sub(siteFS, "docs")
blogFS, _ := fs.Sub(siteFS, "blog")
```

**Suggestion:** Either check and propagate the error, or use `must`-style
helpers that panic with a clear message during startup.

---

## 9. N+1 query in `ListUsers` [DONE]

**File:** `internal/service/user_service.go:176-191`

`ListUsers` fetches all user rows, then calls `hydrateUser` on each one.
`hydrateUser` issues a separate `ListRoleNamesByUser` query per user. For N
users this results in N+1 database round-trips.

**Suggestion:** Write a single sqlc query that joins users with their roles
(or batch-fetch roles for all user IDs), then hydrate in-memory.

---

## 10. `EnsureBootstrapAdmin` bypasses its own validation [DONE]

**File:** `internal/service/user_service.go:374-416`

The method trims inputs and checks for empty strings, but does not call
`normalizeHandle` or `validatePassword` directly — it delegates to
`CreateUser` / `UpdateUser` which do. That's fine, but the early
`TrimSpace` means leading/trailing whitespace is silently stripped from the
password before it reaches `validatePassword`, which _also_ trims. A
password of `"       x"` (7 spaces + 1 char) passes the 8-character check
after double-trimming.

This is inconsistent with the `Register` flow, which passes the raw
password to `validatePassword`.

**Suggestion:** Don't trim the password in `EnsureBootstrapAdmin`; let the
downstream validators handle it consistently.

---

## 11. `Refresh` handler re-parses the subject UUID redundantly [DONE]

**File:** `internal/handler/auth.go:262-291`

The `Refresh` handler parses the refresh token, extracts `claims.Subject`,
parses it into a UUID, issues new tokens for that UUID, and then _also_
calls `GetUser` to verify the user is active. The `loadCurrentViewer`
middleware (which runs on `/app` routes) does the same `GetUser` lookup.

When `Refresh` is called from a non-`/app` route this is correct, but the
handler could avoid the second `uuid.Parse` by getting it from the claims
object directly if `Claims` stored a `uuid.UUID` instead of using the
string `Subject` field.

This is a minor smell — the real cost is the repeated `uuid.Parse` pattern
throughout the codebase (middleware, handler, refresh), which would benefit
from a `Claims.UserID()` helper.

**Suggestion:** Add `func (c *Claims) UserID() (uuid.UUID, error)` to
reduce scattered `uuid.Parse(claims.Subject)` calls.

---

## 12. Flash messages via query parameter are not signed [DEFER]

**File:** `internal/handler/view.go:141-148, 192-198`

Flash messages are passed as `?flash=...` query parameters and rendered
directly into the page. While Go's `html/template` auto-escapes HTML, an
attacker can craft a URL with an arbitrary flash message (phishing vector):

```
https://example.com/app/admin/users?flash=Your+session+expired.+Click+here+to+re-enter+your+password.
```

**Suggestion:** Use a signed cookie or HMAC-protected parameter for flash
messages so only server-originated messages are displayed.

---

## 13. Hardcoded email HTML in service layer [DONE]

**Files:**
- `internal/service/invitation_service.go:119-125`
- `internal/service/password_reset_service.go:89-95`
- `internal/service/access_request_service.go:58-68`

Email body HTML is built with `fmt.Sprintf` and inline string literals
inside service methods. This mixes presentation with business logic, makes
the emails hard to preview or style, and risks forgetting to escape
user-supplied values (though `html.EscapeString` is used in the access
request case).

**Suggestion:** Move email bodies into Go templates (e.g. under
`web/templates/emails/`) and render them in the service or a dedicated
email-rendering layer.

---

## 14. `requestBaseURL` trusts `X-Forwarded-*` headers unconditionally [DONE]

**File:** `internal/handler/admin.go:269-286`

The helper reads `X-Forwarded-Proto` and `X-Forwarded-Host` from the
request and uses them to build absolute URLs embedded in invitation and
password-reset emails. If the app is exposed without a reverse proxy (or
behind a misconfigured one), an attacker can inject arbitrary headers to
redirect email links to a malicious domain.

**Suggestion:** Make the trusted proxy configuration explicit (allowlist of
IPs or CIDR ranges), or derive the base URL from config rather than request
headers.

---

## 15. Repeated `slog.Default()` calls instead of injected logger

**Files:**
- `internal/auth/middleware.go:18,54`
- `internal/handler/auth.go:112,174,202`

Every function that logs calls `slog.Default()` inline. This makes it
impossible to inject a test logger or per-request logger.

**Suggestion:** Accept a `*slog.Logger` in constructors (or derive one from
the request context) instead of reaching for the global default.

---

## 16. `loadCurrentViewer` middleware could use `Claims.UserID()` helper [DONE]

**File:** `internal/server/server.go:171-200`

The middleware manually calls `uuid.Parse(claims.Subject)` — same pattern as
the `RequireAuth` middleware and the `Refresh` handler. See smell #11.

---

## 17. `handler_test.go` missing `baseURL` arguments [DONE]

**File:** `internal/server/handler_test.go:93-95`

```go
authH := handler.NewAuthHandler(userSvc, invSvc, pwdSvc, accessSvc, jwtMgr, false)
adminH := handler.NewAdminHandler(userSvc, invSvc, pwdSvc)
```

`NewAuthHandler` and `NewAdminHandler` now require a `baseURL string`
parameter, but the test file was never updated. The test fails to compile:

```
not enough arguments in call to handler.NewAuthHandler
not enough arguments in call to handler.NewAdminHandler
```

**Suggestion:** Pass an empty string or `"http://localhost:8080"` for the
`baseURL` parameter in both calls.

---

## Summary

| #  | Severity | Category                       |
|----|----------|--------------------------------|
| 1  | Medium   | Duplication                    |
| 2  | Low      | Duplication (downstream of #1) |
| 3  | Medium   | Stale naming / branding        |
| 4  | Medium   | Configuration inconsistency    |
| 5  | Low      | Duplication                    |
| 6  | Low      | Clock skew                     |
| 7  | Low      | Dead code                      |
| 8  | Medium   | Silent error swallowing        |
| 9  | Medium   | Performance (N+1)              |
| 10 | Low      | Validation inconsistency       |
| 11 | Low      | Repeated pattern               |
| 12 | Medium   | Security (phishing vector)     |
| 13 | Low      | Separation of concerns         |
| 14 | Medium   | Security (header injection)    |
| 15 | Low      | Testability / global state     |
| 16 | Low      | Repeated pattern (see #11)     |
| 17 | Low      | Stale test code                |
