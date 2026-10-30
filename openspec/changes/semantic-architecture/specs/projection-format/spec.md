## ADDED Requirements

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

## REMOVED Requirements

### Requirement: Codex selections respect TOML scope and literal identity

**Reason**: Its title or scenario names spell catalog as the glossary does, and OpenSpec renames a scenario only with its requirement.

**Migration**: Read it as "Codex model selections respect TOML scope and literal identity" in this capability, with every scenario.
