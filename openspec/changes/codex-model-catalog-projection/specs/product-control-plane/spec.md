## MODIFIED Requirements

### Requirement: Transactional and inspectable projection

AIGW SHALL prepare and validate every selected client target before mutation,
apply only owned marked projections, and compensate a failed multi-target
operation without overwriting a newer writer. A dry run MUST expose the planned
actions without reading credentials, authenticating, starting a client, or
changing files.

For a provider-prefixed Codex model whose base slug has exactly one match in the
installed client's bundled model table, AIGW SHALL derive a complete catalog
mirror with namespace aliases, bind it to the exact client version and
executable digest, and preserve the selected wire model ID. The catalog,
configuration reference, and sidecar SHALL share the existing compensated
projection transaction. AIGW SHALL NOT adopt a user-authored catalog or a
foreign file at its managed path.

#### Scenario: Multi-target projection fails

- **WHEN** any selected target cannot be prepared or committed
- **THEN** AIGW SHALL report failure and restore only artifacts whose postimage
  still belongs to the failing transaction

#### Scenario: Inspect a dry run

- **WHEN** an operator runs synchronization in dry-run JSON mode
- **THEN** AIGW SHALL return the target and action plan without a credential or
  client-lifecycle side effect

#### Scenario: A provider prefixes a known Codex model

- **WHEN** exactly one suffix of the selected model ID matches the client's own
  bundled model table
- **THEN** AIGW SHALL project aliases for the complete bundled table under the
  derived namespace
- **AND** the provider SHALL continue receiving the original selected model ID.

#### Scenario: Client identity or catalog ownership is not provable

- **WHEN** the installed client changes, generation fails, the managed bytes
  drift, or a foreign catalog occupies the managed path
- **THEN** AIGW SHALL refuse unsafe reuse, adoption, overwrite, or deletion
- **AND** SHALL preserve user-owned state.

#### Scenario: A real client qualifies the projection

- **WHEN** a contributor runs the tracked catalog verification command
- **THEN** it SHALL record client version and executable digest
- **AND** SHALL prove the adapted model resolves like its base slug while an
  unknown model retains fallback behavior
- **AND** SHALL send no model request and alter no persistent Codex home.
