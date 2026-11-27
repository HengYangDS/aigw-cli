## MODIFIED Requirements

### Requirement: Arguments are checked before any change

Every command SHALL check its positional arguments, required flags, flag
combinations and argument shapes before it takes the configuration lock, reads
a Token, sends a request or runs its operation. A refusal SHALL name the
missing argument or flag, or the command's help, never an unrelated command.

#### Scenario: Self update receives invalid archive options

- **WHEN** `aigw self update` receives a blank `--archive` or `--checksums`
  path, or one of them without the other
- **THEN** it SHALL refuse before creating configuration storage or updating.

#### Scenario: Setup receives invalid team file arguments

- **WHEN** `aigw setup --from` receives a blank path, `--token-stdin` without
  `--provider`, or a blank or invalid `--provider`
- **THEN** setup SHALL refuse before creating configuration storage, reading
  the team file or prompting.

#### Scenario: A client command receives invalid arguments

- **WHEN** `aigw client enable` or `aigw client edit` names an unknown client, receives a blank `--program`,
  `--extensions`, `--target` or `--via`, receives both `--program` and `--extensions`, or receives `--extensions` for
  Claude Desktop or Hermes
- **THEN** the command SHALL refuse before creating configuration storage,
  reading credentials or discovering clients.

#### Scenario: A provider or model command receives invalid arguments

- **WHEN** `aigw provider add` receives an invalid provider ID or no valid
  `--endpoint <protocol>=<url>`, `aigw provider edit` only one of
  `--balance-token-stdin` and `--balance-user-id`, or a model command a
  malformed `provider/model`
- **THEN** it SHALL refuse before configuration, lock or credential access.

#### Scenario: An edit names nothing to change

- **WHEN** `aigw provider edit`, `aigw model edit` or `aigw client edit` runs
  without a flag that changes something, or with a blank provider label or
  endpoint
- **THEN** it SHALL refuse before configuration access, naming its help
- **AND** `aigw model edit <provider/model> --label ""` SHALL still clear the
  model's label.
