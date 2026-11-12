# Spec Delta

## MODIFIED Requirements

### Requirement: A team file merges without redirecting providers or clients

`aigw setup --from <file>` SHALL apply a team file in one transaction. The
models, families and recommendations that it names SHALL replace the local ones,
and local-only entries SHALL stay unless `--prune` removes the unused ones. It
SHALL add new providers, take the file's provider labels and fill in a missing
balance probe, but SHALL NOT change an existing provider's endpoints or Token or
replace a client's selection; a differing endpoint or balance probe SHALL stay
local and be reported.

#### Scenario: The file and the configuration define the same model

- **WHEN** the team file and `aigw.toml` both define `dmxapi/gpt-6.1-sol` with
  different labels, families, protocols or capabilities
- **THEN** the model SHALL take the team file's definition
- **AND** models, families and recommendations that only `aigw.toml` names
  SHALL stay.

#### Scenario: The file names an existing provider with another label

- **WHEN** the team file gives `dmxapi` a label that differs from the local one
- **THEN** the provider SHALL take the file's label.

#### Scenario: The file names another endpoint or balance probe

- **WHEN** the team file gives `dmxapi` a `responses` endpoint or a balance
  probe that differs from the local one
- **THEN** the local value and the provider's Token SHALL stay
- **AND** setup SHALL report both values.

#### Scenario: The file gives a balance probe to a provider without one

- **WHEN** the team file gives `ucloud` a balance probe and the local `ucloud`
  has none
- **THEN** the provider SHALL take the file's balance probe.

#### Scenario: A client already has a selection

- **WHEN** the team file recommends another model for a client that already
  selects one
- **THEN** the client's selection SHALL stay
- **AND** the file's recommendation SHALL be stored for that client.

#### Scenario: The file would take a protocol from a client's selection

- **WHEN** the team file's definition of a model omits the protocol of a
  client's selection of that model
- **THEN** the model SHALL keep its local definition
- **AND** setup SHALL report that it kept it.

#### Scenario: A kept provider has no endpoint for a model's protocol

- **WHEN** the team file defines a model of an existing provider on a protocol
  for which the kept provider has no endpoint
- **THEN** setup SHALL add the model and report it as unusable on that protocol
  until `aigw provider edit --endpoint` supplies the endpoint
- **AND** no client SHALL be given the model on that protocol until then.

#### Scenario: The file's capabilities were qualified on another endpoint

- **WHEN** the team file declares capabilities on a protocol whose endpoint in
  the file differs from what the kept provider has
- **THEN** setup SHALL drop those capabilities and report them
- **AND** the model SHALL keep the capabilities that it has locally on that
  protocol.

#### Scenario: A recommendation names a model

- **WHEN** a recommendation names `dmxapi/gpt-6.1-sol-cdx`
- **THEN** setup SHALL store it under that `provider/model`
- **AND** a name that neither the team file nor `aigw.toml` defines SHALL be
  refused before any write, naming that recommendation.

#### Scenario: Prune unused local entries

- **WHEN** `aigw setup --from <file> --prune` runs
- **THEN** the local-only models that no client selects and no remaining
  recommendation names SHALL be removed in the same transaction as the merge
- **AND** so SHALL families that neither a remaining model nor the file names
- **AND** providers, their endpoints and Tokens, and every client's selection
  SHALL stay unchanged.

#### Scenario: The file lists providers or models in another order

- **WHEN** a team file lists providers, or one provider's models, in another
  order than the configuration
- **THEN** what the file names SHALL come first, in the file's order, and the
  entries that only the configuration holds SHALL follow in their local order,
  for providers and for each provider's models alike
- **AND** the merge SHALL change only that display order, never an endpoint, a
  Token or a client's selection.

#### Scenario: Preview a team file

- **WHEN** `aigw setup --from <file> --dry-run --json` runs
- **THEN** it SHALL apply the same merge validation as the write and report
  each difference that stays local, each local model whose definition the file
  replaces, including its label, each model that it would keep or report as
  unusable, each capability that it would drop, and the client selections that
  it would fill
- **AND** it SHALL NOT describe a client configuration file as written or as
  proof that the client or its Token works.
