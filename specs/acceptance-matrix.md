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
- [x] Migration-level immutability trigger assertions in `internal/adapters/sqlite/repository_test.go`
- [x] No-partial-commit error-path regression cases in `internal/adapters/sqlite/record_event_store_test.go`
- [x] Verification command: `go test ./internal/adapters/sqlite ./internal/core/usecase`

## Phase 2

### Event Envelope and Versioning (`specs/phase2/event-envelope.md`)

- [x] Codec tests in `internal/core/usecase/event_codec_test.go`
- [x] Replay/upcast behavior in `internal/core/usecase/replay_test.go`
- [x] Verification command: `go test ./internal/core/usecase`

### Replay and Projection Correctness (`specs/phase2/replay-projection.md`)

- [x] Replay correctness baseline in `internal/core/usecase/replay_test.go`
- [x] Fixture-based projection sequence tests in `internal/core/usecase/replay_test.go`
- [x] Cross-tenant replay isolation regression tests in `internal/core/usecase/replay_test.go`

### Outbox Dispatch Reliability (`specs/phase2/outbox-dispatch.md`)

- [x] Dispatcher flow tests for fetch, publish, and mark-dispatched in `internal/core/usecase/outbox_dispatcher_test.go`
- [x] Retry budget exhaustion and dead-letter status tests in `internal/core/usecase/outbox_dispatcher_test.go`
- [x] Restart-resume durability test using persisted pending events in `internal/core/usecase/outbox_dispatcher_test.go`

## Phase 3

### Security Hardening (`specs/phase3/security-hardening.md`)

- [x] Strict decode and validation behavior in `internal/adapters/httpapi/handler_test.go`
- [x] Explicit log redaction assertions for credentials/secrets in `internal/adapters/httpapi/handler_test.go`
- [x] Idempotency scope tests across tenant and endpoint boundaries in `internal/adapters/httpapi/handler_test.go`

### Observability and SLO (`specs/phase3/observability-slo.md`)

- [x] Structured logging contract tests for correlation fields in `internal/adapters/httpapi/handler_test.go`
- [x] Readiness/health integration tests for dependency failure modes in `internal/adapters/httpapi/handler_test.go`
- [x] Metrics contract tests for write and dispatcher critical paths in `internal/adapters/httpapi/handler_test.go` and `internal/core/usecase/outbox_dispatcher_test.go`

### Release Governance (`specs/phase3/release-governance.md`)

- [x] Workflow policy encoded in `.github/workflows/docker-build.yml`
- [x] Workflow-level automated tests for tag-classification logic in `internal/ci/releasepolicy/policy_test.go`
- [x] Release-note checklist artifact with rollback/migration requirements in `docs/release-checklist.md`

## Baseline verification

```bash
go test ./...
```
