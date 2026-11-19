## MODIFIED Requirements

### Requirement: A Token is stored only after its provider accepts it

Every command that stores a Token, including `aigw provider add`,
`aigw provider rotate`, `aigw setup` and `aigw use`, SHALL store it only after
one authenticated model-free request to that provider accepts it. When the
provider rejects the Token or the request cannot complete, the command SHALL
store nothing for that provider and refuse, except where
`progressive-team-onboarding` lets `aigw setup --from` apply the rest. Output
SHALL NOT contain the Token.

#### Scenario: A provider is added with its Token

- **WHEN** `aigw provider add <provider> --endpoint <p>=<url> --token-stdin`
  reads a Token that the provider accepts
- **THEN** the Token SHALL be stored in that provider's Token slot of the
  selected backend
- **AND** human and JSON output SHALL report that it was validated and stored
  without containing it.

#### Scenario: The provider does not accept a new Token

- **WHEN** the provider rejects a Token that a command validates, or the
  request cannot complete
- **THEN** the command SHALL store nothing for that provider
- **AND** `aigw provider add` SHALL refuse and leave the provider unconfigured,
  `aigw provider rotate` SHALL refuse and keep the previous Token, and
  `aigw setup --from` SHALL keep a previous Token of that provider.

#### Scenario: A Token is requested without a terminal

- **WHEN** `aigw provider add` or `aigw provider rotate` runs without a
  terminal, or with `--json`, and without `--token-stdin`
- **THEN** it SHALL refuse before reading input or sending a request, naming
  `--token-stdin`
- **AND** it SHALL write nothing.

#### Scenario: The environment backend cannot replace a Token

- **WHEN** the environment backend is selected and `aigw provider rotate`
  would replace a Token
- **THEN** the command SHALL refuse before reading input, naming the
  environment variable that supplies the Token
- **AND** no configuration or credential SHALL change.
