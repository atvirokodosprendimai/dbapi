# Phase 2 — Outbox Dispatch Reliability

## Scope

Define delivery and recovery guarantees for asynchronous outbox dispatch.

## Acceptance criteria

- [ ] Dispatcher claims pending rows in bounded batches.
- [ ] Publish success marks outbox rows dispatched with timestamp metadata.
- [ ] Publish failures trigger retry with bounded backoff and attempt tracking.
- [ ] Poison rows are quarantined after retry budget exhaustion (no infinite hot loop).
- [ ] Dispatcher restart resumes safely without duplicating already-marked dispatches.

## Required tests

- [ ] Success-path test for claim, publish, and mark-dispatched flow.
- [ ] Retry behavior test for transient publisher failures.
- [ ] Quarantine/dead-letter test after max attempts.
- [ ] Restart-resume test proving at-least-once semantics with idempotent marks.

## Verification commands

```bash
go test ./internal/core/usecase ./internal/adapters/sqlite
go test ./...
```

## CI gate

- [ ] PR fails if outbox dispatcher reliability tests fail.
