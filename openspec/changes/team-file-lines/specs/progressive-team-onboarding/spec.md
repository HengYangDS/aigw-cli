## MODIFIED Requirements

### Requirement: The shipped team file keeps one general family per other vendor

The shipped team file SHALL hold one family for each product line of Anthropic, OpenAI and Google, and exactly one
general family for each other vendor. A curated line's family SHALL be its newest generally available model, and
another vendor's family its most capable generally available general model, whose provider models qualify. A
qualified successor SHALL replace its family and models, and applying the file SHALL keep explicit client selections
of the replaced models.

#### Scenario: A vendor's more capable general model becomes available

- **GIVEN** the team file holds one general family for a vendor
- **WHEN** that vendor makes a more capable general model generally available
  and its models qualify
- **THEN** the team file SHALL replace that vendor's family and models with the
  successor
- **AND** SHALL NOT keep both as general families
- **AND** applying the team file SHALL keep explicit client selections of the
  replaced models.

#### Scenario: A newer model is not generally available or not qualified

- **WHEN** a vendor's newer model is limited to selected testers or its models
  fail qualification
- **THEN** the team file SHALL keep that vendor's current family.

#### Scenario: The owner names a curated vendor's families

- **WHEN** the owner names the product lines of Anthropic, OpenAI or Google,
  such as Claude Fable, Opus, Sonnet and Haiku
- **THEN** the team file SHALL hold one family for each named line, with every
  provider model of it that qualifies, channel variants included
- **AND** a line that the owner no longer names SHALL leave the file.

#### Scenario: A curated line makes a newer model generally available

- **WHEN** a curated line makes a newer model generally available and its
  provider models qualify, such as GPT-6.2 Astra after GPT-6 Astra
- **THEN** the team file SHALL replace that line's family and models with the
  newer model
- **AND** the repository's tests SHALL fail while one line holds two families.
