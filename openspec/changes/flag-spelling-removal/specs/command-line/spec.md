## MODIFIED Requirements

### Requirement: Each flag name has one meaning

Across commands, `--for` SHALL name a client or `all`, `--account` an Account,
`--route` a Route, `--model` a canonical Model, `--upstream-model` the model ID
that an endpoint serves, `--protocol` an endpoint protocol, `--target` a
client's native configuration file and `--path` the program file. A renamed
flag SHALL keep its earlier spelling for one release, hidden, with a warning
that names its replacement, and every later release SHALL refuse the earlier
spelling.

#### Scenario: An earlier flag spelling is used

- **WHEN** a script runs `aigw setup --model gpt-6.1-sol` in the release that
  renames the flag
- **THEN** the command SHALL use the value as `--upstream-model`
- **AND** standard error SHALL name `--upstream-model` as the replacement.

#### Scenario: An earlier flag spelling outlives its release

- **WHEN** a script runs `aigw setup --model gpt-6.1-sol` in a release after
  the one that hid the spelling
- **THEN** the command SHALL refuse the unknown flag and exit 1
- **AND** it SHALL read no Token and write no configuration or client file.

#### Scenario: The program file is named

- **WHEN** `aigw install --path <file>` runs
- **THEN** AIGW SHALL install the program at that file.
