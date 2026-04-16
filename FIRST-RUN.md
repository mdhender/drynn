# First-Run Setup

Two different scenarios, pick the one that fits:

- **[Scenario A — Fresh database instance](#scenario-a--fresh-database-instance)**: you just cloned the repo and your PostgreSQL server has no drynn databases yet. You need to create roles/databases, apply migrations from empty, mint JWT keys, and seed the admin.
- **[Scenario B — New Gitkraken worktree against an existing DB](#scenario-b--new-gitkraken-worktree-against-an-existing-db)**: your primary checkout is already running against an initialized `drynn_dev`. Gitkraken has spun up a new worktree under `drynn.worktrees/<branch-slug>/` and your shell is inside it. The database is fine — you only need to repopulate the gitignored files this worktree didn't inherit.

If your `pwd` is under `…/drynn.worktrees/…`, you're almost certainly in Scenario B.

Assumes PostgreSQL, Go, Atlas, sqlc, and Hugo are already installed.

---

## Scenario A — Fresh database instance

### 1. Provision roles and databases

See `cmd/db/README.md` for the full SQL. You need:

- `drynn_dev_user` owning database `drynn_dev`
- `drynn_atlas_user` owning database `drynn_atlas` (Atlas scratch DB for diff computation)

### 2. Create config and env files

```bash
cp .env.example .env.development.local   # then edit credentials
go run ./cmd/db init-config --config data/server.json \
  --database-url "postgres://drynn_dev_user:<password>@localhost:5432/drynn_dev?sslmode=disable"
```

### 3. Build the embedded marketing site

`sitefs.go` has `//go:embed web/sitepublic`. If the directory is missing, every `go build` / `go run` fails with `pattern web/sitepublic: no matching files found`.

```bash
make docs
```

### 4. Apply migrations

The `drynn_dev` schema exists before Atlas runs, so `--allow-dirty` is required. Revisions bookkeeping goes to its own schema.

```bash
atlas migrate apply \
  --allow-dirty \
  --dir file://db/migrations \
  --revisions-schema atlas_schema_revisions \
  --url "postgres://drynn_dev_user:<password>@localhost:5432/drynn_dev?sslmode=disable"
```

### 5. Mint JWT keys and seed admin

```bash
go run ./cmd/db jwt-key create --config data/server.json --type access
go run ./cmd/db jwt-key create --config data/server.json --type refresh
go run ./cmd/db seed-admin --config data/server.json \
  --handle admin --email admin@example.test --password '<password>'
```

### 6. Start the server

```bash
go run ./cmd/server -config data/server.json
```

---

## Scenario B — New Gitkraken worktree against an existing DB

Gitkraken's worktree creation copies **only tracked files**. Anything in `.gitignore` — including your env files, `data/server.json`, and the built `web/sitepublic/` — is absent in the new worktree. The databases themselves are untouched and already initialized, so there are **no migrations to apply, no keys to mint, no admin to seed**.

### 1. Copy the gitignored files from your primary checkout

```bash
# from the new worktree (e.g. drynn.worktrees/<branch-slug>/)
PRIMARY=/path/to/primary/drynn

cp "$PRIMARY"/.env* .
mkdir -p data && cp "$PRIMARY/data/server.json" data/server.json
```

### 2. Rebuild the embedded marketing site

```bash
make docs
```

This populates `web/sitepublic/` so Go builds succeed.

### 3. Run

```bash
go run ./cmd/server -config data/server.json
```

That's it — the existing `drynn_dev` already has the schema, keys, and admin from the primary setup.

---

## Configuration path mismatch

Your `.env.development.local` (derived from `.env.example`) typically sets:

```
DRYNN_CONFIG_PATH=data/var/drynn/server.json
```

If you keep `server.json` at `data/server.json` instead, either:

- move the file: `mkdir -p data/var/drynn && mv data/server.json data/var/drynn/server.json`, or
- update `.env.development.local` to match: `DRYNN_CONFIG_PATH=data/server.json`

Otherwise every `cmd/db` and `cmd/server` invocation needs an explicit `--config data/server.json` flag.
