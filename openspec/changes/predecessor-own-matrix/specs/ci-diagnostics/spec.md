## MODIFIED Requirements

### Requirement: Published lifecycle verification selects its predecessor

When a peer's read-only release verification runs the native lifecycle against
an existing release's published bytes, it SHALL consume an explicitly selected
published predecessor of 0.7.0 or later from the same peer, verify that
predecessor by its own signed matrix and its source, and run the upgrade and
rollback journeys from it. Its own matrix SHALL be the files that its signed
manifest names. It SHALL NOT rebuild, retag, sign or publish either release.

#### Scenario: A maintainer selects a published predecessor

- **WHEN** release verification selects native lifecycle and a published
  predecessor tag for an existing release
- **THEN** native acceptance SHALL download that predecessor from the same peer
  and verify its signatures and source before product execution
- **AND** the lifecycle journeys SHALL run against the selected release's
  published archive.

#### Scenario: No predecessor is selected

- **WHEN** release verification selects native lifecycle without a published
  predecessor
- **THEN** the lifecycle journeys SHALL fail before product execution
- **AND** the run SHALL NOT pass as lifecycle evidence.

#### Scenario: A predecessor older than 0.7.0 is selected

- **WHEN** native acceptance selects a published predecessor older than 0.7.0,
  by tag, by artifacts or in an input archive
- **THEN** it SHALL refuse that predecessor before product execution and name
  0.7.0 as the oldest one, because an older release cannot read `aigw.toml`.

#### Scenario: The predecessor was published with another platform matrix

- **WHEN** the selected predecessor's signed manifest names archives for
  platforms that the current release no longer publishes
- **THEN** native acceptance SHALL verify the predecessor against the files
  that its manifest names, with their digests, its signature and its rebuilt
  provenance, not against the current release's platform matrix
- **AND** it SHALL refuse a predecessor whose manifest does not name at least
  one platform archive of its version followed by its dependency reports and
  its provenance, or whose directory holds a file that the manifest does not
  name or lacks one that it names.
