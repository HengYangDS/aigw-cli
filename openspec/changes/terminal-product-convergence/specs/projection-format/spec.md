## ADDED Requirements

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
  host path or secret, and use a stable readable ordering.

### Requirement: Codex selections respect TOML scope and literal identity

AIGW SHALL locate its root-level model, provider, and catalogue selections through
the locked native TOML parser. Quoted keys and multiline values SHALL retain their
meaning. Projection and withdrawal SHALL preserve unrelated source bytes, including
same-named keys inside named profiles, dotted keys, arrays of tables, and strings.
Replacement values SHALL be literal data, never regular-expression substitution
syntax. Invalid or ambiguous selections SHALL fail before applying file changes.

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

AIGW SHALL reconcile an explicit Claude Route selection or synchronization when
the recorded managed hash proves that only the top-level model preference changed.
The prior Route supplies the previous model; the current user settings SHALL NOT
establish their own ownership. Endpoint, credential-helper, and managed credential
changes SHALL remain conflicts. Neighboring user settings SHALL be preserved.
Ownership hashes SHALL compare decoded string values, not equivalent JSON escape
spellings. Transaction compensation SHALL retain the byte-exact observed files.

#### Scenario: A client rewrites equivalent JSON string escapes

- **WHEN** the client escapes slashes or Unicode without changing managed values
- **THEN** ownership verification accepts the unchanged values
- **AND** preview writes nothing and compensation restores the exact observed bytes.

#### Scenario: The user changes or removes the projected model

- **WHEN** the previous model reconstructs the recorded managed hash
- **THEN** synchronization projects the selected Profile's model
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
