## MODIFIED Requirements

### Requirement: Transactional and inspectable projection

AIGW SHALL atomically prepare and validate selected client targets, mutate only
owned projections, compensate failures only while postimages remain
transaction-owned, and expose credential-free, side-effect-free dry runs. For a
Codex model whose family is in Codex's bundled table, it SHALL derive an alias
of that family's entry bound to client version and digest, preserve the wire
model ID, and update all three atomically. Foreign or user-authored catalogs
MUST remain untouched.

#### Scenario: Multi-target projection fails

- **WHEN** any selected target cannot be prepared or committed
- **THEN** AIGW SHALL report failure and restore only artifacts whose postimage
  still belongs to the failing transaction

#### Scenario: Inspect a dry run

- **WHEN** an operator runs synchronization in dry-run JSON mode
- **THEN** AIGW SHALL return the target and action plan without a credential or
  client-lifecycle side effect

#### Scenario: A provider prefixes a known Codex model

- **WHEN** the client's own bundled model table holds the selected model's
  family but not its ID
- **THEN** AIGW SHALL project the complete bundled table plus an alias named
  with the model's ID, whose metadata is the family's entry and whose display
  name is the model's AIGW label without its provider
- **AND** no two entries of the projected table SHALL share a display name,
  ignoring case: a name that the table shows SHALL be followed by the model's
  ID in parentheses, and a table that shows that too SHALL get no alias
- **AND** AIGW SHALL NOT take metadata from a suffix or prefix of the model ID,
  so a model whose family is not in the table SHALL get no alias
- **AND** the provider SHALL continue receiving the original selected model ID.

#### Scenario: Client identity or catalog ownership is not provable

- **WHEN** the installed client changes, generation fails, the managed bytes
  drift, or a foreign catalog occupies the managed path
- **THEN** AIGW SHALL refuse unsafe reuse, adoption, overwrite, or deletion
- **AND** SHALL preserve user-owned state.

#### Scenario: A real client qualifies the projection

- **WHEN** a contributor runs the tracked catalog verification command
- **THEN** it SHALL use the client's public model-catalog command surface
- **AND** SHALL record the exact client version and executable digest
- **AND** SHALL prove the generated alias is present in the effective catalog
- **AND** SHALL prove all alias metadata except `slug` and `display_name`
  equals the bundled base metadata
- **AND** SHALL take the alias's display name as input and prove that no
  other entry of the effective catalog shows it
- **AND** SHALL prove the bundled catalog did not already contain that alias
- **AND** SHALL prove an unrelated unknown entry remains absent
- **AND** SHALL send no model request and alter no persistent Codex home.

#### Scenario: The client no longer exposes a compatible catalog surface

- **WHEN** the installed client rejects the command or returns an invalid catalog
- **THEN** verification SHALL fail explicitly
- **AND** SHALL NOT substitute prompt shape, item counts, private debug settings,
  or a locally maintained model schema.
