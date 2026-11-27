## MODIFIED Requirements

### Requirement: The shipped team file is reviewed and directly usable

The repository SHALL publish one token-free, reviewed `manifests/team.toml` in
schema 9 that `aigw setup --from` applies without credentials or installed
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
  client configured by applying the file
- **AND** each client's recommendations SHALL name at most one model of a
  provider, in the order in which the file declares its providers.

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

#### Scenario: The shipped team file is at schema 9

- **WHEN** the repository ships its team file
- **THEN** it SHALL hold `version = 9` and no recommendation SHALL hold `authentication` or `model_provider`
- **AND** `aigw setup --from` of this release SHALL apply it.
