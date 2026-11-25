## ADDED Requirements

### Requirement: Repository text is portable across supported hosts

Tracked text SHALL use deterministic encoding and line-ending semantics across
supported hosts. Repository-owned verification SHALL express host-independent
behavior with one semantic contract on macOS and Linux. A test SHALL
not infer a portable I/O failure from POSIX permission bits when the target host
does not implement that permission model.

#### Scenario: a native test proves a filesystem error boundary

- **WHEN** repository verification must exercise a filesystem operation failure
- **THEN** the fixture SHALL construct that failure deterministically on every supported host
- **AND** the same production error path SHALL be asserted without a platform skip

#### Scenario: text contains a byte-level defect

- **WHEN** tracked text contains CR line endings, trailing whitespace, or lacks a final newline
- **THEN** repository quality SHALL report the exact file and line

#### Scenario: a contributor uses another supported host

- **WHEN** the repository is cloned under another operating system, user, directory, or Forge
- **THEN** checkout, verification, build, installation, and release contracts SHALL remain discoverable and executable from repository-owned inputs

#### Scenario: a contributor uses another operating system

- **WHEN** Git checks out tracked text
- **THEN** line endings and executable semantics SHALL remain deterministic

#### Scenario: valid text uses a different readable spacing style

- **WHEN** Markdown or configuration uses semantically valid blank-line spacing
- **THEN** the repository-wide byte checker SHALL not reject it
- **AND** formatters, serializers, and review SHALL retain their own scoped authority

### Requirement: Native tests pass on every hosted macOS and Linux runner

The repository's tests SHALL pass on every host whose native job runs them,
including GitLab's Linux and macOS ARM64 hosts. A test of
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
  job-owned directories, as GitLab's Linux jobs do
- **THEN** a test that isolates Mise's configuration SHALL run the tools
  installed there and install none.

### Requirement: The selection tests run on every native platform of each Forge

The exhaustive selection tests SHALL run in every full local proof, and in each Forge's pipeline that verifies a pushed
commit, a merge or pull request, or a tag, once on every native platform that the Forge verifies: in the quality job on
that job's platform, and in the native job of every other platform. A failure of these tests on any platform SHALL fail
that pipeline.

#### Scenario: A proof runs without a remote

- **WHEN** a full proof runs on a host with no Forge configured
- **THEN** it SHALL run the exhaustive selection tests

#### Scenario: A Forge verifies a commit

- **WHEN** a Forge's pipeline verifies a pushed commit, a merge or pull request, or a tag on its macOS and Linux
  hosts
- **THEN** each of those hosts SHALL run the exhaustive selection tests exactly once in that pipeline
- **AND** the same platforms SHALL run them on the other Forge, generated from the same pipeline declaration

#### Scenario: A native job qualifies every quality tool

- **WHEN** a manual native job qualifies every repository quality tool on its host
- **THEN** that job SHALL run the exhaustive selection tests exactly once

#### Scenario: A native job accepts only prebuilt artifacts

- **WHEN** a native job accepts prebuilt release artifacts without source tests
- **THEN** it SHALL NOT run the exhaustive selection tests

## REMOVED Requirements

### Requirement: portable repository text

**Reason**: Its scenario for Windows volumes leaves with Windows support, and OpenSpec keeps every scenario of a modified requirement.

**Migration**: "Repository text is portable across supported hosts" keeps every other statement and scenario of it.

### Requirement: Native tests pass on every hosted platform

**Reason**: Its Windows readiness-record scenario leaves with Windows support.

**Migration**: "Native tests pass on every hosted macOS and Linux runner" keeps every other statement and scenario of it.
