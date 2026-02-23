-- +goose Up
CREATE TABLE IF NOT EXISTS api_keys (
    token_hash TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    name TEXT NOT NULL,
    active INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_api_keys_tenant_id ON api_keys(tenant_id);

CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id TEXT NOT NULL,
    collection TEXT NOT NULL,
    record_id TEXT NOT NULL,
    action TEXT NOT NULL,
    actor TEXT NOT NULL,
    at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_collection_at ON audit_logs(tenant_id, collection, at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_audit_logs_tenant_collection_at;
DROP TABLE IF EXISTS audit_logs;
DROP INDEX IF EXISTS idx_api_keys_tenant_id;
DROP TABLE IF EXISTS api_keys;
