## ADDED Requirements

### Requirement: Claude Desktop keeps its own profile settings

AIGW SHALL judge its Claude Desktop profile by its managed meaning: the
selected gateway, credential helper and arguments, models and tool search,
and AIGW's fixed Chat, Cowork, Code, model-discovery, credential-kind and
authentication settings. A managed setting that the profile omits SHALL take
Claude Desktop's documented default. Every other profile key SHALL belong to
Claude Desktop and SHALL survive AIGW projections.

#### Scenario: Claude Desktop re-saves the profile

- **WHEN** Claude Desktop saves the AIGW profile with its own encoding,
  without settings that equal its defaults, and with its own settings added
- **THEN** synchronization SHALL treat the profile as unchanged and keep its
  bytes
- **AND** status SHALL report Claude Desktop as ready.

#### Scenario: A projection changes AIGW's values

- **WHEN** a Route or reader change projects new values into a profile that
  holds Claude Desktop's own settings
- **THEN** the managed settings SHALL take the new values
- **AND** Claude Desktop's own settings SHALL remain unchanged.

#### Scenario: A managed setting was changed outside AIGW

- **WHEN** the profile's managed meaning differs from AIGW's record
- **THEN** synchronization SHALL refuse without writing any file
- **AND** the refusal SHALL name each changed managed setting it can identify
  and how to restore AIGW's values.

#### Scenario: An earlier AIGW release reads the record

- **WHEN** this release has recorded ownership of an unchanged profile
- **THEN** an earlier release that reads the ownership record SHALL accept the
  profile.
