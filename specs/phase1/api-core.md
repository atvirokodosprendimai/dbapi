# Phase 1 — API Core Contract

## Scope

Freeze API behavior and safety defaults before adding new product surface.

## Acceptance criteria

- [ ] All endpoints return JSON for success and error payloads.
- [ ] Auth is required for protected endpoints (`X-API-Key` or bearer).
- [ ] Tenant is derived from API key; no caller-controlled tenant override.
- [ ] Error taxonomy is stable for core flows:
  - `400` invalid key/category/filter/body
  - `401` unauthorized
  - `404` not found
  - `500` internal server error
- [ ] Bulk operations support idempotency via `Idempotency-Key`.

## Required tests

- [ ] HTTP tests for auth/no-auth/invalid payload/error mapping.
- [ ] Idempotency retry test for bulk endpoints.
- [ ] Tenant-isolation tests for collection endpoints.

## Verification commands

```bash
go test ./internal/adapters/httpapi ./internal/core/usecase
go test ./...
```

## CI gate

- [ ] PR fails if any API contract test fails.
