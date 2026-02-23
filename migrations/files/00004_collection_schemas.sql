-- +goose Up
CREATE TABLE IF NOT EXISTS collection_schemas (
    tenant_id   TEXT NOT NULL,
    collection  TEXT NOT NULL,
    schema_json TEXT NOT NULL CHECK (json_valid(schema_json)),
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL,
    PRIMARY KEY (tenant_id, collection)
);

-- +goose Down
DROP TABLE IF EXISTS collection_schemas;
