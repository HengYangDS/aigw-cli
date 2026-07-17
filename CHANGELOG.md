# Changelog

All notable, user-relevant changes are recorded here.  This chronicle follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and Semantic
Versioning.  A published section must correspond to an existing Git tag; it is
not a plan, a branch name, or an inferred version.  Artifact publication,
platform acceptance, signing, and GA status remain separate evidence.

## [Unreleased]

## [0.1.0-rc.61] - 2026-07-17

### Fixed

- Treat the Changelog release date as source-controlled metadata, rather than
  as the timestamp of one forge's independently signed tag.
- Verify GitLab and GitHub release tags against separate tracked trust anchors;
  the retired GitHub signer is restricted to an explicit immutable inventory.
- Preserve superseded GitLab release headings through an explicit retired-tag
  inventory, and keep the chronology regression fixture valid after a candidate
  tag is created.

## [0.1.0-rc.58] - 2026-07-17

### Fixed

- Pin release builds to Go 1.25.8, use one tracked provider-neutral release
  source manifest, and remove AppleDouble metadata before macOS packaging so
  the equal GitLab and GitHub release planes produce the same artifact bytes.
- Reject a release compiler or provider tuple that differs from the committed
  source contract, and omit forge-specific VCS metadata from portable binaries.

### Changed

- Run the full release matrix on the dedicated macOS arm64 release runner in
  both forge pipelines, with a byte-for-byte comparison gate for every matrix.

## [0.1.0-rc.57] - 2026-07-18

### Fixed

- Reset writable ownership before bounded CI Go-cache eviction, so a read-only
  module entry cannot abort a release package job.
- Verify every linked GitLab and GitHub release asset after creation or reuse;
  publication now fails closed when a release is incomplete or differs from
  locally validated artifacts.

## [0.1.0-rc.56] - 2026-07-18

### Fixed

- Pin release builds to Go 1.25.8, use one tracked provider-neutral release
  source manifest, and remove AppleDouble metadata before macOS packaging so
  the equal GitLab and GitHub release planes produce the same artifact bytes.

### Changed

- Run the full release matrix on the dedicated macOS arm64 release runner in
  both forge pipelines, with a byte-for-byte comparison gate for every matrix.

## [0.1.0-rc.55] - 2026-07-17

### Fixed

- Make the complete 15-artifact release matrix reproducible from one committed
  source epoch, including portable archives, macOS packages, Windows MSI
  metadata, checksums, and the SPDX SBOM.
- Keep GitLab package builds resilient to a transient module-proxy timeout
  without weakening the declared dependency source policy.
- Preserve the MSI PATH environment entry through deterministic package
  metadata and its install/uninstall execution sequence.

### Changed

- Require two byte-identical full-matrix builds before either release plane
  publishes an artifact set.

## [0.1.0-rc.54] - 2026-07-17

### Added

- Add `aigw repair --dry-run [--json]` as a secret-free, lock-free preview for
  restoring legacy JetBrains target membership while retaining the standalone
  Codex target.

### Fixed

- Make route-doctor conflicts recommend the repair preview instead of an Air
  restore command that must fail while Air still selects AIGW at the top level.

## [0.1.0-rc.53] - 2026-07-17

### Fixed

- Keep the shallow branch-history fixture in `test-changelog.sh` independent
  of an outer GitLab or GitHub release-tag environment.

## [0.1.0-rc.52] - 2026-07-17

### Added

- Add host-specific Codex surface classification and `aigw route doctor` for
  local, secret-free ownership diagnostics. The doctor distinguishes ordinary
  standalone Codex CLI, PyCharm Codex, JetBrains Air, and Junie CLI without
  executing a client or probing a provider.
- Add explicit `aigw route fallback air` and `aigw route restore air` commands
  with secret-free dry runs and required `--confirm-host-idle` attestation for
  any mutation.

### Changed

- Make host routing explicit: ordinary standalone Codex CLI receives the AIGW
  full-selection projection; ChatGPT Desktop retains authority over existing
  conversation model choices and transcripts; PyCharm Codex and Junie CLI stay
  JetBrains AI surfaces; and Air stays JetBrains AI except for an explicit,
  non-default fallback.
- Restrict generic Codex setup and repair discovery to standalone targets. An
  explicitly staged Air fallback remains reconcilable, but generic commands no
  longer adopt JetBrains host defaults.

### Fixed

- Reconcile Codex projections as before-to-after transactions with sidecar
  writer attribution, guarded preimage checks, byte-exact rollback, and
  fail-closed handling of incomplete, foreign, or mode-mismatched state.
- Preserve Air's top-level provider/model selection and original bytes while
  staging or restoring its namespaced fallback.

- Mark GitHub releases created from SemVer prerelease tags as prereleases, while
  leaving GA releases unmarked.
- Resolve the newest published GitHub prerelease when GitHub's stable-only
  latest-release endpoint has no result, preserving the independent peer update
  contract for prerelease-only release streams.
- Add a disposable-volume macOS native-package acceptance lane with an owned
  uninstaller and explicit RC-versus-GA evidence boundaries.

## [0.1.0-rc.51] - 2026-07-15

### Fixed

- Build the shallow-history changelog regression from an independent complete fixture, so both GitHub and GitLab release gates exercise the same recoverable history boundary.

## [0.1.0-rc.50] - 2026-07-15

### Fixed

- Make the GitHub release workflow install the supported Homebrew `msitools` formula, which provides `wixl`, and make the shallow-history changelog fixture reproduce CI safely.

## [0.1.0-rc.49] - 2026-07-15

### Added

- Split the self-update implementation into focused coordinator, candidate, archive, installer, GitLab, and GitHub units without changing the equal-forge update contract.

- Add the canonical MIT License and align repository, package, and contributor
  surfaces on the same permissive licensing statement.
- Add a command-oriented entry path in the CLI and documentation: setup, choose,
  check, then follow one explicit next action.
- Add `aigw update --candidate <archive> --checksums <manifest>` for explicit,
  checksum-first, local-only portable update acceptance.
- Add a canonical terminal experience contract for task-first navigation,
  narrow-terminal presentation, and one-action recovery language.

### Changed

- Standardize the repository's user-facing commands, diagnostics, examples,
  documentation, and release metadata on English-only canonical text.
- Simplify the public documentation set around portable installation, explicit
  control-plane boundaries, reproducible source-checkout verification, and a
  clear legal entry point.
- Record branch closeout as a release-governance requirement: after merge, the
  source worktree and branch are removed once reachability and cleanliness are
  proven.
- Model update endpoints as atomic provider, origin, and repository tuples,
  with GitLab and GitHub as equal independently verified release peers.
- Reorganize root help and documentation around Connect, Use every day,
  Recover, and Advanced paths without changing the existing command grammar.
- Make human-readable output adapt to narrow terminals without truncating text
  or changing JSON output.

### Fixed

- Remove repository-specific release endpoints from source builds. Published
  artifacts now receive their release host and project at build time; source
  builds fail closed until both are configured explicitly.
- Enforce the English-only canonical-surface rule for tracked text, including
  test fixtures, through the governance gate.
- Reject disagreeing peer tags or platform artifact bytes before installation;
  retain single-peer updates only when the other configured peer is unavailable.

## [0.1.0-rc.48] - 2026-07-14

### Security

- Require every tag pipeline to verify that its exact annotated release tag carries an SSH signature trusted by the repository-owned signer anchor before packaging, publication, or GitLab Release creation. An unsigned or untrusted tag now fails closed.
