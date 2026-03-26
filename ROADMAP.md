# ROADMAP for v0.3.2

## Release Goal
Make `gohour` installable for end users via:

```bash
go install github.com/riadshalaby/gohour@latest
```

## Definition of Done
- `go install github.com/riadshalaby/gohour@v0.3.2` succeeds on a clean machine.
- Installed binary runs and reports version (`gohour version`).
- README documents install path and first-run flow.
- Existing test suite passes after module/import-path migration.

## Scope
### In Scope (v0.3.2)
- Go module path migration to canonical GitHub module path.
- Internal import path migration.
- Build/release metadata updates (ldflags path).
- Documentation updates for install + quick start.
- Release smoke test for `go install`.

### Out of Scope (v0.3.2)
- New product features.
- UI redesign or workflow changes.
- Refactors unrelated to installability and release correctness.

## Priorities
### P0: Module and Import Path Migration
- Change `go.mod` module from `gohour` to `github.com/riadshalaby/gohour`.
- Rewrite all `gohour/...` imports to `github.com/riadshalaby/gohour/...`.
- Update ldflags references (`gohour/cmd.Version` -> `github.com/riadshalaby/gohour/cmd.Version`) in scripts/docs.
- Run `go mod tidy`.
- Validate with `go test ./...`.

### P1: Installation Documentation
- Add a top-level `Install` section in `README.md` with `go install ...@latest`.
- Clarify minimum Go version and PATH behavior for installed binaries.
- Keep examples consistent with installed-binary usage (`gohour ...`) and local build usage where relevant.

### P1: Release and Verification
- Cut release from `main` as `v0.3.2` (per release rules).
- Smoke-test from outside repo:
  - `go install github.com/riadshalaby/gohour@v0.3.2`
  - `gohour version`
- Publish release notes including install command.

## Milestones
1. **Codebase migration complete**
   - Module path + imports updated; tests green.
2. **Docs complete**
   - README install section merged and verified.
3. **Release complete**
   - `v0.3.2` tagged and `go install` smoke test passed.

## Risks and Mitigations
- **Risk:** Broken imports after module rename.
  - **Mitigation:** mechanical replace + full `go test ./...` before release.
- **Risk:** ldflags/version not injected after path change.
  - **Mitigation:** verify `scripts/build-all.sh` and run `gohour version` from built artifact.
- **Risk:** `@latest` points to wrong commit/tag.
  - **Mitigation:** release only from `main`, tag merge commit, verify with explicit `@v0.3.2`.
