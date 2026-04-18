# cmd/drynn command reference

CLI tool for drynn administrators. Communicates with the server exclusively via the HTTP API — never accesses the database directly.

## Session file

Drynn stores session state in `~/.config/drynn/drynn.json`. The file holds the server URL and authentication tokens:

```json
{
  "server_url": "http://localhost:8080",
  "access_token": "...",
  "refresh_token": "..."
}
```

Once logged in, subsequent commands use the stored server URL and tokens automatically.

## Server URL resolution

Commands that contact the server resolve the URL in this order:

1. `--server` flag
2. `DRYNN_SERVER_URL` environment variable
3. `base_url` from the config file specified by `--config`
4. `server_url` from the existing session file

If none of these provide a URL, the command fails with an error.

## login

Authenticates with the server and saves the session.

```
drynn login [flags]
```

| Flag         | Default | Description                            |
|--------------|---------|----------------------------------------|
| `--email`    | `""`    | Account email address (required)       |
| `--password` | `""`    | Account password (required)            |
| `--server`   | `""`    | Server URL (e.g. http://localhost:8080) |
| `--config`   | `""`    | Path to the server config file         |

Both `--email` and `--password` are required. The server URL must be resolvable via one of the sources listed above.

If `--config` is provided and an existing session with tokens is present, the command fails with a message to run `drynn logout` first. This prevents accidentally switching servers without an explicit logout.

On success, the access token, refresh token, and server URL are saved to the session file for use by future commands.

## logout

Clears authentication tokens from the session file. Does not contact the server.

```
drynn logout
```

The session file is preserved with the `server_url` intact so subsequent logins can reuse it.

## health

Queries the server health endpoint.

```
drynn health [flags]
```

| Flag       | Default | Description                            |
|------------|---------|----------------------------------------|
| `--server` | `""`    | Server URL (e.g. http://localhost:8080) |
| `--config` | `""`    | Path to the server config file         |

Prints the server status and version:

```
status=ok version=0.1.0
```

## version

Prints the build version and exits.

```
drynn version
```
