---
title: Database
weight: 50
---

PostgreSQL database managed with Atlas (migrations) and sqlc
(query generation). Schema source of truth: `db/schema.sql`.

## Tables

### roles

Available roles in the system.

| Column | Type | Constraints |
|--------|------|-------------|
| `id` | BIGSERIAL | PRIMARY KEY |
| `name` | TEXT | UNIQUE, NOT NULL |
| `description` | TEXT | NOT NULL, DEFAULT `''` |

Seeded with: `user`, `admin`, `tester`.

### users

User accounts.

| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PRIMARY KEY, DEFAULT `gen_random_uuid()` |
| `handle` | TEXT | UNIQUE, NOT NULL |
| `email` | TEXT | UNIQUE, NOT NULL |
| `password_hash` | TEXT | NOT NULL |
| `is_active` | BOOLEAN | NOT NULL, DEFAULT `TRUE` |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT `NOW()` |
| `updated_at` | TIMESTAMPTZ | NOT NULL, DEFAULT `NOW()` |

**Check constraints:**

| Name | Rule |
|------|------|
| `users_handle_lowercase` | `handle = lower(handle)` |
| `users_email_lowercase` | `email = lower(email)` |
| `users_handle_format` | `handle ~ '^[a-z0-9_]+$'` |
| `users_handle_length` | 3–32 characters |
| `users_email_length` | 3–320 characters |

**Trigger:** `users_set_updated_at` — auto-updates `updated_at` on
row update.

### jwt_signing_keys

Cryptographic keys for JWT signing and verification.

| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PRIMARY KEY, DEFAULT `gen_random_uuid()` |
| `token_type` | TEXT | NOT NULL, CHECK IN (`access`, `refresh`) |
| `algorithm` | TEXT | NOT NULL, DEFAULT `HS256` |
| `secret` | BYTEA | NOT NULL |
| `state` | TEXT | NOT NULL, DEFAULT `active`, CHECK IN (`active`, `retired`, `revoked`) |
| `verify_until` | TIMESTAMPTZ | nullable |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT `NOW()` |
| `updated_at` | TIMESTAMPTZ | NOT NULL, DEFAULT `NOW()` |

**Unique index:** `jwt_signing_keys_active_token_type_idx` — one
active key per token type.

**Trigger:** `jwt_signing_keys_set_updated_at`.

### user_roles

Many-to-many junction between users and roles.

| Column | Type | Constraints |
|--------|------|-------------|
| `user_id` | UUID | FK `users(id)` ON DELETE CASCADE |
| `role_id` | BIGINT | FK `roles(id)` ON DELETE CASCADE |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT `NOW()` |

Primary key: `(user_id, role_id)`.
Index: `user_roles_role_id_idx` on `role_id`.

### invitations

Invitation codes for user registration.

| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PRIMARY KEY, DEFAULT `gen_random_uuid()` |
| `email` | TEXT | NOT NULL, lowercase, 3–320 chars |
| `code` | TEXT | UNIQUE, NOT NULL |
| `invited_by` | UUID | FK `users(id)` ON DELETE CASCADE |
| `used_by` | UUID | FK `users(id)` ON DELETE SET NULL, nullable |
| `used_at` | TIMESTAMPTZ | nullable |
| `expires_at` | TIMESTAMPTZ | NOT NULL |
| `archived_at` | TIMESTAMPTZ | nullable |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT `NOW()` |

Indexes: `invitations_code_idx`, `invitations_email_idx`.

### password_reset_tokens

One-time tokens for the password reset flow.

| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PRIMARY KEY, DEFAULT `gen_random_uuid()` |
| `user_id` | UUID | FK `users(id)` ON DELETE CASCADE |
| `code` | TEXT | UNIQUE, NOT NULL |
| `expires_at` | TIMESTAMPTZ | NOT NULL |
| `used_at` | TIMESTAMPTZ | nullable |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT `NOW()` |

Indexes: `password_reset_tokens_code_idx`,
`password_reset_tokens_user_id_idx`.

## Extensions

- `pgcrypto` — provides `gen_random_uuid()`.

## Named queries

Queries are in `db/queries/*.sql`. Generated Go code is in
`db/sqlc/`.

### users.sql

| Query | Type | Description |
|-------|------|-------------|
| `CreateUser` | `:one` | Create user, return row |
| `GetUserByEmail` | `:one` | Fetch by email |
| `GetUserByID` | `:one` | Fetch by UUID |
| `GetUserForAuthByEmail` | `:one` | Fetch for authentication |
| `ListUsers` | `:many` | All users, newest first |
| `UpdateUserProfile` | `:one` | Update handle and email |
| `AdminUpdateUser` | `:one` | Update handle, email, is_active |
| `SetUserPassword` | `:exec` | Update password hash |
| `DeleteUser` | `:exec` | Delete user |
| `ListRoles` | `:many` | All roles |
| `ListRoleNamesByUser` | `:many` | Role names for a user |
| `RemoveAllRolesFromUser` | `:exec` | Clear roles |
| `AddRoleToUser` | `:exec` | Assign role by name |
| `CountUsersByRole` | `:one` | Count users with a role |

### jwt_signing_keys.sql

| Query | Type | Description |
|-------|------|-------------|
| `CreateJWTSigningKey` | `:one` | Create key, return row |
| `GetActiveJWTSigningKey` | `:one` | Active key for token type |
| `GetActiveJWTSigningKeyForUpdate` | `:one` | With row lock |
| `GetJWTSigningKeyByID` | `:one` | Fetch by UUID |
| `GetJWTSigningKeyByIDForUpdate` | `:one` | With row lock |
| `RetireJWTSigningKey` | `:one` | Set state to retired |
| `DeleteJWTSigningKey` | `:exec` | Permanently delete |

### invitations.sql

| Query | Type | Description |
|-------|------|-------------|
| `CreateInvitation` | `:one` | Create invitation |
| `GetInvitationByID` | `:one` | Fetch by UUID |
| `GetInvitationByCode` | `:one` | Fetch by code |
| `ListInvitations` | `:many` | Filtered list (excludes archived) |
| `MarkInvitationUsed` | `:exec` | Mark as redeemed |
| `UpdateInvitationExpiry` | `:exec` | Change expiration |
| `ArchiveInvitation` | `:exec` | Soft-delete |

### password_reset_tokens.sql

| Query | Type | Description |
|-------|------|-------------|
| `CreatePasswordResetToken` | `:one` | Create token |
| `GetPasswordResetTokenByCode` | `:one` | Fetch by code |
| `ListActivePasswordResetTokensByUserID` | `:many` | Unexpired, unused tokens |
| `MarkPasswordResetTokenUsed` | `:exec` | Mark as used |
| `DeletePasswordResetTokensByUserID` | `:exec` | Purge all for user |

## Type mappings (sqlc)

| Database type | Go type |
|---------------|---------|
| `uuid` | `github.com/google/uuid.UUID` |
| `pg_catalog.timestamptz` | `time.Time` |

## Tooling

- **Atlas** — schema diffing and migrations. Config: `atlas.hcl`.
  Migrations: `db/migrations/`.
- **sqlc** — query code generation. Config: `sqlc.yaml`.
  Output: `db/sqlc/` (never hand-edit).
