## Context

The repository owns dependency truth in its Mise, npm, Go and CUE declarations,
and Renovate is the single proposal policy. The candidates come from the pinned
Renovate image's `dryRun=full` run against GitHub `dev`, which published
nothing.

## Decisions

- Advance only the candidates Renovate's policy admits: stable releases, npm
  releases at least three days old. Newer releases inside that window (OpenSpec
  1.14.3, Claude Code 2.1.303) wait for the next maintenance run.
- Move direct Go requirements only; indirect modules keep the versions that
  minimal version selection derives from them.
- Regenerate every lock and peer projection with its native owner; edit no
  generated file by hand.
- Mise 2026.10.14 fails an install on a tool option its backend does not read.
  On that version `mise config explain tools` recognizes every option in the
  declaration, so the image update keeps installs valid.
- The CUE authority derives the Mise version from the image and pins the
  runner-owned Windows ARM64 executable and shim digests. Take both digests from
  the official release archive verified against its published `SHASUMS256.txt`,
  and replace the runner's files with the same bytes in a window without jobs.
- `jdx/mise-action` 5.2.0 changes its default cache key, so the first run after
  the update misses the cache once.
- golangci-lint 2.15.0 reports unchecked error-channel receives and a shadowed
  builtin in three tests; the tests now assert or report those values.
- The repository's OpenSpec gate rejects findings at every severity, and
  OpenSpec reports an INFO finding for any Change that declares `skip_specs`.
  This Change therefore carries its real delta, the runner-owned tool scenario,
  instead of declaring `skip_specs`.

## Risks / Trade-offs

- New golangci-lint or markdownlint rules can report findings. Repair the source
  in this Change; do not suppress a rule or relax a policy.
- A newly published artifact can be withdrawn. The locks bind exact checksums,
  and the full proof and both peers' CI run against the updated graph.
- Between the runner replacement and GitLab's first run of the new declaration,
  an older GitLab pipeline would fail its Mise identity check. Replace the
  runner's Mise only after the pipelines of the previous head have finished.
