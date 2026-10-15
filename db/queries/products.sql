-- File: db/queries/products.sql
-- Usage: SQL for products (create, get, list, count, export, update, soft delete). sqlc generates Go from it.

-- name: CreateProduct :one
INSERT INTO products (code, name, description, currency, min_amount, max_amount, annual_rate, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetProduct :one
SELECT * FROM products WHERE id = $1 AND deleted_at IS NULL;

-- name: ListProducts :many
SELECT * FROM products
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('search')::text IS NULL
       OR strpos(lower(name || ' ' || code), lower(sqlc.narg('search')::text)) > 0)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('page_limit') OFFSET sqlc.arg('page_offset');

-- name: CountProducts :one
SELECT count(*) FROM products
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('search')::text IS NULL
       OR strpos(lower(name || ' ' || code), lower(sqlc.narg('search')::text)) > 0);

-- name: ListProductsForExport :many
SELECT * FROM products WHERE deleted_at IS NULL ORDER BY created_at, id;

-- name: UpdateProduct :one
UPDATE products
SET name = $2,
    description = $3,
    min_amount = $4,
    max_amount = $5,
    annual_rate = $6,
    status = $7,
    updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteProduct :execrows
UPDATE products SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;
