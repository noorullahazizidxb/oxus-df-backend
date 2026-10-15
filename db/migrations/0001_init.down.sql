-- File: db/migrations/0001_init.down.sql
-- Usage: Undoes 0001_init.up.sql (drops admin_users and audit_logs).

DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS admin_users;
