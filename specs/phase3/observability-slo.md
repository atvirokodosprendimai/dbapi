# Phase 3 — Observability, SLOs, and Operability

## Scope

Define measurable reliability and debugging capabilities for production operation.

## Acceptance criteria

- [ ] Structured logs include request ID, tenant ID, route, status, and latency.
- [ ] Health/readiness checks cover DB connectivity and migration state.
- [ ] Dispatcher and write-path metrics expose throughput, failure, and retry signals.
- [ ] Error responses include stable machine-readable codes for alert correlation.
- [ ] SLO targets are documented for API availability and write latency.

## Required tests

- [ ] Integration test for health/readiness status transitions.
- [ ] Metrics exposure test for critical counters and histograms.
- [ ] Log-shape test ensuring mandatory correlation fields are present.

## Verification commands

```bash
go test ./internal/adapters/httpapi ./internal/app
go test ./...
```

## CI gate

- [ ] PR fails if health/metrics/log contract tests fail.
