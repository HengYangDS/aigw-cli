## MODIFIED Requirements

### Requirement: Warnings are owned failures

Repository-owned source and configuration warnings SHALL block qualification
until resolved at their semantic owner. Supply-chain security findings SHALL
remain nonblocking and fully disclosed under the
[release evidence contract](../../../../../docs/governance/change-and-release-policy.md#reproducible-assets).
Scanner execution, evidence completeness and artifact validity SHALL remain
required. Successful subprocess exit SHALL NOT erase captured diagnostics or
establish warning-free acceptance.

#### Scenario: A supported gate emits a warning

- **WHEN** the warning is attributable to repository source or configuration
- **THEN** the gate fails until the cause is removed
- **AND** a blanket filter, baseline, or ignored exit code is not accepted as
  the repair.

#### Scenario: Native validation distinguishes advice from failure

- **WHEN** OpenSpec reports `INFO`, `WARNING`, `ERROR`, or a failed summary
- **THEN** the repository gate fails and reports every finding without filtering
  it, even if the native validator classifies the item as informational
- **AND** unknown severity, malformed evidence, or diagnostic output failure
  also fails admission.

#### Scenario: Successful child diagnostics remain observable

- **WHEN** a supported subprocess writes stdout and stderr before exiting zero
- **THEN** two-stream capture SHALL preserve both bounded byte strings separately
- **AND** native journey evidence SHALL expose redacted diagnostic output
- **AND** a stdout-only API MAY retain its explicit result-only success contract.

#### Scenario: Successful quality tool emits a native diagnostic

- **WHEN** a supported CI tool exits zero with explicit native stderr warning or error markers
- **THEN** the common command runner SHALL reject qualification using the existing diagnostic classifier
- **AND** truncated diagnostic evidence SHALL prevent a warning-free success claim
- **AND** ordinary progress and zero-finding count fields SHALL remain admissible
- **AND** complete diagnostics SHALL remain observable without an additional parser or warning authority.

#### Scenario: A native tool exits before diagnostic pipe writes drain

- **WHEN** a native CI tool immediately exits after writing stderr
- **THEN** its complete diagnostic stream SHALL remain observable after exit
- **AND** standard output SHALL retain its live execution contract
- **AND** capture, read, replay or cleanup failure SHALL prevent qualification
- **AND** exact temporary capture files SHALL be reclaimed without changing unrelated files.

#### Scenario: Successful native inference emits a capability warning

- **WHEN** Codex, Claude or Hermes exits zero but reports an explicit native stderr warning, error or traceback
- **THEN** public verification SHALL report incomplete qualification in its own words, and SHALL show the client's
  text only on request, bounded and with credentials redacted
- **AND** verification SHALL NOT write a successful checkpoint or claim unproved inference
- **AND** missing native model metadata SHALL remain visible without fabricated catalog entries
- **AND** ordinary version, session and progress stderr SHALL NOT become a warning failure.

#### Scenario: Native Codex rejects an inherited temporary-home relationship

- **WHEN** catalog or verification runs under an inherited temporary root
- **THEN** its native child SHALL receive an owned temporary root that does not contain its selected Codex home
- **AND** parent environment and user configuration SHALL remain unchanged
- **AND** native catalog warnings SHALL prevent qualification rather than silently discard diagnostics
- **AND** failed and successful probes SHALL reclaim their exact owned workspace.

#### Scenario: Native rule conformance exceeds its aggregate execution budget

- **WHEN** independent positive and negative Go rule fixtures qualify the same policy
- **THEN** one bounded native batch SHALL execute every isolated fixture package
- **AND** valid fixtures SHALL have no findings and each invalid fixture SHALL retain its intended diagnostic
- **AND** root resolution, format and type-failure contracts SHALL remain independently exercised
- **AND** policy thresholds, native diagnostics and the original package timeout SHALL NOT be weakened.
