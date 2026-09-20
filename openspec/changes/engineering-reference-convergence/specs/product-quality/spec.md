# Spec Delta

## ADDED Requirements

### Requirement: Engineering-reference quality is demonstrated by behavior

AIGW SHALL qualify as an engineering reference only when its necessary product behavior is expressed through cohesive owners, narrow interfaces, deterministic configuration, observable failure semantics, and tests capable of disproving the design. Additional abstractions, frameworks, rules, documents, or generated artifacts SHALL NOT count as quality unless they remove greater accidental complexity or protect a named risk.

#### Scenario: An implementation is proposed as a reference pattern

- **WHEN** a maintainer evaluates a product path for reference quality
- **THEN** the path SHALL demonstrate its invariant through public behavior, focused adversarial tests, and a reproducible contributor journey
- **AND** every retained abstraction and dependency SHALL identify the responsibility or maintenance cost it removes.

#### Scenario: A simpler complete design exists

- **WHEN** two implementations satisfy the same supported behavior and evidence obligations
- **THEN** AIGW SHALL retain the design with fewer authorities, states, dependencies, and failure modes
- **AND** delete the superseded implementation and its unconsumed tests, configuration, and documentation.

### Requirement: Quality constraints are comprehensive and proportionate

The repository quality graph SHALL cover every tracked source, test, configuration, documentation, schema, workflow, and generated projection with the mature native tool for each concern where it provides net value. Numeric limits SHALL be derived from observed risk and reviewed distributions; a tighter number SHALL be adopted only when it improves maintainability without fragmenting coherent logic or encouraging cosmetic restructuring.

#### Scenario: A quality threshold is tightened

- **WHEN** a maintainer proposes a lower size, complexity, nesting, parameter, coverage, or performance threshold
- **THEN** the proposal SHALL include current distribution, affected semantic owners, false-positive cost, and remediation path
- **AND** the accepted limit SHALL preserve coherent domain expression.

#### Scenario: A tracked format has no effective gate

- **WHEN** repository inventory finds a current format or generated projection outside the executable quality graph
- **THEN** the existing quality authority SHALL add the appropriate mature validator or explicitly remove the unsupported carrier
- **AND** a hand-written duplicate checker SHALL not be introduced when a maintained tool supplies the contract.

## MODIFIED Requirements

### Requirement: Portable exact-version CI bootstrap

GitLab Linux bootstrap SHALL consume the exact Mise image version and digest
from the CUE authority, prepare the complete operating-system capability
closure required by the repository's declared execution graph, and install the
repository-locked tool graph. It SHALL also own required execution modes such as
CGO enablement. Every Linux job SHALL inherit that owner rather than duplicate
system-package or tool installation, substitute another image or installer, or
weaken integrity and transport policy.

#### Scenario: The repository graph needs a Linux host capability

- **WHEN** a locked tool, trust check, native compiler path, or race-enabled test
  needs an operating-system capability absent from the selected Linux image
- **THEN** the CUE-owned Linux toolchain SHALL declare and install the minimal
  complete package set before invoking Mise or the repository graph
- **AND** all GitLab Linux jobs SHALL inherit the same preparation without
  job-local copies, alternate images, or unbounded retries
- **AND** jobs that exercise CGO-dependent behavior SHALL explicitly enable CGO
  through the same CUE projection.

#### Scenario: Transient HTTP transport failure

- **WHEN** an installer or asset transfer encounters a transient transport error
- **THEN** the native distribution client may retry within its bounded policy
- **AND** unresolved transport or integrity failure stops the job without
  claiming bootstrap success or substituting an unverified tool
- **AND** a retry retains the selected lock and integrity requirements.
