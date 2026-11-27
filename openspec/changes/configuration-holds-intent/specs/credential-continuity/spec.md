## MODIFIED Requirements

### Requirement: The credential reader decodes a frozen view

The credential reader SHALL decode from `aigw.toml` only each provider's ID and endpoints and each client's enabled
state, so that an enabled client is served by its provider's Token. It SHALL ignore `version`, every other field and
unknown protocol keys, match fingerprints exactly and check no client's selection. The file SHALL keep its name across
schema 9, because every reader since 0.7.0 reads a client without `authentication` as one that its provider's Token
serves.

#### Scenario: Unknown fields and a newer version are ignored

- **GIVEN** `aigw.toml` holds fields this reader does not know and
  `version = 99`
- **WHEN** a client calls the reader with a fingerprint that matches one of its
  providers' endpoints
- **THEN** the reader SHALL return that provider's Token
- **AND** its standard output SHALL contain only the Token.

#### Scenario: A field outside the view is malformed

- **WHEN** a field outside the view holds a value the full program would
  reject, such as a label of the wrong type or an unknown capability
- **THEN** the reader SHALL still return the matching Token
- **AND** a document that is not valid TOML SHALL fail at the parse stage, as
  "Credential configuration failures name the reader and stage" requires.

#### Scenario: An unknown protocol key is ignored

- **WHEN** a provider's endpoints hold a protocol key this reader does not know
- **THEN** the reader SHALL ignore that key
- **AND** it SHALL still return the Token for a fingerprint that matches an
  endpoint of a protocol it knows.

#### Scenario: The reader refuses a client it cannot serve

- **WHEN** the named client is disabled, or no provider endpoint matches the fingerprint
- **THEN** the reader SHALL return no Token and SHALL read no credential.

#### Scenario: The reader checks no selection

- **GIVEN** a running session's credential command carries the fingerprint of
  a provider endpoint that its client no longer selects
- **WHEN** the session calls the reader while that provider and endpoint exist
- **THEN** the reader SHALL return that provider's Token
- **AND** once the provider or that endpoint is gone, it SHALL return no Token.

#### Scenario: A client section still holds authentication

- **GIVEN** an enabled client's section in `aigw.toml` still holds `authentication`, as a version 8 file can
- **WHEN** that client calls the reader with a fingerprint that matches one of its providers' endpoints
- **THEN** the reader SHALL return that provider's Token, whatever `authentication` holds.

#### Scenario: A running session keeps its Token across schema 9

- **GIVEN** a running session's credential command names the reader of a release from 0.7.0 to 0.11.0
- **WHEN** the operator edits `aigw.toml` to schema 9, keeping that provider and endpoint, and `aigw sync` runs
- **THEN** that command SHALL keep returning its provider's Token without a client reload.
