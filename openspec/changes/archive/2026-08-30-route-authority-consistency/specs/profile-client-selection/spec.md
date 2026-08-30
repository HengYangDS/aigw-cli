## MODIFIED Requirements

### Requirement: Ambiguous or conflicting selection fails closed

Client selection SHALL have exactly one authority per invocation. A named
Profile SHALL supply its declared client; otherwise an explicit client SHALL
select that client's current Route. AIGW SHALL NOT infer either value from
names, models, endpoints, providers, or the other client's Route.

#### Scenario: Profile and client selectors are combined

- **WHEN** an operator supplies both `--profile` and `--for`
- **THEN** the command SHALL fail before credential or network access
- **AND** SHALL instruct the operator to keep exactly one selector.

#### Scenario: Selected profile has no declared client

- **WHEN** `--profile` selects an invalid Profile without an admitted client
- **THEN** the command SHALL fail before credential or network access
- **AND** SHALL report that the Profile lacks its required client declaration.

#### Scenario: Explicit client conflicts with the profile

- **WHEN** an operator supplies both `--for` and `--profile`, whether or not
  their client values would match
- **THEN** the command SHALL reject the redundant selectors
- **AND** SHALL instruct the operator to keep exactly one selector.

#### Scenario: Explicit client uses its selected Route

- **WHEN** an operator supplies `--for <client>` without `--profile`
- **THEN** the command SHALL use only that client's selected Route
- **AND** SHALL report the exact missing Route when none is selected.
