# cmd/db command reference

Database management CLI for drynn. All commands that access the database read connection details from the server config file.

## Global behavior

Every command accepts `--config` to specify the config file path. Default: `data/var/drynn/server.json` (overridden by `HOBO_CONFIG_PATH` env var).

Commands that access the database (`seed-admin`, `set-password`, `jwt-key`) load the `database_url` from the config file, open a connection pool, and verify connectivity before proceeding.

## init-config

Writes the JSON config file used by `cmd/server`.

```
cmd/db init-config [flags]
```

| Flag                | Default                      | Description                                               |
|---------------------|------------------------------|-----------------------------------------------------------|
| `--config`          | `data/var/drynn/server.json` | Path to the server config file                            |
| `--app-addr`        | `:8080`                      | Server listen address                                     |
| `--database-url`    | *(required)*                 | PostgreSQL connection string                              |
| `--base-url`        | *(required)*                 | Absolute base URL for invitation and password-reset links |
| `--data-dir`        | `data/var/drynn/data`        | Path for server-managed data files                        |
| `--jwt-access-ttl`  | `15m`                        | Access token lifetime                                     |
| `--jwt-refresh-ttl` | `168h`                       | Refresh token lifetime                                    |
| `--cookie-secure`   | `false`                      | Set Secure flag on auth cookies                           |
| `--force`           | `false`                      | Overwrite an existing config file                         |

Creates the config directory and data directory if they do not exist. Fails if the config file already exists unless `--force` is set.

## seed-admin

Creates or updates the bootstrap administrator through the service layer.

```
cmd/db seed-admin [flags]
```

| Flag         | Default                      | Description                    |
|--------------|------------------------------|--------------------------------|
| `--config`   | `data/var/drynn/server.json` | Path to the server config file |
| `--handle`   | `""`                         | Admin handle (required)        |
| `--email`    | `""`                         | Admin email (required)         |
| `--password` | `""`                         | Admin password (required)      |

All three of `--handle`, `--email`, and `--password` are required. Uses the same normalization, uniqueness checks, password hashing, and role assignment as the web application. Fails if the handle or email is already in use by a different user.

## set-password

Sets a user's password, identified by email address.

```
cmd/db set-password [flags]
```

| Flag         | Default                      | Description                    |
|--------------|------------------------------|--------------------------------|
| `--config`   | `data/var/drynn/server.json` | Path to the server config file |
| `--email`    | `""`                         | User email address (required)  |
| `--password` | `""`                         | New password (required)        |

Validates the password (minimum 8 characters), hashes it with bcrypt, and updates the database. Fails if no user exists with the given email.

## jwt-key

Manages database-backed JWT signing keys. Has three subcommands.

```
cmd/db jwt-key <create|expire|delete> [flags]
```

### jwt-key create

Generates a new active signing key for a token type and retires the previous active key.

```
cmd/db jwt-key create [flags]
```

| Flag               | Default                      | Description                                   |
|--------------------|------------------------------|-----------------------------------------------|
| `--config`         | `data/var/drynn/server.json` | Path to the server config file                |
| `--type`           | `access`                     | Token type: `access` or `refresh`             |
| `--verify-old-for` | token TTL                    | Verification grace period for the retired key |

Generates a random 32-byte HMAC secret. If an active key of the same type exists, it is retired with a verification grace period (defaults to the configured TTL for that token type).

### jwt-key expire

Retires a specific key by UUID.

```
cmd/db jwt-key expire [flags]
```

| Flag           | Default                      | Description                                                 |
|----------------|------------------------------|-------------------------------------------------------------|
| `--config`     | `data/var/drynn/server.json` | Path to the server config file                              |
| `--id`         | `""`                         | Signing key UUID (required)                                 |
| `--verify-for` | `0`                          | Duration to continue allowing verification after retirement |

With `--verify-for 0` (the default), the key stops verifying immediately.

### jwt-key delete

Deletes a non-active signing key by UUID.

```
cmd/db jwt-key delete [flags]
```

| Flag       | Default                      | Description                    |
|------------|------------------------------|--------------------------------|
| `--config` | `data/var/drynn/server.json` | Path to the server config file |
| `--id`     | `""`                         | Signing key UUID (required)    |

Active keys cannot be deleted. Rotate or expire them first.
