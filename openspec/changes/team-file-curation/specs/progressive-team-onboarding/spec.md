# Spec Delta

## MODIFIED Requirements

### Requirement: The shipped team file keeps one general family per other vendor

The shipped team file SHALL hold exactly one general family for each vendor
other than Anthropic, OpenAI and Google, whose families the owner names. That
family SHALL be the vendor's most capable generally available general model
whose provider models qualify. A qualified successor SHALL replace that family
and its models rather than add a second general family, and applying the file
SHALL keep explicit client selections of the replaced models.

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

- **WHEN** the owner names the families of Anthropic, OpenAI or Google
- **THEN** the team file SHALL hold exactly those families, with every
  provider model of them that qualifies, channel variants included
- **AND** a family that the owner no longer names SHALL leave the file.
