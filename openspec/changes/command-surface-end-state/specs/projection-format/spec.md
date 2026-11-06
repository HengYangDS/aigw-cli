## ADDED Requirements

### Requirement: Credential commands match the selected provider and endpoint

Every Codex provider that uses a provider Token SHALL authenticate through the
absolute AIGW command. The command SHALL carry the fingerprint of its client,
provider and endpoint and compare it with the client's selected model before
reading a Token. AIGW SHALL neither invoke native login nor change client
credentials. Client-native authentication SHALL get no AIGW Token command;
naming and catalog selection SHALL NOT alter credential ownership.

#### Scenario: A Codex provider uses a provider Token

- **WHEN** AIGW synchronizes a default or explicit Codex provider that uses a
  provider Token
- **THEN** the provider table declares `wire_api = "responses"`
- **AND** its auth command invokes the absolute AIGW executable with
  `credential codex <fingerprint>`
- **AND** no Token value is copied into client files.

#### Scenario: Return to the default provider

- **WHEN** Codex's selection changes from an explicit provider to the default
  provider
- **THEN** the old attributed provider table is removed transactionally
- **AND** the declared authentication ownership is preserved.

#### Scenario: A retained command no longer matches the selected model

- **WHEN** a credential command is invoked after its client, provider or
  endpoint differs from the client's selected model
- **THEN** it returns a synchronization and reload instruction
- **AND** reads no Token and writes no credential bytes to standard output.

#### Scenario: Model or label changes without a credential-identity change

- **WHEN** only the selected model of the same provider and endpoint, or its
  label, changes
- **THEN** the existing fingerprint still matches
- **AND** the command reads the same provider's current Token.

#### Scenario: Multiple Codex homes use the same provider Token

- **WHEN** setup discovers multiple supported, AIGW-owned Codex targets
- **THEN** it writes the matching credential command into each target
- **AND** reads or stores only the selected provider's Token
- **AND** leaves client-owned credentials unchanged.

### Requirement: Previews name each client configuration file

Each client SHALL declare the identity of every configuration file that it
plans to write. A preview from `aigw sync --dry-run`, or from the `--dry-run`
of another command that writes client files, SHALL report each client
configuration file that the write would change, with that identity and any
credential reader that it would install, in human and JSON output, naming
models as `provider/model`.

#### Scenario: Preview sync for every supported client

- **WHEN** `aigw sync --dry-run --json` plans changes for Claude Code, Claude
  Desktop, Codex and Hermes
- **THEN** their `surface_id` values SHALL be `claude-settings`,
  `claude-desktop-config-library`, the identity of the Codex target's home and
  `hermes-home-default`
- **AND** no file SHALL report the identity of another client.

#### Scenario: Preview a selection

- **WHEN** `aigw use codex dmxapi/gpt-6.1-sol --dry-run` runs
- **THEN** its human output SHALL name Codex's configuration file and
  `dmxapi/gpt-6.1-sol`
- **AND** SHALL NOT name an Account, a Route, a projection or a binding.

### Requirement: Sync converges every enabled client

`aigw sync` SHALL be the one command that rediscovers installed clients and
converges each enabled client's configuration files with `aigw.toml`. It SHALL
write a client configuration file only when the file differs from what AIGW
would write, and its preview SHALL report the same changes without writing.

#### Scenario: A client is discovered after its selection

- **WHEN** a client with a selection was installed after the last `aigw sync`
- **THEN** `aigw sync` SHALL discover it and write AIGW's values into its
  configuration file
- **AND** `aigw sync --dry-run` SHALL report that change without writing.

#### Scenario: Every client configuration file already matches

- **WHEN** every client configuration file already holds what AIGW would write
- **THEN** `aigw sync` SHALL write no client configuration file
- **AND** its preview SHALL report no change to them.

### Requirement: Codex model choices respect TOML scope and literal identity

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

#### Scenario: A root selection changes outside an explicit selection

- **WHEN** the root provider or model value no longer matches the attributed
  AIGW projection
- **THEN** `aigw sync`, unless it adopts a model-only change that
  `model-selection` admits, SHALL reject the conflict before writing
  configuration, sidecar, catalog, or credential state
- **AND** `aigw check --depth client` MAY replace the values only in its
  private copy
- **AND** an explicit `aigw use` MAY replace the root values only after the
  provider block, scheduler, catalog, and sidecar still prove the existing
  AIGW projection.

## MODIFIED Requirements

### Requirement: Codex provider ownership follows TOML values

AIGW SHALL identify provider and authentication tables through the locked TOML
parser, never comment spans. One canonical rendering of complete owned values
SHALL define the projection hash; equivalent syntax SHALL preserve ownership,
while invalid, missing, unknown, or changed owned values fail before writing.
`requires_openai_auth` SHALL retain absent, true, or false identity until a
matching projection is reconciled to the selection; reading it SHALL NOT
enable it in a new projection.

#### Scenario: The native client edits another configuration table

- **WHEN** `codex mcp add` or `codex mcp remove` edits an isolated Codex Home
- **THEN** unchanged provider values remain valid without `aigw sync`
- **AND** synchronization and withdrawal preserve the client-owned MCP tables
- **AND** actual endpoint or authentication edits remain ownership conflicts.

#### Scenario: Upgrade retains a proven native authentication preference

- **WHEN** an installed predecessor left `requires_openai_auth` in its owned
  provider table
- **THEN** the current parser reconstructs the same recorded provider identity
- **AND** synchronization installs the current authentication projection
- **AND** a user change to that preference fails before any file is written.

### Requirement: Claude model preference and connection ownership are distinct

AIGW SHALL accept Claude Code's native model preference only when its sidecar
proves a model-only edit to an owned connection. Sync SHALL preserve it while
the selected model and credential scope are unchanged; otherwise it SHALL
reproject the selected model. External connection edits remain conflicts.
Inspection SHALL distinguish native preference from upstream model proof, and
compensation SHALL preserve observed bytes.

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
- **AND** preview reports an already converged client configuration file
  without writing any file
- **AND** withdrawal preserves the user's later model preference, including absence.

#### Scenario: AIGW changes only the credential helper executable

- **WHEN** the previous model reconstructs the recorded managed hash and the
  selected model and its provider and endpoint credential scope remain
  unchanged
- **THEN** AIGW projects the new helper without replacing the native model
- **AND** inspection still identifies the model as a native preference
- **AND** rollback restores the exact observed settings and
  sidecar bytes.

#### Scenario: AIGW changes the selected connection

- **WHEN** the selected model, its provider, or the endpoint changes
- **THEN** projection uses the selected model rather than carrying a native
  preference across a different credential scope.

#### Scenario: Read-only inspection sees a model-only preference

- **WHEN** the sidecar hash is reproduced by substituting only the previously
  selected model for the user's current top-level model
- **THEN** inspection accepts the AIGW-owned endpoint and credential helper
  without changing settings, sidecar bytes, or the user's model
- **AND** it identifies the model as a native preference rather than treating
  its spelling as a proven upstream wire model
- **AND** a changed selected model or credential scope reprojects the selected
  model.

#### Scenario: A model edit accompanies a connection or credential edit

- **WHEN** substituting only the previous model cannot reproduce the managed hash
- **THEN** synchronization rejects the conflict without adopting the changed fields.

### Requirement: Projection conflicts are rejected before configuration writes

The synchronization transaction SHALL preflight every participating client
before persisting configuration, backup, or verified snapshot changes. A
forwarding selection SHALL share the same configuration snapshot, backup,
verified snapshot and compensation boundary without changing what credential
readers decode. Failures after actual writes SHALL compensate only captured
owned postimages, never re-run an inverse reconciliation against the same
rejected state.

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
- **AND** configuration, backup, verified snapshot and client sidecar cleanup do
  not stop at the first conflict
- **AND** all encountered causes remain identifiable in the returned error.

#### Scenario: A retained credential reader observes forwarding

- **WHEN** the installed program selects a forwarding destination for a client
- **THEN** a retained credential reader SHALL still decode the configuration
  that it reads, including one restored from the verified snapshot, and
  resolve the same provider credential identity
- **AND** the destination SHALL remain bound to its recorded upstream identity.

#### Scenario: Forwarding changes during verification or commit

- **WHEN** forwarding state no longer matches the captured preimage
- **THEN** AIGW SHALL reject the stale commit or verified snapshot
- **AND** SHALL preserve the newer state and restore only the unchanged files
  that its failed attempt owns.

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
- **AND** SHALL direct the operator to inspect client state with
  `aigw check --depth local --verbose` before retrying.

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

- **WHEN** a model or credential reader change projects new values into a
  profile that holds Claude Desktop's own settings
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

### Requirement: An outside edit to managed Codex settings has one recovery action

When AIGW refuses because AIGW-managed Codex settings were changed outside AIGW,
every surface that reports the refusal, including errors, `check` and `status`,
SHALL give the same next action: undo the outside edit, then run `aigw sync`.
AIGW SHALL NOT advise a command that refuses on the same state, overwrite the
edit, or relax the guard.

#### Scenario: Retry budgets were edited outside AIGW

- **WHEN** the operator changed managed Codex retry budgets outside AIGW
- **THEN** `use --dry-run`, `client edit --dry-run`, `sync --dry-run`, `sync`,
  `check` and `status` SHALL report the same next action
- **AND** none of them SHALL advise `aigw check` or `aigw sync` alone
- **AND** client files and owned state SHALL stay unchanged.

#### Scenario: The outside edit is undone

- **WHEN** the operator restores AIGW's values and runs `aigw sync`
- **THEN** synchronization SHALL succeed and `check` SHALL report no drift.

## REMOVED Requirements

### Requirement: Projection-matched Account Token delivery

**Reason**: Its heading and scenarios name Accounts and Routes, which
providers and models replace.

**Migration**: "Credential commands match the selected provider and endpoint"
in this capability keeps every scenario under the new names.

### Requirement: Repair previews name each projection surface

**Reason**: `aigw repair` is removed; `aigw sync` does everything that it did.

**Migration**: "Previews name each client configuration file" and "Sync
converges every enabled client" in this capability.

### Requirement: Codex model selections respect TOML scope and literal identity

**Reason**: One scenario names Route selection and verification and
`aigw repair`, which are replaced.

**Migration**: "Codex model choices respect TOML scope and literal identity"
in this capability keeps every scenario, one under a new name.
