## ADDED Requirements

### Requirement: The shipped catalogue keeps one general Model per other vendor

The shipped team catalogue SHALL hold exactly one general Model for each vendor
other than Anthropic and OpenAI, whose Models are curated per client. That Model
SHALL be the vendor's most capable generally available general model whose
Routes qualify. A qualified successor SHALL replace that Model and its Routes
rather than add a second general Model, and import SHALL keep explicit Client
Bindings to the replaced Routes.

#### Scenario: A vendor's more capable general model becomes available

- **GIVEN** the catalogue holds one general Model for a vendor
- **WHEN** that vendor makes a more capable general model generally available
  and its Routes qualify
- **THEN** the catalogue SHALL replace that vendor's Model and Routes with the
  successor
- **AND** SHALL NOT keep both as general Models
- **AND** importing the catalogue SHALL keep explicit Client Bindings to the
  replaced Routes.

#### Scenario: A newer model is not generally available or not qualified

- **WHEN** a vendor's newer model is limited to selected testers or its Routes
  fail qualification
- **THEN** the catalogue SHALL keep that vendor's current Model.
