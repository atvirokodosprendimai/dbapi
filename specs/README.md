# SDD Blueprint

This folder contains the project-specific SDD (Specification-Driven Development) blueprint for `dbapi`.

The process is split into 3 phases. Each phase has:

- mandatory specification files,
- executable acceptance criteria,
- verification commands,
- CI gate expectations.

## How to use

1. Pick one phase file and complete all unchecked criteria.
2. Encode every acceptance criterion in tests or scriptable checks.
3. Run verification commands locally.
4. Merge only when CI gates for that phase are satisfied.

Use `specs/acceptance-matrix.md` to track which criteria are already enforced by tests and which still need implementation.

## Verification baseline

```bash
go test ./...
```

## Phase map

- Phase 1: `specs/phase1/`
  - `api-core.md`
  - `cqrs-sqlite.md`
  - `audit-outbox.md`
- Phase 2: `specs/phase2/`
  - `event-envelope.md`
  - `replay-projection.md`
  - `outbox-dispatch.md`
- Phase 3: `specs/phase3/`
  - `security-hardening.md`
  - `observability-slo.md`
  - `release-governance.md`

All files above are mandatory for a complete blueprint.
