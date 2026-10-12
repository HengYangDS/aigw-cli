## MODIFIED Requirements

### Requirement: Route selection is explicitly client-scoped

A Route SHALL identify one Account, one canonical Model, its exact upstream
identifier, and admitted protocol interfaces independently of client state.
A Client Binding SHALL select one compatible Route for one client.
Non-interactive selection SHALL require both identities; interactive selection
MAY prompt only for admitted choices. An explicit Codex forwarding destination
SHALL remain separate from the selected Account upstream.

#### Scenario: Select a Route for one client

- **WHEN** the operator runs `aigw use --for <client> <route>`
- **THEN** AIGW SHALL update only that client's binding and owned projection
- **AND** SHALL preserve every other client's selection, files, and credentials.

#### Scenario: One Route serves several clients

- **GIVEN** one Route exposes interfaces admitted by several clients
- **WHEN** the operator selects it independently for those clients
- **THEN** AIGW SHALL retain one Route and one Client Binding per client
- **AND** SHALL NOT duplicate the Route or introduce a global selection.

#### Scenario: Preview a Codex forwarding selection

- **WHEN** the operator supplies `--forwarding-endpoint`, `--dry-run`, and
  `--json` to an explicit Codex Route selection
- **THEN** AIGW SHALL report the proposed destination and Account upstream
- **AND** SHALL plan only the selected client's exact targets and actions,
  rejecting owned-target conflicts before rendering a successful preview
- **AND** SHALL read no credential value, issue no network request, and change
  no configuration, projection, backup, or checkpoint.

#### Scenario: Select and remove a forwarding destination

- **WHEN** the operator selects an explicit Codex forwarding destination or
  uses `--direct` to restore its selected Account endpoint
- **THEN** only the Codex destination SHALL change
- **AND** Account, provider, credential-reader identity, other clients, models,
  effort, and conversation state SHALL remain unchanged
- **AND** invalid or incompatible destinations and conflicting flags SHALL
  fail before credential access or writes.

#### Scenario: Preserve forwarding on an unchanged upstream

- **WHEN** ordinary Route selection retains the same Codex Account and protocol
- **THEN** AIGW SHALL preserve its explicit forwarding destination
- **AND** explicit Route selection changing the upstream identity SHALL withdraw
  the old destination.

#### Scenario: A direct Account or protocol edit invalidates forwarding

- **WHEN** an Account or protocol edit differs from the recorded forwarding
  upstream identity
- **THEN** AIGW SHALL refuse validation and persistence without changing owned files
- **AND** SHALL require restoring direct mode before editing that upstream
- **AND** SHALL NOT silently rebind the old forwarding destination.
