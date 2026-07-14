# Changelog

All notable, user-relevant changes are recorded here.  This chronicle follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and Semantic
Versioning.  A published section must correspond to an existing Git tag; it is
not a plan, a branch name, or an inferred version.  Artifact publication,
platform acceptance, signing, and GA status remain separate evidence.

## [Unreleased]

无。

## [0.1.0-rc.46] - 2026-07-14

### Fixed

- Reuse a pre-provisioned account credential during non-interactive `aigw setup`; the read-only `AIGW_SECRET_BACKEND=env` backend now validates and references `AIGW_TOKEN_<ACCOUNT>` without prompting for or persisting a duplicate Token.
- Preserve explicit credential intent: `--token-stdin` still takes precedence when a writable secret backend is selected.
- Make every configured Codex projection atomic: prepare all targets before writing and restore exact config/sidecar pre-state if any commit fails.
- Recognize already-canonical legacy Codex sidecars, reject development binaries as healthy user installs, and prevent source builds from replacing the default user binary path.
- Prune retained runner tags before validating the release chronicle, so deleted candidates cannot contaminate later CI.

### Added

- Provide `aigw sync --dry-run [--json]` as a credential-free projection plan; it never writes configuration, binds authentication, restarts clients, or changes sessions.

### Documentation

- Establish the AIGW control-plane / Responses data-plane boundary, canonical governance and evidence surfaces, and the explicit separation of Codex Desktop model authority from local provider configuration.
- Clarify environment-secret setup for repeatable container and CI acceptance without plaintext credentials.
