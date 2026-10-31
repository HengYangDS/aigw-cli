# projection-format Specification

## Purpose

Define how AIGW's generated projections derive from one canonical model with a
byte-stable text layout, and how a client projection writes only the native
values that AIGW owns: conflicts are refused before any write, projection is
transactional and inspectable with matching Account Token delivery, and
recovery is verified.

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

AIGW SHALL accept Claude Code's native model preference only when its sidecar
proves a model-only edit to an owned connection. Sync SHALL preserve it while
Route model and credential scope are unchanged; otherwise it SHALL reproject
the Route model. External connection edits remain conflicts. Inspection SHALL
distinguish native preference from upstream model proof, and compensation
SHALL preserve observed bytes.

#### Scenario: A client rewrites equivalent JSON string escapes

- **WHEN** the client escapes slashes or Unicode without changing managed values
- **THEN** ownership verification accepts the unchanged values
- **AND** readiness remains valid and synchronization performs no writes to
  settings or ownership state merely to normalize JSON spelling
- **AND** a real projection change still compensates to the exact observed bytes.

#### Scenario: The user changes or removes the projected model

- **WHEN** the previous model reconstructs the recorded managed hash
- **THEN** synchronization of an unchanged connection performs no write and
  preserves the user's model preference, including absence
- **AND** preview reports an already-converged projection without writing any file
- **AND** withdrawal preserves the user's later model preference, including absence.

#### Scenario: AIGW changes only the credential helper executable

- **WHEN** the previous model reconstructs the recorded managed hash and the
  Route model and Account/endpoint credential scope remain unchanged
- **THEN** AIGW projects the new helper without replacing the native model
- **AND** inspection still identifies the model as a native preference
- **AND** rollback restores the exact observed settings and
  sidecar bytes.

#### Scenario: AIGW changes the selected connection

- **WHEN** the Route model, Account, or endpoint changes
- **THEN** projection uses the selected Route's model rather than carrying
  a native preference across a different credential scope.

#### Scenario: Read-only inspection sees a model-only preference

- **WHEN** the sidecar hash is reproduced by substituting only the prior
  Route's model for the user's current top-level model
- **THEN** inspection accepts the AIGW-owned endpoint and credential helper
  without changing settings, sidecar bytes, or the user's model
- **AND** it identifies the model as a native preference rather than treating
  its spelling as a proven upstream wire model
- **AND** a changed Route model or credential scope reprojects the Route model.

#### Scenario: A model edit accompanies a connection or credential edit

- **WHEN** substituting only the previous model cannot reproduce the managed hash
- **THEN** synchronization rejects the conflict without adopting the changed fields.

### Requirement: Projection conflicts are rejected before configuration writes

The synchronization transaction SHALL preflight every participating client before
persisting configuration, backup, or checkpoint changes. A forwarding selection
SHALL share the same snapshot, backup, checkpoint and compensation boundary
without changing the predecessor-readable main schema. Failures after actual
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

#### Scenario: A retained credential reader observes forwarding

- **WHEN** the installed successor selects a forwarding destination for a client
- **THEN** a retained predecessor reader SHALL still decode the main configuration
  and the verified checkpoint, and resolve the same Account credential identity
- **AND** the destination SHALL remain bound to its recorded upstream identity.

#### Scenario: Forwarding changes during verification or commit

- **WHEN** forwarding state no longer matches the captured preimage
- **THEN** AIGW SHALL reject the stale commit or checkpoint
- **AND** SHALL preserve the newer state and restore only the unchanged files
  that its failed attempt owns.

### Requirement: Transactional and inspectable projection

AIGW SHALL atomically prepare and validate selected client targets, mutate only
owned projections, compensate failures only while postimages remain
transaction-owned, and expose credential-free, side-effect-free dry runs. For a
uniquely matched provider-prefixed Codex model, it SHALL derive a full
bundled-catalog alias mirror bound to client version and digest, preserve the
wire model ID, and update all three atomically. Foreign or user-authored
catalogs MUST remain untouched.

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
- **THEN** it SHALL use the client's public model-catalog command surface
- **AND** SHALL record the exact client version and executable digest
- **AND** SHALL prove the generated alias is present in the effective catalog
- **AND** SHALL prove all alias metadata except `slug` equals the bundled base
  metadata
- **AND** SHALL prove the bundled catalog did not already contain that alias
- **AND** SHALL prove an unrelated unknown entry remains absent
- **AND** SHALL send no model request and alter no persistent Codex home.

#### Scenario: The client no longer exposes a compatible catalog surface

- **WHEN** the installed client rejects the command or returns an invalid catalog
- **THEN** verification SHALL fail explicitly
- **AND** SHALL NOT substitute prompt shape, item counts, private debug settings,
  or a locally maintained model schema.

### Requirement: Projection-matched Account Token delivery

Every Account-Token Codex provider SHALL authenticate through the absolute AIGW
command. The helper SHALL carry the projected client, Account, and endpoint
fingerprint and compare it with the selected Route before reading a Token. AIGW
SHALL neither invoke native login nor change client credentials. Client-native
Routes SHALL project no AIGW Token helper; naming and catalog selection
SHALL NOT alter credential ownership.

#### Scenario: Account-Token provider projection

- **WHEN** AIGW synchronizes a default or explicit Account-Token provider
- **THEN** the provider table declares `wire_api = "responses"`
- **AND** its auth command invokes the absolute AIGW executable with
  `credential codex <projection-fingerprint>`
- **AND** no Token value is copied into client files.

#### Scenario: Return to the default provider

- **WHEN** a Route changes from an explicit provider to the default provider
- **THEN** the old attributed provider table is removed transactionally
- **AND** the Route's declared authentication ownership is preserved.

#### Scenario: A retained projection no longer matches the selected Route

- **WHEN** a helper is invoked after its client, Account or endpoint differs
  from the selected Route
- **THEN** it returns a synchronization and reload instruction
- **AND** reads no Token and writes no credential bytes to standard output.

#### Scenario: Model or label changes without a credential-identity change

- **WHEN** only the selected model or label changes
- **THEN** the existing helper fingerprint still matches
- **AND** the helper reads the same Account's current Token.

#### Scenario: Multiple Codex homes receive the same Account-Token route

- **WHEN** setup discovers multiple admitted, AIGW-owned Codex targets
- **THEN** it projects the matching credential helper into each target
- **AND** reads or stores only the selected Account's Token
- **AND** leaves client-owned credentials unchanged.

### Requirement: Client projection failure reports verified recovery state

After a configuration commit, a failed native client projection or credential
entrypoint finalization SHALL report whether AIGW configuration restoration
succeeded. Human and JSON output SHALL
use the same outcome and safe next action without exposing a private file path,
credential value, or internal error. Configuration restoration SHALL NOT be
presented as proof that every client or credential entrypoint was restored.

#### Scenario: Configuration was restored after projection failure

- **WHEN** a client file rejects the projection and the configuration snapshot
  is restored
- **THEN** the error SHALL say that configuration was restored
- **AND** SHALL direct the operator to inspect client state with `aigw doctor`
  before retrying.

#### Scenario: Configuration restoration is incomplete

- **WHEN** a client projection fails and restoring the configuration fails
- **THEN** the error SHALL report incomplete restoration rather than claim a
  successful rollback
- **AND** SHALL not expose the underlying file path or imply retry is safe.

#### Scenario: Credential entrypoint finalization fails after projection

- **WHEN** client projection succeeds but its credential entrypoint fails
  finalization and compensation is attempted
- **THEN** the error SHALL report the observed configuration restoration result
- **AND** failed client or entrypoint compensation SHALL not conceal that result
- **AND** public output SHALL retain safe recovery guidance without private paths.

### Requirement: Claude Desktop keeps its own profile settings

AIGW SHALL judge its Claude Desktop profile by its managed meaning: the
selected gateway, credential helper and arguments, models and tool search,
and AIGW's fixed Chat, Cowork, Code, model-discovery, credential-kind and
authentication settings. A managed setting that the profile omits SHALL take
Claude Desktop's documented default. Every other profile key SHALL belong to
Claude Desktop and SHALL survive AIGW projections.

#### Scenario: Claude Desktop re-saves the profile

- **WHEN** Claude Desktop saves the AIGW profile with its own encoding,
  without settings that equal its defaults, and with its own settings added
- **THEN** synchronization SHALL treat the profile as unchanged and keep its
  bytes
- **AND** status SHALL report Claude Desktop as ready.

#### Scenario: A projection changes AIGW's values

- **WHEN** a Route or reader change projects new values into a profile that
  holds Claude Desktop's own settings
- **THEN** the managed settings SHALL take the new values
- **AND** Claude Desktop's own settings SHALL remain unchanged.

#### Scenario: A managed setting was changed outside AIGW

- **WHEN** the profile's managed meaning differs from AIGW's record
- **THEN** synchronization SHALL refuse without writing any file
- **AND** the refusal SHALL name each changed managed setting it can identify
  and how to restore AIGW's values.

#### Scenario: An earlier AIGW release reads the record

- **WHEN** this release has recorded ownership of an unchanged profile
- **THEN** an earlier release that reads the ownership record SHALL accept the
  profile.

### Requirement: Repair previews name each projection surface

Each client adapter SHALL declare the surface identity of every projection it
plans, and repair previews SHALL report that identity for each projection in
human and JSON output.

#### Scenario: Preview repair for every admitted client

- **WHEN** `aigw repair --dry-run --json` plans projections for Claude, Claude
  Desktop, Codex and Hermes
- **THEN** their `surface_id` values SHALL be `claude-settings`,
  `claude-desktop-config-library`, the identity of the Codex target's home and
  `hermes-home-default`
- **AND** no projection SHALL report the surface of another client.

### Requirement: An outside edit to managed Codex settings has one recovery action

When AIGW refuses because AIGW-managed Codex settings were changed outside AIGW,
every surface that reports the refusal, including errors, `check`, `status` and
`doctor`, SHALL give the same next action: undo the outside edit, then run
`aigw sync`. AIGW SHALL NOT advise a command that refuses on the same state,
overwrite the edit, or relax the guard.

#### Scenario: Retry budgets were edited outside AIGW

- **WHEN** the operator changed managed Codex retry budgets outside AIGW
- **THEN** `use --dry-run`, `sync --dry-run`, `sync`, `check`, `status` and
  `doctor` SHALL report the same next action
- **AND** none of them SHALL advise `aigw check` or `aigw sync` alone
- **AND** client files and owned state SHALL stay unchanged.

#### Scenario: The outside edit is undone

- **WHEN** the operator restores AIGW's values and runs `aigw sync`
- **THEN** synchronization SHALL succeed and `check` SHALL report no drift.

### Requirement: Codex model selections respect TOML scope and literal identity

AIGW SHALL use the locked native TOML parser for Codex root model, provider,
and catalog selections. Projection and withdrawal SHALL preserve unrelated
bytes, including quoted, multiline, dotted, and named-profile keys and arrays
of tables. Replacement values SHALL be literal, never regex substitutions.
Invalid or ambiguous selections SHALL fail before writes. Root ownership SHALL
require the attributed sidecar and recorded semantic values; decorative
comments alone do not grant authority.

#### Scenario: Project and withdraw a literal model selection

- **WHEN** the original root selection uses quoted keys, multiline values, or
  comments and the selected model contains dollar signs, quotes, or backslashes
- **THEN** projection retains the exact selected value
- **AND** withdrawal restores the recorded original selection source
- **AND** named-profile settings remain unchanged through both operations.

#### Scenario: Root and named-profile catalogs coexist

- **WHEN** a named profile has its own catalog and the selected root model
  needs a proven catalog projection
- **THEN** AIGW projects only the root reference and its owned catalog
- **AND** withdrawal removes only those owned outputs.

#### Scenario: A selection cannot be represented safely

- **WHEN** a selection is malformed, ambiguous, invalid UTF-8, or contains an
  inadmissible control character
- **THEN** the operation reports the input error without normalizing it into a
  different model or applying configuration writes.

#### Scenario: The native client removes decorative ownership comments

- **GIVEN** the attributed sidecar, provider values, scheduler values, and
  catalog still match the last AIGW projection
- **WHEN** the native client rewrites the root provider and model assignments
  without their decorative ownership comments
- **THEN** inspection SHALL accept unchanged semantic values
- **AND** synchronization SHALL restore the canonical comments without changing
  unrelated client configuration.

#### Scenario: A root selection changes outside explicit Route selection

- **WHEN** the root provider or model value no longer matches the attributed
  AIGW projection
- **THEN** repair, and synchronization unless it adopts a model-only change
  under "Synchronization adopts a Codex model choice", SHALL reject the
  conflict before writing configuration, sidecar, catalog, or credential
  state
- **AND** an explicit Route verification MAY replace the values only in its
  private projection copy
- **AND** an explicit operator Route selection MAY replace the root values only
  after the provider block, scheduler, catalog, and sidecar still prove the
  existing AIGW projection.
