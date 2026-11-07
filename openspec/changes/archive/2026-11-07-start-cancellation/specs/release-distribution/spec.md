## MODIFIED Requirements

### Requirement: Release construction owns only its operation resources

Release work SHALL own its workspace, private replacement backup, processes,
and cleanup. Commands SHALL preserve failure causes, directories, environments,
parent output handles, bounded captures, and complete streamed channels.
Interruption SHALL propagate through every stage; return SHALL terminate only
owned process-group or Job descendants. Signal interception SHALL be scoped to
active non-interactive work and restore prior behavior.

#### Scenario: Existing output has an unrelated neighboring file

- **WHEN** a release replaces an existing output directory
- **THEN** files outside that directory and the operation's private workspace
  SHALL remain unchanged, regardless of their names
- **AND** failed publication SHALL restore the prior output; if restoration
  also fails, the error SHALL retain both causes and identify the preserved backup.

#### Scenario: Output was published but predecessor cleanup failed

- **WHEN** the new output is installed and removal of its private backup fails
- **THEN** the result SHALL report completed publication and the cleanup error
- **AND** the retained predecessor SHALL remain discoverable at the reported path.

#### Scenario: A build tool leaves a background descendant

- **WHEN** its direct process exits or the release command is interrupted
- **THEN** the process owner SHALL terminate descendants still in its native
  process group or Job and preserve unrelated processes
- **AND** cancellation, process exit and cleanup failures SHALL remain
  distinguishable to callers.

#### Scenario: A tool emits a large build log

- **WHEN** a release tool streams output larger than the captured-result budget
- **THEN** both output channels SHALL reach their caller-owned destinations
- **AND** the child SHALL receive the selected directory, environment and closed
  standard input rather than ambient interactive shell state.

#### Scenario: The host interrupts an isolated child invocation

- **WHEN** an interrupt or termination signal arrives while an owned child runs
- **THEN** its caller SHALL receive cancellation after owned-process cleanup
- **AND** returning from the invocation SHALL restore the previous host signal
  behavior rather than retaining a handler during later interactive input.

#### Scenario: A tool's deadline ends while it starts

- **WHEN** a release tool's deadline or cancellation ends after its process is
  created but before the process runs
- **THEN** the caller SHALL receive that deadline or cancellation with the
  start failure
- **AND** the stopped process SHALL be reaped through bounded cleanup.
