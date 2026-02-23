# GitHub Copilot Instructions for `dbapi`

Apply Specification-Driven Development (SDD) for all non-trivial changes.

## SDD workflow (required)

1. Clarify spec first.
2. Implement the smallest verifiable slice.
3. Run tools and use outputs as ground truth.
4. Iterate until acceptance criteria pass.
5. Update docs/spec tracking only after behavior is verified.

Treat generated code as a draft; deterministic tools decide correctness.

## Spec format to produce before coding

Use this checklist in planning comments/PR body/work notes:

- User story
- Acceptance criteria
- Non-goals
- Constraints (runtime, security, performance)
- Edge cases (at least 3)
- Verification commands

## Project architecture expectations

- Language/runtime: Go
- Storage: SQLite via GORM + Goose migrations
- API style: JSON HTTP API, tenant scoped by API key
- Architecture: hexagonal adapters/usecases/domain; CQRS-inspired sqlite access (`ReadTX`/`WriteTX`)
- Event model: state + immutable audit events + outbox

## Repository conventions

- Prefer existing abstractions and naming before introducing new ones.
- Keep diffs small; avoid mixing broad refactors with feature fixes.
- Add tests with behavior changes.
- Avoid placeholder comments/TODO code unless explicitly requested.

## Verification commands

Run relevant focused tests, then full suite:

```bash
go test ./internal/adapters/httpapi ./internal/adapters/sqlite ./internal/core/usecase
go test ./...
```

For docs-only changes, still ensure examples are syntactically valid and match current endpoints.

## SDD artifacts in this repo

Use and keep aligned:

- `specs/README.md`
- `specs/acceptance-matrix.md`
- `specs/implementation-plan.md`
- `specs/phase1/*.md`
- `specs/phase2/*.md`
- `specs/phase3/*.md`

When implementing uncovered acceptance items, update the matrix status in the same PR.

## Security and reliability rules

- Never bypass tenant isolation derived from auth context.
- Preserve idempotency semantics for bulk operations.
- Keep audit rows immutable and transactional guarantees intact.
- Do not leak secrets (API keys, bearer tokens) in logs.

## Output quality bar

Before finalizing, ensure:

- Tests pass.
- Error handling maps to existing API taxonomy (`400/401/404/500`).
- New behavior is covered by tests and documented where needed.
- Changes are explainable in terms of acceptance criteria, not only implementation detail.
