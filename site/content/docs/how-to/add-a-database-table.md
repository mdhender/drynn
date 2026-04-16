---
title: Add a database table
weight: 90
---

This guide walks through adding a new table, generating a migration,
writing queries, and wiring the generated code into the service
layer. The example adds a `game_sessions` table.

## 1. Edit the schema

Open `db/schema.sql` and add the table definition:

```sql
CREATE TABLE game_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER game_sessions_set_updated_at
BEFORE UPDATE ON game_sessions
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
```

`db/schema.sql` is the single source of truth for the database
structure. Never edit migration files directly.

## 2. Generate the migration

```sh
atlas migrate diff add_game_sessions --env local
```

This compares `db/schema.sql` against the current migration state
and writes a new migration file in `db/migrations/`. Review the
generated SQL to confirm it matches your intent.

## 3. Lint the migration

```sh
atlas migrate lint --env local --latest 1
```

Fix any warnings before applying.

## 4. Apply the migration

```sh
atlas migrate apply --url "$DATABASE_URL"
```

## 5. Write queries

Create `db/queries/game_sessions.sql`:

```sql
-- name: CreateGameSession :one
INSERT INTO game_sessions (name, created_by, starts_at, ends_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetGameSession :one
SELECT * FROM game_sessions WHERE id = $1;

-- name: ListGameSessions :many
SELECT * FROM game_sessions ORDER BY starts_at DESC;

-- name: DeleteGameSession :exec
DELETE FROM game_sessions WHERE id = $1;
```

Each query starts with a `-- name:` comment that tells sqlc the Go
method name and return style (`:one`, `:many`, `:exec`,
`:execrows`).

## 6. Generate Go code

```sh
sqlc generate
```

This regenerates all files in `db/sqlc/`. Never hand-edit that
directory — your changes will be overwritten.

## 7. Use in the service layer

Create a service (or add methods to an existing one) that wraps the
generated queries:

```go
type GameService struct {
    queries *sqlc.Queries
}

func NewGameService(pool *pgxpool.Pool) *GameService {
    return &GameService{queries: sqlc.New(pool)}
}

func (s *GameService) Create(ctx context.Context, name string, createdBy uuid.UUID, startsAt time.Time) (*sqlc.GameSession, error) {
    row, err := s.queries.CreateGameSession(ctx, sqlc.CreateGameSessionParams{
        Name:      name,
        CreatedBy: createdBy,
        StartsAt:  startsAt,
    })
    if err != nil {
        return nil, fmt.Errorf("create game session: %w", err)
    }

    return &row, nil
}
```

## 8. Verify

```sh
go build ./...
go test ./...
```

The build confirms the generated code compiles and your service
layer uses it correctly.
