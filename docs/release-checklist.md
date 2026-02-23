# Release Checklist

Use this checklist before publishing a production release tag.

## Version and tagging

- [ ] Release tag is stable semver (`vX.Y.Z`) when promoting `latest`.
- [ ] Pre-release tags (`-rc`, `-beta`, `-alpha`) do not update `latest`.

## Verification

- [ ] CI test workflow passed (`go test ./...`).
- [ ] Container image build/publish completed for required platforms.
- [ ] Multi-arch manifest inspection confirms `linux/amd64` and `linux/arm64`.

## Migration plan

- [ ] Migration files for this release are listed.
- [ ] Forward migration impact and expected runtime are documented.
- [ ] Backward compatibility constraints are documented.

## Rollback plan

- [ ] Rollback trigger criteria are defined.
- [ ] Safe rollback procedure is documented (app + DB implications).
- [ ] Data recovery or replay requirements are documented.

## Communication

- [ ] Release notes summarize user-facing changes.
- [ ] Operational notes include monitoring points and known risks.
