# cmd/server command reference

HTTP server for drynn. Serves the web application, static assets, and API endpoints.

## Usage

```
cmd/server [flags]
```

| Flag       | Default                      | Description                    |
|------------|------------------------------|--------------------------------|
| `--config` | `data/var/drynn/server.json` | Path to the server config file |

The default config path is overridden by the `HOBO_CONFIG_PATH` environment variable when set.

## Configuration

The server loads configuration from a JSON file written by `cmd/db init-config`, then applies environment variable overrides. Environment variables take precedence over the JSON file.

| JSON field        | Environment variable | Default               | Description                        |
|-------------------|----------------------|-----------------------|------------------------------------|
| `app_addr`        | `APP_ADDR`           | `:8080`               | Listen address                     |
| `database_url`    | `DATABASE_URL`       | (required)            | PostgreSQL connection string       |
| `data_dir`        | `DATA_DIR`           | `data/var/drynn/data` | Path for server-managed data files |
| `jwt_access_ttl`  | `JWT_ACCESS_TTL`     | `15m`                 | Access token lifetime              |
| `jwt_refresh_ttl` | `JWT_REFRESH_TTL`    | `168h`                | Refresh token lifetime             |
| `cookie_secure`   | `COOKIE_SECURE`      | `false`               | Set Secure flag on auth cookies    |

Create or update the initial administrator with `cmd/db seed-admin` — the server itself does not perform any admin bootstrapping.

## Startup prerequisites

The server requires the following before it will start:

1. A reachable PostgreSQL database with migrations applied.
2. A valid config file (or `DATABASE_URL` in the environment).
3. One active JWT signing key for `access` and one for `refresh` in the `jwt_signing_keys` table.

If any prerequisite is missing, the server exits with a fatal error.

## Startup sequence

1. Load and merge config (JSON file, then environment overrides).
2. Open a PostgreSQL connection pool and verify connectivity.
3. Verify at least one active signing key exists per token type.
4. Compile templates and register routes.
5. Listen on the configured address.

## Shutdown

The server handles `SIGINT` and `SIGTERM`. On receipt, it initiates a graceful shutdown with a 10-second timeout, allowing in-flight requests to complete before closing the database pool and exiting.

## Routes

### Public (no auth)

| Method | Path        | Handler                   |
|--------|-------------|---------------------------|
| GET    | `/`         | Landing page              |
| GET    | `/register` | Registration form         |
| POST   | `/register` | Submit registration       |
| GET    | `/signin`   | Sign-in form              |
| POST   | `/signin`   | Submit sign-in            |
| POST   | `/logout`   | Sign out (clears cookies) |
| POST   | `/refresh`  | Refresh access token      |

### App (auth required)

| Method | Path           | Handler                    |
|--------|----------------|----------------------------|
| GET    | `/app`         | Redirect to `/app/profile` |
| GET    | `/app/profile` | Profile form               |
| POST   | `/app/profile` | Update profile             |

### Admin (auth + admin role required)

| Method | Path                          | Handler            |
|--------|-------------------------------|--------------------|
| GET    | `/app/admin/users`            | List users         |
| GET    | `/app/admin/users/new`        | Create user form   |
| POST   | `/app/admin/users`            | Submit create user |
| GET    | `/app/admin/users/:id/edit`   | Edit user form     |
| POST   | `/app/admin/users/:id`        | Submit user update |
| POST   | `/app/admin/users/:id/delete` | Delete user        |

### Static

| Method | Path        | Source        |
|--------|-------------|---------------|
| GET    | `/static/*` | `web/static/` |
