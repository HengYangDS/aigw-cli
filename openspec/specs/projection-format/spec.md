# projection-format Specification

## Purpose

Define the canonical text layout of repository-projected specifications so
generated and reviewed source remain byte-stable.

## Requirements

### Requirement: Projected specifications follow canonical text layout

A projected OpenSpec specification SHALL end with exactly one newline and no
terminal blank line.

#### Scenario: Archive projection is verified

- **WHEN** an archived Change updates a canonical specification
- **THEN** the projected specification satisfies the repository text-layout rule

### Requirement: Generated projections have one canonical model

Generated client configuration, CI, release metadata, specifications, and other
reviewed text SHALL be deterministic projections of one named canonical model.
Repository validation SHALL regenerate or compare the projection and reject
semantic or byte-level drift.

#### Scenario: A generated projection is reviewed

- **WHEN** a projection differs from its canonical model
- **THEN** validation identifies the owning model and projection
- **AND** the projection is regenerated rather than independently patched.

#### Scenario: Equivalent output is rendered twice

- **WHEN** unchanged canonical input is rendered in the same supported toolchain
- **THEN** both outputs are byte-identical, end with one newline, contain no
  secret value, and use a stable readable ordering
- **AND** portable repository projections contain no operator-specific path
- **AND** a local client projection may contain the explicitly selected absolute
  executable or configuration path required by its native contract, without
  leaking that host input into the portable source.

### Requirement: Codex selections respect TOML scope and literal identity

AIGW SHALL locate its root-level model, provider, and catalogue selections through
the locked native TOML parser. Quoted keys and multiline values SHALL retain their
meaning. Projection and withdrawal SHALL preserve unrelated source bytes, including
same-named keys inside named profiles, dotted keys, arrays of tables, and strings.
Replacement values SHALL be literal data, never regular-expression substitution
syntax. Invalid or ambiguous selections SHALL fail before applying file changes.
Root provider and model ownership SHALL derive from the attributed sidecar and
the recorded semantic values; decorative ownership comments SHALL NOT be the
sole authorization boundary.

#### Scenario: Project and withdraw a literal model selection

- **WHEN** the original root selection uses quoted keys, multiline values, or
  comments and the selected model contains dollar signs, quotes, or backslashes
- **THEN** projection retains the exact selected value
- **AND** withdrawal restores the recorded original selection source
- **AND** named-profile settings remain unchanged through both operations.

#### Scenario: Root and named-profile catalogues coexist

- **WHEN** a named profile has its own catalogue and the selected root model
  needs a proven catalogue projection
- **THEN** AIGW projects only the root reference and its owned catalogue
- **AND** withdrawal removes only those owned outputs.

#### Scenario: A selection cannot be represented safely

- **WHEN** a selection is malformed, ambiguous, invalid UTF-8, or contains an
  inadmissible control character
- **THEN** the operation reports the input error without normalizing it into a
  different model or applying configuration writes.

#### Scenario: The native client removes decorative ownership comments

- **GIVEN** the attributed sidecar, provider values, scheduler values, and
  catalogue still match the last AIGW projection
- **WHEN** the native client rewrites the root provider and model assignments
  without their decorative ownership comments
- **THEN** inspection SHALL accept unchanged semantic values
- **AND** synchronization SHALL restore the canonical comments without changing
  unrelated client configuration.

#### Scenario: A root selection changes outside explicit Route selection

- **WHEN** the root provider or model value no longer matches the attributed
  AIGW projection
- **THEN** ordinary synchronization and repair SHALL reject the conflict before
  writing configuration, sidecar, catalogue, or credential state
- **AND** an explicit Route verification MAY replace the values only in its
  private projection copy
- **AND** an explicit operator Route selection MAY replace the root values only
  after the provider block, scheduler, catalogue, and sidecar still prove the
  existing AIGW projection.

### Requirement: Codex provider ownership follows TOML values

AIGW SHALL identify provider and authentication tables through the locked TOML
parser, never comment spans. One canonical rendering of complete owned values
SHALL define the projection hash; equivalent syntax SHALL preserve ownership,
while invalid, missing, unknown, or changed owned values fail before writing.
`requires_openai_auth` SHALL retain absent, true, or false identity until a
matching projection is reconciled to the current Route; reading it SHALL NOT
enable it in a new projection.

#### Scenario: The native client edits another configuration table

- **WHEN** `codex mcp add` or `codex mcp remove` edits an isolated Codex Home
- **THEN** unchanged provider values remain valid without a repair command
- **AND** synchronization and withdrawal preserve the client-owned MCP tables
- **AND** actual endpoint or authentication edits remain ownership conflicts.

#### Scenario: Upgrade retains a proven native authentication preference

- **WHEN** an installed predecessor left `requires_openai_auth` in its owned
  provider table
- **THEN** the current parser reconstructs the same recorded provider identity
- **AND** synchronization installs the current authentication projection
- **AND** a user change to that preference fails before any file is written.

### Requirement: Codex scheduler edits follow native table boundaries

AIGW SHALL identify scheduler tables and assignments through the locked TOML
parser. Header-like text in strings and same-named keys in other tables SHALL
remain untouched. Quoted table and key names, comments, and supported integer
spellings SHALL resolve to the same scheduler values. Withdrawal SHALL restore
recorded scheduler values without removing ownership-like text elsewhere.

#### Scenario: User instructions contain example scheduler settings

- **WHEN** a multiline user string contains scheduler-looking tables and keys
- **THEN** projection changes only the real scheduler tables
- **AND** an independent native TOML decode observes the configured limits
- **AND** withdrawal preserves the instruction string and neighboring profiles.

#### Scenario: Equivalent table and integer spellings

- **WHEN** the user quotes table or key names, annotates a table header, or uses
  a supported signed, hexadecimal, or underscore-separated integer
- **THEN** capture records the actual scheduler value
- **AND** projection, validation, and withdrawal agree on that semantic scope.

### Requirement: Claude model preference and connection ownership are distinct

AIGW SHALL reconcile an explicit Claude Route when the recorded managed hash
proves that only the top-level model changed. The prior Route supplies the old
model; current user settings SHALL NOT establish ownership. Endpoint, helper,
and managed-credential changes SHALL remain conflicts. AIGW SHALL preserve
neighboring settings and byte-exact compensation, and SHALL compare decoded
strings rather than equivalent JSON escapes.

#### Scenario: A client rewrites equivalent JSON string escapes

- **WHEN** the client escapes slashes or Unicode without changing managed values
- **THEN** ownership verification accepts the unchanged values
- **AND** readiness remains valid and synchronization performs no writes to
  settings or ownership state merely to normalize JSON spelling
- **AND** a real projection change still compensates to the exact observed bytes.

#### Scenario: The user changes or removes the projected model

- **WHEN** the previous model reconstructs the recorded managed hash
- **THEN** synchronization projects the selected Route's model
- **AND** preview reports Claude's pending projection without writing any file
- **AND** withdrawal preserves the user's later model preference, including absence.

#### Scenario: A model edit accompanies a connection or credential edit

- **WHEN** substituting only the previous model cannot reproduce the managed hash
- **THEN** synchronization rejects the conflict without adopting the changed fields.

### Requirement: Projection conflicts are rejected before configuration writes

The synchronization transaction SHALL preflight every participating client before
persisting configuration, backup, or checkpoint changes. Failures after actual
writes SHALL compensate only captured owned postimages, never re-run an inverse
reconciliation against the same rejected state.

#### Scenario: Preflight finds a conflicting client projection

- **WHEN** a client rejects the planned transition
- **THEN** configuration and client files remain unchanged
- **AND** the result reports preflight rejection, not a rollback attempt.

#### Scenario: An applied transition is compensated

- **WHEN** a later write fails after a projection was applied
- **THEN** compensation restores the exact observed preimage, including user edits
- **AND** a concurrent edit to the postimage is preserved and reported as a conflict.

#### Scenario: One owned file conflicts during compensation

- **WHEN** an external writer changes one file after AIGW applied a transition
- **THEN** compensation preserves that file and restores every independent file
  that still matches AIGW's postimage
- **AND** configuration, backup, checkpoint and client sidecar cleanup do not stop
  at the first conflict
- **AND** all encountered causes remain identifiable in the returned error.
