---
title: CLI
weight: 20
---

Drynn ships three binaries: `cmd/server`, `cmd/db`, and
`cmd/email`. All read the same config file.

## cmd/server

HTTP server. Serves the web application, static assets, and
documentation.

```
cmd/server [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `data/var/drynn/server.json` | Config file path |

Pass `version` as the first argument to print the version and exit.

### Startup prerequisites

1. PostgreSQL reachable with migrations applied.
2. Valid config file (or `DATABASE_URL` set).
3. One active JWT signing key each for `access` and `refresh`.

### Shutdown

Handles `SIGINT` and `SIGTERM` with a 10-second graceful timeout.

---

## cmd/db

Database management CLI. Every subcommand that accesses the database
reads the connection string from the config file.

```
cmd/db <command> [flags]
```

### init-config

Writes the JSON config file used by `cmd/server`.

```
cmd/db init-config [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `data/var/drynn/server.json` | Output path |
| `--app-addr` | `:8080` | Listen address |
| `--database-url` | *(required)* | PostgreSQL connection string |
| `--base-url` | *(required)* | Absolute base URL for invitation and password-reset links |
| `--data-dir` | `data/var/drynn/data` | Data directory |
| `--jwt-access-ttl` | `15m` | Access token lifetime |
| `--jwt-refresh-ttl` | `168h` | Refresh token lifetime |
| `--cookie-secure` | `false` | Secure cookie flag |
| `--mailgun-api-key` | *(empty)* | Mailgun API key |
| `--mailgun-sending-domain` | *(empty)* | Mailgun sending domain |
| `--mailgun-from-address` | *(empty)* | Sender address |
| `--mailgun-from-name` | *(empty)* | Sender display name |
| `--request-access-enabled` | `false` | Enable access request form |
| `--admin-contact-email` | *(empty)* | Access request destination |
| `--force` | `false` | Overwrite existing file |

Creates config and data directories if missing. Fails if the file
exists unless `--force` is set.

### seed-admin

Creates or updates the bootstrap administrator.

```
cmd/db seed-admin [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `data/var/drynn/server.json` | Config file path |
| `--handle` | *(required)* | Admin handle |
| `--email` | *(required)* | Admin email |
| `--password` | *(required)* | Admin password |

Uses the same validation as web registration. Assigns `user` and
`admin` roles. If a user with the email already exists, updates
their handle and password.

### seed-testers

Creates seeded test accounts.

```
cmd/db seed-testers [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `data/var/drynn/server.json` | Config file path |
| `--count` | *(required, >= 1)* | Number of accounts to ensure exist |

Creates `tester_1@drynn.test` through `tester_N@drynn.test`
with the `user` and `tester` roles. Accounts are created with a
sentinel password that prevents sign-in — set real passwords via
`set-password` or the admin UI.

Idempotent: if the count is already met, does nothing.

### set-password

Sets a user's password by email.

```
cmd/db set-password [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `data/var/drynn/server.json` | Config file path |
| `--email` | *(required)* | User email address |
| `--password` | *(required)* | New password (min 8 characters) |

### jwt-key create

Generates a new active signing key and retires the previous one.

```
cmd/db jwt-key create [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `data/var/drynn/server.json` | Config file path |
| `--type` | `access` | Token type: `access` or `refresh` |
| `--verify-old-for` | token TTL | Grace period for the retired key |

The grace period defaults to the configured TTL for the token type.
Set to `0` to immediately invalidate existing tokens.

### jwt-key expire

Retires a specific key by UUID.

```
cmd/db jwt-key expire [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `data/var/drynn/server.json` | Config file path |
| `--id` | *(required)* | Signing key UUID |
| `--verify-for` | `0` | Verification grace period |

### jwt-key delete

Deletes a non-active signing key.

```
cmd/db jwt-key delete [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `data/var/drynn/server.json` | Config file path |
| `--id` | *(required)* | Signing key UUID |

Active keys cannot be deleted — rotate or expire them first.

### version

Prints the application version.

---

## cmd/email

Email sender CLI. Uses Mailgun credentials from the config file.

```
cmd/email <command> [flags]
```

### send

Sends a single HTML email.

```
cmd/email send [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `data/var/drynn/server.json` | Config file path |
| `--to` | *(required)* | Recipient address |
| `--subject` | *(required)* | Subject line |
| `--body` | *(empty)* | HTML body (mutually exclusive with `--body-file`) |
| `--body-file` | *(empty)* | Path to HTML body file |

One of `--body` or `--body-file` is required. Mailgun must be
configured in the config file.

### version

Prints the application version.
