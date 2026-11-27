## MODIFIED Requirements

### Requirement: Setup refuses a team file in another form

`aigw setup --from` SHALL refuse, before any write, a team file that has a `[clients]` section or a `version` other
than 9. The refusal for version 8 SHALL name the edits that schema 9 needs, as for `aigw.toml`. The refusal for
version 7 SHALL name that file's conversion path.

#### Scenario: A team file has a clients section

- **WHEN** a team file contains a `[clients]` section
- **THEN** setup SHALL refuse before any write.

#### Scenario: A team file has another version

- **WHEN** a team file's `version` is not 9
- **THEN** setup SHALL refuse before any write and name that version
- **AND** for version 8 the refusal SHALL also name the edits that schema 9 needs.

### Requirement: Deferred activation resumes through sync

`aigw sync` SHALL later configure newly installed clients and newly available
environment Tokens without repeating setup or selecting in bulk. Setup and sync
SHALL fill only clients without a selection, from the declared primary and
then the alternatives in order, and only with a usable Token. A model that no
recommendation names SHALL stay manual-only, and an explicit selection SHALL
never be replaced.

#### Scenario: An environment Token becomes available later

- **WHEN** an environment Token for a provider becomes available after a team
  file was applied
- **THEN** `aigw sync` SHALL select that provider's first compatible
  recommended model, with its declared protocol, for each client without a
  selection
- **AND** SHALL NOT require the primary provider or any unrelated Token
- **AND** existing client selections SHALL stay.

#### Scenario: A recommendation survives deferred setup

- **WHEN** setup stores a recommendation while no required Token is available
- **THEN** the recommendation SHALL persist without becoming a selection
- **AND** later activation SHALL follow the declared order, not alphabetical
  order.

#### Scenario: An explicit selection is temporarily unavailable

- **WHEN** a client selects a model whose provider has no Token and another
  compatible provider gains one
- **THEN** setup and `aigw sync` SHALL keep that selection
- **AND** SHALL NOT replace it with a recommendation or another provider.

#### Scenario: A recommended model is removed

- **WHEN** an unselected recommended model is removed
- **THEN** its recommendation SHALL change in the same configuration
  transaction
- **AND** other recommendations and every client's selection SHALL stay.

#### Scenario: Codex is installed after setup

- **WHEN** Codex is installed after a team file was applied
- **THEN** `aigw sync` SHALL write only AIGW's values into Codex's
  configuration
- **AND** `aigw status` SHALL name the fix of the provider's Token as the next step when Codex's selected provider has
  none
- **AND** no unrelated provider Token SHALL be required.

### Requirement: Configuration export writes a team file

`aigw config export` SHALL write the configuration as a schema 9 team file: its
providers without Tokens or balance credentials, models, families and
recommendations, in configuration order, without a `[clients]` section, and
`aigw setup --from` SHALL accept the result.

#### Scenario: Export a configuration

- **WHEN** `aigw config export` runs on a configuration with providers, models,
  families, recommendations and client selections
- **THEN** its output SHALL hold `version = 9`, and the providers, models, families and recommendations in configuration
  order
- **AND** SHALL hold no `[clients]` section, Token or balance credential.

#### Scenario: Apply an exported team file

- **WHEN** another machine runs `aigw setup --from` with the exported file
- **THEN** setup SHALL accept it and apply its providers, models, families and
  recommendations.
