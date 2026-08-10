# This repository has moved

`smartmontools-go` has moved into
[`dianlight/smartmontools-sdk`](https://github.com/dianlight/smartmontools-sdk),
as `bindings/go/`. See [issue #17](https://github.com/dianlight/smartmontools-sdk/issues/17)
and [PR #18](https://github.com/dianlight/smartmontools-sdk/pull/18) for the
full rationale: this repository and the SDK repository had drifted into a
circular build-time dependency (each one fetched a build artifact from the
other, both pinned to the same unsynchronised `v0.4.1` tag), and the fix was
to make the native SDK the single root with language bindings layered on top.

## What to do

- **New module path**: `github.com/dianlight/smartmontools-sdk/bindings/go`
  (was `github.com/dianlight/smartmontools-go`).
- **New tag prefix**: `bindings/go/vX.Y.Z` (was `vX.Y.Z`).
- **Licence**: relicensed from GPL-3.0 to GPL-2.0-or-later, matching the SDK
  repository as a whole.
- Import-path rewrite recipe:
  [`docs/migration/import-path-migration.md`](https://github.com/dianlight/smartmontools-sdk/blob/main/docs/migration/import-path-migration.md)
- Repository/directory mapping:
  [`docs/migration/smartmontools-go-to-bindings-go.md`](https://github.com/dianlight/smartmontools-sdk/blob/main/docs/migration/smartmontools-go-to-bindings-go.md)
- Version compatibility matrix:
  [`docs/migration/compatibility-matrix.md`](https://github.com/dianlight/smartmontools-sdk/blob/main/docs/migration/compatibility-matrix.md)

## What happens to this repository

`v0.4.2` is the final release here, published purely to carry this notice.
The repository stays read-write (not archived) until downstream consumers
have migrated, then will be archived.

No new features, and no fixes beyond what's needed to keep the last release
buildable, will land here going forward. All future development happens in
`bindings/go/` in the SDK repository.
