## ADDED Requirements

### Requirement: Terminal layout follows semantic fields and display width

Human-facing command output SHALL use the existing presentation owner for
terminal-cell measurement, ANSI-aware layout and wrapping. Related command and
description fields SHALL share one measured column and switch as a group to
stacked layout when content cannot fit or contains multiple lines. Command
metadata SHALL own executable names; manual padding and shell-comment syntax
SHALL NOT substitute for structured explanatory fields. Existing native text
libraries SHALL own word and grapheme wrapping without dropping content.

Every public command's help SHALL retain its native option meaning at wide and
narrow widths. The final option display SHALL use the same width-aware renderer;
pflag SHALL remain the option-grammar owner. Color SHALL NOT alter text alignment.
The tests SHALL verify display width, field preservation and actual description
columns rather than reproduce manually padded source strings. Machine output,
credential-helper output and interactive input contracts SHALL remain unchanged.
Executable examples and follow-up commands SHALL preserve their original
characters, quoted whitespace and explicit line breaks. The terminal SHALL own
visual soft wrapping; a copyable command's logical line MAY exceed the available
width. Explanatory usage grammar SHALL remain width-aware text rather than an
executable command. Formatting SHALL NOT introduce shell continuation syntax.

#### Scenario: Copy an executable command from a narrow terminal

- **WHEN** an executable example or follow-up command exceeds the terminal width
- **THEN** its output SHALL preserve the command bytes apart from presentation
  indentation, optional styling and the final output newline
- **AND** neither quoted whitespace nor argument tokens SHALL be rewritten to
  satisfy an explanatory-text width rule.

#### Scenario: Starting commands have different lengths

- **WHEN** root help presents the setup, selection and readiness journey
- **THEN** command and explanation SHALL be distinct fields with one description
  column derived from the group's longest label
- **AND** a renamed root command SHALL propagate without hard-coded product text.

#### Scenario: A terminal is narrow or a value spans multiple lines

- **WHEN** human output contains long labels, unbroken values or multiple lines
- **THEN** the renderer SHALL retain text and continuation indentation within the
  available display width and use stacked rows where necessary
- **AND** colored and plain output SHALL preserve the same layout.

#### Scenario: Inspect the complete public command tree

- **WHEN** every public command renders help at supported narrow and wide widths
- **THEN** headings, descriptions, usage grammar and options SHALL fit those widths
- **AND** native option meaning and credential-free help SHALL remain intact.

## MODIFIED Requirements

### Requirement: complete delivery evidence

Quality completion SHALL require distinct evidence for the complete local
graph, exact-HEAD proof, native hosted CI, independent peer publication, exact
branch and tag identity, asset integrity, installation, runtime acceptance, and
repository housekeeping. A release SHALL be complete only when its one signed
tag object, immutable assets, checksums, peer-native Release records, and
supported-platform acceptance are verified at their owning boundaries. Every
selected peer SHALL verify its own objects and assets; native platform evidence
MAY be supplied by the admitted aggregate executor set without duplicating
unavailable runners or weakening the platform requirement.

macOS public-release binaries SHALL have an explicitly supplied certificate-bound
identity and Hardened Runtime before archive construction. Offline qualification
signing and archive timestamps SHALL use the release epoch. Production
distribution SHALL separately require a publisher-controlled Apple-issued
Developer ID identity, secure timestamp, successful notarization and native
artifact verification.
Reproducible payload evidence SHALL remain distinct from externally issued
timestamp and notarization evidence. Selected peers SHALL receive the same
immutable distribution bytes, not independently re-signed artifacts.
Missing signing inputs SHALL stop public-release construction without
prompting, provisioning an identity or substituting ad-hoc signing. Native
acceptance SHALL select only its host operating system; unrelated platform
credentials SHALL NOT be prerequisites. Consuming published assets SHALL NOT
require their private signing keys.
End users SHALL NOT require developer membership or private signing material
to install and use published AIGW. A publisher and an end user MAY be the same
person, but their roles and prerequisites SHALL remain distinct.

Explicit local delivery SHALL reuse the existing archive, signature, provenance,
installation and rollback owners. Its version SHALL bind the source commit,
remain distinct from the release baseline and precede the next release.
Local macOS archives MAY use native ad-hoc signing without claiming publisher
identity, notarization or retained native-Keychain authorization. Verification
SHALL require trusted artifact signatures and the exact signed source commit.
Both Forge publishers SHALL reject local delivery identities even when a signed
tag exists. The default public build SHALL NOT silently select local delivery.
Route health checks SHALL evaluate actual configuration and credentials rather
than infer operational failure or distribution trust from version spelling.

macOS retained-Keychain qualification SHALL exercise partitioned database format
`0x200` and both designated-requirement and application-partition authorization.
A self-signed certificate with a matching designated requirement SHALL NOT count
as proof of stable partition identity across changed code hashes. The current
same-executable reader qualification SHALL use an approved Apple-recognized
publisher identity. Every admitted reader architecture SHALL independently prove
retained-credential access through CLI updates, actual reader-byte updates and
rollback without prompts or ACL mutation. Initial enrollment or legacy-item
re-enrollment SHALL require its own explicit authorization and SHALL NOT
substitute for routine retained-item acceptance. A stable adapter proposal SHALL
prove its caller and security-update boundaries before replacing the current
reader; a successful unchanged-reader case alone SHALL NOT admit that design.

Ordinary macOS source tests SHALL exclude genuine host Keychain operations
through the native `keychain_integration` build tag while retaining fake-ABI
tests and every product package. Static analysis SHALL include the tagged tests.
Every real Keychain test entrypoint SHALL require the explicit disposable-host
scope before native access; a temporary path SHALL NOT establish host isolation.
Explicit system-credential qualification SHALL include the tag in the existing
native coverage invocation, including full-quality execution, without changing
the package inventory or coverage floor. The scope declaration SHALL NOT be
represented as a sandbox or proof that the machine is disposable. Source-only
success SHALL NOT substitute for required native credential evidence.

Manual native qualification MAY select one supported platform through the
existing CUE-owned Forge projections. Omitted or `all` selection SHALL preserve
the complete available platform set. Selection SHALL NOT remove source quality
or narrow review, accepted-branch push or tag admission. A partial manual result
SHALL prove only its selected platform, not complete release readiness or
required review admission. A Forge's unavailable runner SHALL remain an explicit
capacity boundary rather than an inferred product pass.

#### Scenario: A maintainer qualifies an updated Windows toolchain

- **WHEN** manual verification explicitly selects Windows and full quality
- **THEN** the Windows native job SHALL run its existing complete quality,
  source and packaged lifecycle commands without launching unrelated native jobs
- **AND** source quality SHALL remain selected and other required platforms
  SHALL retain their independent evidence obligations.

#### Scenario: A platform selector is supplied during release or review admission

- **WHEN** a tag, review or accepted-branch push triggers verification
- **THEN** its complete available native set SHALL remain selected
- **AND** manual qualification SHALL NOT replace missing review checks.

#### Scenario: A user consumes the publisher's signed release

- **WHEN** a user installs and invokes published AIGW
- **THEN** no developer membership or private signing key SHALL be required
- **AND** the exact reader's native credential permission SHALL remain a separate
  acceptance obligation from the artifact signature and distribution trust.

#### Scenario: A proposed adapter survives only CLI-only updates

- **WHEN** unchanged adapter bytes retain item access while the CLI changes
- **THEN** actual adapter replacement, rollback and caller authorization SHALL
  remain unproved until their own retained-item journeys pass
- **AND** preserving a vulnerable old reader SHALL NOT satisfy safe updates.

#### Scenario: Ordinary source verification runs on an operator workstation

- **WHEN** Go tests run without the Keychain integration build tag
- **THEN** native Keychain test files SHALL be excluded while product packages
  and fake-ABI tests remain selected
- **AND** this result SHALL NOT qualify retained system credentials.

#### Scenario: Explicit Keychain qualification has no disposable-host scope

- **WHEN** macOS native qualification enables system credentials without
  `AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE=ephemeral-host`
- **THEN** it SHALL fail before executing any quality, test or release command
- **AND** directly selected integration tests SHALL reject that scope before
  native credential access.

#### Scenario: Explicit Keychain qualification uses a disposable host

- **WHEN** macOS native qualification selects both system credentials and the
  disposable-host scope
- **THEN** the existing coverage invocation SHALL include integration tests once
- **AND** selecting full quality SHALL preserve that same inclusion and scope.

#### Scenario: A private Keychain fixture models the system authorization boundary

- **WHEN** native credential tests create an isolated Keychain
- **THEN** they SHALL prove partitioned format and the item's partition ACL
- **AND** a same-byte reader SHALL retain access while a changed self-signed
  code-hash partition SHALL be denied without modifying access policy.

#### Scenario: Offline certificate-signed archives are rebuilt

- **WHEN** offline qualification uses identical source, toolchain, isolated
  signing inputs and release epoch on either side of a wall-clock boundary
- **THEN** the complete archive matrix SHALL be byte-identical
- **AND** macOS binaries extracted from the archives SHALL satisfy the declared
  native signature requirement and enable Hardened Runtime
- **AND** that result SHALL NOT establish production timestamping, notarization
  or access to retained credentials.

#### Scenario: Native Linux or Windows acceptance builds its asset

- **WHEN** native acceptance runs without macOS signing inputs
- **THEN** it SHALL build only its current operating system's declared targets
- **AND** full release construction SHALL still require the complete matrix.

#### Scenario: An authorized local delivery has no publisher certificate

- **WHEN** an operator explicitly selects local construction
- **THEN** the existing pipeline SHALL create source-bound signed artifacts
  without requiring a public-distribution certificate
- **AND** native lifecycle and real-client acceptance SHALL consume those exact
  artifacts before installation
- **AND** public publication and native store trust SHALL remain separate claims.

#### Scenario: Native signing authority is missing

- **WHEN** macOS or complete release construction lacks a required identity,
  password-file or designated-requirement input
- **THEN** it SHALL stop before external build execution
- **AND** existing release output, credentials and host trust SHALL remain intact.

#### Scenario: System Keychain qualification lacks a supplied signing identity

- **WHEN** the current same-executable macOS acceptance requests system
  credential-store verification without publisher-supplied signing inputs
- **THEN** fixture preparation SHALL fail before generating a disposable signing
  identity, constructing an archive or touching the system credential store
- **AND** the diagnostic SHALL identify missing signing authority rather than
  a later credential-read failure
- **AND** supplying inputs SHALL NOT itself establish certificate trust or
  retained-credential authorization.

#### Scenario: both publication planes complete

- **WHEN** GitLab and GitHub independently publish one accepted product release
- **THEN** their commit and annotated tag object identifiers SHALL equal local
  Git exactly
- **AND** their asset manifests and supported-platform semantics SHALL agree.

#### Scenario: local proof passes but delivery is incomplete

- **WHEN** hosted CI, a selected peer, exact object identity, asset integrity,
  installation, runtime acceptance, or lane retirement remains unverified
- **THEN** the repository SHALL report that stage as incomplete and SHALL NOT
  claim terminal completion.

#### Scenario: terminal closeout succeeds

- **WHEN** every delivery stage passes for the exact accepted product object
  and obsolete lanes, policies, compatibility paths, temporary assets, and
  stale runtime residue are retired
- **THEN** the repository MAY report completion with receipts for each
  independent boundary.

#### Scenario: release metadata exists without publication

- **WHEN** `VERSION` and `CHANGELOG` name a release but either selected peer
  lacks its exact signed tag object, Release record, or assets
- **THEN** delivery SHALL remain incomplete.
