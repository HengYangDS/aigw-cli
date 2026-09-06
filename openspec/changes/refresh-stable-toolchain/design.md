## Context

See [proposal.md](proposal.md). `mise.toml` and `mise.lock` already own the
repository tool closure, `go.mod` owns the supported Go language version, and
`.config/ci/pipeline.cue` owns generated CI projections. The Mise registry's
Aqua mapping still expects the pre-v4 asset names and cannot install 4.0.1;
Mise's GitHub backend resolves and executes the upstream 4.0.1 asset.

## Goals / Non-Goals

**Goals:**

- Keep one version owner and one executable contract for each tool.
- Make a clean locked install resolve current stable Go and EditorConfig
  Checker on every declared platform.
- Preserve all existing source, native, release, and Forge behavior.
- Delete superseded host runtimes only after the accepted source no longer
  references them.

**Non-Goals:**

- A new dependency manager, updater, wrapper, alias, or compatibility path.
- Any product, credential, client, runtime-service, or network change.

## Decisions

Use `github:editorconfig-checker/editorconfig-checker` as the sole v4 backend.
Keeping the registry alias was rejected because its current Aqua metadata
cannot match the upstream v4 archive names. Keeping both backends was rejected
because it creates two local authorities for one executable.

Rename the source-gate invocation from `ec` to `editorconfig-checker` rather
than adding a shim. Update the command-sequence test first so the old behavior
fails before production code changes. Keep the existing flags because v4
advertises the same options.

Advance the Go directive and Mise pin together to 1.27.1. Regenerate
`mise.lock` with Mise rather than editing checksums or release URLs by hand.
Regenerate GitHub and GitLab CI files from `.config/ci/pipeline.cue`; generated
files remain projections, not additional policy owners.

## Risks / Trade-offs

- **Backend asset naming changes again** -> locked URLs and checksums fail
  closed during installation.
- **The v4 executable changes behavior** -> run the real EditorConfig check and
  the complete source gate before landing.
- **Go patch release changes compilation** -> run focused tool tests, the full
  source gate, and native release verification before acceptance.
- **Host cleanup races an old checkout** -> scan all local manifests again
  after landing and retire old runtimes only when no consumer remains.

## Migration Plan

1. Change the regression expectations and observe the focused test fail.
2. Update the executable call, tool pins, Go directive, lockfile, and generated
   CI projections.
3. Run focused tests, strict OpenSpec validation, full source and native release
   gates, then land through the governed candidate and accepted branches.
4. Verify local and remote accepted refs, archive this change, and remove the
   superseded Mise installations and the temporary work lane.

Rollback is a single revert of the accepted commit before host retirement. If
verification fails, retain the isolated lane and leave installed consumers
unchanged.
