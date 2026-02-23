# Implementation Plan

This plan converts every unchecked item in `specs/acceptance-matrix.md` into actionable implementation work.

## Priority order

1. Phase 1 gaps (stability foundations)
2. Phase 2 gaps (event and reliability correctness)
3. Phase 3 gaps (operability and release hardening)

## Phase 1 backlog

### 1) Migration immutability trigger assertions

- **Goal:** prove audit rows are immutable at DB level.
- **Target files:**
  - `migrations/files/00002_auth_and_audit.sql`
  - `internal/adapters/sqlite/repository_test.go`
- **Tasks:**
  - Add migration-level assertions that update/delete on audit rows fail.
  - Add tests that verify error shape and unchanged row contents.
- **Done when:** tests fail if triggers are removed or weakened.

### 2) No-partial-commit regression cases

- **Goal:** prove mutation side effects are atomic.
- **Target files:**
  - `internal/adapters/sqlite/repository_test.go`
  - `internal/adapters/sqlite/record_event_store_test.go`
- **Tasks:**
  - Force a side-effect failure (audit or outbox write path) in test setup.
  - Assert state row is not committed when side effect fails.
- **Done when:** failures roll back all writes in a single transaction.

## Phase 2 backlog

### 3) Projection fixture sequence tests

- **Goal:** verify projection correctness across realistic event streams.
- **Target files:**
  - `internal/core/usecase/replay_test.go`
- **Tasks:**
  - Add table-driven fixtures for create/update/delete/mixed sequences.
  - Assert final projection state for each fixture.
- **Done when:** projection output is deterministic for each fixture.

### 4) Cross-tenant replay isolation tests

- **Goal:** ensure replay cannot mix tenant data.
- **Target files:**
  - `internal/core/usecase/replay_test.go`
- **Tasks:**
  - Seed events for multiple tenants in the same test run.
  - Assert replay for tenant A never emits/applies tenant B events.
- **Done when:** tenant leakage fails tests.

### 5) Outbox dispatcher reliability tests

- **Goal:** encode dispatcher delivery guarantees.
- **Target files:**
  - `internal/core/usecase/outbox_dispatcher.go`
  - `internal/core/usecase/outbox_dispatcher_test.go` (new)
- **Tasks:**
  - Add success-path claim/publish/mark-dispatched tests.
  - Add transient-failure retry and max-attempt quarantine tests.
  - Add restart-resume scenario using persisted outbox rows.
- **Done when:** at-least-once behavior and retry policy are test-enforced.

## Phase 3 backlog

### 6) Log redaction assertions

- **Goal:** guarantee secrets never appear in logs.
- **Target files:**
  - `internal/adapters/httpapi/handler_test.go`
  - logging adapter files in `internal/adapters/...` (as needed)
- **Tasks:**
  - Capture logs in tests and assert token/API key absence.
  - Add positive assertions for safe metadata only.
- **Done when:** tests fail on accidental secret logging.

### 7) Idempotency scope tests

- **Goal:** prove idempotency key scope is tenant + endpoint aware.
- **Target files:**
  - `internal/adapters/httpapi/handler_test.go`
- **Tasks:**
  - Reuse same key across different tenants and endpoints.
  - Assert no cross-tenant or cross-route replay collision.
- **Done when:** scope boundaries are enforced by tests.

### 8) Observability contract tests

- **Goal:** enforce operational telemetry contracts.
- **Target files:**
  - `internal/app/app.go`
  - `internal/adapters/httpapi/handler_test.go`
  - metrics-related adapters (if present)
- **Tasks:**
  - Add structured log field presence tests (request ID, tenant ID, route, status, latency).
  - Add health/readiness dependency-failure tests.
  - Add metrics surface tests for write and dispatcher paths.
- **Done when:** correlation fields and core metrics are stable under test.

### 9) Release governance checks

- **Goal:** test release workflow policy, not only define it.
- **Target files:**
  - `.github/workflows/docker-build.yml`
  - `.github/workflows/*` (policy test workflow if needed)
  - `docs/` (release notes checklist artifact)
- **Tasks:**
  - Add workflow tests for stable semver vs prerelease tag classification.
  - Add checklist template with migration + rollback sections.
- **Done when:** CI verifies tag policy and release note requirements.

## Execution batches

- **Batch A:** items 1-2 (Phase 1) - completed
- **Batch B:** items 3-5 (Phase 2) - completed
- **Batch C:** items 6-9 (Phase 3) - completed

Each batch should end with:

```bash
go test ./...
```
