## MODIFIED Requirements

### Requirement: Shipped team catalogue is a curated capability contract

Team manifests SHALL declare Accounts, canonical Models, provider Routes, and
per-client Recommendations separately. Model and Route IDs SHALL be stable
lower-case; provider wire IDs and channels SHALL remain exact. Route admission
requires authenticated inference and compatible client/protocol evidence.
Setup SHALL need only one compatible Account, preserve explicit Client
Bindings, and exclude inferred Route matrices and proxy endpoints.

#### Scenario: One Account offers a subset of Models

- **WHEN** one connected Account has a compatible recommended Route but lacks
  Routes for other canonical Models or clients
- **THEN** setup SHALL activate only its usable unselected Client Bindings
- **AND** preserve explicit selections and defer unavailable capabilities.

#### Scenario: A provider uses a channel-specific wire ID

- **WHEN** DMXAPI exposes a CC, SSVIP, or CDX channel variant of one Model
- **THEN** that Route SHALL reference the same canonical Model as its base
  Route and carry its exact provider wire ID
- **AND** its lower-case Route ID SHALL not be derived from wire spelling.

#### Scenario: Ordinary Route display needs no duplicate label

- **WHEN** a Route omits `label`
- **THEN** human, JSON, and native-client presentation SHALL derive
  `Account · Model` from the declared labels
- **AND** an explicit channel or user label SHALL override that derivation
- **AND** native manifest export SHALL omit a stored label equal to the
  derived form while preserving an explicit distinct label.

#### Scenario: Account lists follow the declared presentation order

- **WHEN** Accounts declare presentation `order` values
- **THEN** `aigw account list`, human and JSON, SHALL list Accounts by that
  order and then by identity, as Route lists already do
- **AND** serialized configuration and manifest export SHALL keep their
  lexical identity order.
