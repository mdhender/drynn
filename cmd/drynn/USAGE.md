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

## test-hexmap

Generates a hex map populated with star systems and writes a self-contained HTML visualization. Local diagnostic; does not contact the server.

```
drynn test-hexmap [flags]
```

| Flag                 | Default | Description                                                 |
|----------------------|---------|-------------------------------------------------------------|
| `--radius`           | `15`    | Disk radius in hexes                                        |
| `--systems`          | `100`   | Number of star systems to place                             |
| `--min-distance`     | `0`     | Minimum hex distance between systems                        |
| `--no-merge`         | *(off)* | Discard placements that are too close instead of merging their stars (default is to merge) |
| `--seed1`            | `20`    | PRNG seed value 1                                           |
| `--seed2`            | `20`    | PRNG seed value 2                                           |
| `--use-random-seeds` | *(off)* | Use random seeds instead of `--seed1`/`--seed2`             |
| `--coords`           | *(off)* | Render axial `(q,r)` coordinates inside occupied hexes      |
| `--out`              | `.`     | Output directory for the generated `hexmap.html`            |

Writes `hexmap.html` under `--out` and prints placement statistics (system count, star breakdown, pairwise and nearest-neighbor distances).

## test-galaxy

Generates a full galaxy (hex map + stars + planets) using `internal/worldgen` and optionally writes a self-contained HTML page. Local diagnostic; does not contact the server.

```
drynn test-galaxy [flags]
```

| Flag                 | Default | Description                                                 |
|----------------------|---------|-------------------------------------------------------------|
| `--radius`           | `15`    | Disk radius in hexes                                        |
| `--systems`          | `100`   | Target number of star systems to place                      |
| `--min-distance`     | `0`     | Minimum hex distance between systems                        |
| `--no-merge`         | *(off)* | Discard placements that are too close instead of merging their stars (default is to merge) |
| `--seed1`            | `20`    | PRNG seed value 1                                           |
| `--seed2`            | `20`    | PRNG seed value 2                                           |
| `--use-random-seeds` | *(off)* | Use random seeds instead of `--seed1`/`--seed2`             |
| `--html`             | *(off)* | Write `galaxy.html` with the hex map                        |
| `--coords`           | *(off)* | Render axial `(q,r)` coordinates inside occupied hexes      |
| `--planets`          | *(off)* | Include a per-system planet report in the generated HTML    |
| `--pixel-size`       | `0`     | Hex pixel size (0 = auto-fit to ~1280×1280)                 |
| `--out`              | `.`     | Output directory for `galaxy.html`                          |

Always prints system/star/planet counts and a star-count breakdown. When `--html` is set, writes `galaxy.html` to `--out`; `--coords`, `--planets`, and `--pixel-size` only affect the HTML output.

## simulate

Simulates the GM's interactive cluster-creation workflow locally: generates a galaxy, writes `galaxy.html`, then produces a home-system template for each planet count 3–9 and writes `home-system-{N}.html`. Optionally writes the full run state to a deterministic JSON file for review and golden file tests. Local diagnostic; does not contact the server.

```
drynn simulate [flags]
```

| Flag                 | Default | Description                                                        |
|----------------------|---------|--------------------------------------------------------------------|
| `--radius`           | `15`    | Disk radius in hexes                                               |
| `--systems`          | `100`   | Target number of star systems to place                             |
| `--min-distance`     | `0`     | Minimum hex distance between systems                               |
| `--no-merge`         | *(off)* | Discard placements that are too close instead of merging their stars (default is to merge) |
| `--seed1`            | `20`    | PRNG seed value 1                                                  |
| `--seed2`            | `20`    | PRNG seed value 2                                                  |
| `--use-random-seeds` | *(off)* | Use random seeds instead of `--seed1`/`--seed2`                    |
| `--out`              | `.`     | Output directory for HTML reports                                  |
| `--json`             | `""`    | Also write deterministic run state to this JSON path               |

Galaxy and template generation share a single PRNG stream, so a given `(seed1, seed2)` pair reproduces the entire run byte-for-byte. Template acceptance uses the viability window `(53, 57)` exclusive; for each planet count, candidate stars (those with exactly that many planets) are iterated in `Star.ID` order with no separate attempt cap. If no candidate yields a viable template for a planet count, the corresponding `home-system-{N}.html` contains an "unavailable" report and the command continues.

## game

Parent command for managing games via the API. All subcommands require an active session (run `drynn login` first) and an admin account on the server.

```
drynn game <command> [flags]
```

| Subcommand | Description                              |
|------------|------------------------------------------|
| `create`   | create a new game from a config file     |
| `list`     | list all games                           |
| `show`     | show details of a game                   |
| `update`   | update a game (not yet implemented)      |
| `delete`   | delete a game                            |

All subcommands inherit `--server` and `--config` from the top-level server URL resolution rules. Responses from the server are printed to stdout verbatim (JSON). Errors returned by the server are surfaced using the `error` field from the response body.

### game create

Creates a game by POSTing the contents of a JSON config file to `/api/v1/games`.

```
drynn game create --file <path> [flags]
```

| Flag     | Default | Description                                 |
|----------|---------|---------------------------------------------|
| `--file` | *(required)* | Path to a JSON config file             |
| `--server` | `""`  | Server URL                                  |
| `--config` | `""`  | Path to the server config file              |

The file body is forwarded verbatim — the CLI does not decode it. This preserves forward compatibility: unknown fields added to the config schema (seeds, world-gen inputs) flow through without CLI changes. JSON validity is checked client-side before sending.

On success (HTTP 201), the response body (`{"id":N}`) is printed.

### game list

Lists all games.

```
drynn game list [flags]
```

Prints the JSON array returned by `GET /api/v1/games`.

### game show

Fetches a single game by ID.

```
drynn game show --id <id> [flags]
```

| Flag    | Default | Description          |
|---------|---------|----------------------|
| `--id`  | *(required)* | Game ID        |

Prints the JSON object returned by `GET /api/v1/games/:id`.

### game update

Reserved for future use. Currently the server returns `501 not yet implemented` and the CLI surfaces that message as an error.

```
drynn game update --id <id> [flags]
```

### game delete

Deletes a game by ID.

```
drynn game delete --id <id> [flags]
```

| Flag    | Default | Description          |
|---------|---------|----------------------|
| `--id`  | *(required)* | Game ID        |

On success (HTTP 204), prints `deleted`.
