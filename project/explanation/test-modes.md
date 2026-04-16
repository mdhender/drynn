# Short tests vs. integration tests

The test suite has two modes controlled by Go's `-short` flag. The split exists because some tests need a running Docker daemon and a real Postgres instance, which not every environment has and which add seconds of startup cost. The `-short` flag lets developers get fast feedback on pure logic without waiting for infrastructure.

## What short mode runs

Short mode (`go test -short ./...`) runs tests that have no external dependencies. These cover normalizers, validators, password hashing, viewer role checks, URL helpers, template data mapping, and the `serviceMessage` error-to-string translation table. They verify that business rules encoded in Go functions behave correctly.

Short mode is what CI runs in the **test-short** job. It catches logic regressions in under two seconds with zero infrastructure.

## What full mode adds

Full mode (`go test ./...`) runs everything short mode does, plus:

- **Service tests** exercise every exported method on `UserService`, `InvitationService`, and `PasswordResetService` against a real Postgres database. They verify that SQL queries, transactions, constraint violations, and role assignments work end-to-end through the sqlc-generated code.

- **Handler tests** send HTTP requests through the full echo router (with real JWT authentication, middleware, and a recording template renderer) and check status codes, redirect locations, flash messages, and which template was rendered.

- **The smoke test** walks through a complete user lifecycle — register via invitation, sign out, sign in as admin, create a user via the admin panel, sign out, then sign in as the newly-created user — carrying cookies between requests like a browser would.

All of these use testcontainers to start a single shared `postgres:16-alpine` container per test binary. Each test gets its own database cloned from a migrated template, so tests are isolated and can run in parallel.

## When to use which

Use **short** when you are iterating on logic that doesn't touch the database — editing a normalizer, adjusting a role check, changing how an error maps to a user-facing message. The feedback loop is instant.

Use **full** when you change anything that interacts with Postgres (queries, migrations, service methods), modify handler behavior (routes, redirects, form processing), or touch the auth/middleware chain. These tests catch problems that pure-logic tests cannot: constraint violations, transaction rollback behavior, cookie propagation, and middleware ordering.

CI runs both: short tests gate every PR cheaply, and the integration job catches database and HTTP regressions before merge.
