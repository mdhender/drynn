# How to run tests

## Run short tests (no Docker required)

```bash
go test -short ./...
```

This runs unit tests only — normalizers, password hashing, viewer roles, request helpers, and similar pure-logic functions. It finishes in under two seconds and needs nothing beyond Go itself.

## Run all tests including integration (Docker required)

```bash
go test -count=1 -timeout 300s ./...
```

This runs the full suite: unit tests, service tests against a real Postgres instance, handler tests through the HTTP layer, and the end-to-end smoke test. Docker must be running — testcontainers starts a `postgres:16-alpine` container automatically and tears it down when the binary exits.

The `-count=1` flag disables test caching so integration tests always exercise a fresh database. The `-timeout 300s` flag gives the container time to start on first run.

## Run tests for a single package

```bash
go test ./internal/service/...          # short tests only
go test -count=1 ./internal/service/... # includes integration tests
```

## Run a specific test by name

```bash
go test -run TestUserService_Register -count=1 ./internal/service/...
```

## Troubleshooting

**"rootless Docker not found" or "failed to create Docker provider"**
Docker Desktop (or the Docker daemon) is not running. Start it, then retry. Short tests (`-short`) skip these automatically.

**Tests hang or time out**
The Postgres container image may be downloading for the first time. Subsequent runs reuse the cached image. Increase the timeout if needed: `-timeout 600s`.

**"port already in use"**
Another testcontainers run may not have cleaned up. Run `docker ps` to check for orphaned `postgres:16-alpine` containers and remove them with `docker rm -f <id>`.
