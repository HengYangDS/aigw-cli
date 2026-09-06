## Why

The accepted toolchain still pins Go 1.27.0 and EditorConfig Checker 3.11.2,
while current stable releases are 1.27.1 and 4.0.1. The old EditorConfig
backend cannot resolve the v4 release assets, so retaining it would leave a
stale, non-upgradable authority beside a working official-release backend.

## What Changes

- Refresh the Go language directive and locked Mise runtime from 1.27.0 to
  1.27.1.
- **BREAKING:** replace the obsolete `editorconfig-checker` Aqua backend and
  its `ec` executable contract with the official GitHub release backend and
  the `editorconfig-checker` executable.
- Regenerate the existing CI projections from their single CUE owner.
- After the accepted repository no longer consumes them, retire the local Go
  1.27.0, EditorConfig Checker 3.11.2, and redundant direct-install residues.
- Do not add a compatibility alias, wrapper, second dependency manager, or
  parallel version source.

## Capabilities

This maintenance change does not alter product behavior and therefore opts out
of specification deltas.

## Impact

The material source scope is `go.mod`, `mise.toml`, `mise.lock`,
`tools/ci/main.go`, `tools/ci/main_test.go`, `tools/ci/projection_test.go`,
`.config/ci/pipeline.cue`, `.github/workflows/verify.yml`, and
`.gitlab-ci.yml`. Runtime retirement is a post-acceptance host operation, not a
second repository authority.
