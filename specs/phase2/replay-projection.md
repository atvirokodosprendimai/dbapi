# Phase 2 — Replay and Projection Correctness

## Scope

Guarantee deterministic replay behavior from immutable events into query-ready projections.

## Acceptance criteria

- [ ] Replay consumes events in stable order (`occurred_at`, then `event_id`).
- [ ] Replay is idempotent when run repeatedly over the same event range.
- [ ] Projection builders are pure from input event to output state (no hidden side effects).
- [ ] Corrupt or unsupported historical event payloads fail with explicit typed errors.
- [ ] Tenant boundaries are preserved during replay and projection rebuilds.

## Required tests

- [ ] Replay idempotency test with duplicate run assertions.
- [ ] Cross-tenant replay isolation test.
- [ ] Unsupported schema version replay failure test.
- [ ] Projection fixture tests for create/update/delete event sequences.

## Verification commands

```bash
go test ./internal/core/usecase
go test ./...
```

## CI gate

- [ ] PR fails if replay or projection correctness tests fail.
