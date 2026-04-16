---
title: Roles
weight: 40
---

Roles control access to routes and features. They are stored in the
database and loaded on every request.

## Available roles

| Name | Description | Capabilities |
|------|-------------|--------------|
| `user` | Authenticated user | Access `/app/profile`, edit own profile |
| `admin` | Administrator | Access `/app/admin/*`, manage users and invitations |
| `tester` | Seeded test account | Same as `user`; identifies test accounts |

## Assignment rules

- **Registration:** new users receive the `user` role.
- **`seed-admin`:** assigns `user` + `admin`.
- **`seed-testers`:** assigns `user` + `tester`.
- **Admin UI:** admins can set any role combination through the
  edit-user form.
- **Normalization:** `user` is always included regardless of input.
  Unknown role names are silently dropped.

## Role checking

Roles are **not** embedded in JWT tokens. They are loaded from the
database on each request by the `loadCurrentViewer` middleware, so
role changes take effect immediately without requiring the user to
sign out.

In Go code, use `viewer.HasRole("admin")`. In templates, use
`{{if hasRole .CurrentUser.Roles "admin"}}`.

## Route enforcement

The `requireRole` middleware checks the viewer's roles and returns
403 Forbidden if the required role is missing. It is applied to
route groups:

```go
adminGroup := appGroup.Group("/admin")
adminGroup.Use(requireRole(service.RoleAdmin))
```

## Invariants

- At least one active user must hold the `admin` role at all times.
- The last admin cannot be deactivated, have admin removed, or be
  deleted.
- Users cannot delete their own account.

## Database schema

**`roles` table:**

| Column | Type | Description |
|--------|------|-------------|
| `id` | BIGSERIAL | Primary key |
| `name` | TEXT (unique) | Role name |
| `description` | TEXT | Human-readable description |

**`user_roles` table:**

| Column | Type | Description |
|--------|------|-------------|
| `user_id` | UUID | FK to `users(id)`, cascade delete |
| `role_id` | BIGINT | FK to `roles(id)`, cascade delete |
| `created_at` | TIMESTAMPTZ | Assignment timestamp |

Primary key: `(user_id, role_id)`.

## Go constants

```go
const (
    RoleUser   = "user"
    RoleAdmin  = "admin"
    RoleTester = "tester"
)
```

Defined in `internal/service/user_service.go`.
