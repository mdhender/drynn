# Burndown — Game CRUD

Nine tasks, ordered by dependency (bottom-up). Each task is sized for a single coding-agent context window.
**Do not start a task until its dependencies are completed.**

---

## Task BD-1 — Migrate `cmd/db` to `ff/v4` [DONE]

**Depends on:** none

### Goal

Replace the manual `switch` + `flag.FlagSet` dispatch in `cmd/db/main.go`
with an `ff.Command` tree using `github.com/peterbourgon/ff/v4`. Preserve
all existing behavior exactly.

### Files to create/modify

| File                  | Action                                         |
|-----------------------|------------------------------------------------|
| `go.mod`              | **edit** — add `github.com/peterbourgon/ff/v4` |
| `go.sum`              | **generated** — `go mod tidy`                  |
| `cmd/db/main.go`      | **edit** — rewrite to ff/v4 command tree       |
| `cmd/db/main_test.go` | **create**                                     |

### Command tree

```
db (root command, no Exec — dispatches to subcommands)
├── init-config
├── seed-admin
├── seed-testers
├── set-password
├── jwt-key           (parent command, fallback Exec for help/unknown)
│   ├── create
│   ├── expire
│   └── delete
└── version
```

### Flag sets

Use a shared non-command parent flag set for `--config` so leaf commands
inherit it via `SetParent`:

```go
dbCommonFlags := ff.NewFlagSet("db-common")
configPath := dbCommonFlags.StringLong("config", config.DefaultPath(), "path to the server config file")
```

Each leaf command creates its own flag set with `.SetParent(dbCommonFlags)`,
mapping all existing `flag.FlagSet` flags to `ff.NewFlagSet` equivalents:

- **`init-config`**: `--app-addr`, `--database-url`, `--data-dir`,
  `--jwt-access-ttl`, `--jwt-refresh-ttl`, `--cookie-secure`, `--base-url`,
  `--mailgun-api-key`, `--mailgun-sending-domain`, `--mailgun-from-address`,
  `--mailgun-from-name`, `--request-access-enabled`, `--admin-contact-email`,
  `--force`
- **`seed-admin`**: `--handle`, `--email`, `--password`
- **`seed-testers`**: `--count`
- **`set-password`**: `--email`, `--password`
- **`jwt-key create`**: `--type`, `--verify-old-for`
- **`jwt-key expire`**: `--id`, `--verify-for`
- **`jwt-key delete`**: `--id`
- **`version`**: no flags (standalone `ff.NewFlagSet("version")`, no parent)
- **`logout`** (if applicable): no flags

### Control flow: split Parse / Run

The key architectural change: use `root.Parse(args)` and `root.Run(ctx)`
separately so the database is opened only when the selected command needs it.

```go
func run(ctx context.Context, args []string) error {
    // 1. Build command tree (all flags defined, Exec closures capture pointers)
    // 2. root.Parse(args)
    // 3. selected := root.GetSelected()
    // 4. If selected is in needsDB set → openDatabase(ctx, *configPath)
    //    Store cfg and pool; defer pool.Close()
    // 5. root.Run(ctx)
}
```

Commands that **need DB**: `seed-admin`, `seed-testers`, `set-password`,
`jwt-key create`, `jwt-key expire`, `jwt-key delete`.

Commands that **do not need DB**: `init-config`, `version`, root fallback,
`jwt-key` fallback.

### Behavior preservation

1. **Keep custom usage printers.** Do not switch to `ffhelp.Command`. The
   existing `usage()` and `jwtKeyUsage()` functions stay as-is.
2. **Keep exact error messages** for unknown commands and missing flags.
3. **Keep dotenv loading and signal context** in `main()`, before calling `run`.
4. `main()` becomes a thin wrapper:
   ```go
   func main() {
       log.SetFlags(0)
       // dotenv loading...
       ctx, stop := signal.NotifyContext(...)
       defer stop()
       if err := run(ctx, os.Args[1:]); err != nil {
           if errors.Is(err, ff.ErrHelp) { return }
           log.Fatal(err)
       }
   }
   ```

### Acceptance tests (`cmd/db/main_test.go`)

Tests should verify CLI wiring without needing a real database. Use
package-level function vars or small wrappers as test seams for
`openDatabase` and service actions.

- **`TestDB_Version_NoDB`** — `db version` prints the version string and
  does not attempt to open a database.
- **`TestDB_InitConfig_NoDB`** — `db init-config` with valid flags does
  not attempt to open a database.
- **`TestDB_JWTKey_NoSubcommand`** — `db jwt-key` with no nested
  subcommand prints jwt-key usage and returns help-like behavior.
- **`TestDB_JWTKey_UnknownSubcommand`** — `db jwt-key wat` returns error
  containing `unknown jwt-key command "wat"`.
- **`TestDB_UnknownCommand`** — `db frobnicate` returns error containing
  `unknown command "frobnicate"`.
- **`TestDB_Help`** — `db help` and `db --help` print root usage.
- **`TestDB_LeafHelp`** — `db seed-admin --help` prints seed-admin usage.

Run: `go test ./cmd/db/...`

---

## Task BD-2 — Migrate `cmd/drynn` to `ff/v4` [DONE]

**Depends on:** BD-1 (so that `go.mod` already has the `ff/v4` dependency)

### Goal

Replace the manual `switch` + `flag.FlagSet` dispatch in
`cmd/drynn/main.go` with an `ff.Command` tree. Preserve all existing
behavior exactly. Shape the code so the `game` subcommand (BD-8) can be
appended without refactoring.

### Files to create/modify

| File                      | Action                                    |
|---------------------------|-------------------------------------------|
| `cmd/drynn/main.go`       | **edit** — rewrite to ff/v4 command tree  |
| `cmd/drynn/main_test.go`  | **create**                                |

### Files to leave unchanged

- `cmd/drynn/session.go` — keep `loadSession`, `saveSession`,
  `clearTokens`, `resolveServerURL` as-is.

### Command tree

```
drynn (root command, no Exec — dispatches to subcommands)
├── login
├── logout
├── health
└── version
```

Do **not** add `game` yet. That is BD-8.

### Flag sets

Use a shared non-command parent flag set for `--server` and `--config`:

```go
serverFlags := ff.NewFlagSet("server-common")
server := serverFlags.StringLong("server", "", "server URL (e.g. http://localhost:8080)")
configPath := serverFlags.StringLong("config", "", "path to the server config file")
```

Leaf flag sets:

- **`login`**: `SetParent(serverFlags)` + `--email`, `--password`
- **`health`**: `SetParent(serverFlags)` (no additional flags)
- **`logout`**: standalone `ff.NewFlagSet("logout")` (no parent — doesn't
  need server/config)
- **`version`**: standalone `ff.NewFlagSet("version")` (no parent)

### Control flow: split Parse / Run

```go
func run(args []string) error {
    // 1. Build command tree
    // 2. root.Parse(args)
    // 3. selected := root.GetSelected()
    // 4. Pre-run setup based on selected command:
    //    - login, health: loadSession(), resolveServerURL()
    //    - login: enforce --config + existing session rule
    //    - logout, version: no pre-run
    // 5. root.Run(context.Background())
}
```

Shared runtime state captured by closures:

```go
type drynnRuntime struct {
    session   sessionData
    serverURL string
}
```

### Pre-run behavior by command

- **`login`**: load session; if `--config` set and session has
  `AccessToken`, fail with `"existing session found; run 'drynn logout'
  before using --config"`; resolve server URL.
- **`health`**: load session; resolve server URL.
- **`logout`**: no pre-run. Exec calls `clearTokens()`.
- **`version`**: no pre-run. Exec prints version.

### Ready for `game` subcommand (BD-8)

The `game` subcommand will later:
1. Create its flag set with `SetParent(serverFlags)`
2. Append to `root.Subcommands`
3. Add a case in the pre-run switch for session + server resolution

No refactoring of existing code needed.

### Behavior preservation

1. Keep custom `usage()` printer. Do not use `ffhelp.Command`.
2. Keep existing server URL resolution precedence (flag > env > config >
   session) inside `resolveServerURL` — do not use ff env options.
3. Keep `httpClient` as a package-level var.

### Acceptance tests (`cmd/drynn/main_test.go`)

Use `httptest.NewServer` and a temp session directory to test without a
real server.

- **`TestDrynn_Login_SavesSession`** — run `login --email ... --password
  ... --server <testserver>`; assert request body matches current format;
  assert session file written with `server_url`, `access_token`,
  `refresh_token`.
- **`TestDrynn_Login_ConfigWithExistingSession`** — pre-create session
  with `access_token`; run `login --config some.json ...`; assert error
  `"existing session found; run 'drynn logout' before using --config"`;
  assert no HTTP request sent.
- **`TestDrynn_Logout_ClearsTokens`** — pre-create session; run `logout`;
  assert `access_token` and `refresh_token` empty; assert `server_url`
  preserved.
- **`TestDrynn_Health_ServerURL`** — run `health --server <testserver>`;
  assert request hits `/api/v1/health`.
- **`TestDrynn_Version_NoSideEffects`** — run `version`; assert version
  printed; assert no session or HTTP activity.
- **`TestDrynn_UnknownCommand`** — `drynn frobnicate` returns error
  containing `unknown command`.
- **`TestDrynn_Help`** — `drynn help` and `drynn --help` print root usage.

Run: `go test ./cmd/drynn/...`

---

## Task BD-3 — sqlc game queries + test fixture builder [DONE]

**Depends on:** none (independent of BD-1/BD-2)

### Goal

Add the four SQL queries that back game CRUD and a `GameBuilder` to the
test-fixtures package so that every subsequent task can seed games.

### Files to create/modify

| File                                         | Action                              |
|----------------------------------------------|-------------------------------------|
| `db/queries/games.sql`                       | **create**                          |
| `db/sqlc/games.sql.go`                       | **generated** — run `sqlc generate` |
| `internal/testfixtures/testfixtures.go`      | **edit** — add `GameBuilder`        |
| `internal/testfixtures/game_builder_test.go` | **create**                          |

### SQL queries (`db/queries/games.sql`)

```sql
-- name: CreateGame :one
INSERT INTO games (name)
VALUES ($1)
RETURNING *;

-- name: GetGameByID :one
SELECT * FROM games WHERE id = $1;

-- name: ListGames :many
SELECT * FROM games ORDER BY created_at DESC, id DESC;

-- name: DeleteGame :exec
DELETE FROM games WHERE id = $1;
```

After writing the file, run `sqlc generate`. Expected generated methods:

```go
func (q *Queries) CreateGame(ctx context.Context, name string) (Game, error)
func (q *Queries) GetGameByID(ctx context.Context, id int64) (Game, error)
func (q *Queries) ListGames(ctx context.Context) ([]Game, error)
func (q *Queries) DeleteGame(ctx context.Context, id int64) error
```

### Fixture builder (add to `internal/testfixtures/testfixtures.go`)

```go
type GameBuilder struct {
    f    *Fixtures
    name string
}

func (f *Fixtures) NewGame() *GameBuilder {
    n := f.next()
    return &GameBuilder{f: f, name: fmt.Sprintf("game_%d", n)}
}

func (b *GameBuilder) Name(name string) *GameBuilder { b.name = name; return b }

func (b *GameBuilder) Build(ctx context.Context) sqlc.Game {
    b.f.t.Helper()
    game, err := b.f.q.CreateGame(ctx, b.name)
    if err != nil {
        b.f.t.Fatalf("testfixtures: create game %q: %v", b.name, err)
    }
    return game
}
```

### Acceptance tests (`internal/testfixtures/game_builder_test.go`)

- **`TestFixtures_NewGame_Build`** — builds a game with defaults; inserted
  row exists; `status == "setup"`; `current_turn == 0`.
- **`TestFixtures_NewGame_NameOverride`** —
  `fix.NewGame().Name("Alpha").Build(ctx)` returns a row with
  `Name == "Alpha"`.

Run: `go test ./internal/testfixtures/...`

---

## Task BD-4 — GameService create / get / list [DONE]

**Depends on:** BD-3

### Goal

Add `GameService` with create, get, and list operations. All business
logic lives here; handlers will be thin wrappers.

### Files to create/modify

| File                                    | Action                     |
|-----------------------------------------|----------------------------|
| `internal/service/game_service.go`      | **create**                 |
| `internal/service/errors.go`            | **edit** — add game errors |
| `internal/service/game_service_test.go` | **create**                 |

### Error values (add to `internal/service/errors.go`)

```go
ErrGameNotFound             = errors.New("game not found")
ErrInvalidGameName          = errors.New("name is required")
ErrGameUpdateNotImplemented = errors.New("not yet implemented")
```

### Types and functions (`internal/service/game_service.go`)

```go
package service

const (
    GameStatusSetup     = "setup"
    GameStatusActive    = "active"
    GameStatusCompleted = "completed"
)

type Game struct {
    ID          int64
    Name        string
    Status      string
    CurrentTurn int32
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type CreateGameInput struct {
    Name string
}

type GameService struct {
    pool    *pgxpool.Pool
    queries *sqlc.Queries
}

func NewGameService(pool *pgxpool.Pool) *GameService

func (s *GameService) CreateGame(ctx context.Context, input CreateGameInput) (*Game, error)
func (s *GameService) GetGame(ctx context.Context, gameID int64) (*Game, error)
func (s *GameService) ListGames(ctx context.Context) ([]Game, error)
```

Helpers:

```go
func normalizeGameName(name string) (string, error) // TrimSpace; reject empty → ErrInvalidGameName
func mapGame(row sqlc.Game) *Game                    // convert pgtype timestamps → time.Time
```

### Implementation notes

- `CreateGame` trims and validates name, then calls `queries.CreateGame`.
  Status and current_turn come from DB defaults.
- `GetGame` calls `queries.GetGameByID`; maps `pgx.ErrNoRows` →
  `ErrGameNotFound`.
- `ListGames` calls `queries.ListGames`, maps each row.

### Acceptance tests (`internal/service/game_service_test.go`)

- **`TestGameService_CreateGame`** — creates `"Alpha"`; returned
  `Name == "Alpha"`, `Status == GameStatusSetup`, `CurrentTurn == 0`.
- **`TestGameService_CreateGame_TrimsName`** — input `"  Alpha  "` →
  stored name `"Alpha"`.
- **`TestGameService_CreateGame_BlankName`** — input `"   "` → returns
  `ErrInvalidGameName`.
- **`TestGameService_GetGame`** — fixture-seeded game can be loaded by ID.
- **`TestGameService_GetGame_NotFound`** — unknown ID → `ErrGameNotFound`.
- **`TestGameService_ListGames`** — seed two games; returns both in
  `created_at DESC, id DESC` order.

Run: `go test ./internal/service/...`

---

## Task BD-5 — GameService delete + update stub [DONE]

**Depends on:** BD-4

### Goal

Complete the service layer. Delete actually removes the row; update
immediately returns `ErrGameUpdateNotImplemented`.

### Files to modify

| File                                    | Action   |
|-----------------------------------------|----------|
| `internal/service/game_service.go`      | **edit** |
| `internal/service/game_service_test.go` | **edit** |

### Functions to add

```go
func (s *GameService) DeleteGame(ctx context.Context, gameID int64) error
func (s *GameService) UpdateGame(ctx context.Context, gameID int64) error
```

### Implementation notes

- `DeleteGame` first calls `GetGameByID` to confirm existence (returning
  `ErrGameNotFound` for missing rows), then calls `queries.DeleteGame`.
- `UpdateGame` immediately returns `ErrGameUpdateNotImplemented`. No SQL,
  no body model.

### Acceptance tests

- **`TestGameService_DeleteGame`** — seed a game; `DeleteGame` succeeds;
  subsequent `GetGame` returns `ErrGameNotFound`.
- **`TestGameService_DeleteGame_NotFound`** — unknown ID → `ErrGameNotFound`.
- **`TestGameService_UpdateGame_NotImplemented`** — any ID → `ErrGameUpdateNotImplemented`.

Run: `go test ./internal/service/...`

---

## Task BD-6 — API handler create / list / show + route wiring [DONE]

**Depends on:** BD-5

### Goal

Expose `POST`, `GET /`, and `GET /:id` under `/api/v1/games`. Require
admin authentication. Wire `GameService` into the server bootstrap.

### Files to create/modify

| File                              | Action                                                                        |
|-----------------------------------|-------------------------------------------------------------------------------|
| `internal/handler/api.go`         | **edit** — add `games` field + update constructor                             |
| `internal/handler/api_games.go`   | **create**                                                                    |
| `internal/server/server.go`       | **edit** — instantiate `GameService`, pass to `NewAPIHandler`, register routes |
| `internal/server/handler_test.go` | **edit** — update `testServer`, add tests                                     |

### Handler changes (`internal/handler/api.go`)

Update `APIHandler` struct and constructor:

```go
type APIHandler struct {
    users      *service.UserService
    games      *service.GameService
    jwtManager *auth.Manager
    logger     *slog.Logger
}

func NewAPIHandler(
    users *service.UserService,
    games *service.GameService,
    jwtManager *auth.Manager,
    logger *slog.Logger,
) *APIHandler
```

### New file: `internal/handler/api_games.go`

DTOs:

```go
type apiCreateGameRequest struct {
    Name string `json:"name"`
}

type apiCreateGameResponse struct {
    ID int64 `json:"id"`
}

type apiGameResponse struct {
    ID          int64  `json:"id"`
    Name        string `json:"name"`
    Status      string `json:"status"`
    CurrentTurn int32  `json:"current_turn"`
    CreatedAt   string `json:"created_at"`
    UpdatedAt   string `json:"updated_at"`
}
```

Handler methods:

```go
func (h *APIHandler) CreateGame(c *echo.Context) error
func (h *APIHandler) ListGames(c *echo.Context) error
func (h *APIHandler) GetGame(c *echo.Context) error
```

Helpers:

```go
func parseGameID(c *echo.Context) (int64, error)        // strconv.ParseInt(c.Param("id"), 10, 64)
func apiGameFromService(g *service.Game) apiGameResponse // timestamps → time.RFC3339 in UTC
```

### Handler behavior

| Endpoint             | Success      | Errors                                                                                    |
|----------------------|--------------|-------------------------------------------------------------------------------------------|
| `POST /api/v1/games` | `201 {"id":N}` | invalid JSON → `400 {"error":"invalid request body"}`; blank name → `400 {"error":"name is required"}` |
| `GET /api/v1/games`  | `200 [...]`  | —                                                                                         |
| `GET /api/v1/games/:id` | `200 {...}` | bad `:id` → `400 {"error":"invalid game id"}`; missing → `404 {"error":"game not found"}` |

### Route registration (`internal/server/server.go`)

```go
gameService := service.NewGameService(db)
apiHandler := handler.NewAPIHandler(userService, gameService, jwtManager, logger)

// inside registerRoutes — add a sub-group with auth + admin role:
apiGamesGroup := apiGroup.Group("/games")
apiGamesGroup.Use(auth.RequireAuth(jwtManager))
apiGamesGroup.Use(loadCurrentViewer(userService))
apiGamesGroup.Use(requireRole(service.RoleAdmin))
apiGamesGroup.POST("", apiHandler.CreateGame)
apiGamesGroup.GET("", apiHandler.ListGames)
apiGamesGroup.GET("/:id", apiHandler.GetGame)
```

### Test harness changes (`internal/server/handler_test.go`)

Add `games *service.GameService` to `testServer`; instantiate in
`newTestServer`; add a bearer-header helper:

```go
func (ts *testServer) authHeader(t testing.TB, userID uuid.UUID) string
```

### Acceptance tests

- **`TestAPI_Games_Create_Admin`** — admin Bearer; `POST /api/v1/games`
  with `{"name":"Alpha"}`; status `201`; response has non-zero `id`.
- **`TestAPI_Games_Create_InvalidBody`** — malformed JSON; status `400`;
  `{"error":"invalid request body"}`.
- **`TestAPI_Games_Create_BlankName`** — `{"name":"   "}`; status `400`;
  `{"error":"name is required"}`.
- **`TestAPI_Games_Create_IgnoresUnknownFields`** —
  `{"name":"Alpha","seed":12345}`; status `201`; stored name is `"Alpha"`.
  Proves future config growth won't break the current API.
- **`TestAPI_Games_List_Admin`** — two seeded games; `GET /api/v1/games`;
  `200`; array length 2.
- **`TestAPI_Games_Show_Admin`** — seeded game; `GET /api/v1/games/:id`;
  `200`; body fields match.
- **`TestAPI_Games_Show_NotFound`** — unknown ID; `404 {"error":"game not found"}`.
- **`TestAPI_Games_Show_InvalidID`** — `/api/v1/games/not-a-number`;
  `400 {"error":"invalid game id"}`.

Run: `go test ./internal/server/...`

---

## Task BD-7 — API delete / update routes + auth JSON responses [DONE]

**Depends on:** BD-6

### Goal

Add `DELETE` and `PUT` routes. Fix auth middleware so `/api/...` paths
get JSON `401`/`403` responses instead of HTML redirects.

### Files to create/modify

| File                              | Action                                      |
|-----------------------------------|---------------------------------------------|
| `internal/handler/api_games.go`   | **edit** — add `DeleteGame`, `UpdateGame`   |
| `internal/server/server.go`       | **edit** — register new routes              |
| `internal/auth/middleware.go`     | **edit** — fix `unauthorized` for API paths |
| `internal/server/handler_test.go` | **edit** — add tests                        |

### Handler methods

```go
func (h *APIHandler) UpdateGame(c *echo.Context) error
func (h *APIHandler) DeleteGame(c *echo.Context) error
```

### Handler behavior

| Endpoint                   | Success          | Errors                                                                                    |
|----------------------------|------------------|-------------------------------------------------------------------------------------------|
| `PUT /api/v1/games/:id`    | —                | always `501 {"error":"not yet implemented"}` (also `400` for invalid ID)                  |
| `DELETE /api/v1/games/:id` | `204 No Content` | bad `:id` → `400 {"error":"invalid game id"}`; missing → `404 {"error":"game not found"}` |

### Auth middleware fix (`internal/auth/middleware.go`)

Update `unauthorized(c *echo.Context) error` so requests whose path starts
with `/api/` return:

```go
return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
```

Similarly, update `requireRole` in `internal/server/server.go` so API
paths return:

```go
return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
```

Preserve existing redirect behavior for non-API paths.

### Route registration

```go
apiGamesGroup.PUT("/:id", apiHandler.UpdateGame)
apiGamesGroup.DELETE("/:id", apiHandler.DeleteGame)
```

### Acceptance tests

- **`TestAPI_Games_Delete_Admin`** — admin Bearer; delete seeded game;
  `204`; subsequent GET returns `404`.
- **`TestAPI_Games_Delete_NotFound`** — unknown ID; `404 {"error":"game not found"}`.
- **`TestAPI_Games_Delete_InvalidID`** — `/api/v1/games/not-a-number`;
  `400 {"error":"invalid game id"}`.
- **`TestAPI_Games_Update_NotImplemented`** — admin Bearer;
  `PUT /api/v1/games/:id`; `501 {"error":"not yet implemented"}`.
- **`TestAPI_Games_AuthRequired`** — no auth on `GET /api/v1/games`;
  `401 {"error":"authentication required"}`.
- **`TestAPI_Games_AdminRequired`** — non-admin Bearer;
  `GET /api/v1/games`; `403 {"error":"forbidden"}`.

Run: `go test ./internal/server/... ./internal/auth/...`

---

## Task BD-8 — CLI `game create` command [TODO]

**Depends on:** BD-2, BD-7

### Goal

Add the `drynn game create --file game.json` command. The CLI is purely
an HTTP client — **no database imports, no pgx, no sqlc.** The config
file body is forwarded verbatim so future fields (seeds, world-gen
inputs) are preserved without CLI changes.

### Files to create/modify

| File                     | Action                                             |
|--------------------------|----------------------------------------------------|
| `cmd/drynn/main.go`      | **edit** — add `game` command to ff/v4 tree        |
| `cmd/drynn/game.go`      | **create**                                         |
| `cmd/drynn/game_test.go` | **create**                                         |

### Command tree addition

```
drynn
├── login
├── logout
├── health
├── version
└── game              (parent command, fallback Exec for help/unknown)
    └── create
```

The `game` flag set uses `SetParent(serverFlags)` so `--server` and
`--config` are inherited by all game subcommands.

### Functions (`cmd/drynn/game.go`)

```go
func newAuthenticatedRequest(method, url string, body io.Reader, session sessionData) (*http.Request, error)
func readAPIError(resp *http.Response, fallback string) error
```

The `game create` Exec closure is defined inline when building the command
tree.

### Create command: `--file` flag

```go
gameCreateFlags := ff.NewFlagSet("create").SetParent(gameFlags)
file := gameCreateFlags.StringLong("file", "", "path to game config JSON file")
```

### Implementation notes

- `game create` Exec flow:
  1. Require `--file` flag; fail if empty
  2. Use pre-run `drynnRuntime` (session + serverURL already resolved)
  3. Fail if `session.AccessToken == ""` with
     `"not logged in; run 'drynn login' first"`
  4. Read file bytes; validate `json.Valid(bytes)` — reject bad JSON
     client-side
  5. `POST /api/v1/games` with `Authorization: Bearer <token>`,
     `Content-Type: application/json`, body = raw file bytes
  6. On `201`, print response body to stdout
  7. Non-2xx → surface error via `readAPIError`

- `newAuthenticatedRequest` sets `Authorization: Bearer <token>`.
- `readAPIError` decodes `{"error":"..."}` from body, falling back to
  HTTP status text.
- The config file body is forwarded **verbatim** — the CLI never decodes
  it into a struct. This ensures future fields like `seed` or world-gen
  inputs survive without CLI changes.

### Pre-run setup

Add `game` and `game create` to the pre-run switch in `run()`: load
session and resolve server URL (same as `login` / `health`).

### Acceptance tests (`cmd/drynn/game_test.go`)

Use `httptest.NewServer` and a temp session directory to test without a
real server or database.

- **`TestRunGameCreate_PostsConfigFileVerbatimWithBearerToken`** —
  session has server URL + access token; config file has
  `{"name":"Alpha","seed":123}`; test server asserts: method `POST`,
  path `/api/v1/games`, header `Authorization: Bearer <token>`, body
  still contains `"seed":123`; command returns nil.
- **`TestRunGameCreate_RequiresFile`** — missing `--file`; returns error.
- **`TestRunGameCreate_InvalidJSONFile`** — file contains `{bad json`;
  returns error before any HTTP request.
- **`TestRunGameCreate_RequiresLogin`** — empty access token; returns
  error containing `not logged in`.

Run: `go test ./cmd/drynn/...`

---

## Task BD-9 — CLI list / show / delete / update + help text [TODO]

**Depends on:** BD-8

### Goal

Complete the CLI with the remaining four game subcommands and update all
help text.

### Files to modify

| File                     | Action                                          |
|--------------------------|-------------------------------------------------|
| `cmd/drynn/game.go`      | **edit** — add list/show/delete/update commands |
| `cmd/drynn/game_test.go` | **edit** — add tests                            |
| `cmd/drynn/main.go`      | **edit** — update top-level usage               |

### Command tree addition

```
drynn
└── game
    ├── create    (from BD-8)
    ├── list
    ├── show
    ├── update
    └── delete
```

### Flag sets

- **`list`**: `SetParent(gameFlags)` (no additional flags)
- **`show`**: `SetParent(gameFlags)` + `--id`
- **`update`**: `SetParent(gameFlags)` + `--id`
- **`delete`**: `SetParent(gameFlags)` + `--id`

### Implementation notes

- `list`: `GET /api/v1/games`; print response body to stdout.
- `show`: require `--id`; `GET /api/v1/games/:id`; print body.
- `delete`: require `--id`; `DELETE /api/v1/games/:id`; on `204`
  print `"deleted"`.
- `update`: require `--id`; `PUT /api/v1/games/:id`; if server returns
  `501`, surface `"not yet implemented"`.
- All commands use `newAuthenticatedRequest` from BD-8.
- All commands use `readAPIError` for non-2xx responses.

### Help text

Top-level usage should include:

```
  game      manage games (create, list, show, update, delete)
```

`game` sub-usage should list:

```
usage: drynn game <command> [flags]

commands:
  create    create a new game from a config file
  list      list all games
  show      show details of a game
  update    update a game (not yet implemented)
  delete    delete a game
```

### Acceptance tests

- **`TestRunGameList_UsesBearerAndPrintsBody`** — test server returns
  JSON array; command sends Bearer token; stdout matches server body.
- **`TestRunGameShow_UsesBearerAndPrintsBody`** — test server returns
  one JSON object; command sends Bearer token; stdout matches.
- **`TestRunGameDelete_UsesBearerAndDeletesByID`** — test server asserts
  method/path/header; command prints `"deleted"`.
- **`TestRunGameUpdate_PropagatesNotImplemented`** — server returns
  `501 {"error":"not yet implemented"}`; command returns error containing
  `"not yet implemented"`.
- **`TestUsage_IncludesGameCommand`** — top-level usage output contains
  `"game"`.

Run: `go test ./cmd/drynn/...`

---

## Standardized error messages

All API error responses use this exact format and these exact strings:

| Message                   | HTTP Status |
|---------------------------|-------------|
| `invalid request body`    | 400         |
| `name is required`        | 400         |
| `invalid game id`         | 400         |
| `game not found`          | 404         |
| `authentication required` | 401         |
| `forbidden`               | 403         |
| `not yet implemented`     | 501         |
| `internal error`          | 500         |

---

## Dependency graph

```
BD-1 (ff/v4 cmd/db)
  └─▶ BD-2 (ff/v4 cmd/drynn)
        └─▶ BD-8 (CLI game create) ◀── BD-7
              └─▶ BD-9 (CLI list/show/delete/update)

BD-3 (queries + fixtures)     ← independent of BD-1/BD-2
  └─▶ BD-4 (service create/get/list)
        └─▶ BD-5 (service delete + update stub)
              └─▶ BD-6 (API create/list/show + routes)
                    └─▶ BD-7 (API delete/update + auth fix)
                          └─▶ BD-8 (CLI game create)
```

Two parallel tracks can run concurrently:
- **Track A**: BD-1 → BD-2
- **Track B**: BD-3 → BD-4 → BD-5 → BD-6 → BD-7

They converge at BD-8, which depends on both BD-2 and BD-7.
