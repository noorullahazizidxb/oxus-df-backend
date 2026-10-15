-- File: db/queries/audit.sql
-- Usage: SQL for the audit trail. sqlc generates Go functions from the "-- name:" lines.

-- name: InsertAuditLog :exec
INSERT INTO audit_logs (actor_id, action, entity_type, entity_id, request_id, client_ip, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7);
