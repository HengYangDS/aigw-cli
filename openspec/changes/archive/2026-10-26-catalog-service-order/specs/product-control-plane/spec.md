## ADDED Requirements

### Requirement: Route presentation groups admitted service types

Route presentation SHALL order by Account presentation rank and identity,
admitted protocol set in native service display order, Model presentation rank
and identity, then Route identity. This applies without explicit ranks.
Multi-interface Routes SHALL use their complete ordered protocol set without
selecting a runtime protocol. Canonical enumeration, exports, and Client
Bindings SHALL remain unchanged.

#### Scenario: A higher-ranked Model belongs to a later service group

- **WHEN** one Account has a Chat Completions Route ranked before a Responses
  Route by Model
- **THEN** human and JSON Route lists SHALL place the Responses group first
- **AND** preserve Model order within each service group.

#### Scenario: Listing does not change routing identity

- **WHEN** ranked, unranked, or multi-interface Routes are listed
- **THEN** repeated human and JSON listings SHALL have the same presentation order
- **AND** preserve stored configuration, canonical enumeration, exports, and
  explicit Client Bindings.
