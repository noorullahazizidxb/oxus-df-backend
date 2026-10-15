-- File: db/queries/auth.sql
-- Usage: SQL for admin users. sqlc generates Go functions from the "-- name:" lines.

-- name: GetAdminUserByEmail :one
SELECT * FROM admin_users WHERE email = $1;

-- name: CountAdminUsers :one
SELECT count(*) FROM admin_users;

-- name: CreateAdminUser :one
INSERT INTO admin_users (email, password_hash, full_name, role)
VALUES ($1, $2, $3, $4)
RETURNING *;
