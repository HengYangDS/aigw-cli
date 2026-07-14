# Changelog

All notable, user-relevant changes are recorded here.  This chronicle follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and Semantic
Versioning.  A published section must correspond to an existing Git tag; it is
not a plan, a branch name, or an inferred version.  Artifact publication,
platform acceptance, signing, and GA status remain separate evidence.

## [Unreleased]

### Fixed

- Make repeated `aigw sync` recognize legacy Codex sidecars that already
  represent the canonical projection, rather than reporting a spurious update.
- Prepare every Codex projection before the first write and restore every
  target on a failed commit, so a multi-target synchronization is atomic.

### Documentation

- Define the AIGW control-plane / Responses data-plane boundary and organize
  canonical architecture, governance, decision, evidence, and historical
  documentation surfaces.

## [0.1.0-rc.44] - 2026-07-14

### Added

- Introduce account-scoped secrets, purpose-labelled runtime profiles, explicit
  default and client routes, and secret-free team manifests.
- Provide isolated Claude and Codex adapters, native credential binding, a
  minimal real-response verifier, and a secret-free verified-configuration
  checkpoint for rollback.
- Deliver portable archives and native macOS, Linux, and Windows packages for
  `amd64` and `arm64`, with checksums, SPDX SBOMs, installer lifecycle tests,
  and package-layout verification.
- Add a portable-program rollback path that retains exactly one previous
  AIGW-owned binary without touching user configuration, credentials, or
  client state.

### Changed

- Establish schema v2 as the only accepted local and team-manifest structure;
  model IDs remain upstream-canonical and client identity is represented by the
  route, not by a model-name suffix.
- Make first use local-first and provider-neutral: no default provider,
  endpoint, model, token, background service, or listening port is assumed.
- Define release evidence so local packaging, hosted CI, physical-platform
  acceptance, artifact publication, signing, notarization, and GA claims are
  separately verifiable.

### Fixed

- Redact bearer, URL-escaped, and structured credentials before diagnostic or
  gateway text reaches terminal output.
- Make portable install, upgrade, rollback, and uninstall resilient to an
  empty or polluted `PATH`, while preserving user configuration and limiting
  cleanup to AIGW-owned files.
- Make account imports non-destructive, diagnose the default route
  consistently, and reject retired `-cdx` model aliases and disposable Claude
  shim targets.
- Validate checksums and package metadata across the supported release matrix,
  including Linux package paths and Windows installer semantics.

### Security

- Keep tokens in platform credential stores and ensure team manifests,
  portable artifacts, rollback records, diagnostics, and release evidence do
  not embed secrets.
