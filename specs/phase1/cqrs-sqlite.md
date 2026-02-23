# Phase 1 — CQRS SQLite Contract

## Scope

Lock the single-writer/multi-reader behavior for SQLite through explicit transaction lanes.

## Acceptance criteria

- [ ] All read-only repository operations use `ReadTX`.
- [ ] All mutating repository operations use `WriteTX`.
- [ ] Reader and writer connections are separate and role-specific.
- [ ] PRAGMAs are applied per connection (DSN `_pragma`), including:
  - WAL
  - synchronous mode
  - foreign keys
  - busy timeout
  - trusted schema
  - `query_only=1` for reader, `query_only=0` for writer

## Required tests

- [ ] Unit test for generated DSN pragma set.
- [ ] Repo tests proving read and write paths still function under CQRS wrapper.

## Verification commands

```bash
go test ./internal/adapters/sqlite/... ./internal/app
go test ./...
```

## CI gate

- [ ] PR fails if CQRS adapter or sqlite repo tests fail.
