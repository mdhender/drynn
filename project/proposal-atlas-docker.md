# Replace Dedicated Postgres Diff Instance with Ephemeral Atlas Dev DB

## Overview

This task removes the need for a permanently running local Postgres instance used for Atlas schema diffs and replaces it with an **ephemeral, Docker-backed dev database**.

This aligns with:

* Simpler local setup
* Lower resource usage
* Cleaner, reproducible migrations
* Better parity with CI workflows

---

## Goals

* Eliminate dedicated Postgres instance used for Atlas diffs
* Use ephemeral Postgres containers for schema diffing
* Keep developer workflow simple and deterministic
* Maintain compatibility with existing migration structure

---

## Non-Goals

* Do not change production database configuration
* Do not modify application runtime database (SQLite remains unchanged)
* Do not couple Atlas to Testcontainers-Go

---

## Key Concept

Atlas requires a **dev database** to:

* Load schema state
* Execute migrations
* Compute diffs

This database:

* Must be **empty**
* Should be **temporary**
* Does not need to persist

---

## Solution

Use Atlas’s built-in Docker integration:

```bash
--dev-url "docker://postgres/15/dev?search_path=public"
```

This instructs Atlas to:

* Start a temporary Postgres container
* Run the diff operation
* Discard the container afterward

---

## Configuration

### atlas.hcl

```hcl
env "local" {
  dev = "docker://postgres/15/dev?search_path=public"
}
```

---

## Usage

### Generate Migration

```bash
atlas migrate diff add_users --env local
```

### Apply Migrations (unchanged)

```bash
atlas migrate apply --url "sqlite://file.db"
```

---

## Architecture Separation

### Atlas (Schema Management)

* Uses `docker://postgres/...`
* Ephemeral container per command
* No dependency on application runtime

### Testcontainers-Go (Testing)

* Used only in Go integration tests
* Starts containers programmatically
* Independent from Atlas

---

## Why Not Use Testcontainers for Atlas?

Testcontainers-Go:

* Requires Go runtime
* Designed for test lifecycle

Atlas:

* CLI-driven
* Needs a database URL, not a container API

While it is technically possible to point Atlas at a Testcontainers instance, it adds unnecessary complexity and provides no benefit over `docker://`.

---

## Benefits

### Simplicity

* No local Postgres setup required
* No ports, credentials, or manual cleanup

### Consistency

* Every diff runs against a clean database
* Eliminates drift and hidden state

### Reliability

* Matches CI/CD behavior
* Reduces “works on my machine” issues

### Resource Efficiency

* No idle database process
* Containers only exist during command execution

---

## Requirements

* Docker Desktop (or compatible runtime) must be running

---

## Limitations

### Docker-in-Docker

If Atlas is executed inside a container:

* `docker://` URLs will not work unless Docker is accessible inside that container

For local Mac development:

* No issue (Docker Desktop handles this)

---

## Common Pitfalls

### Non-Clean Database

Atlas requires an empty dev database.

Do NOT:

* Reuse a running Postgres instance
* Point `--dev-url` at a persistent DB

Failure symptom:

```
Error: connected database is not clean
```

---

### Docker Not Running

Failure symptom:

* Atlas cannot connect to Docker
* Diff command fails immediately

---

## Implementation Checklist

* [ ] Remove local/dedicated Postgres diff instance
* [ ] Add `env "local"` to `atlas.hcl`
* [ ] Update developer docs to use `--env local`
* [ ] Verify Docker is installed and running
* [ ] Run test diff to confirm functionality
* [ ] Ensure CI environment supports Docker

---

## Acceptance Criteria

* Developers can run `atlas migrate diff` without local Postgres
* No persistent Postgres instance is required
* Migrations are generated correctly
* Workflow is documented and repeatable

---

## Future Enhancements (Optional)

* Pin Postgres version (e.g., 15 vs 16) for deterministic diffs
* Add Makefile targets for common Atlas commands
* Integrate into CI pipeline for automated validation

---

## Summary

This change replaces a persistent, manually managed Postgres instance with a **clean, ephemeral container per Atlas operation**.

Result:

* Simpler setup
* Cleaner diffs
* Better alignment with modern container-based workflows
