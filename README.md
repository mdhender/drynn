# Dryn

Dryn is a 4X PBBG game with a sci-fi theme.

## Features

- Public landing page with registration and sign-in flows.
- JWT-based authentication with access and refresh cookies.
- Lowercase, unique handles and emails enforced in both the service layer and the database.
- Role-aware app shell with `user` and `admin` database roles, plus a synthetic `guest` sentinel for unauthenticated sessions.
- Admin user management and self-service profile editing.
- Database-backed JWT signing key lifecycle managed by a CLI.

## Project Layout

```text
cmd/
  db/         Database management CLI
  server/     Web server entrypoint
db/
  migrations/ Atlas migration files
  queries/    sqlc query definitions
  schema.sql  Desired database schema
  sqlc/       Generated sqlc code
internal/
  auth/       JWT, password hashing, auth middleware
  config/     Persisted server config loading/writing
  handler/    HTTP handlers
  server/     Echo app bootstrap and route registration
  service/    Business logic
web/
  components/ Reusable template partials
  static/     CSS and static assets
  templates/  Go templates
```

## Project dependencies

Tools and services you'll need to build, run, and deploy Hobo.

- **Go 1.26+** — toolchain for building `cmd/server`, `cmd/db`, and running tests.
- **PostgreSQL 16+** — primary datastore; the schema and migrations target PG 16 features.
- **Atlas CLI** — schema-diff migration tool; run `atlas migrate diff` after editing `db/schema.sql` and `atlas migrate apply` to apply pending migrations.
- **sqlc CLI** — generates typed query code under `db/sqlc/` from `db/queries/*.sql`.
- **make** — task runner for the `Makefile` targets (`build`, `test`, `lint`, `generate`, `docs`).
- **Hugo Extended** — builds the static documentation site under `site/` that the server embeds and serves at `/docs/` (wired up in Phase 8).
- **Mailgun account** — transactional email provider for invitations and password-reset messages; domain and API key go into `server.json`.

## Bootstrapping

1. Prepare the database and schema.
2. Apply migrations, initialize the persisted config, create JWT signing keys, and seed an admin with [cmd/db/README.md](cmd/db/README.md).
3. Start the application with [cmd/server/README.md](cmd/server/README.md).

The server will refuse to start until there is one active signing key for `access` and one for `refresh`.

## First-Run Checklist

For the verified local setup using `postgres://drynn_dev_user:strong-password-here@localhost:5432/drynn_dev?sslmode=disable`:

- Use the dedicated-schema setup and first-run commands in [cmd/db/README.md](cmd/db/README.md).
- Start the server with the example in [cmd/server/README.md](cmd/server/README.md).
- Then open `http://127.0.0.1:8080/signin` and sign in with `admin@example.com` / `password123`.

## Configuration

The primary runtime configuration is stored in a JSON file created by `cmd/db init-config`. The server also supports environment overrides for:

- `HOBO_CONFIG_PATH`
- `APP_ADDR`
- `DATABASE_URL`
- `DATA_DIR`
- `JWT_ACCESS_TTL`
- `JWT_REFRESH_TTL`
- `COOKIE_SECURE`

Use `cmd/db seed-admin` to create or update the initial administrator account.

## Database Management

The database CLI supports:

- `init-config` to write the persisted server config file.
- `seed-admin` to create or update an admin user through the existing service layer.
- `jwt-key create` to generate a new active key and retire the previous active key for that token type.
- `jwt-key expire` to retire a specific key with an optional verification grace period.
- `jwt-key delete` to remove a non-active key.

See [cmd/db/README.md](cmd/db/README.md) for the exact commands, the dedicated-schema Atlas setup, and the verified first-run example.

## Development Commands

```bash
go test ./...
go build ./cmd/db ./cmd/server
sqlc generate
atlas migrate hash --dir file://db/migrations
```

## Notes

- Do not hand-edit files under `db/sqlc/`.
- Do not write raw SQL in Go code; add queries under `db/queries/` instead.
- Update `db/schema.sql` first, then create a new migration.
