# cmd/server

`cmd/server` starts the Echo v5 web application.

## Responsibilities

- Load the persisted server config file.
- Connect to PostgreSQL.
- Ensure active JWT signing keys exist.
- Construct the Echo app, register routes, and start serving HTTP.

## Usage

```bash
go run ./cmd/server -config data/var/drynn/server.json
```

If `-config` is not provided, the default path is `data/var/drynn/server.json`, or the value of `HOBO_CONFIG_PATH` if that environment variable is set.

## Startup Requirements

Before starting the server, the following must already exist:

- A reachable PostgreSQL database.
- The application schema migrated into that database.
- One active `access` signing key.
- One active `refresh` signing key.

If either signing key is missing, startup fails immediately.

## Config Fields

The JSON config file contains:

- `version`
- `app_addr`
- `database_url`
- `data_dir`
- `jwt_access_ttl`
- `jwt_refresh_ttl`
- `cookie_secure`

These values are loaded by `internal/config` and can be overridden by environment variables such as `APP_ADDR`, `DATABASE_URL`, `JWT_ACCESS_TTL`, and `COOKIE_SECURE`.

## Typical Flow

```bash
atlas migrate apply --dir file://db/migrations --url "$DATABASE_URL"
go run ./cmd/db init-config --database-url "$DATABASE_URL" --base-url "http://localhost:8080"
go run ./cmd/db jwt-key create --type access
go run ./cmd/db jwt-key create --type refresh
go run ./cmd/db seed-admin --handle admin --email admin@example.com --password 'change-me-now'
go run ./cmd/server
```

## First-Run Example

```bash
export DATABASE_URL='postgres://drynn_dev_user:strong-password-here@localhost:5432/drynn_dev?sslmode=disable'

atlas migrate apply --allow-dirty --dir file://db/migrations --url "$DATABASE_URL"
go run ./cmd/db init-config --config data/var/drynn/server.json --database-url "$DATABASE_URL" --base-url "http://localhost:8080"
go run ./cmd/db jwt-key create --config data/var/drynn/server.json --type access
go run ./cmd/db jwt-key create --config data/var/drynn/server.json --type refresh
go run ./cmd/db seed-admin --config data/var/drynn/server.json --handle admin --email admin@example.com --password 'password123'
go run ./cmd/server -config data/var/drynn/server.json
```

## Notes

- Access and refresh tokens are issued as HTTP-only cookies.
- JWT verification loads the referenced signing key from the database using the token `kid` header.
- The initial admin account is created out-of-band via `cmd/db seed-admin`; the server itself does not bootstrap any users.
