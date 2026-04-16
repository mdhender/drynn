-- name: CreateUser :one
INSERT INTO users (
    handle,
    email,
    password_hash,
    is_active
) VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING *;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = $1;

-- name: ListUsers :many
SELECT *
FROM users
ORDER BY created_at DESC;

-- name: GetUserForAuthByEmail :one
SELECT *
FROM users u
WHERE u.email = $1
;

-- name: UpdateUserProfile :one
UPDATE users
SET
    handle = $2,
    email = $3
WHERE id = $1
RETURNING *;

-- name: AdminUpdateUser :one
UPDATE users
SET
    handle = $2,
    email = $3,
    is_active = $4
WHERE id = $1
RETURNING *;

-- name: SetUserPassword :exec
UPDATE users
SET password_hash = $2
WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;

-- name: ListRoles :many
SELECT id, name, description
FROM roles
ORDER BY name;

-- name: ListRoleNamesByUser :many
SELECT r.name
FROM roles r
JOIN user_roles ur ON ur.role_id = r.id
WHERE ur.user_id = $1
ORDER BY r.name;

-- name: RemoveAllRolesFromUser :exec
DELETE FROM user_roles
WHERE user_id = $1;

-- name: AddRoleToUser :exec
INSERT INTO user_roles (user_id, role_id)
SELECT $1, r.id
FROM roles r
WHERE r.name = $2
ON CONFLICT (user_id, role_id) DO NOTHING;

-- name: CountUsersByRole :one
SELECT COUNT(*)
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
WHERE r.name = $1;
