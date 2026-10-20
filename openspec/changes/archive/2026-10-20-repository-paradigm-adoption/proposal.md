# Repository Paradigm Adoption

## Why

A runtime built from current ETHOS `dev` refuses every AIGW proof with 96
gaps against the shared repository paradigm. There are 57 CI steps that hold
inline shell programs, 33 decision-record format gaps, and 6 declaration gaps:
`.ethos/workspace.toml` restates the shared release-mirror role and signing
policy, `.ethos/release.toml` lacks the release mirror, the 0.4.1 changelog
section opens with prose, and the toolchain has no `fix` verb. AIGW therefore
stays on an older runtime. That blocks the GitLab receipt catch-up in
`gitlab-peer-reconciliation`, and the paradigm requires an adopter to migrate
in the same batch as its runtime switch.

## What Changes

- Decision records adopt the shared rationale structure: front matter names
  the record identity and disposition, the body holds exactly five rationale
  sections, and records carry no execution content.
- GitLab native jobs run one native command. The CI tool derives the
  acceptance arguments from the job's declared inputs. It also owns the
  manual-pipeline guard, the trust-file defaults, the review protected-file
  probe, the Linux Secret Service session, the Windows client supply from a
  public input package, the source bootstrap and a requested lock refresh. One
  implementation serves every platform instead of three shell programs.
- GitHub's Windows client supply and its cleanup move into the CI tool, and the
  release-evidence step resolves the tag commit itself.
- GitLab selects the artifact-only tool closure, and the Linux compiler only for
  source runs, through `rules:variables` instead of shell, as GitHub selects
  them through workflow expressions.
- A step that must run before the locked toolchain exists, or after it may have
  failed, stays shell. This covers OS packages, D-Bus sessions, the job-owned
  Mise state, download source and cleanup, and Windows' verification of the
  runner-owned Mise. Each such step is recorded in
  `.config/ci/retained-steps.toml` by Forge file, jobs and text digest.
- `.ethos/workspace.toml` is removed; `.ethos/release.toml` declares the
  release mirror. AIGW's commit-provenance verifier applies the shared SSH
  signing policy without a workspace table and refuses a table that restates
  it. The `fix` task applies the Prettier formatting that `check` verifies,
  the Mise floor becomes 2026.10.14, and the 0.4.1 changelog preface becomes
  an entry.
- AIGW rebinds to the runtime built from current ETHOS `dev` in the same batch.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `repository-organization`: gains the requirements "Decision records share one
  rationale structure" and "Repository declarations follow the shared
  paradigm".
- `ci-diagnostics`: gains the requirement "CI steps run one native command".

## Impact

Product behavior, configuration and release artifacts do not change. Native
jobs keep their source and artifact-only tool scopes. Commits still
require SSH signatures; only the declaration that states it moves to the
shared policy. Inbound links to removed decision subsections move to the new
sections.

Out of scope: ETHOS and proxy changes, and the GitLab release-tag receipts,
which follow the rebind under `gitlab-peer-reconciliation`.
