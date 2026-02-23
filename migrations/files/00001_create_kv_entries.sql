-- +goose Up
CREATE TABLE IF NOT EXISTS kv_entries (
    key TEXT PRIMARY KEY,
    category TEXT NOT NULL,
    value TEXT NOT NULL CHECK (json_valid(value)),
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_kv_entries_category_key ON kv_entries(category, key);

-- +goose Down
DROP INDEX IF EXISTS idx_kv_entries_category_key;
DROP TABLE IF EXISTS kv_entries;
