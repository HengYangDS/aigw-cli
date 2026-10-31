## Purpose

Define AIGW's own installation, update and rollback: the files an installation
describes, the resources an update owns, and the boundary between replacing
the program and reconciling clients.

## ADDED Requirements

### Requirement: Windows path derivation preserves the selected namespace

Derived configuration, data, credential, client-settings and installation paths
SHALL retain the namespace of the explicitly supplied Windows base. Appending
AIGW-owned path components MUST NOT collapse a UNC or extended-length prefix
into a drive-rooted path. Derivation SHALL remain independent of the observer's
operating system and SHALL NOT access a network share to construct its name.

#### Scenario: A user directory is on a network share

- **WHEN** Windows environment paths identify a UNC share or an extended-length
  UNC or drive namespace
- **THEN** every derived AIGW path retains that namespace and base location
- **AND** native Windows path construction agrees with the derived result for
  the supported base forms.

### Requirement: Portable installation describes its own current files

`aigw installation` SHALL describe the invoked portable program from current
files without mutation, credential reads, valid Account configuration, or
executing a retained program. Versioned JSON SHALL bind absolute command and
payload paths, version, size, and SHA-256. A rollback copy SHALL use the same
identity model; absence SHALL be explicit. Unreadable or nonregular files SHALL
fail at their boundary. No persisted registry or checkout-only receipt SHALL
supply the result.

#### Scenario: Observe an installed program without source or configuration

- **GIVEN** a portable installation outside its source checkout
- **AND** Account configuration is absent or malformed
- **WHEN** the operator runs `aigw installation --json`
- **THEN** it reports the current program's file identity without modifying any
  file, reading credentials or starting a client
- **AND** a missing rollback copy is represented as `null`.

#### Scenario: Observe program replacement and rollback

- **WHEN** the operator observes an installation after update or rollback
- **THEN** the result reflects the files actually present at those paths
- **AND** install, update, rollback, uninstall and observation use one rollback
  path rule rather than parallel naming implementations.

### Requirement: Local candidate identity is verified before a no-op result

An explicitly selected local portable candidate SHALL pass checksum and
target-layout admission even when its version equals the running version.
Equal version labels SHALL NOT substitute for equal program bytes. Admission
and comparison SHALL preserve the current executable and retained predecessor.

#### Scenario: Reapply the exact current program

- **WHEN** a verified same-version candidate contains the current executable's
  exact bytes
- **THEN** update reports an already-matching program without executing or
  replacing it, changing its predecessor, or consulting a remote source.

#### Scenario: Reapply an unverified or different same-version program

- **WHEN** the archive, checksum, target program, or current executable is
  unavailable or invalid, or the verified candidate's program bytes differ
- **THEN** update fails without reporting a verified no-op
- **AND** current and retained programs remain unchanged.

### Requirement: Repeated portable installation preserves rollback identity

Portable installation SHALL compare executable bytes before rotating the
predecessor. Identical reinstall SHALL preserve both program identities and MAY
restore executable permission; a fresh install SHALL create no predecessor.
Installation SHALL preserve shell configuration and `PATH`, identify the
installed executable, and explain direct-path invocation before assuming name
resolution.

#### Scenario: Install outside the command search path

- **GIVEN** the destination directory is absent from `PATH`
- **WHEN** portable installation succeeds
- **THEN** the result identifies the installed path and explains that `PATH`
  remains unchanged
- **AND** the operator can invoke that executable directly without changing
  shell configuration or relying on another installation.

#### Scenario: Reinstall an unchanged downloaded program

- **GIVEN** a portable executable is installed from another path
- **WHEN** the same executable bytes are installed again
- **THEN** the current program remains in place
- **AND** the predecessor retains its prior bytes or remains absent.

### Requirement: Program replacement has an explicit client reconciliation boundary

Update and rollback SHALL activate program bytes without silently changing
client settings or schema, then direct reconciliation through `aigw sync`.
Rollback SHALL first qualify the exact retained program against an isolated
configuration copy without credentials or client discovery. Failure SHALL
preserve both programs and operator configuration. Compatibility restoration
SHALL be explicit; release qualification SHALL use the real predecessor and
retained client state.

#### Scenario: The predecessor cannot read the current configuration

- **WHEN** the current configuration contains a capability unknown to the
  retained program
- **THEN** program rollback fails before replacing either program file
- **AND** the error identifies configuration compatibility rather than claiming
  that the previous program became active
- **AND** after an explicit compatible configuration restoration, rollback can
  activate the predecessor and that predecessor can export the configuration.

#### Scenario: A replacement changes the credential helper contract

- **WHEN** an operator activates another program version
- **THEN** the active program's explicit synchronization SHALL reconcile its
  AIGW-owned client projection before the operator resumes client work
- **AND** lifecycle acceptance SHALL execute the projected helper rather than
  infer its arguments from the candidate implementation.

#### Scenario: A supported release pair retains enabled clients

- **WHEN** a configured client remains enabled through upgrade, rollback and
  re-upgrade between a qualified predecessor and successor
- **THEN** each program replacement SHALL preserve the AIGW configuration bytes,
  including explicit client locations and selected Routes
- **AND** real-client acceptance SHALL check the exact active executable and
  complete a request through that retained integration after each transition.

### Requirement: Online update owns its temporary resources and preserves failure causes

An online update SHALL own all peer download directories within one operation
workspace and attempt to remove it on every return, preserving unrelated files
and the installation outcome. Failures SHALL keep their original causes, while
public diagnostics expose only safe state and recovery guidance. Forge
subprocesses SHALL disable native prompts explicitly. Each peer's checksum
manifest SHALL be authenticated against the embedded release public key before
checksums, extraction, or execution.

#### Scenario: Release construction binds one public key

- **WHEN** release construction embeds the release public key
- **THEN** it SHALL freeze that key once and use the same value to verify the
  actual checksum signature
- **AND** downloaded metadata SHALL NOT supply trust
- **AND** reported failures SHALL keep exact owned paths for internal callers.

#### Scenario: A checksum-consistent candidate lacks valid authentication

- **WHEN** an online peer supplies an archive and matching checksum manifest
- **AND** its signature is absent, malformed, signed by another key, or bound
  to different manifest bytes
- **THEN** the updater SHALL reject it before any candidate execution
- **AND** the current program and retained rollback bytes SHALL stay unchanged
- **AND** its exact owned download workspace SHALL be removed.

#### Scenario: Signing inputs change after release admission

- **WHEN** release construction froze the public key embedded in its programs
- **AND** another key signs the actual checksum manifest
- **THEN** construction SHALL reject the output before replacing accepted assets
- **AND** the prior accepted output SHALL remain unchanged.

#### Scenario: A release peer is temporarily unavailable

- **WHEN** a configured peer fails during metadata or asset transport
- **THEN** the updater SHALL apply the same unavailability classification for
  GitLab and GitHub and continue with another admitted peer
- **AND** if every peer is unavailable, the error SHALL retain every peer's cause
- **AND** authentication or integrity failures SHALL remain terminal.

#### Scenario: Workspace cleanup fails

- **WHEN** the operation's workspace cannot be fully removed
- **THEN** the error SHALL preserve its exact workspace and cleanup cause for
  internal inspection together with any preceding failure
- **AND** public output SHALL omit private paths and raw subprocess diagnostics
- **AND** if replacement completed, the diagnostic SHALL state that the program
  was updated rather than imply an unchanged or rolled-back installation.

#### Scenario: Replacement staging cleanup fails after activation

- **WHEN** a candidate or rollback program was activated but its staging
  directory cannot be removed
- **THEN** the program transition SHALL retain both its completed activation
  result and the cleanup failure
- **AND** public output SHALL report the active replacement and incomplete
  cleanup without claiming that program replacement failed.

### Requirement: Failure recovery retains owned-resource cleanup outcomes

Credential writes, explicit client verification and coverage execution SHALL
retain their original failure when resource cleanup also fails. Cleanup SHALL
name the exact owned resource and SHALL NOT silently report success.

#### Scenario: A credential replacement fails before publication

- **WHEN** replacement cannot rename its staging file and staging removal also fails
- **THEN** the original credential SHALL remain unchanged and both failure causes
  SHALL be observable with the exact retained staging name
- **AND** once replacement commits, cleanup SHALL NOT remove a subsequently
  recreated file at the old staging name.

#### Scenario: Client verification creates additional output

- **WHEN** a client verification command writes into its working directory
- **THEN** that directory SHALL be private to the verification operation and
  reclaimed on success or failure
- **AND** cleanup failure SHALL retain the invocation error and exact workspace.

#### Scenario: Coverage measurement succeeds but route cleanup fails

- **WHEN** a coverage command cannot remove its owned temporary route
- **THEN** its exit status SHALL indicate failure and its diagnostic SHALL name
  that file without suppressing any preceding test failure.

#### Scenario: A guarded file write fails during staging

- **WHEN** writing, setting permissions or syncing an owned temporary file fails
- **THEN** the commit helper SHALL close its handle exactly once and retain any
  close failure alongside the original error
- **AND** the writer SHALL remove only its failed staging file and report any
  cleanup failure without modifying unrelated files
- **AND** after a successful rename, it SHALL relinquish the old staging name.
