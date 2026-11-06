## ADDED Requirements

### Requirement: The self commands own the program lifecycle

AIGW's own program SHALL be installed, updated, rolled back, removed and
described through `aigw self install`, `aigw self update`, `aigw self rollback`,
`aigw self uninstall` and `aigw self status`, which replace `install`, `update`,
`update --rollback`, `uninstall` and `installation` without an alias.
`aigw self rollback` SHALL restore only the retained program, and
`aigw config rollback` only configuration and client configuration files.

#### Scenario: A former program command is run

- **WHEN** an operator runs `aigw install`, `aigw update`,
  `aigw update --rollback`, `aigw uninstall` or `aigw installation`
- **THEN** AIGW SHALL refuse it as an unknown command before any effect
- **AND** no hidden spelling SHALL run it
- **AND** `aigw self update --candidate` SHALL fail as an unknown flag, because
  `--archive` selects a local archive.

#### Scenario: Program and configuration rollbacks stay apart

- **WHEN** `aigw self rollback` completes
- **THEN** `aigw.toml`, its backup and every client configuration file SHALL be
  unchanged
- **AND** after `aigw config rollback` completes, the active and retained
  programs SHALL be unchanged.

### Requirement: Official archives embed only the public release source

An official build SHALL embed the public GitHub release origin and repository
as its only release source. A GitLab release peer SHALL be named only at run
time, through `AIGW_GITLAB_RELEASE_ORIGIN` and `AIGW_GITLAB_RELEASE_REPOSITORY`.

#### Scenario: A build environment names a GitLab release source

- **WHEN** release construction runs with either GitLab release variable set
- **THEN** it SHALL refuse before building and name the variable to unset.

#### Scenario: An operator updates from a GitLab peer

- **WHEN** `aigw self update` runs with both GitLab release variables set
- **THEN** it SHALL read that peer as well as the embedded GitHub source.

## MODIFIED Requirements

### Requirement: A local archive's identity is verified before a no-op result

An archive that `aigw self update --archive` selects SHALL pass checksum and
target-layout admission even when its version equals the running version.
Equal version labels SHALL NOT substitute for equal program bytes. Admission
and comparison SHALL preserve the current executable and retained predecessor.

#### Scenario: Reapply the exact current program

- **WHEN** a verified same-version archive contains the current executable's
  exact bytes
- **THEN** `aigw self update` reports an already-matching program without
  executing or replacing it, changing its predecessor, or consulting a remote
  source.

#### Scenario: Reapply an unverified or different same-version program

- **WHEN** the archive, checksum, target program, or current executable is
  unavailable or invalid, or the verified archive's program bytes differ
- **THEN** `aigw self update` fails without reporting a verified no-op
- **AND** current and retained programs remain unchanged.

### Requirement: Portable installation describes its own current files

`aigw self status` SHALL describe the invoked portable program from current
files without mutation, credential reads, valid configuration, or executing a
retained program. Versioned JSON SHALL bind absolute command and payload paths,
version, size, and SHA-256. A rollback copy SHALL use the same identity model;
absence SHALL be explicit. Unreadable or nonregular files SHALL fail at their
boundary. No persisted registry or checkout-only receipt SHALL supply the
result.

#### Scenario: Observe an installed program without source or configuration

- **GIVEN** a portable installation outside its source checkout
- **AND** the configuration is absent, malformed or not yet imported
- **WHEN** the operator runs `aigw self status --json`
- **THEN** it reports the current program's file identity without modifying any
  file, reading credentials or starting a client
- **AND** a missing rollback copy is represented as `null`.

#### Scenario: Observe program replacement and rollback

- **WHEN** the operator observes an installation after update or rollback
- **THEN** the result reflects the files actually present at those paths
- **AND** install, update, rollback, uninstall and observation use one rollback
  path rule rather than parallel naming implementations.

### Requirement: Program replacement has an explicit client reconciliation boundary

Update and rollback SHALL activate program bytes without silently changing
client settings or schema, then direct reconciliation through `aigw sync`.
Rollback SHALL first qualify the exact retained program against an isolated
configuration copy without credentials or client discovery. Failure SHALL
preserve both programs and operator configuration. Compatibility restoration
SHALL be explicit; release qualification SHALL use the real predecessor and
retained client state.

#### Scenario: The predecessor cannot read the current configuration

- **WHEN** the configuration staged for the retained program contains a field
  or capability unknown to that program
- **THEN** program rollback fails before replacing either program file
- **AND** the error identifies configuration compatibility rather than claiming
  that the previous program became active
- **AND** after an explicit compatible configuration restoration, rollback can
  activate the predecessor and that predecessor can export the configuration.

#### Scenario: A replacement changes the credential helper contract

- **WHEN** an operator activates another program version
- **THEN** the active program's explicit `aigw sync` SHALL reconcile its
  AIGW-owned client configuration files before the operator resumes client work
- **AND** lifecycle acceptance SHALL execute the helper those files name rather
  than infer its arguments from the new implementation.

#### Scenario: A supported release pair retains enabled clients

- **WHEN** a configured client remains enabled through upgrade, rollback and
  re-upgrade between a qualified predecessor and successor
- **THEN** each program replacement SHALL preserve the AIGW configuration bytes,
  including explicit client locations and selected models
- **AND** real-client acceptance SHALL check the exact active executable and
  complete a request through that retained integration after each transition.

### Requirement: Program commands preview their effect

`aigw self install`, `aigw self update` and `aigw self uninstall` with
`--dry-run` SHALL report the program file, the version and the rollback copy
that they would write or remove. `aigw self update --dry-run` SHALL read only
public release metadata, and SHALL NOT download, verify or run an archive or
change a Keychain item. `aigw self rollback --dry-run` SHALL report the retained
program, its version and the configuration file that it would stage.

#### Scenario: An update is previewed

- **WHEN** `aigw self update --dry-run` runs while a newer release is available
- **THEN** it SHALL report the installed and the newer version
- **AND** no file, rollback copy or Keychain item SHALL change.

#### Scenario: An uninstall is previewed

- **WHEN** `aigw self uninstall --dry-run` runs
- **THEN** it SHALL report the program file and the rollback copy that it would
  remove
- **AND** both SHALL remain.

#### Scenario: A program rollback is previewed

- **WHEN** `aigw self rollback --dry-run` runs
- **THEN** it SHALL report the retained program, its version and the
  configuration file that it would stage, without running that program
- **AND** no file or Keychain item SHALL change.

#### Scenario: An uninstall meets a directory

- **WHEN** `aigw self uninstall` runs, with or without `--dry-run`, and its
  program path or its rollback copy's path holds a directory
- **THEN** it SHALL refuse before it withdraws any client or removes any file.

## RENAMED Requirements

- FROM: `### Requirement: Local candidate identity is verified before a no-op result`
- TO: `### Requirement: A local archive's identity is verified before a no-op result`
