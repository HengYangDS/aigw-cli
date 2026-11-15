## MODIFIED Requirements

### Requirement: Credential availability is observable without disclosure

AIGW SHALL observe whether a provider's Token or balance credential is present
without retrieving its secret value. The observation SHALL distinguish present,
absent, and backend failure; it SHALL NOT report a backend failure as absence.
One command SHALL ask the credential backend for each fact of a credential's
availability once, and again only after the command itself changes that
credential.

#### Scenario: Read-only journey observes a present credential

- **WHEN** status, setup, sync, local checks, provider and model listings, or
  client configuration logic needs only credential availability
- **THEN** AIGW queries credential metadata without retrieving the value
- **AND** does not initiate authentication or credential mutation

#### Scenario: Credential backend observation fails

- **WHEN** the selected credential backend cannot determine availability
- **THEN** AIGW reports the backend failure
- **AND** does not describe the credential as missing

#### Scenario: Authentication needs the credential value

- **WHEN** AIGW performs an explicit provider or client authentication action
- **THEN** it retrieves the credential value from the same selected backend
- **AND** no availability cache or alternate backend becomes authoritative

#### Scenario: One command needs a credential's availability more than once

- **WHEN** one command needs the availability of the same Token or the same
  balance credential for its admission, its preview, its change and its next
  step
- **THEN** it asks the credential backend for each fact once
- **AND** it asks again after the command itself stores or removes that
  credential
- **AND** naming the environment variables that the read-only environment
  backend sees set reads the process's own environment and is outside this
  rule.

#### Scenario: A command undoes the Token that it stored

- **WHEN** a command stores a Token, a later step fails, and the command
  removes that Token again
- **THEN** what the command reports afterwards rests on an observation made
  after the removal, on every credential backend
- **AND** `aigw use` states that the Token that was entered was not kept.
