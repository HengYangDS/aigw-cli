## Context

The morning Change `supply-chain-refresh-oct19` moved aigw to Mise 2026.10.14
and replaced the Windows runner's Mise. GitLab pipelines 11617 and 11618 and
GitHub run 39230363335 then showed, in every native Windows job, that
`mise.exe` 2026.10.14 finds its own release's `mise-shim.exe` untagged. The
jobs pass because the warning sits outside classified gates. Renovate's
`dryRun=full` run on `d02033e0` at 20:31 lists the remaining candidates.

## Decisions

- Advance Mise to 2026.10.15 rather than return to 2026.10.8: its official
  windows-arm64 archive (`e6aa014e…`, verified against `SHASUMS256.txt`) ships a
  shim tagged `mise-shim-version:2026.10.15`; the image index digest is
  `sha256:329c5517…`.
- Take `jdx/mise-action` v5.2.1 (tag commit `fc42bcc7…`), which stops a missing
  packslip from weakening its checksum verification.
- Take OpenSpec 1.14.3, which replaces `fast-glob` with `tinyglobby` and so no
  longer pulls `braces`. The same advisory (GHSA-vfj7-8cjw-p6xm) remains through
  `@mermaid-lint/core` → `micromatch`, whose only offered fix is a major
  downgrade; npm audit falls from five high entries to three. Take Renovate
  44.168.1 (index `sha256:8dad0229…`).
- Keep Codex 0.162.1 and Claude Code 2.1.296 for Windows native acceptance until
  real journeys admit newer versions.
- Leave `min_version` to the repository-paradigm batch that already owns it.

## Risks / Trade-offs

- Mise 2026.10.15 changes `mise fmt`, task directory errors, npm locking under
  aube and Windows archive moves; none of these is a contract aigw depends on.
- Between the runner replacement and GitLab's first run of the new declaration,
  an older GitLab pipeline would fail its Mise identity check. Replace the
  runner's Mise only after the previous head's pipelines have finished.

## Rollout

These are post-archive obligations of the delivery, not checklist tasks:

1. Integrate the proved, archived head and publish it to GitHub through a
   reviewed proposal.
2. Before GitLab receives it, pause both Windows runners, replace the runner's
   Mise from the verified release in a window without jobs, verify its digests
   and version, and restore the VM's isolation and the runners.
3. Publish to GitLab, verify both peers' `dev` and `main` refs and CI, and
   confirm that the Windows jobs no longer log the shim warning.
