---
title: Add a new role
weight: 30
---

This guide walks through adding a custom role to Drynn. Roles
control access to routes and features. The app ships with three
roles — `user`, `admin`, and `tester` — but you can add more.

## 1. Add the role to the schema

Insert the new role in `db/schema.sql` alongside the existing seed
rows:

```sql
INSERT INTO roles (name, description)
VALUES
    ('user', 'Authenticated user'),
    ('admin', 'Administrator'),
    ('tester', 'Seeded test account'),
    ('moderator', 'Content moderator');
```

## 2. Generate a migration

```sh
atlas migrate diff add_moderator_role --env local
```

Review the generated file in `db/migrations/`. It should contain a
single `INSERT` statement.

## 3. Apply the migration

```sh
atlas migrate apply --url "$DATABASE_URL"
```

## 4. Add a Go constant

Open `internal/service/user_service.go` and add a constant next to
the existing role names:

```go
const (
    RoleUser      = "user"
    RoleAdmin     = "admin"
    RoleTester    = "tester"
    RoleModerator = "moderator"
)
```

## 5. Update normalizeRoles

In the same file, find `normalizeRoles()` and add the new role to
both the `switch` and the ordering slice:

```go
func normalizeRoles(roles []string) []string {
    seen := map[string]struct{}{RoleUser: {}}
    for _, role := range roles {
        role = strings.ToLower(strings.TrimSpace(role))
        switch role {
        case RoleUser, RoleAdmin, RoleTester, RoleModerator:
            seen[role] = struct{}{}
        }
    }

    normalized := make([]string, 0, len(seen))
    for _, role := range []string{RoleUser, RoleAdmin, RoleTester, RoleModerator} {
        if _, ok := seen[role]; ok {
            normalized = append(normalized, role)
        }
    }

    return normalized
}
```

## 6. Make the role assignable in the admin UI

Open `internal/handler/admin.go` and update `selectedRoles()` to
include the new role:

```go
func selectedRoles(roleMap map[string]bool) []string {
    roles := make([]string, 0, len(roleMap))
    for _, role := range []string{service.RoleUser, service.RoleAdmin, service.RoleModerator} {
        if roleMap[role] {
            roles = append(roles, role)
        }
    }

    return roles
}
```

The admin user-edit form reads its checkbox list from the roles
table, so the new role appears automatically once the migration is
applied.

## 7. Guard routes (optional)

If you want routes that only moderators can access, create a new
route group in `internal/server/server.go`:

```go
modGroup := appGroup.Group("/mod")
modGroup.Use(requireRole(service.RoleModerator))
modGroup.GET("/queue", modHandler.ShowQueue)
```

## 8. Verify

```sh
go build ./...
go test ./...
```

Assign the new role to a user through the admin panel and confirm
the role appears on their profile.
