## ADDED Requirements

### Requirement: Product packages form one downward layer stack

The architecture gate SHALL declare product packages and repository tools as
one ordered stack of layers. A package's production files SHALL import only
its own descendants, the seams that the policy declares for its subtree, and
packages in lower layers. A repository tool's production files SHALL import
product packages only below the use-case layer. Test files are fixtures and
stay outside the stack.

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
- **THEN** the source-size gate fails and names the specification and its
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

### Requirement: Portable source

Product source SHALL NOT encode a personal identity, home directory, private
Forge coordinate, local checkout path, credential, signing key, fingerprint,
signing program, trust anchor, foreign-application private path, or external
service lifecycle. CI SHALL provide trust material only from protected context.

#### Scenario: Build in another team environment

- **WHEN** the repository is cloned under a different user, directory, host, or
  Forge
- **THEN** build, verification, setup, repair, and uninstall SHALL not require
  the original contributor's machine, account, key, IDE, or workstation state

#### Scenario: An operator uses the installed command

- **WHEN** another team installs AIGW on a supported host
- **THEN** the installed executable SHALL provide the user command surface
- **AND** repository tools SHALL remain developer-only entrypoints
- **AND** documented configuration, credentials and discovery inputs SHALL not
  require an author-specific path, identity, key, service or foreign product.

#### Scenario: An operator selects environment credentials

- **WHEN** `AIGW_SECRET_BACKEND=env` is selected
- **THEN** the documented Account variable mapping SHALL supply credentials
  without persistence
- **AND** the user guide SHALL explain process inheritance and direct installed
  command invocation without exposing real credentials.

### Requirement: Enforced semantic ownership and quality

Each behavior and policy SHALL have one semantic owner. Composition roots SHALL
assemble declared owners; source gates SHALL enforce positive package topology,
dependency direction, public surfaces, portability, and the canonical coverage
policy. Compatibility facades and duplicate policy owners SHALL not be retained.

#### Scenario: semantic ownership regresses

- **WHEN** a change violates declared topology or dependency direction, or misses the canonical coverage policy
- **THEN** verification SHALL fail with the exact semantic owner and evidence gap.

#### Scenario: Architecture or coverage regresses

- **WHEN** a change violates declared semantic ownership or dependency direction, or misses the canonical package or aggregate coverage policy
- **THEN** local and hosted verification SHALL fail before publication.

#### Scenario: Foreign-host absolute path enters policy

- **WHEN** policy contains an absolute or parent-traversing path in another host's syntax
- **THEN** validation SHALL reject it identically on macOS, Linux, and Windows.

#### Scenario: No admitted branch authority exists

- **WHEN** no maintained admitted analyzer can measure a proposed branch metric
- **THEN** the quantitative policy SHALL omit that unsupported claim
- **AND** native statement evidence and complete package observation SHALL remain
  enforced without being relabeled as branch coverage.

#### Scenario: A tool needs shared release policy

- **WHEN** repository release tooling and product update read the same release artifact format
- **THEN** both import that format from its one product owner
- **AND** each still validates its own authority-bound inputs: the release tool its source and lock, product update its trust anchors and downloaded bytes.

#### Scenario: A legacy concatenated name remains

- **WHEN** a package appears outside the declared direct-owner topology or a repository tool imports a product package outside its admitted layers
- **THEN** the architecture gate fails with the exact path and dependency.

#### Scenario: an ordinary provider is added

- **WHEN** a provider is added below the existing provider owner without changing topology
- **THEN** the existing positive topology admits it without changing repository-shape policy.

### Requirement: Deterministic local verification

Local verification MUST use controlled fixtures rather than an undeclared
public network dependency or host-provided Git templates and hooks. A native
qualification that needs public services SHALL run only in a quality gate that
declares network access.

#### Scenario: A local test depends on external state

- **WHEN** a verification test would contact a public endpoint instead of its
  controlled fixture
- **THEN** local verification SHALL fail or the fixture SHALL intercept the
  exact request without public network I/O

#### Scenario: A host provides Git templates

- **WHEN** the host configures a Git template directory with hooks,
  configuration or exclusions
- **THEN** repositories created by verification tests SHALL contain none of
  that content

#### Scenario: A host provides global Git hooks

- **WHEN** the host's global Git configuration names a hook directory
- **THEN** verification tests SHALL NOT execute those hooks
- **AND** fixtures SHALL keep their own hook directories and the host's other
  global settings

#### Scenario: A native tool needs public services

- **WHEN** a test qualifies a native tool against public advisory or license
  services
- **THEN** it SHALL run only in a quality gate that declares network access
- **AND** offline behavior tests SHALL cover the same evidence handling with
  controlled fixtures

### Requirement: Source-bound quantitative evidence

A quantitative acceptance observation MUST retain its raw counts and bind the
measured source revision, tree, toolchain and policy through the owning
verification record. Displayed percentages SHALL derive from those counts.
Native output and its invocation context MAY jointly carry this evidence;
tracked source SHALL NOT require a self-referential commit identifier or a
parallel claim-digest registry.

#### Scenario: Quantitative evidence is incomplete or inconsistent

- **WHEN** an acceptance observation omits its measured source context or raw
  counts, or its percentage disagrees with those counts
- **THEN** it SHALL NOT establish the claimed quantitative acceptance
- **AND** the native measurement owner SHALL retain the raw failure rather than
  manufacture missing facts in a second record.

### Requirement: Foreign applications remain independent

AIGW SHALL NOT depend on, discover, configure, align, verify, repair, or control
foreign applications or their private runtime state.

#### Scenario: A foreign application is installed

- **WHEN** AIGW runs on a machine with unrelated applications
- **THEN** product behavior and acceptance SHALL remain independent of those
  applications, their configuration, sessions, caches, and runtime

### Requirement: Native client fixtures are repository-controlled

Cross-platform tests SHALL construct client executables from test-owned
fixtures rather than borrowing unrelated host toolchain executables.

#### Scenario: Windows tests an unreadable Claude executable

- **WHEN** native Windows verification exercises client-executable read failures
- **THEN** the fixture SHALL be an isolated executable controlled by the test
- **AND** the result SHALL not depend on the installed Go toolchain path or contents.

### Requirement: Native Markdown formatting preserves semantic containers

Current authored Markdown SHALL use the locked original Prettier formatter with
preserved prose wrapping and native embedded-language formatting. The existing
fix and check commands SHALL consume the same declared inventory without ambient
configuration. Other lint rules SHALL accept formatter-canonical containers;
immutable archives and native byte-exact ignores SHALL remain unchanged.

#### Scenario: Canonical lists retain their semantic separation

- **WHEN** native Prettier formats tight, loose, nested or quoted lists, including
  wrapped paragraphs, fenced examples and tables
- **THEN** the existing check SHALL accept the native output without a spacing
  postprocessor or custom paragraph parser
- **AND** native ignored examples and unrelated source SHALL retain their bytes.

#### Scenario: A real formatting journey converges once

- **WHEN** authored inputs differ from their native canonical output
- **THEN** the check SHALL refuse without mutation
- **AND** the fix SHALL produce the native output on that same inventory
- **AND** a second fix SHALL leave every byte unchanged and the check SHALL pass.
