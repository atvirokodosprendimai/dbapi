# Phase 3 — Release Governance and Delivery Safety

## Scope

Codify release controls so artifacts are reproducible and promotion is policy-driven.

## Acceptance criteria

- [ ] CI blocks container build/publish when test suite fails.
- [ ] Multi-arch image publish is deterministic and manifest-verified.
- [ ] `latest` is published only for stable semver tags (`vX.Y.Z`).
- [ ] Pre-release tags (`-rc`, `-beta`, `-alpha`) never update `latest`.
- [ ] Release notes include migration and rollback guidance.

## Required tests

- [ ] Workflow test/matrix assertions for tag classification logic.
- [ ] Manifest verification step test for published image digests.
- [ ] Dry-run release checklist validation for migration safety notes.

## Verification commands

```bash
go test ./...
```

## CI gate

- [ ] PR fails if release workflow policy checks fail.
