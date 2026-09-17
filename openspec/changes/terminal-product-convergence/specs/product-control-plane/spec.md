## MODIFIED Requirements

### Requirement: Release construction owns only its operation resources

Release construction and native acceptance SHALL retain tool and cleanup failure
causes and identify the exact workspace requiring cleanup. Replacing release
output SHALL use an operation-owned private backup rather than deleting a
predictably named sibling of the requested output.

Release commands SHALL propagate interruption through construction, verification
and publication. Non-interactive tools SHALL share the product's process owner,
preserve explicit working directories and environments, and keep output handles
in the parent. Returning from an invocation SHALL terminate its remaining owned
process-group or Job descendants without selecting unrelated host processes.
Captured results SHALL retain their existing size limits; streamed build logs
SHALL preserve both output channels without applying those capture limits.
The process owner SHALL observe host interruption only during an active
non-interactive invocation and restore the previous signal behavior on return.
Interactive input SHALL retain its own signal handling outside that boundary.

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
