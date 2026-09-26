# Spec Delta

## MODIFIED Requirements

### Requirement: Source acceptance precedes delivery completion

Source acceptance SHALL require valid Change artifacts, exact-source quality,
and authorized object-preserving integration. Active Changes MAY reach accepted
or release refs before delivery completes. Tasks SHALL close implementation and
candidate acceptance before archive; canonical specs and design SHALL retain
delivery duties until proved. Archive SHALL precede stable tagging without
claiming delivery or blocking local integration. OpenSpec SHALL remain the sole
Change-artifact validator.

#### Scenario: Active Change reaches source verification

- **WHEN** source verification observes an active Change in a work lane,
  proposal, accepted branch or release branch
- **THEN** the same official artifact validation and product quality graph run
- **AND** the presence of its task carrier alone SHALL NOT reject valid source
- **AND** malformed artifacts or failed quality checks still block acceptance
- **AND** source acceptance SHALL NOT mark pending publication, installation or
  cleanup complete.

#### Scenario: Archive preserves pending delivery

- **WHEN** source and candidate tasks are complete but a peer's CI, assets,
  installed lifecycle or retirement remains pending
- **THEN** archive SHALL retain those duties in canonical specs and design
- **AND** it SHALL NOT claim the pending external delivery is complete.
