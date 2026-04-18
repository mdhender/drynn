---
title: Routes
weight: 30
---

All routes are registered in `internal/server/server.go`.

## Public routes

No authentication required.

| Method | Path | Handler | Notes |
|--------|------|---------|-------|
| GET | `/` | `ShowHome` | Landing page |
| GET | `/register` | `ShowRegister` | Requires `?code=` from invitation |
| POST | `/register` | `Register` | |
| GET | `/signin` | `ShowSignIn` | |
| POST | `/signin` | `SignIn` | Rate limited |
| GET | `/forgot-password` | `ShowForgotPassword` | |
| POST | `/forgot-password` | `ForgotPassword` | Rate limited |
| GET | `/request-access` | `ShowRequestAccess` | Only if enabled in config |
| POST | `/request-access` | `RequestAccess` | Rate limited |
| GET | `/reset-password` | `ShowResetPassword` | Requires `?code=` |
| POST | `/reset-password` | `ResetPassword` | Rate limited |
| POST | `/logout` | `SignOut` | Clears auth cookies |
| POST | `/refresh` | `Refresh` | Refreshes access token |

### Health checks

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/healthz` | `Healthz` | Always returns 200 |
| GET | `/readyz` | `Readyz` | 200 if database reachable, 503 otherwise |

### Rate limiting

POST routes for `/signin`, `/forgot-password`, `/request-access`,
and `/reset-password` are rate limited per IP address.

## Authenticated routes

Middleware: `RequireAuth` (validates JWT) +
`loadCurrentViewer` (loads user from database).

Unauthenticated requests to these routes redirect to `/signin`.

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/app` | *(redirect)* | Redirects to `/app/profile` |
| GET | `/app/profile` | `ShowProfile` | User profile form |
| POST | `/app/profile` | `UpdateProfile` | Update handle and email |

## Admin routes

Middleware: all authenticated middleware + `requireRole("admin")`.

Returns 403 if the user lacks the `admin` role.

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/app/admin/users` | `ListUsers` | User list |
| GET | `/app/admin/users/new` | `ShowCreateUser` | Create user form |
| POST | `/app/admin/users` | `CreateUser` | Submit new user |
| GET | `/app/admin/users/:id/edit` | `ShowEditUser` | Edit user form |
| POST | `/app/admin/users/:id` | `UpdateUser` | Submit user update |
| POST | `/app/admin/users/:id/reset-password` | `SendPasswordReset` | Email password reset |
| POST | `/app/admin/users/:id/delete` | `DeleteUser` | Delete user |
| GET | `/app/admin/invitations` | `ListInvitations` | Invitation list |
| GET | `/app/admin/invitations/new` | `ShowInviteForm` | Invite form |
| POST | `/app/admin/invitations` | `SendInvitation` | Send invitation |
| POST | `/app/admin/invitations/:id/resend` | `ResendInvitation` | Resend invitation |
| POST | `/app/admin/invitations/:id/archive` | `ArchiveInvitation` | Archive invitation |

## API routes

Routes mounted under `/api/v1`. Consumed by the `drynn` CLI and
future external clients. All responses are JSON.

### Public API

| Method | Path | Handler | Notes |
|--------|------|---------|-------|
| GET | `/api/v1/health` | `Health` | Returns `{"status":"ok","version":"..."}` |
| POST | `/api/v1/login` | `Login` | Rate limited; returns access and refresh tokens |

### Admin API — games

Middleware: `RequireAuth` (validates JWT Bearer header) +
`loadCurrentViewer` + `requireRole("admin")`.

Unlike the browser-facing routes, these return JSON error responses
rather than HTML redirects:

- `401 {"error":"authentication required"}` when the Bearer token is missing or invalid
- `403 {"error":"forbidden"}` when the user lacks the `admin` role

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/api/v1/games` | `CreateGame` | Create a game (`{"name":"..."}`) → `201 {"id":N}` |
| GET | `/api/v1/games` | `ListGames` | List all games → `200 [...]` |
| GET | `/api/v1/games/:id` | `GetGame` | Fetch one game → `200 {...}` |
| PUT | `/api/v1/games/:id` | `UpdateGame` | Reserved; currently `501 {"error":"not yet implemented"}` |
| DELETE | `/api/v1/games/:id` | `DeleteGame` | Delete a game → `204 No Content` |

### Standardized API error responses

| Message                   | HTTP status |
|---------------------------|-------------|
| `invalid request body`    | 400         |
| `name is required`        | 400         |
| `invalid game id`         | 400         |
| `authentication required` | 401         |
| `forbidden`               | 403         |
| `game not found`          | 404         |
| `not yet implemented`     | 501         |
| `internal error`          | 500         |

## Static assets

| Method | Path | Source |
|--------|------|--------|
| GET | `/static/*` | `web/static/` |

## Authentication flow

- Sign-in and registration issue JWT access + refresh tokens as
  HTTP-only cookies (`hobo_access`, `hobo_refresh`).
- Access token expires after `jwt_access_ttl` (default 15m).
- Refresh token expires after `jwt_refresh_ttl` (default 7 days).
- When the access token expires, the `RequireAuth` middleware
  attempts a transparent refresh using the refresh cookie.
- If refresh fails, the user is redirected to `/signin`.
