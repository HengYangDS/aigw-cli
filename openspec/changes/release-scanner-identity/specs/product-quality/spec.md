## ADDED Requirements

### Requirement: Dependency scanning runs only the locked scanner image

Dependency scanning SHALL run only the scanner executable whose SHA-256 equals
the asset that the source's Mise lock selects for the host platform and which
reports the locked version, and the scan SHALL observe that same executable.

#### Scenario: The scanner on PATH is not the locked image

- **WHEN** the resolved scanner's SHA-256 differs from the lock's asset for the
  host platform, it reports another version, or its image changes between the
  version report and the scan
- **THEN** the scan SHALL fail before writing either normalized report
- **AND** release construction SHALL NOT start GoReleaser, SBOM generation or
  signing.

### Requirement: Dependency reports name their scanner

Both dependency reports SHALL name the scanner's tool, version, platform,
SHA-256 and host-independent arguments, and each scan SHALL retain a local
receipt of its exact invocation and its raw report's digest.

#### Scenario: A scan names its scanner

- **WHEN** a scan runs with the locked image
- **THEN** both reports SHALL name the same scanner, with the source root and
  the evidence directory written as `{source}` and `{evidence}`
- **AND** the receipt SHALL record the executable's path and SHA-256, its
  version report, the exact arguments and the raw report's SHA-256.

### Requirement: Release provenance binds the locked scanner

Release provenance SHALL list the locked scanner asset that schema 2 dependency
reports name, verification SHALL refuse reports that name another image, and
schema 1 reports SHALL name no scanner and contribute no scanner dependency.

#### Scenario: A release names its scanner

- **WHEN** release construction writes schema 2 reports
- **THEN** the provenance SHALL list the lock's asset URL and SHA-256 for the
  scanner they name.

#### Scenario: An earlier release is verified

- **WHEN** verification reads schema 1 reports that name no scanner
- **THEN** the provenance SHALL include no scanner dependency and SHALL verify
  as before
- **AND** a schema 2 report without a scanner, or reports that name different
  scanners, SHALL be refused.
