-- File: db/migrations/0002_products.up.sql
-- Usage: Creates the products table with constraints and indexes.

CREATE TABLE products (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        VARCHAR(32)   NOT NULL,
    name        VARCHAR(120)  NOT NULL,
    description TEXT          NOT NULL DEFAULT '',
    currency    VARCHAR(3)    NOT NULL,
    min_amount  NUMERIC(18,2) NOT NULL,
    max_amount  NUMERIC(18,2) NOT NULL,
    annual_rate NUMERIC(5,2)  NOT NULL,
    status      VARCHAR(16)   NOT NULL DEFAULT 'draft',
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    CONSTRAINT products_status_check CHECK (status IN ('draft', 'active', 'archived')),
    CONSTRAINT products_amount_check CHECK (min_amount > 0 AND max_amount >= min_amount),
    CONSTRAINT products_rate_check CHECK (annual_rate >= 0 AND annual_rate <= 100)
);

-- A code can be reused once the old product is soft-deleted.
CREATE UNIQUE INDEX products_code_key ON products (code) WHERE deleted_at IS NULL;
CREATE INDEX products_status_idx ON products (status) WHERE deleted_at IS NULL;
CREATE INDEX products_created_at_idx ON products (created_at DESC) WHERE deleted_at IS NULL;
