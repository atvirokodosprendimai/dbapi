-- +goose Up
CREATE TABLE IF NOT EXISTS audit_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id TEXT NOT NULL UNIQUE,
    schema_version INTEGER NOT NULL,
    tenant_id TEXT NOT NULL,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    aggregate_version INTEGER NOT NULL,
    action TEXT NOT NULL,
    actor TEXT NOT NULL,
    source TEXT NOT NULL,
    request_id TEXT NOT NULL,
    correlation_id TEXT NOT NULL,
    causation_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    before_json TEXT,
    after_json TEXT,
    changed_fields_json TEXT,
    occurred_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_events_tenant_occurred ON audit_events(tenant_id, occurred_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_tenant_aggregate ON audit_events(tenant_id, aggregate_type, aggregate_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_tenant_action ON audit_events(tenant_id, action, id DESC);

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_audit_events_no_update
BEFORE UPDATE ON audit_events
BEGIN
    SELECT RAISE(ABORT, 'audit_events are immutable');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_audit_events_no_delete
BEFORE DELETE ON audit_events
BEGIN
    SELECT RAISE(ABORT, 'audit_events are immutable');
END;
-- +goose StatementEnd

CREATE TABLE IF NOT EXISTS outbox_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id TEXT NOT NULL UNIQUE,
    tenant_id TEXT NOT NULL,
    topic TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    status TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at DATETIME NOT NULL,
    last_error TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    dispatched_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_status_next ON outbox_events(status, next_attempt_at, id);
CREATE INDEX IF NOT EXISTS idx_outbox_events_tenant_status ON outbox_events(tenant_id, status, id);

-- +goose Down
DROP INDEX IF EXISTS idx_outbox_events_tenant_status;
DROP INDEX IF EXISTS idx_outbox_events_status_next;
DROP TABLE IF EXISTS outbox_events;

DROP TRIGGER IF EXISTS trg_audit_events_no_delete;
DROP TRIGGER IF EXISTS trg_audit_events_no_update;
DROP INDEX IF EXISTS idx_audit_events_tenant_action;
DROP INDEX IF EXISTS idx_audit_events_tenant_aggregate;
DROP INDEX IF EXISTS idx_audit_events_tenant_occurred;
DROP TABLE IF EXISTS audit_events;
