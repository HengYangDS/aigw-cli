# Changelog

All notable, user-relevant changes are recorded here.  This chronicle follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and Semantic
Versioning.  A published section must correspond to an existing Git tag; it is
not a plan, a branch name, or an inferred version.  Artifact publication,
platform acceptance, signing, and GA status remain separate evidence.

## [Unreleased]

## [0.1.0-rc.75] - 2026-07-30

### Added

- Add Claude Fable 5, Opus 5, Sonnet 5 and GPT-5.6 Luna, Sol, Terra UCloud
  profiles to the ready-to-use team manifest, including UCloud's
  Anthropic-compatible endpoint.
- Add the regular DMXAPI Claude Opus 5 profile alongside its CC and SSVIP
  variants.

### Fixed

- Prune stale direct GitLab release tags from persistent CI workspaces without
  removing qualified GitHub provenance, and retain removed release chronology
  in the append-only retirement inventory.
- Align provider-native commit trust anchors with the currently registered
  GitLab and GitHub SSH signing keys.

## [0.1.0-rc.74] - 2026-07-29

### Fixed

- Isolate Changelog regression fixtures from an outer tag workflow's selected
  release identity.

## [0.1.0-rc.73] - 2026-07-29

### Fixed

- Pass the selected provider root tag explicitly through GitHub verification
  and release jobs so cached qualified tags cannot alter release chronology.
- Keep native macOS, Linux, and Windows source verification blocking for RCs,
  while reserving rooted macOS package-lifecycle acceptance for GA credentials.

## [0.1.0-rc.72] - 2026-07-29

### Added

- Add a repository-wide Go statement-coverage gate that tests every package
  under `./...` and requires both every package and the aggregate to remain
  strictly above 95 percent.
- Add blocking native Linux and Windows verification alongside the existing
  macOS release runners, with Windows acceptance enforced on GitHub.

### Changed

- Replace the DMXAPI Qwen 3.7 Max team profile with a token-free UCloud
  Account placeholder at its mainland OpenAI-compatible endpoint while
  preserving all recommended routes.
- Make post-floor GitLab and GitHub commit email and SSH-signature verification
  mandatory, and change GitHub projection to signed, forward-only commits
  without a history-rewrite escape.

### Fixed

- Make Windows paths, file modes, archive names, recovery storage, deferred
  self-update, discovery, and launcher tests follow native platform semantics.
- Make transaction snapshots read bytes and metadata from one file handle, and
  remove timing-dependent Linux and Windows process/file race fixtures.
- Preserve signed, forward-only GitHub projection for merge branches forked
  from older canonical ancestors, including Keychain-backed signing.

## [0.1.0-rc.71] - 2026-07-27

### Added

- Add first-time `aigw setup --from <team-profiles.toml>` for token-free team
  manifest v3, including reviewed route recommendations, per-Account hidden
  Token prompts, real Claude-client validation when discoverable, strict Codex
  endpoint probes, and bounded rollback.
- Add an offline forge synchronization contract that requires exact canonical
  commit identity on GitLab and complete ordered source-tree history in the
  identity-rewritten GitHub projection.

### Fixed

- Use protocol-specific authentication headers for direct Claude and Codex
  connectivity checks, so Anthropic-compatible endpoints no longer receive an
  OpenAI bearer header.
- Preserve qualified GitHub provenance tags while refreshing canonical GitLab
  tags, even when global Git fetch pruning is enabled, and record retired GitLab
  `rc.58` chronology so fresh CI checkouts do not depend on workstation refs.

## [0.1.0-rc.70] - 2026-07-24

### Added

- Add `aigw profile rename [old] [new]` and `aigw account rename [old] [new]` commands with support for interactive selection, scripting-friendly arguments, and a safe two-phase account credential migration.

### Fixed

- Remove runtime and static exclusions for specific `gpt-5.6-*-cdx` profile and model IDs; model identity is now constrained only by general format, references, and client protocol capabilities.

## [0.1.0-rc.69] - 2026-07-21

### Fixed

- Disable Go-cache save in self-hosted GitHub verification so a completed gate
  cannot remain in runner cleanup after its visible test steps finish.
- Remove the temporary GitHub-only release exception inventory after RC.67 and
  RC.68 gained independent GitLab provenance, CI, and release records; release
  chronology now admits only versions completed on both forge planes.
- Declare complete Git history in source-controlled GitLab CI, preventing a
  project-level shallow-clone setting from misclassifying published chronology.
- Require separate local GitLab, GitHub verification, and GitHub release
  runners; GitHub verification accepts only trusted `main`, tag, and manual
  workflows, while the release runner never executes verification code.
- Verify every GitHub provenance tag whose source tree is represented by the
  selected canonical branch before updating its identity projection.
- Require three bounded recovery observations before an initial HTTP 401 is
  treated as a persistent invalid Token; mixed results remain retry-only and
  never mutate credentials.
- Build the isolated GitHub identity projection from a detached temporary
  source ref, so clearing its branch namespace cannot turn the complete source
  tree into staged additions before the projection rewrite begins.
- Fail closed when a registered source worktree cannot be inspected during
  branch closeout, and require mandatory governance commands to remain in the
  enforcing GitHub Actions `run` block or GitLab `verify.script` block rather
  than inert environment data or non-blocking `after_script` configuration;
  conditional or non-blocking job and step settings cannot bypass those gates.
- Treat `.serena/` at any depth as disposable local developer-tool state, and
  keep GitHub Changelog chronology confined to its qualified provider tag
  namespace without admitting unscoped GitLab tags.
- Check and surface terminal-output, persistence, archive, HTTP, and process
  cleanup failures instead of silently discarding them, while retaining fluent
  command presentation and explicit recovery behavior.
- Run tracked Staticcheck and Errcheck checks in local and hosted verification,
  so unchecked errors and analysis regressions fail before a release candidate
  is prepared.

## [0.1.0-rc.68] - 2026-07-19

### Fixed

- Preserve the exact provider/model order in Air host-mirror fingerprints and
  reject quoted, escaped, incomplete, or otherwise malformed protected TOML
  key/table aliases as foreign residue without rejecting valid dotted keys,
  while fuzzing malformed projection and router-log inputs, including a route
  path normalization edge case.
- Report bounded recovery-ledger and quarantine health from the existing
  read-only `route doctor` command even when Air is missing, without exposing
  private digests, case details, paths, or raw routes. Private recovery reads,
  writes, removals, and rollback are descriptor-bound and fail closed on
  symlink traversal, unsafe permissions, special files, or unexpected storage
  residue.
- Verify every byte of all 15 assets on an existing GitLab Release against the
  locally validated matrix, rejecting missing, extra, duplicate, or mismatched
  assets without updating the Release or forwarding its job token to a
  cross-host download redirect.
- Make `aigw route doctor` fail closed for unbound or foreign Air residue:
  human and JSON results state that no AIGW mutation is admitted and suggest
  only the read-only `aigw route doctor --json`, rather than repair, recovery,
  checks, or path-bearing actions that cannot change the JetBrains-owned route.
- Validate GitHub branch chronology against provider-native release trees when
  its projected commit identity differs from the canonical history.
- Prove source-branch closeout across canonical and identity-rewriting forge
  histories, including clean CI fixtures that do not inherit a runner identity.
- Reject credential-shaped literals in tracked source and test fixtures while
  retaining explicit, non-secret redaction sentinels.

## [0.1.0-rc.67] - 2026-07-19

### Fixed

- Distinguish a reference-proven JetBrains-owned Air host mirror from a true
  exact AIGW orphan, and recover only the latter through a case-bound
  quarantine and settlement workflow.
- Add secret-free, read-only Air route attestation from bounded forwarding
  evidence without exposing raw routes, credentials, prompts, responses, or
  sessions.
- Preserve existing GitLab Releases by verifying them read-only instead of
  updating them after a publication conflict.

## [0.1.0-rc.66] - 2026-07-18

### Fixed

- Keep `aigw route recover air --dry-run --json` idempotent and path-free when
  Air is already externally owned and no AIGW sidecar remains.
- Refuse to create an AIGW-managed Claude launcher from recognizable
  ephemeral Go source-run or compiler output, preventing a durable shim
  from pointing to a binary that disappears after the command exits.

## [0.1.0-rc.65] - 2026-07-18

### Fixed

- Recover JetBrains Air only from a verified stale AIGW full-selection and
  fallback-sidecar mismatch, removing AIGW residue and explicit target
  membership without fabricating a JetBrains selection or touching sessions.

## [0.1.0-rc.64] - 2026-07-17

### Fixed

- Keep successful provider CLI diagnostics on stderr out of updater protocol
  output, so GitLab release-asset discovery remains valid when `glab` emits
  local configuration warnings.
- Recover GitHub private prerelease discovery and asset retrieval through the
  local `gh` credential path when the official API intentionally returns an
  anonymous 404. No GitHub token is read, exported, or persisted by AIGW.

## [0.1.0-rc.63] - 2026-07-17

### Fixed

- Advance the pinned release compiler from Go 1.25.8 to the current stable
  Go 1.25.12 patch across source metadata and both independent CI planes.
- Keep `aigw route doctor --json` strictly machine-readable when it reports a
  route-ownership conflict; the command now returns a non-zero status without
  appending a human terminal card to the JSON stream.
- Refresh provider tag refs before validating shared chronology, prefer an
  active GitHub provenance tag over a same-named GitLab retirement record, and
  exercise both cases in the Changelog regression suite.
- Isolate the next-candidate fixture from an outer tagged pipeline so release
  verification keeps testing ordinary-branch behavior after a tag is cut.
- Clear outer GitLab and GitHub provider markers from ordinary-branch fixtures
  so cross-forge tag pipelines keep exercising the intended positive and
  negative chronology cases.
- Preserve a selected, locally signed pre-push tag while chronology admission
  checks remote history, so release dry-runs cannot discard the provenance
  object they are validating.
- Refresh GitHub verification tags as annotated remote objects and exercise
  both GitLab and GitHub trust anchors in the signature regression, preventing
  a valid GitHub provenance tag from being treated as an unverifiable fixture.
- Disable the GitHub release workflow's nonessential Go-cache save so a
  completed release does not remain in a long post-publication compression
  step.
- Define the private GitHub Free release plane as signed, independently
  verified provenance rather than claiming unavailable host-enforced tag
  immutability; AIGW automation still never rewrites provider-native tags.

## [0.1.0-rc.62] - 2026-07-17

### Fixed

- Treat the Changelog release date as source-controlled metadata, rather than
  as the timestamp of one forge's independently signed tag.
- Verify GitLab and GitHub release tags against separate tracked trust anchors;
  the retired GitHub signer is restricted to an explicit immutable inventory.
- Preserve superseded GitLab release headings through an explicit retired-tag
  inventory, and keep the chronology regression fixture valid after a candidate
  tag is created.

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
