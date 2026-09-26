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
