## MODIFIED Requirements

### Requirement: Providers, models, families and clients have distinct authority

A provider SHALL own an ID, a label, one endpoint per protocol and its Token. A model SHALL belong to one provider and
be named `provider/model`, where the part after the first `/` is the provider's own model ID, byte for byte and unique
within that provider. A family SHALL group the models of one vendor model. Only `[clients.<id>]` SHALL own a client's
selected model, protocol, program declaration, enabled intent and client-only options. Each capability requires
separate proof.

#### Scenario: A provider repeats a model ID

- **WHEN** a team file or `aigw.toml` gives one provider the same model ID twice
- **THEN** validation SHALL refuse it without changing any file.

#### Scenario: One provider serves different protocols per model

- **GIVEN** a provider has `responses` and `chat` endpoints
- **AND** two of its models declare different protocols
- **WHEN** a client resolves either model
- **THEN** only that model's declared protocols SHALL be eligible
- **AND** `anthropic`, `responses` and `chat` SHALL stay distinct protocols
- **AND** an endpoint of the provider SHALL NOT imply that a model works there.

#### Scenario: Responses text and Responses reasoning differ

- **GIVEN** a model can complete a basic `responses` text request
- **WHEN** its reasoning, tool, continuation, or compaction behavior has not
  been qualified for the selected client
- **THEN** AIGW SHALL report only the proven text capability
- **AND** SHALL NOT present the model as Codex-compatible or
  reasoning-qualified.

#### Scenario: The operator disables a client

- **WHEN** a client is disabled and AIGW's values are withdrawn from its
  configuration files
- **THEN** `aigw sync`, discovery and `aigw setup --from` SHALL keep it
  disabled
- **AND** its selected model, its provider's Token and other clients SHALL
  remain intact.

### Requirement: One configuration file holds providers, models and families

AIGW SHALL keep its configuration in one credential-free `aigw.toml` with `version = 9`, holding providers with their
models, families, recommendations and clients. Commands SHALL take their configuration only from that file. Array
order SHALL be display order, and decoding SHALL refuse a field that schema 9 does not define. A version 8 file SHALL
be refused before any other field is read, with the edits that schema 9 needs. A team file SHALL use the same format
without `[clients]`.

#### Scenario: Lists follow the order of the file

- **WHEN** `aigw.toml` lists providers, and the models of one provider, in an
  order that is not alphabetical
- **THEN** `aigw provider list`, `aigw model list`, their JSON output and
  `aigw config export` SHALL keep that order.

#### Scenario: A retired field is present

- **WHEN** `aigw.toml` contains an `order`, `purpose` or `lifecycle` field
- **THEN** loading it SHALL fail without changing any file.

#### Scenario: A model is deprecated

- **WHEN** a team file declares `deprecated = true` for a model
- **THEN** a client that selects it SHALL keep its selection
- **AND** `aigw model list` SHALL mark it, and `aigw model search` SHALL
  report it as `deprecated` while its provider's catalog lists it
- **AND** a recommendation that names it SHALL fail validation
- **AND** only a team file SHALL set `deprecated`.

#### Scenario: A model is its own family

- **WHEN** a model omits `family`
- **THEN** it SHALL be its own family and need no `[[families]]` entry.

#### Scenario: A file carries a credential

- **WHEN** `aigw.toml` or a team file contains a Token, password,
  authorization header, API key or equivalent credential field
- **THEN** AIGW SHALL reject it without changing local configuration.

#### Scenario: A version 8 file is refused with its edits

- **WHEN** a command loads an `aigw.toml` that has `version = 8`
- **THEN** it SHALL refuse before reading any other field and change no file
- **AND** its fix SHALL name three edits: set `version = 9`, rename each client's `executable` to `program`, and
  delete `authentication` and `model_provider` wherever they appear
- **AND** that text SHALL be the same for every version 8 file, and no command SHALL rewrite the file
- **AND** the credential reader, which ignores `version`, SHALL still serve Tokens from it.

#### Scenario: A schema 8 key remains after the edits

- **WHEN** an `aigw.toml` with `version = 9` still holds `executable`, `authentication` or `model_provider`
- **THEN** loading it SHALL fail, naming that field and its line, without changing any file.

## REMOVED Requirements

### Requirement: Every client selects a compatible model

**Reason**: It moves to `model-selection`, which owns how each client selects a model and protocol.

**Migration**: `model-selection` holds it under the same title with its schema 9 text and every scenario.
