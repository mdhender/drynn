# cmd/db

`cmd/db` manages persisted server configuration and database bootstrap tasks.

For the project overview, prerequisites, and high-level startup flow, see [../../README.md](../../README.md). For the runtime server entrypoint, see [../server/README.md](../server/README.md).

## Commands

### `init-config`

Writes the JSON config file used by `cmd/server`.

```bash
go run ./cmd/db init-config \
  --config data/var/drynn/server.json \
  --database-url "$DATABASE_URL" \
  --data-dir data/var/drynn/data
```

Useful flags:

- `--app-addr`
- `--database-url`
- `--data-dir`
- `--jwt-access-ttl`
- `--jwt-refresh-ttl`
- `--cookie-secure`
- `--force`

The command creates the config directory and data directory if needed.

### `seed-admin`

Creates or updates the bootstrap administrator through `internal/service.UserService`.

```bash
go run ./cmd/db seed-admin \
  --config data/var/drynn/server.json \
  --handle admin \
  --email admin@example.com \
  --password 'change-me-now'
```

This reuses the same normalization, uniqueness, password hashing, and role rules as the web app.

### `jwt-key create`

Creates a new active signing key for a token type.

```bash
go run ./cmd/db jwt-key create --config data/var/drynn/server.json --type access
go run ./cmd/db jwt-key create --config data/var/drynn/server.json --type refresh
```

Behavior:

- Generates a random 32-byte HMAC secret.
- Inserts a new active key for the requested token type.
- Retires the previous active key of that type, if one exists.
- Defaults the previous key's verification grace period to the configured token TTL.

Optional flag:

- `--verify-old-for`

### `jwt-key expire`

Retires a specific key by ID and optionally allows it to keep verifying tokens for a short grace period.

```bash
go run ./cmd/db jwt-key expire \
  --config data/var/drynn/server.json \
  --id <key-uuid> \
  --verify-for 10m
```

### `jwt-key delete`

Deletes a non-active key.

```bash
go run ./cmd/db jwt-key delete \
  --config data/var/drynn/server.json \
  --id <key-uuid>
```

Active keys cannot be deleted directly. Rotate or expire them first.

## Typical Bootstrap Sequence

```bash
atlas migrate apply --dir file://db/migrations --url "$DATABASE_URL"
go run ./cmd/db init-config --database-url "$DATABASE_URL"
go run ./cmd/db jwt-key create --type access
go run ./cmd/db jwt-key create --type refresh
go run ./cmd/db seed-admin --handle admin --email admin@example.com --password 'change-me-now'
```

## Using a Dedicated Postgres Schema

Create a schema for Atlas to use for migration analysis:

```sql
CREATE ROLE drynn_atlas_user WITH LOGIN PASSWORD 'strong-password-here';
CREATE DATABASE drynn_atlas OWNER drynn_atlas_user;
\c drynn_atlas
CREATE SCHEMA drynn_atlas AUTHORIZATION drynn_atlas_user;
-- Atlas scratch schema. The checked-in atlas.hcl sets search_path=drynn_dev
-- on its dev URL so the scratch schema mirrors the application schema name;
-- without this CREATE, `atlas migrate diff` fails with `schema "drynn_dev"
-- was not found`.
CREATE SCHEMA drynn_dev AUTHORIZATION drynn_atlas_user;
ALTER ROLE drynn_atlas_user IN DATABASE drynn_atlas SET search_path TO drynn_atlas, public;
GRANT ALL ON SCHEMA drynn_atlas TO drynn_atlas_user;
GRANT ALL ON SCHEMA drynn_dev TO drynn_atlas_user;
```

If you want all application tables to live outside `public`, create a dedicated role, database, and schema first:

```sql
CREATE ROLE drynn_dev_user WITH LOGIN PASSWORD 'strong-password-here';
CREATE DATABASE drynn_dev OWNER drynn_dev_user;
\c drynn_dev
CREATE SCHEMA drynn_dev AUTHORIZATION drynn_dev_user;
ALTER ROLE drynn_dev_user IN DATABASE drynn_dev SET search_path TO drynn_dev, public;
GRANT ALL ON SCHEMA drynn_dev TO drynn_dev_user;
```

With that in place, use a database URL that connects as `drynn_dev_user`. The role-level `search_path` ensures the unqualified tables in `db/schema.sql` land in `drynn_dev` instead of `public`.

For the first Atlas migration apply against this database, use:

```bash
export DATABASE_URL='postgres://drynn_dev_user:strong-password-here@localhost:5432/drynn_dev?sslmode=disable'

atlas migrate apply \
  --allow-dirty \
  --dir file://db/migrations \
  --revisions-schema atlas_schema_revisions \
  --url "$DATABASE_URL"
```

The extra flags matter:

- `--allow-dirty` is required because the dedicated `drynn_dev` schema already exists before Atlas applies the first migration.
- `--revisions-schema atlas_schema_revisions` keeps Atlas bookkeeping out of the application schema and out of `public`.

For future migration generation, override the checked-in `atlas.hcl` defaults and use a schema-aware dev database:

```bash
atlas migrate diff <name> \
  --dir file://db/migrations \
  --to file://db/schema.sql \
  --dev-url 'postgres://drynn_atlas_user:strong-password-here@localhost:5432/drynn_atlas?sslmode=disable' \
  --schema drynn_dev
```

Using `--schema drynn_dev` plus a `search_path` of `drynn_dev,public` keeps the managed objects in the application schema while still allowing references to extension objects such as `pgcrypto`.

## First-Run Example

```bash
export DATABASE_URL='postgres://drynn_dev_user:strong-password-here@localhost:5432/drynn_dev?sslmode=disable'

atlas migrate apply --allow-dirty --dir file://db/migrations --url "$DATABASE_URL"

go run ./cmd/db init-config \
  --config data/var/drynn/server.json \
  --database-url "$DATABASE_URL" \
  --data-dir data/var/drynn/data

go run ./cmd/db jwt-key create --config data/var/drynn/server.json --type access
go run ./cmd/db jwt-key create --config data/var/drynn/server.json --type refresh

go run ./cmd/db seed-admin \
  --config data/var/drynn/server.json \
  --handle admin \
  --email admin@example.com \
  --password 'password123'
```

## Operational Notes

- The command reads the same config file format as `cmd/server`.
- `jwt-key create` and `jwt-key expire` keep old keys available for verification during a configurable grace window.
- JWT secrets live in PostgreSQL, not in environment variables.
- The server expects at least one active key for each token type before startup.
