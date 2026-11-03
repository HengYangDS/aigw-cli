## ADDED Requirements

### Requirement: Native tests pass on every hosted platform

The repository's tests SHALL pass on every host whose native job runs them,
including GitLab's Linux, Windows and macOS ARM64 hosts. A test of
host-specific behavior SHALL verify the outcome that the specification
declares for the host where it runs. A bound on a helper process SHALL stop a
hang, not a slow host.

#### Scenario: A host has no declared client layout

- **WHEN** a native client supply test runs on a host for which the repository
  declares no official client layout, such as GitLab's Linux ARM64 runner
- **THEN** it SHALL verify that the supply refuses the host before it creates
  a supply directory or exports a client
- **AND** a test that needs the supply's installers to start SHALL skip on that
  host and name the refusal.

#### Scenario: Windows briefly holds a readiness record

- **WHEN** a test reads the record that a helper process published to signal
  its readiness on Windows, and the host still holds the file
- **THEN** the read SHALL retry within a bounded time before the test fails.

#### Scenario: A loaded host slows a helper process

- **WHEN** a helper process that a test starts completes its work later on a
  loaded host than on an idle one
- **THEN** the test SHALL accept the helper's result within a bound that stops
  only a hang.

## MODIFIED Requirements

### Requirement: Tests keep Mise state private

A test that runs Mise SHALL direct Mise's state directory into the test's own
workspace, so that trust and tracking records for temporary checkouts never
reach the operator's Mise state. It SHALL keep the data and cache directories
that its caller selected and SHALL NOT install a tool, so the caller's
installed tools stay in use and the caller's tool store stays unchanged.

#### Scenario: A test runs Mise in a temporary checkout

- **WHEN** the test runs Mise in its temporary checkout
- **THEN** Mise's tracking record SHALL be written inside the test workspace.

#### Scenario: A hosted job selects Mise's tool directories

- **WHEN** a hosted job directs Mise's data and cache directories into
  job-owned directories, as GitLab's Linux and Windows jobs do
- **THEN** a test that isolates Mise's configuration SHALL run the tools
  installed there and install none.
