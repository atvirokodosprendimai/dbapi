# ADR 0001: Event-Ready Hybrid Model

## Status

Accepted

## Context

The service is currently a tenant-scoped JSON API over SQLite with hexagonal architecture. It supports KV and collection record endpoints used by automation systems such as n8n.

The project needs strong auditability and integration events without immediately adopting full event sourcing complexity.

## Decision

Adopt an event-ready hybrid model:

1. Keep state tables (`kv_entries`) as the authoritative online write/read model.
2. For each successful mutation, write state change + immutable audit event + outbox event atomically in one transaction.
3. Dispatch outbox events asynchronously to external integrations (webhook/NATS in later adapters).
4. Keep event envelope versioned and replay-friendly for future projection rebuild and eventual event-sourcing migration.

## Event Envelope Contract

Canonical fields:

- `event_id`: globally unique ID for deduplication.
- `event_type`: e.g. `record.created`, `record.updated`, `record.deleted`.
- `schema_version`: integer event contract version (starts at `1`).
- `tenant_id`: tenant scope.
- `aggregate_type`: currently collection name.
- `aggregate_id`: record ID.
- `aggregate_version`: monotonically increasing per aggregate in event store scope.
- `occurred_at`: RFC3339 UTC timestamp.
- `correlation_id`: request/workflow correlation.
- `causation_id`: causation chain id if available.
- `actor`: API key/client actor.
- `source`: source system (`api`, `n8n`, etc).
- `payload`: event-specific data.

## Delivery Guarantees

- Outbox dispatch uses at-least-once semantics.
- Consumers must deduplicate by `event_id`.
- Retry with backoff for transient failures.

## Security and Isolation

- Audit and outbox rows are tenant-scoped.
- Audit events are append-only and immutable at database level.
- Audit query API enforces tenant scoping from authenticated API key.

## Consequences

Positive:

- Better compliance and forensics.
- Reliable n8n integration path.
- Smooth migration runway toward full event sourcing.

Trade-offs:

- More write-path complexity.
- Additional operational component (outbox dispatcher).
