# Phase 2 — Event Envelope and Versioning

## Scope

Standardize event contract and schema evolution behavior.

## Acceptance criteria

- [ ] Envelope fields are consistently emitted:
  - `event_id`, `event_type`, `schema_version`
  - `tenant_id`, `aggregate_type`, `aggregate_id`, `aggregate_version`
  - `occurred_at`, `correlation_id`, `causation_id`, `actor`, `source`
  - `payload`
- [ ] `schema_version` matches current codec version.
- [ ] Upcaster chain behavior is deterministic.

## Required tests

- [ ] Event codec normalization tests.
- [ ] Missing-upcaster error test.
- [ ] Envelope JSON marshal/unmarshal fixture test.

## Verification commands

```bash
go test ./internal/core/usecase
go test ./...
```

## CI gate

- [ ] PR fails on event codec test failures.
