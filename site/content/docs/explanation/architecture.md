---
title: Architecture
weight: 10
---

Drynn follows a layered, dependency-injected architecture that
separates HTTP handling, business logic, and data access into
distinct tiers. Each layer has a clear responsibility and
communicates with its neighbors through explicit interfaces.

## The three layers

**Handlers** (`internal/handler/`) are thin HTTP controllers. They
accept an Echo `*echo.Context`, extract form data or URL parameters,
call a service method, and render a response. Handlers never touch
the database directly and never contain business rules.

**Services** (`internal/service/`) hold all business logic:
validation, password hashing, role normalization, invariant
enforcement. A service owns a `*sqlc.Queries` instance and works
with the database through type-safe generated code. Services return
domain-specific errors (`ErrInvalidHandle`, `ErrEmailTaken`) that
handlers translate into user-facing messages.

**Database** (`db/sqlc/`) is generated code — the output of running
`sqlc generate` against the SQL queries in `db/queries/`. The
schema source of truth lives in `db/schema.sql`, and migrations are
managed by Atlas. No one writes Go code in this layer; the tooling
does it.

This separation means a handler change (adding a flash message,
rearranging a form) cannot break business logic, and a service
change (tightening a validation rule) cannot break the HTTP
interface.

## Dependency wiring

Dependencies flow downward and are injected through constructors.
The `server.New()` function in `internal/server/server.go` builds
the entire graph at startup:

1. **Database pool** — `pgxpool.New()` connects to PostgreSQL.
2. **Key store** — `auth.NewKeyStore()` wraps the pool for JWT key
   access.
3. **JWT manager** — `auth.NewManager()` receives the key store and
   TTL config.
4. **Services** — `UserService`, `InvitationService`,
   `PasswordResetService`, `AccessRequestService`, each receiving
   the pool.
5. **Handlers** — Each handler struct receives only the services it
   needs.
6. **Template renderer** — Compiles all `.gohtml` templates once.
7. **Echo router** — Routes and middleware are registered last.

Every dependency is explicit. There are no globals, no service
locators, no init functions that quietly set up state. If a
dependency is missing, the program fails to compile.

## The four handler groups

Handlers are split by access level, each as a struct with a
constructor:

| Group | Prefix | Auth | Purpose |
|-------|--------|------|---------|
| `PublicHandler` | `/` | None | Landing page |
| `AuthHandler` | `/` | None | Sign-in, registration, password reset, access requests |
| `AppHandler` | `/app` | Required | User profile |
| `AdminHandler` | `/app/admin` | Required + admin | User CRUD, invitations |

A fifth group, `HealthHandler`, serves `/healthz` and `/readyz`
without middleware so that orchestration platforms can probe the
server without authentication.

## Middleware pipeline

Middleware runs in registration order. The sequence matters because
each layer depends on the one before it:

1. **Request logger** — Logs method, path, status, duration, and
   viewer ID. Skips health-check paths.
2. **Recover** — Catches panics and returns 500.
3. **FetchMetadata** — CSRF protection via Go 1.25's
   `http.CrossOriginProtection`. Rejects cross-origin
   state-changing requests.
4. **RequireAuth** *(app group only)* — Validates the JWT access
   token from cookies. If expired, attempts a silent refresh using
   the refresh token. Redirects to `/signin` on failure.
5. **loadCurrentViewer** *(app group only)* — Reads the user ID
   from JWT claims, loads the full user record and roles from the
   database, and sets a `Viewer` in the context.
6. **requireRole** *(admin group only)* — Checks that the viewer
   has the required role. Returns 403 if not.

By the time a handler runs, the context contains a fully loaded,
current-state viewer with roles. Roles are never cached in the
token — they are always fresh from the database.

## Template rendering

Templates use Go's `html/template` with explicit composition:

- **Layouts** (`web/templates/layouts/`) define the outer page
  structure. `public.gohtml` has the unauthenticated chrome;
  `app.gohtml` adds the sidebar.
- **Components** (`web/components/`) are small reusable blocks:
  flash messages, sidebar navigation.
- **Pages** (`web/templates/pages/`) define a `content` block that
  the layout injects.

The renderer in `internal/server/render.go` pre-compiles each page
as a single template combining its layout, all components, and the
page file. A map keyed by name (e.g. `"public/signin"`,
`"app/profile"`) allows handlers to render by name. Execution
always starts at the `base` block defined in the layout.

Each page has a corresponding view-data struct (e.g.
`ProfileViewData`) that embeds `BaseViewData` — title, current
path, flash message, and current user. This gives templates
type-safe access to the data they need.

## Static assets and embedded content

Static files (`web/static/`) are served at `/static/*`. Hugo-built
documentation and blog content are embedded at build time and
served at `/docs/*` and `/blog/*`. These routes bypass the handler
pipeline entirely — they are static file mounts, not routed through
middleware.

## Startup and shutdown

The server starts by building the full dependency graph, compiling
templates, and binding to the configured address. If any step
fails — database unreachable, signing keys missing, template syntax
error — the server exits immediately with a clear error. There is
no partial startup.

Shutdown handles `SIGINT` and `SIGTERM` with a 10-second graceful
timeout, allowing in-flight requests to complete before closing the
database pool.
