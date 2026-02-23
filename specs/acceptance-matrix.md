# Acceptance Matrix

This checklist maps each spec acceptance area to executable verification in the current repository.

Legend:

- `[x]` covered by an existing test or enforced check
- `[ ]` planned and not yet fully enforced

## Phase 1

### API Core Contract (`specs/phase1/api-core.md`)

- [x] Auth and error mapping coverage in `internal/adapters/httpapi/handler_test.go`
- [x] Tenant isolation behavior in `internal/adapters/httpapi/handler_test.go`
- [x] Bulk idempotency checks in `internal/adapters/httpapi/handler_test.go`
- [x] Verification command: `go test ./internal/adapters/httpapi ./internal/core/usecase`

### CQRS SQLite Contract (`specs/phase1/cqrs-sqlite.md`)

- [x] DSN pragma coverage in `internal/adapters/sqlite/gormsqlite/db_test.go`
- [x] Read/write path behavior in `internal/adapters/sqlite/...` tests
- [x] Verification command: `go test ./internal/adapters/sqlite/... ./internal/app`

### Audit and Outbox Atomicity (`specs/phase1/audit-outbox.md`)

- [x] Store mutation side effects baseline in `internal/adapters/sqlite/repository_test.go`
- [ ] Add explicit migration-level immutability trigger assertions
- [ ] Add explicit no-partial-commit error-path regression cases
- [x] Verification command: `go test ./internal/adapters/sqlite ./internal/core/usecase`

## Phase 2

### Event Envelope and Versioning (`specs/phase2/event-envelope.md`)

- [x] Codec tests in `internal/core/usecase/event_codec_test.go`
- [x] Replay/upcast behavior in `internal/core/usecase/replay_test.go`
- [x] Verification command: `go test ./internal/core/usecase`

### Replay and Projection Correctness (`specs/phase2/replay-projection.md`)

- [x] Replay correctness baseline in `internal/core/usecase/replay_test.go`
- [ ] Add fixture-based projection sequence tests for complex streams
- [ ] Add cross-tenant replay isolation regression test case set

### Outbox Dispatch Reliability (`specs/phase2/outbox-dispatch.md`)

- [ ] Add dispatcher flow tests for claim, publish, and mark-dispatched steps
- [ ] Add retry budget exhaustion and quarantine/dead-letter tests
- [ ] Add restart-resume durability test across process boundaries

## Phase 3

### Security Hardening (`specs/phase3/security-hardening.md`)

- [x] Strict decode and validation behavior in `internal/adapters/httpapi/handler_test.go`
- [ ] Add explicit log redaction assertions for credentials/secrets
- [ ] Add idempotency scope tests across tenant and endpoint boundaries

### Observability and SLO (`specs/phase3/observability-slo.md`)

- [ ] Add structured logging contract tests for correlation fields
- [ ] Add readiness/health integration tests for dependency failure modes
- [ ] Add metrics contract tests for write and dispatcher critical paths

### Release Governance (`specs/phase3/release-governance.md`)

- [x] Workflow policy encoded in `.github/workflows/docker-build.yml`
- [ ] Add workflow-level automated tests for tag-classification branches
- [ ] Add release-note checklist artifact with rollback/migration requirements

## Baseline verification

```bash
go test ./...
```
