## MODIFIED Requirements

### Requirement: A client uses one model

`aigw use <client> <provider/model>` SHALL make that model the client's
selection and SHALL change only that client's selection and the client
configuration files that AIGW owns for it, except that a model or protocol that
it adds SHALL also update the files of other clients that list that provider's
models, and the result SHALL name them. Each client SHALL keep its own
selection, without a global one; `client-chosen-model` owns a model chosen in
another client.

#### Scenario: Select a model for one client

- **WHEN** the operator runs `aigw use codex dmxapi/gpt-6.1-sol` for a model
  that already declares `responses`
- **THEN** AIGW SHALL update only Codex's selection and the client
  configuration files that AIGW owns for Codex
- **AND** SHALL preserve every other client's selection, files and credentials
- **AND** its next action SHALL be `aigw check codex`.

#### Scenario: An added model reaches the clients that list its provider

- **WHEN** `aigw use codex dmxapi/gpt-6.2` adds that model while Hermes lists
  the `responses` models of DMXAPI
- **THEN** Hermes's client configuration file SHALL list the model in the same
  transaction, and the result SHALL name Hermes
- **AND** Hermes's selection SHALL stay unchanged.

#### Scenario: One model serves several clients

- **GIVEN** one model has protocols that several clients speak
- **WHEN** the operator selects it for each of those clients
- **THEN** AIGW SHALL keep one model and one selection per client
- **AND** SHALL NOT copy the model or introduce a global selection.

## REMOVED Requirements

### Requirement: Synchronization adopts a Hermes or Codex model choice

**Reason**: `client-chosen-model` owns a model chosen in a client, for every command and not for `aigw sync` alone.

**Migration**: `aigw sync --json` names a followed selection in `followed_selections` and no longer in
`adopted_selections`. A script that ran the Fix of a synchronization that found no added model for the client's
choice now receives the command that restores the recorded selection; the command that adds the chosen model is
named in the failure's text.
