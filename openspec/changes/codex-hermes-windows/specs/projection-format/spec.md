## ADDED Requirements

### Requirement: Codex's model table carries the selected model's declared context window

In the model table that AIGW projects for Codex, the entry that describes the selected model, its alias or the
model's own entry, SHALL carry the model's declared context window as its `context_window` and as its
`max_context_window`. Every other field of that entry and every other entry SHALL stay as the client's own table has
them. For each Codex target, `aigw check codex` SHALL report the window that Codex is given, or why AIGW gives it none.

#### Scenario: An alias carries the declared window

- **GIVEN** the client's bundled table holds the selected model's family but not its ID
- **WHEN** AIGW projects Codex's files
- **THEN** the alias that AIGW adds for the model SHALL carry the declared window in both fields
- **AND** the family's own entry and every other entry SHALL be unchanged.

#### Scenario: The selected model is an entry of the client's table

- **WHEN** the selected model's ID is itself an entry of the client's bundled table
- **THEN** AIGW SHALL project the complete bundled table with that entry's two window fields set to the model's
  declared context window
- **AND** nothing else of the table SHALL change.

#### Scenario: A check reports the window that Codex is given

- **GIVEN** the table that AIGW wrote for a Codex target holds an entry for the selected model
- **WHEN** `aigw check codex` runs
- **THEN** a check of its own SHALL pass and report that entry's context window
- **AND** it SHALL name the target as the other local checks of that target do.

#### Scenario: A check finds a window other than the declared one

- **GIVEN** the table that AIGW wrote for a Codex target holds an entry for the selected model whose
  `context_window` or `max_context_window` is not the model's declared context window
- **WHEN** `aigw check codex` runs
- **THEN** the check of its own SHALL fail and report both fields as the table carries them beside the declared window
- **AND** its repair SHALL be `aigw sync`, which writes the table again.

#### Scenario: The client's table does not hold the model's family

- **WHEN** the selected model's family is not in the client's bundled table
- **THEN** AIGW SHALL project the table without an entry for the model
- **AND** `aigw check codex` SHALL report, in a check of its own that passes, that Codex's table has no entry for the
  model's family, so Codex chooses the window itself whatever the model declares.

#### Scenario: AIGW writes no table for a Codex target

- **WHEN** a Codex target's configuration names a `model_catalog_json` that AIGW did not write, or the selected model
  runs under one of Codex's own model providers
- **THEN** AIGW SHALL write no table for that target
- **AND** `aigw check codex` SHALL report, in a check of its own that passes, that Codex is not given the declared
  window, with that reason.

#### Scenario: The declared window of the selected model changes

- **WHEN** `aigw model edit` changes the declared window of the model that Codex selects
- **THEN** the command SHALL rewrite Codex's table with the new window
- **AND** `aigw sync --dry-run` afterwards SHALL plan no change.

## MODIFIED Requirements

### Requirement: Transactional and inspectable projection

AIGW SHALL atomically prepare and validate selected client targets, mutate only
owned projections, compensate failures only while postimages remain
transaction-owned, and expose credential-free, side-effect-free dry runs. For a
Codex model whose family is in Codex's bundled table and whose ID is not itself
an entry of it, it SHALL derive an alias of that family's entry bound to client
version and digest, preserve the wire model ID, and update all three
atomically. Foreign or user-authored catalogs MUST remain untouched.

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
  with the model's ID, whose metadata is the family's entry apart from the two
  window fields and whose display name is the model's AIGW label without its
  provider
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
- **AND** SHALL prove that the entry which describes the model is present in
  the effective catalog
- **AND** SHALL prove that this entry's metadata, except `slug`, `display_name`
  and the two window fields, equals the bundled base metadata: the family's for
  an alias, the model's own for a model that is itself an entry
- **AND** SHALL take the declared context window as input, prove that the
  effective entry carries it in both window fields, and record the two numbers
  that it read
- **AND** for an alias SHALL take its display name as input, prove that no
  other entry of the effective catalog shows it, and prove that the bundled
  catalog did not already contain that alias
- **AND** SHALL prove an unrelated unknown entry remains absent
- **AND** SHALL send no model request and alter no persistent Codex home.

#### Scenario: The client no longer exposes a compatible catalog surface

- **WHEN** the installed client rejects the command or returns an invalid catalog
- **THEN** verification SHALL fail explicitly
- **AND** SHALL NOT substitute prompt shape, item counts, private debug settings,
  or a locally maintained model schema.
