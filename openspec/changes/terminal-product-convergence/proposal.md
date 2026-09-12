## Why

AIGW is already a working local control plane, but its setup, credential,
selection, synchronization, and readiness journeys still expose overlapping
concepts and can make optional Providers, clients, or compatibility services
look mandatory. The repository also needs one terminal engineering shape so
that stricter quality, portable delivery, and future Provider or client
extensions reduce rather than compound accidental complexity.

## What Changes

- **BREAKING** Remove the ambiguous global-default interpretation from route
  selection. Each Profile belongs to exactly one client, and each enabled
  client independently selects one Profile.
- Make reviewed team setup progressive: import the complete secret-free
  catalogue, connect any available subset of Accounts, activate only installed
  clients backed by usable Accounts, and leave every other capability in an
  explicit deferred state.
- Make `use`, `sync`, `status`, `check`, `doctor`, and credential recovery agree
  on one state model. Installing a supported client or supplying a Token later
  must require only an idempotent synchronization, not a second hidden global
  selection step.
- Keep credential storage portable and intentional: native stores are used only
  when available, environment credentials remain an explicit read-only mode,
  and ordinary observation never opens a prompt or reads secret values.
- Preserve AIGW as a narrow control plane. External gateways are ordinary
  optional endpoints; AIGW neither identifies nor owns the product behind an
  endpoint and does not import, install, supervise, or encode its lifecycle.
- Establish low-cost Provider and client extension contracts whose ordinary
  path is declarative metadata plus a narrow adapter and conformance tests, not
  core branching or copied workflows.
- Converge public results on precise human and JSON semantics with one problem,
  current state, effect, and safe next action.
- Reshape source, tests, repository tools, configuration, documentation, and
  generated CI around their semantic owners; delete parallel implementations,
  compatibility residue, hard-coded duplicate identity, stale evidence shells,
  and other entities without a current consumer.
- Consolidate quality policy into one positive responsibility map backed by
  mature formatters, linters, type and architecture checks, security and
  supply-chain tools, and rational measured thresholds.
- Use locked `mise` tasks as the sole cross-platform development entrypoint;
  update direct dependencies to current stable compatible releases and prove
  clean-room reconstruction without ambient host tools.
- Generate semantically equivalent GitHub and GitLab pipelines from one CUE
  model while assigning macOS, Linux, and Windows facts only to runners capable
  of proving them.
- Prove build, installation, update, rollback, uninstall, credential, and real
  client projection journeys on the supported native platforms before release;
  retain the current working release as the rollback baseline until replacement
  evidence is complete.
- Close with signed clean source, aligned local and dual-Forge `main`/`dev`,
  green exact-commit and release evidence, deleted proposal and Work Lane
  residue, and no undisclosed P0 or P1 defect.

## Capabilities

### New Capabilities

None. The terminal product capabilities already have canonical owners; this
Change completes and simplifies them instead of creating parallel concepts.

### Modified Capabilities

- `product-control-plane`: Clarify independent per-client routing, optional
  external gateway composition, the Provider and client extension boundary,
  lifecycle symmetry, and portable release admission.
- `progressive-team-onboarding`: Make partial Account availability, absent
  clients, later installation, and later credential supply first-class normal
  states with one continuation path.
- `profile-client-selection`: Remove global-default ambiguity and make every
  selection operation affect only the Profile's declared client.
- `secret-storage`: Define deterministic backend selection and verified native,
  environment, failure, migration, and non-interactive observation behavior on
  macOS, Linux, and Windows.
- `cli-readiness`: Align setup, synchronization, status, check, doctor, and live
  verification on one typed state and actionable result contract.
- `product-quality`: Replace fragmented or negative policy with complete,
  positive, tool-owned quality and delivery coverage.
- `repository-organization`: Align logical and physical topology, development
  environments, documentation navigation, generated projections, retention,
  versioning, and final repository hygiene.
- `ci-diagnostics`: Define exact-event, exact-commit, capability-aware dual-Forge
  evidence without monolithic or meaningless duplicate execution.
- `projection-format`: Make generated configuration, documentation, and CI
  projections deterministic, readable, and derived from their canonical model.

## Impact

This Change may alter CLI defaults, result schemas, configuration migration,
credential-backend selection, package boundaries, repository paths, CI jobs,
release metadata, and documentation links. Existing working installations stay
untouched until a candidate has passed source, native-platform, released-binary,
upgrade, rollback, uninstall, credential, and real-client acceptance. No change
may make an external gateway, a Forge, a particular Provider, or a currently
installed client an AIGW runtime dependency.
