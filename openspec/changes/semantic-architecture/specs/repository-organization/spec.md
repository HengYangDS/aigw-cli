## ADDED Requirements

### Requirement: Product packages form one downward layer stack

The architecture gate SHALL declare product packages as one ordered stack of
layers. A package's production files SHALL import only its own subtree's seam
and packages in lower layers, and a repository tool's production files SHALL
import only its own subtree and product packages below the use-case layer.
Test files are fixtures and stay outside the stack.

#### Scenario: An import points upward or sideways

- **WHEN** a production file imports a package in its own or a higher layer,
  or one client adapter imports another
- **THEN** the architecture gate fails and names the importing package, the
  imported package and both layers
- **AND** no per-edge allowance can admit the import.

#### Scenario: A package joins a declared layer

- **WHEN** a package is added under a declared layer's path prefix and imports
  only lower layers
- **THEN** the architecture gate admits it without a policy change.

### Requirement: Client knowledge stays in its client package

Product code that names an admitted client or encodes its native files,
settings, commands or protocols SHALL live in that client's package. Outside
it, only the composition root that registers adapters and test fixtures SHALL
name a client.

#### Scenario: A shared package names a client

- **WHEN** a product package outside a client's package contains an identifier
  or string literal that spells that client's ID
- **THEN** the architecture gate fails with the path, the line and the client
  ID.

#### Scenario: A client's native value contains another client's ID

- **WHEN** a client's own native value contains another client's ID, such as
  the Hermes API mode `codex_responses`
- **THEN** the architecture policy declares that exact value as a native term
  of the first client
- **AND** the gate accepts it only as that literal in that client's package and
  fails when the declared term no longer occurs there.

#### Scenario: A client is added

- **WHEN** a test-only rehearsal adapter is registered
- **THEN** its change touches only its own package, the test composition root
  and its fixtures.

### Requirement: Command packages mirror the command tree

Each top-level command SHALL be implemented by one package named after it.
Acceptance scripts SHALL run the program in its own process for every
command's help, a representative success, a representative refusal and its
JSON output on macOS, Linux and Windows.

#### Scenario: A command has no package or script

- **WHEN** a top-level command is registered without a package of its name or
  without acceptance scripts
- **THEN** the repository gate fails and names the command.

#### Scenario: Command output changes

- **WHEN** a change alters a command's help, exit status or JSON output
- **THEN** that command's acceptance script fails until the same change
  updates the script.

### Requirement: One canonical name per concept

The product concept glossary SHALL give each concept one canonical term and
its rejected synonyms. Code identifiers, package names, specifications and
documentation SHALL use the canonical term, and no two packages SHALL share a
base name.

#### Scenario: A rejected synonym appears

- **WHEN** a tracked code, specification or documentation file uses a declared
  rejected synonym
- **THEN** the text gate fails with the path, the line and the canonical term.

#### Scenario: A package base name repeats

- **WHEN** two product or tool packages share a base name
- **THEN** the architecture gate fails and names both package paths.

### Requirement: Main specifications stay within one reading unit

Each main specification SHALL own one capability and stay within the
repository's document line limit. Moving a requirement to another
specification SHALL keep every one of its scenarios.

#### Scenario: A specification exceeds the limit

- **WHEN** a main specification exceeds the document line limit
- **THEN** the documentation gate fails and names the specification and its
  line count.

#### Scenario: A requirement changes owner

- **WHEN** a change moves a requirement to another capability
- **THEN** the locked merge keeps all of its scenarios under the new owner
- **AND** the previous specification no longer states it.

### Requirement: Local release builds write under build/artifacts

The repository's local release build SHALL write its candidate to a directory
named for the version and commit under `build/artifacts/`, and SHALL write no
release output to any other repository path.

#### Scenario: An operator builds a release candidate

- **WHEN** an operator runs the declared release build task
- **THEN** the candidate appears under `build/artifacts/<version>-<commit>/`
- **AND** no `dist/` directory is created at the repository root.
