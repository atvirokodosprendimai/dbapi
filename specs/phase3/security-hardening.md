# Phase 3 — Security Hardening and Abuse Resistance

## Scope

Move from functional security controls to production hardening defaults.

## Acceptance criteria

- [ ] API key validation remains constant-time and uses approved hash storage.
- [ ] Sensitive headers and secret material are never written to logs.
- [ ] Request body limits and decode strictness prevent oversized or unknown-field abuse.
- [ ] Tenant isolation is enforced for all read and write paths, including filter queries.
- [ ] Idempotency keys are scoped to tenant and endpoint to prevent cross-route replay.

## Required tests

- [ ] Negative tests for unauthorized and cross-tenant access attempts.
- [ ] HTTP tests validating strict decode and body size limit behavior.
- [ ] Logging test or harness assertion that secrets are redacted/omitted.
- [ ] Idempotency scope test across tenants and endpoints.

## Verification commands

```bash
go test ./internal/adapters/httpapi ./internal/core/usecase ./internal/adapters/sqlite
go test ./...
```

## CI gate

- [ ] PR fails on any security regression test failure.
