## MODIFIED Requirements

### Requirement: Team setup connects Tokens without requiring them

`aigw setup --from` SHALL require no particular provider's Token, client or
external service, and SHALL apply the team file whatever becomes of each Token.
It SHALL prompt only in a terminal, take one provider's Token from standard
input when `--token-stdin` comes with `--provider <id>`, and otherwise defer a
provider and name its fix. It SHALL store a Token only after the validation that
`secret-storage` defines, and a failed one SHALL leave a held Token in place.

#### Scenario: No Token or client is present

- **WHEN** a team file is applied without a terminal on a host with no
  supported client and no provider Token
- **THEN** setup SHALL save the file's providers, models, families and
  recommendations without prompting
- **AND** SHALL report that no provider has a Token
- **AND** SHALL show the smallest provider-scoped command that supplies one.

#### Scenario: A script supplies one provider's Token

- **WHEN** `aigw setup --from team.toml --provider dmxapi --token-stdin` runs
- **THEN** only the `dmxapi` Token SHALL be read from standard input, validated
  and stored
- **AND** each other provider without a Token SHALL be deferred with its fix.

#### Scenario: The provider does not accept a supplied Token

- **WHEN** a piped Token fails that validation
- **THEN** setup SHALL apply the team file, store no Token for that provider and
  set up the clients that the other Tokens serve
- **AND** a Token that the provider held SHALL stay and serve its clients
- **AND** SHALL report the provider's reason and next step, in human output in
  place of its success line and in JSON as the document's `error` and `impact`
- **AND** SHALL exit 1, so that the script sees that its Token was not stored.

#### Scenario: Piped input holds no Token

- **WHEN** `--token-stdin` reads input that is empty or not one Token
- **THEN** setup SHALL refuse before any write.

#### Scenario: Token input names more than one provider

- **WHEN** `--token-stdin` comes with more than one `--provider`
- **THEN** setup SHALL refuse before reading the file or standard input.

#### Scenario: A named provider's Token is requested in a terminal

- **WHEN** `aigw setup --from <file> --provider dmxapi` runs in a terminal
- **THEN** setup SHALL prompt only for the `dmxapi` Token, without echoing it.

#### Scenario: A provider is named without a terminal or Token input

- **WHEN** `aigw setup --from <file> --provider dmxapi` runs without a terminal
  or with `--json`, and without `--token-stdin`
- **THEN** setup SHALL refuse before any write and name `--token-stdin`.

#### Scenario: A terminal asks for a needed Token

- **WHEN** `aigw setup --from <file>` runs in a terminal without `--provider`,
  and one or more of the file's providers have no Token
- **THEN** setup SHALL first ask which of those providers the person has a
  Token for, listing each by its label and accepting no choice
- **AND** SHALL prompt, without echoing, only for the chosen providers' Tokens,
  where Enter alone skips that provider
- **AND** a `--json` or non-terminal run SHALL defer those providers, because
  `command-line` lets neither prompt.

#### Scenario: A Token typed in a terminal is not accepted

- **WHEN** text entered at a prompt is not one Token or fails that validation
- **THEN** setup SHALL name the reason, and the provider when it answered, and
  offer to enter the Token again or to skip that provider, and first to try the
  same Token again when the validation got no verdict that a later request may
  give
- **AND** a provider skipped after a refusal SHALL be reported as not
  accepted, and after no verdict as not verified, each with its next step,
  while setup applies the team file, stores the other accepted Tokens and exits 0.

#### Scenario: Tokens come from the environment

- **WHEN** `--token-stdin` is given while the read-only environment backend
  supplies Tokens
- **THEN** setup SHALL refuse before any write and name the provider's
  environment variable.

#### Scenario: One provider's Token is available

- **WHEN** exactly one provider in the team file has a Token
- **THEN** setup SHALL fill each client without a selection from that
  provider's recommended models, in declared order
- **AND** a model that no recommendation names SHALL stay manual-only
- **AND** the other providers' missing Tokens SHALL NOT block setup.

### Requirement: The shipped team file is reviewed and directly usable

The repository SHALL publish one token-free, reviewed `manifests/team.toml` in
schema 8 that `aigw setup --from` applies without credentials or installed
clients. It SHALL declare providers, models, families and recommendations, with
stable lower-case family IDs and each model ID exactly as its provider serves
it. A model SHALL enter it only with authenticated inference and client and
protocol evidence, and it SHALL hold no inferred model matrix or proxy endpoint.

#### Scenario: The shipped team file stays reviewed

- **WHEN** the repository ships its team file
- **THEN** fictitious providers, workstation paths and parallel example files
  SHALL NOT remain
- **AND** a model outside the recommendations SHALL require explicit selection
  even if its provider has a Token or its family matches.

#### Scenario: A team member applies the reviewed file

- **WHEN** a team member downloads `manifests/team.toml` and applies it with
  `aigw setup --from`
- **THEN** AIGW SHALL apply its providers, models, families and
  recommendations without a second provider or model-name policy
- **AND** Tokens SHALL stay outside the file
- **AND** recommended selections SHALL come from the file rather than from
  model-version literals in this specification.

#### Scenario: One provider offers a subset of families

- **WHEN** one provider with a Token has a compatible recommended model but no
  models for other families or clients
- **THEN** setup SHALL fill only the clients without a selection that it can
  serve
- **AND** SHALL keep explicit selections and defer the other capabilities.

#### Scenario: Every provider that serves a recommended family is recommended

- **WHEN** a provider in the shipped team file has a model of the family that a
  client's first recommendation names, and the client can use that model
- **THEN** that client's recommendations SHALL name a model of that provider
- **AND** a team member holding only that provider's Token SHALL get that
  client configured by applying the file.

#### Scenario: A provider uses a channel-specific model ID

- **WHEN** DMXAPI serves a CC, SSVIP or CDX channel variant of one vendor model
- **THEN** that model SHALL name the same family as its base model and keep the
  provider's exact model ID.

#### Scenario: A model needs no duplicate label

- **WHEN** a model omits `label`
- **THEN** human, JSON and client presentation SHALL derive its label from the
  provider's and the family's labels, as `Provider · Family`, or as
  `Provider · model-id` when the model is its own family
- **AND** a model whose ID differs from its family's and begins or ends with
  that family's ID, ignoring case, SHALL add the rest of its ID as written
  without the separators between them, as `Provider · Family · cdx` for
  `gpt-6.2-sol-cdx` or `Provider · Family · codex` for `codex-gpt-6.2-sol`
- **AND** an explicit channel or user label SHALL override that derivation
- **AND** `aigw config export` SHALL omit a stored label equal to the derived
  one while keeping a distinct explicit label.

#### Scenario: Two models of one provider share a label

- **WHEN** a configuration or team file gives two models of one provider the
  same label, explicit or derived, ignoring case
- **THEN** AIGW SHALL refuse it, name both models and the label, and name a
  fix that does not first load the refused configuration.
