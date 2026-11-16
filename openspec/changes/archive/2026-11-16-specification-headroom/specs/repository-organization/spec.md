## MODIFIED Requirements

### Requirement: Main specifications stay within one reading unit

Each main specification SHALL own one capability and stay within the
repository's document line limit. Moving a requirement to another
specification SHALL keep every one of its scenarios, and a change that only
moves requirements SHALL leave each of them unchanged, word for word.

#### Scenario: A specification exceeds the limit

- **WHEN** a main specification exceeds the document line limit
- **THEN** the source-size gate fails and names the specification and its
  line count.

#### Scenario: A requirement changes owner

- **WHEN** a change moves a requirement to another capability
- **THEN** the locked merge keeps all of its scenarios under the new owner
- **AND** the previous specification no longer states it.

#### Scenario: Requirements move without a change

- **WHEN** a change moves requirements between main specifications and changes
  none of them
- **THEN** every requirement of the sources before the move stands byte for
  byte in exactly one main specification after it
- **AND** no other main specification changes.
