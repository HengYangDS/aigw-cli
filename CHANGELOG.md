# Changelog

All notable, user-relevant changes are recorded here.  This chronicle follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and Semantic
Versioning.  A published section must correspond to an existing Git tag; it is
not a plan, a branch name, or an inferred version.  Artifact publication,
platform acceptance, signing, and GA status remain separate evidence.

## [Unreleased]

### Added

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
- Model update endpoints as atomic provider, origin, and repository tuples, with
  GitLab primary and GitHub availability-only fallback roles.
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
- Prevent a healthy GitLab update from contacting GitHub, and prevent GitLab
  integrity failures from silently failing over to GitHub.

## [0.1.0-rc.48] - 2026-07-14

### Security

- Require every tag pipeline to verify that its exact annotated release tag carries an SSH signature trusted by the repository-owned signer anchor before packaging, publication, or GitLab Release creation. An unsigned or untrusted tag now fails closed.
