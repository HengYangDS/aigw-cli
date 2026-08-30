## MODIFIED Requirements

### Requirement: Ambiguous or redundant selection fails closed

Client selection SHALL have exactly one authority per invocation. A named
Profile SHALL supply its declared client; otherwise an explicit client SHALL
select that client's current Route. AIGW SHALL NOT infer either value from
names, models, endpoints, providers, or the other client's Route.

#### Scenario: Profile and client selectors are combined

- **WHEN** an operator supplies both `--profile` and `--for`
- **THEN** the command SHALL fail before credential or network access
- **AND** SHALL instruct the operator to keep exactly one selector.

#### Scenario: Named Profile supplies its client

- **WHEN** an operator supplies `--profile <profile>` without `--for`
- **THEN** the command SHALL use the Profile's required client declaration
- **AND** SHALL reject an unknown Profile without guessing.

#### Scenario: Explicit client uses its selected Route

- **WHEN** an operator supplies `--for <client>` without `--profile`
- **THEN** the command SHALL use only that client's selected Route
- **AND** SHALL report the exact missing Route when none is selected.
