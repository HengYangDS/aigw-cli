## ADDED Requirements

### Requirement: Forwarding keeps the upstream it records

A forwarded Client Binding SHALL record the upstream identity and protocol it
reaches. A change to the bound Account's endpoint or protocol, an Account
rename or a manifest import that would change them SHALL be refused before any
write until the operator selects the direct connection. AIGW SHALL NOT silently
rebind a forwarded destination.

#### Scenario: An Account edit would change a forwarded upstream

- **WHEN** an Account endpoint or protocol edit, an Account rename or a manifest
  import would change the upstream of a forwarded binding
- **THEN** AIGW SHALL refuse before any write
- **AND** SHALL name `aigw use --for <client> <route> --direct` as the next
  action
- **AND** client files and owned state SHALL stay unchanged.

#### Scenario: A selection keeps the same upstream

- **WHEN** a forwarded client selects a Route whose upstream identity and
  protocol equal the recorded ones
- **THEN** forwarding SHALL remain selected.

#### Scenario: The operator selects the direct connection

- **WHEN** `aigw use --direct` selects a Route for a forwarded client
- **THEN** the binding SHALL drop the forwarding endpoint and its recorded
  upstream together
- **AND** a later Account edit SHALL succeed.

#### Scenario: A retained reader restores a checkpoint

- **WHEN** a retained 0.4.17 program restores a checkpoint written by this
  version
- **THEN** the checkpoint SHALL keep the shape that reader decodes.
