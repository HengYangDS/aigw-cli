## ADDED Requirements

### Requirement: Codex lists the models that its table describes

In the model table that AIGW projects for Codex, AIGW SHALL list the models that a client lists for the selected
provider and protocol, the selected model and the first model of each other family in configuration order, wherever
the client's bundled table holds the model's own entry or its family's. Every other entry that the client lists SHALL
stay in the table, hidden from Codex's picker. An entry that the client hides SHALL stay hidden, and so SHALL an alias
of it.

#### Scenario: The picker lists the configured families

- **GIVEN** the selected provider serves `gpt-6.2-sol-cdx` in the family `gpt-6.2-sol`, and `gpt-6.1-terra` and
  `deepseek-v4.1` each as its own family, on the Responses protocol
- **AND** the client's table lists `gpt-6.2-sol`, `gpt-6.1-terra` and `gpt-5.5` and hides `codex-auto-review`
- **WHEN** AIGW projects Codex's table with `gpt-6.2-sol-cdx` selected
- **THEN** the table SHALL list the alias `gpt-6.2-sol-cdx` and the entry `gpt-6.1-terra`
- **AND** SHALL hide `gpt-6.2-sol` and `gpt-5.5` and keep `codex-auto-review` as the client has it
- **AND** SHALL hold no entry for `deepseek-v4.1`.

#### Scenario: A listed model's entry is one that the client hides

- **WHEN** the entry that describes a listed model, its own or its family's, is hidden in the client's table
- **THEN** the model's entry SHALL stay hidden.

#### Scenario: The table describes no listed model

- **WHEN** the client's table describes none of the listed models
- **THEN** the projected table SHALL list no entry and SHALL keep every entry of the client's table, hidden.
