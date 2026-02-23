# Phase 1 — Audit and Outbox Atomicity

## Scope

Ensure mutation integrity: state + audit + outbox are written atomically.

## Acceptance criteria

- [ ] Record upsert/delete writes domain state and event side effects in one transaction.
- [ ] Audit rows are immutable in DB (no update/delete).
- [ ] Outbox rows are created for each successful mutation event.
- [ ] Failure in any side-effect step rolls back the transaction.

## Required tests

- [ ] Store-level tests for upsert and delete producing audit + outbox rows.
- [ ] Migration test ensuring audit immutability triggers exist.
- [ ] Error-path test confirming no partial commit.

## Verification commands

```bash
go test ./internal/adapters/sqlite ./internal/core/usecase
go test ./...
```

## CI gate

- [ ] PR fails if transactional mutation tests fail.
