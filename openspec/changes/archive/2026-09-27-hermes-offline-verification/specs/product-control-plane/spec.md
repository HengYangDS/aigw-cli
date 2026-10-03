# Spec Delta

## ADDED Requirements

### Requirement: Hermes verification excludes unrelated update services

When AIGW verifies an enabled Hermes Client Binding, it SHALL invoke the native
client against the selected Route without requiring the client's upstream
software-update service. The disposable verification environment SHALL preserve
the operator's Hermes configuration and credentials, and version-probe failures
SHALL be classified without exposing raw vendor output or private paths.

#### Scenario: Vendor update service is unavailable

- **WHEN** the selected inference endpoint is healthy but the Hermes software
  update service is unavailable
- **THEN** AIGW can observe the native client version and complete one bounded
  request for the selected Model without contacting the update service
- **AND** no user's Hermes configuration or credential is changed.

#### Scenario: Hermes version probe fails

- **WHEN** the native Hermes version probe times out or exits unsuccessfully
- **THEN** verification fails with the corresponding cause category
- **AND** it does not report successful inference, retry the probe, prompt for
  credentials, or disclose raw stderr, Token material, or private paths.

### Requirement: Pre-tag artifact acceptance binds to signed source

Before a stable tag exists, AIGW SHALL accept an explicitly selected artifact
candidate only when its trusted signature and canonical provenance match the
current signed source commit and locked inputs. Candidate mode SHALL require a
clean checkout and reject a selected or same-version local release tag. Release
verification and publication SHALL still require a signed tag. Supplied
artifact bytes SHALL not be rebuilt or replaced.

#### Scenario: Signed candidate precedes its release tag

- **WHEN** an operator selects candidate acceptance for a signed artifact
  matrix before the stable tag exists
- **THEN** AIGW verifies the artifact signer, signed HEAD, provenance, and
  supplied native bytes before running client and lifecycle acceptance.

#### Scenario: Candidate mode could bypass a release tag

- **WHEN** a release tag is selected or a same-version local tag exists
- **THEN** candidate acceptance is rejected
- **AND** release verification and publication still require tag-signature
  validation.

## MODIFIED Requirements

### Requirement: Terminal local release readiness

A local release candidate SHALL require complete canonical intent, admitted
stable direct dependencies, faithful quantitative evidence, passing native
source gates, and a reproducible installable matrix. Hosted CI and delivery
SHALL consume, not gate, that accepted result. Change tasks SHALL close
implementation and candidate acceptance before archive; canonical specs and
design SHALL retain publication, installed-asset and lane-retirement duties
until proved.

#### Scenario: A stable direct dependency update is available

- **WHEN** the declared Go toolchain reports a newer stable direct module
  version
- **THEN** `go.mod` and `go.sum` SHALL be refreshed together
- **AND** the complete native source gate SHALL pass before integration.

#### Scenario: Only an unneeded transitive update is reported

- **WHEN** the module query reports a newer transitive version but `go mod why`
  shows the main module does not need it
- **THEN** AIGW SHALL leave selection with the direct dependency owner
- **AND** SHALL NOT add an explicit pin merely to display the newest version.

#### Scenario: A canonical document contains placeholder authority

- **WHEN** a specification purpose remains `TBD` or describes generation
  history
- **THEN** terminal closeout SHALL fail until the purpose states current
  product semantics directly.

#### Scenario: Protected branches are projected

- **WHEN** a proven accepted local `main` is selected for one peer
- **THEN** its signature and exact object SHALL be verified before publication
- **AND** remote `main` and `dev` SHALL advance atomically to that object
- **AND** the other peer SHALL not be queried or mutated.

#### Scenario: External delivery follows local readiness

- **WHEN** source has passed exact-HEAD proof and authorized integration
- **THEN** native hosted verification and each optional peer MAY independently
  consume that exact accepted result
- **AND** accepted-ref and tag CI, published assets, installed lifecycle, and
  lane retirement SHALL each require their own post-archive evidence
- **AND** archive SHALL NOT claim those outcomes or block local acceptance when
  a peer is unavailable.

#### Scenario: A clean runner materializes npm tools

- **WHEN** source verification starts without an existing npm installation
- **THEN** bootstrap SHALL install the exact committed npm dependency graph with
  install scripts disabled
- **AND** direct and transitive selections SHALL remain bound to the lockfile
- **AND** registry signatures SHALL verify through the ecosystem verifier.

#### Scenario: The verified release is published

- **WHEN** source and artifact acceptance admit a release for publication
- **THEN** each selected Forge SHALL receive the same locally signed commit,
  annotated tag and immutable asset matrix without re-signing or reconstruction
- **AND** each peer SHALL verify its own publication independently.
