## ADDED Requirements

### Requirement: Explicit client boundary

AIGW SHALL admit Claude Code and Codex as its implemented client integrations.
Codex CLI and Codex Desktop SHALL share one Codex Home projection; AIGW MUST NOT
invent a separate Desktop adapter or mutate Desktop-only GUI settings. A future
client SHALL require a dedicated adapter with independently proven configuration,
credential, verification, rollback, and uninstall boundaries.

#### Scenario: Codex CLI and Desktop are installed together

- **WHEN** AIGW discovers the default Codex Home
- **THEN** it SHALL project the selected provider into that one shared
  `config.toml` without editing conversations or Desktop-only settings

#### Scenario: A future client is proposed

- **WHEN** a provider happens to support the future client's wire protocol
- **THEN** AIGW SHALL NOT treat that provider capability as adapter admission

#### Scenario: One admitted client is absent

- **WHEN** setup cannot discover an admitted client's required executable or
  configuration surface
- **THEN** AIGW SHALL leave that client untouched and configure only the
  discoverable admitted clients

#### Scenario: Hermes is present without an admitted Adapter

- **WHEN** Hermes is installed but no Hermes Adapter has completed admission
- **THEN** AIGW SHALL NOT inspect, configure, launch, or mutate Hermes

### Requirement: Host-independent policy paths

Repository-relative policy paths SHALL use one lexical grammar on every host.
Validation MUST reject POSIX roots, Windows drive, UNC or device roots,
backslashes, empty or dot segments, and parent traversal.

#### Scenario: Foreign-host absolute path enters policy

- **WHEN** a repository policy contains an absolute or parent-traversing path in
  syntax native to a different operating system than the current runner
- **THEN** validation SHALL reject it with the same result on macOS, Linux, and
  Windows

### Requirement: Current stable build supply chain

AIGW source and CI SHALL use current stable supported build inputs rather than
retain obsolete versions as compatibility targets. `go.mod` SHALL own the exact
Go compiler and module graph, while the CI gate policy SHALL own immutable
GitHub Action revisions projected into workflows.

#### Scenario: A stable dependency or CI action release is available

- **WHEN** a release candidate is prepared and a newer stable supported input
  exists
- **THEN** the candidate SHALL advance the owning source, resolve the resulting
  graph, and pass native and repository gates before publication

#### Scenario: Reproducibility is required

- **WHEN** CI or release packaging executes after the stable-input upgrade
- **THEN** it SHALL reproduce the exact versions recorded by the owning source
  without treating the superseded versions as an accepted compatibility floor

## MODIFIED Requirements

### Requirement: Portable source

Product source SHALL NOT encode a personal identity, home directory, private
Forge coordinate, local checkout path, credential, signing key, signing
program, or trust anchor. CI SHALL convert Forge-supplied trust material into a
runner-temporary file only at the checker boundary, without logging or
committing that material.

#### Scenario: Hosted CI receives trust content

- **WHEN** a Forge exposes allowed-signers content as a CI variable
- **THEN** the workflow SHALL write it to a runner-temporary file and pass only
  that path to provenance verification

#### Scenario: Build in another team environment

- **WHEN** the repository is cloned under a different user, directory, or Forge
- **THEN** ordinary build and source verification SHALL not require the original
  contributor's machine, path, account, key, fingerprint, or remote
