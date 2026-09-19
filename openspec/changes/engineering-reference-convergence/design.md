# Design

## Context

AIGW 0.1.0 is an immutable published baseline. The repository already declares the intended product boundaries and many strong local and hosted gates, but accumulated delivery work can still leave physical topology, user journeys, tests, documentation, and quality policy harder to understand than the product requires. This Change converges those surfaces without rewriting the stable tag or importing generic lifecycle state into AIGW.

## Goals / Non-Goals

**Goals:**

- Establish one evidence-backed map from product journeys and invariants to semantic owners.
- Repair behavior before reorganizing its files, then make logical and physical ownership agree.
- Remove unconsumed entities and parallel semantics before adding tools or abstractions.
- Make setup, deferred activation, synchronization, credentials, client projection, installation, recovery, and extension natural on every supported platform.
- Make the repository independently understandable and reproducible by a new contributor.

**Non-Goals:**

- Rebuild AIGW as a traffic gateway, daemon, desktop application, or package-manager framework.
- Add a tutorial subsystem, duplicate status ledger, compatibility facade, or local ETHOS state machine.
- Rewrite 0.1.0 artifacts, tags, historical OpenSpec archives, or user-owned client configuration.
- Change Codex Responses Proxy implementation from this repository.

## Decisions

### 1. Audit by semantic closure, not by directory

Begin with the public journeys and invariants, then trace their source, tests, configuration, documentation, and evidence. Each closure ends with one owner, focused tests, full gates where needed, updated documentation, and deletion of displaced material. A file-by-file cleanup without this trace is rejected because it can polish the wrong topology.

### 2. Delete before adding

For every duplicate helper, wrapper, configuration fragment, compatibility path, or document, first identify its current consumer and protected invariant. Delete it when both are absent. Reuse an existing owner when present. Add an entity only when no current owner can express the behavior without increasing coupling; record the displaced complexity in the existing decision register.

### 3. Product journeys define dependency order

Converge the paths in this order: configuration and route authority; credentials; client projections; installation and recovery; provider/client extension; repository topology; quality graph; documentation; performance and final acceptance. This order prevents structural refactors from preserving broken behavior and avoids running expensive matrices before local semantics stabilize.

### 4. AIGW and Proxy compose only through explicit endpoints

AIGW owns Accounts, Tokens, Profiles, Routes, and client projections. It carries no Proxy lifecycle, state, or mandatory loopback default. A profile may select a direct provider endpoint or any independently managed compatible endpoint. Proxy owns protocol translation and runtime traffic when explicitly installed. Tests use an external endpoint contract rather than importing Proxy implementation.

### 5. Quality has one declarative graph

The repository keeps one machine-readable mapping from tracked carrier classes to mature formatters, linters, analyzers, tests, security checks, and generated projections. GitHub and GitLab remain deterministic projections of that graph. Custom code is retained only for AIGW-specific semantics that general tools cannot express. Threshold changes require measured distributions and named risks, not aesthetic severity.

### 6. Cross-platform claims consume real released bytes

Source tests establish contracts; native jobs establish host behavior; published-artifact jobs establish distribution behavior. macOS, Linux, and Windows each exercise build, install, update, rollback, uninstall, credential mode, and client projection using the selected immutable release bytes. Unsupported platform trust, client availability, or credential service behavior remains explicit rather than inferred.

### 7. Documentation teaches by tracing the product

The root README remains the concise product entry point. Task-oriented guides explain complete user and contributor journeys; architecture documents explain stable boundaries; decisions record chosen trade-offs; research remains evidence for future choices. Code, commands, diagrams, tables, and links are validated through the same repository quality graph. No private local file may be a shared prerequisite.

## Risks / Trade-offs

- **Large scope can create churn** → complete one semantic closure at a time and require deletion plus focused acceptance before the next structural move.
- **Stricter gates can reward fragmentation** → derive thresholds from distributions and preserve coherent domain units.
- **Native evidence can become expensive** → run focused local falsification first, freeze inputs, then reuse exact matching immutable evidence.
- **External tools can expand the maintenance surface** → admit only stable tools that replace more code and operational burden than they add.
- **Breaking cleanup can surprise existing users** → remove only unsupported or unconsumed behavior; document migration for supported public contracts.

## Accepted baseline

Subsequent comparisons use the immutable stable release rather than an RC or a
mutable checkout:

- source commit: `0e4c411410b264ab587aa90b8d237acc5e06fa79`;
- signed tag object: `ccd4853751fad10fc851207c5a348d4f8915b2c0`
  (`v0.1.0`);
- release inventory: the ten entries signed by `checksums.txt.sig`, including
  native archives for macOS, Linux, and Windows;
- peer identity: GitHub and GitLab expose the same tag object, peeled commit,
  checksum manifest, signature, and provenance bytes;
- installed product: Homebrew Cask `aigw 0.1.0` at
  `/opt/homebrew/Caskroom/aigw/0.1.0/aigw`, accepted as a notarized Developer ID
  application;
- retained user state: Keychain storage is available, Claude selects
  `ucloud-claude-fable-5-1`, Codex selects `ucloud-gpt-6-astra`, and current
  `status`, `check`, and `doctor` observations pass;
- transition evidence: GitHub workflow `35445819802` exercised
  `0.1.0-rc.118` to `0.1.0`, rollback, and forward recovery on macOS, Linux,
  and Windows; published-artifact verification passed in GitHub jobs
  `35445069751`, `35445072366`, and `35445075077`, and GitLab pipeline `7538`.

The preceding RC executable, portable installation directory, and rollback copy
are absent. Historical signed release records remain chronology, not an active
installation or compatibility path.

## Initial semantic inventory

The first repository-wide inventory establishes the surfaces that later closures
must preserve or deliberately remove:

- the public command graph contains one `aigw` composition root and the setup,
  selection, readiness, credential, client, installation, recovery, catalogue,
  and provider-administration journeys listed by `aigw --help`;
- the production graph contains 43 Go packages. Its principal dependency
  boundaries run from CLI adapters into configuration, presentation, process,
  secrets, synchronization, and transaction owners; the architecture gate
  currently reports no undeclared package or import edge;
- focused behavior is co-located with its semantic owner, while cross-command,
  client, upgrade, and release journeys live in explicitly named acceptance
  packages rather than in a second implementation;
- repository configuration has distinct owners for architecture, coverage,
  dependency policy, Go analysis, size, Markdown, secret scanning, TOML,
  release construction, and the CUE CI graph. GitHub and GitLab workflow files
  are generated projections of that CI graph;
- current documentation has one index and distinct architecture, concepts,
  decisions, experience, governance, guides, operations, research, history, and
  legal domains; every current document below `docs/` has an inbound tracked
  link;
- the tracked repository contains no `evidence`, `claims`, `chronicle`,
  `.code-memory`, or `.ethos/state` directory. Serena state and installed Node
  packages are ignored Work Lane-local development state;
- after the stable delivery was accepted, both peer proposal refs were removed,
  local and peer `dev` converged on the signed object `b2fbec3a`, the superseded
  `stable-macos-release` Work Lane was retired, and this Change now owns the sole
  active Work Lane.

This is an ownership inventory, not a claim that every current package or file
is already optimal. Task 1.4 carries the consumer-level deletion audit; later
journeys may still prove that an apparent owner is redundant or misplaced.

## Migration Plan

1. Freeze and inventory the current accepted product, published bytes, user journeys, tracked carriers, dependencies, and residue.
2. Repair and verify each product journey while retaining AIGW 0.1.0 as the working baseline.
3. Reorganize source and tests around the verified semantic owners; delete superseded material in the same closure.
4. Consolidate quality and CI projections, then upgrade direct dependencies under the complete graph.
5. Rewrite current documentation from the accepted design and verify navigation and rendering.
6. Run exact-HEAD, native, published-artifact, installation, performance, and residue acceptance; publish only a new version when product bytes change.
7. Archive this Change and retire its proposal and Work Lane through ETHOS after every task is evidenced.
