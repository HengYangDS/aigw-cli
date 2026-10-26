# Spec Delta

## ADDED Requirements

### Requirement: Retry policy belongs to one client connection

A Client Binding SHALL own optional native request and stream retry budgets for
its selected connection. Undeclared budgets SHALL preserve client defaults.
Declaration, preview, resolution, projection, persistence and recovery SHALL
agree. AIGW SHALL NOT infer policy from provider names or URLs, execute retries,
or promise exactly-once inference.

#### Scenario: An operator delegates recovery to the endpoint

- **WHEN** Codex selects a connection with both retry budgets explicitly zero
- **THEN** its native provider table SHALL contain both zero values
- **AND** public preview and subsequent sync SHALL preserve the declaration,
  Account, provider identity, model, reasoning settings and unrelated clients.

#### Scenario: Native defaults remain undeclared

- **WHEN** a connection has no retry policy, or the operator explicitly selects
  native retries
- **THEN** AIGW SHALL omit the corresponding native provider fields
- **AND** SHALL NOT substitute a guessed default.

#### Scenario: A connection changes while policy is selected

- **WHEN** a selected upstream identity or effective destination changes
- **THEN** selection SHALL require explicit retry intent before any write
- **AND** a model-only selection SHALL retain the same connection policy
- **AND** an explicit verification override for a different connection SHALL NOT
  inherit that policy.

#### Scenario: The client cannot project the declared policy

- **WHEN** the selected client has no admitted native retry-budget capability
- **THEN** AIGW SHALL refuse before credential access, network access or writes
- **AND** SHALL NOT replace the capability with a helper or wrapper.

### Requirement: Retry policy preserves guarded recovery

Retry-policy changes SHALL use the existing guarded selection transaction and
native recovery. External edits SHALL remain semantic client-configuration
conflicts. Program rollback SHALL test the retained reader's actual ability to
read the declaration, refuse without mutation when incompatible, and SHALL NOT
silently lose policy or require manual field deletion.

#### Scenario: A managed budget was changed outside AIGW

- **WHEN** a native budget differs from the recorded owned projection
- **THEN** sync and selection previews SHALL identify a Codex configuration
  conflict without presenting it as a TOML syntax error
- **AND** SHALL preserve the external edit and owned state.

#### Scenario: A policy transaction fails after persistence

- **WHEN** native projection fails after the policy configuration was written
- **THEN** compensation SHALL restore exact unchanged owned preimages
- **AND** SHALL preserve newer external edits and report incomplete compensation.

#### Scenario: The retained program cannot read policy

- **WHEN** program rollback's isolated configuration probe refuses the declaration
- **THEN** the active and retained programs, configuration and credentials SHALL
  remain unchanged
- **AND** recovery SHALL name a compatible program or native configuration
  rollback, not manual policy deletion or an arbitrary version threshold.
