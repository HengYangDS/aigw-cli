## ADDED Requirements

### Requirement: Human output names clients and protocols by their labels

Human output SHALL name each client by its registry display label and each
protocol by its protocol label, in rows, prose and problems alike, and SHALL
NOT capitalize an identifier into a third name. Help SHALL list the
identifiers that a flag accepts, and copyable commands SHALL keep identifiers.

#### Scenario: A client with a multi-word label appears in human output

- **WHEN** check, test, verify, status, sync, repair, migration, selection,
  route or account output names Claude Desktop
- **THEN** it SHALL use `Claude Desktop`
- **AND** JSON output SHALL keep the identifier `claude-desktop`.

#### Scenario: Help lists client choices

- **WHEN** help describes a `--for` flag
- **THEN** it SHALL list the identifiers that the flag admits.

#### Scenario: Human output shows a protocol

- **WHEN** selection or migration output shows a binding's protocol
- **THEN** it SHALL use the protocol label, such as `OpenAI Responses`.
