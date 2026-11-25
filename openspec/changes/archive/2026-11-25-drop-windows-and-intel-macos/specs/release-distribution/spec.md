## MODIFIED Requirements

### Requirement: Platform trust is independent of version identity

Version parsing SHALL NOT establish distribution trust. Credential-free macOS
builds SHALL retain ad-hoc signatures, Hardened Runtime, and release-epoch
timestamps. Public macOS distribution requires Developer ID signing and
accepted notarization before publication.
Reproducibility, publisher trust, notarization, and credential authorization
SHALL remain separate claims. Consumers require no publisher secret or
developer membership.

#### Scenario: A version parses but distribution evidence is missing

- **WHEN** a valid stable version lacks required product or channel evidence
- **THEN** distribution SHALL stop with the specific missing evidence
- **AND** version parsing alone SHALL NOT claim release readiness.

#### Scenario: A user consumes the publisher's signed release

- **WHEN** a user installs and invokes published AIGW
- **THEN** no developer membership or private signing key SHALL be required
- **AND** the exact reader's native credential permission SHALL remain separate
  from artifact signature and distribution trust.

### Requirement: Final distribution bytes have one authority

Signing and notarization SHALL precede final inventory and checksums. Detached
SSH manifest signatures, provenance, and native acceptance remain required. Each
selected Forge and package-manager projection SHALL identify the same accepted
bytes. Native acceptance builds host targets; full construction emits the
complete matrix. Reproducibility remains separate from trusted timestamps and
notarization. Published bytes SHALL NOT be replaced under an existing release
identity.

#### Scenario: A published release already exists

- **WHEN** a newly signed artifact differs from a published artifact
- **THEN** it SHALL require a new release identity rather than replacing the published bytes.

#### Scenario: Internal archives are rebuilt without publisher credentials

- **WHEN** construction uses identical source, toolchain, and release epoch on
  either side of a wall-clock boundary without publisher credentials
- **THEN** the complete archive matrix SHALL be byte-identical
- **AND** extracted macOS binaries SHALL pass native signature verification
  with ad-hoc signatures and Hardened Runtime
- **AND** that result SHALL NOT establish publisher trust, notarization, or
  access to retained credentials.

#### Scenario: Native acceptance builds its asset

- **WHEN** native acceptance runs on macOS or Linux without Apple
  signing credentials
- **THEN** it SHALL build only its current operating system's declared targets
- **AND** full release construction SHALL still require the complete matrix
- **AND** construction SHALL leave existing credentials and host trust intact.

#### Scenario: Selected peers expose identical release assets

- **WHEN** GitLab and GitHub publish one accepted product release
- **THEN** their immutable asset bytes and manifests SHALL agree
- **AND** their supported-platform semantics SHALL be equivalent.

#### Scenario: The complete matrix is constructed

- **WHEN** full release construction emits the archive matrix
- **THEN** it SHALL contain exactly the `darwin-arm64`, `linux-arm64` and
  `linux-amd64` archives with their checksums and provenance
- **AND** no Windows archive, zip archive, `.exe` or `darwin-amd64` archive
  SHALL be built, signed or published
- **AND** the Homebrew cask SHALL name `darwin-arm64` as its only macOS archive.

#### Scenario: A Mac without ARM64 installs the cask

- **WHEN** a user installs the Homebrew cask on a Mac whose processor is not
  ARM64
- **THEN** the cask SHALL declare that it depends on ARM64, so that Homebrew
  refuses the installation
- **AND** no AIGW program SHALL be installed on that Mac.

### Requirement: Native released-artifact lifecycle acceptance

On macOS and Linux, AIGW SHALL prove the public installed-program
lifecycle using portable archives and checksums shaped like release assets:
install the older program, update, roll back, recover forward, and uninstall.
The journey MUST preserve declared configuration and credentials and finish
without an installed executable, rollback copy, staging file, or other owned
lifecycle residue.

#### Scenario: Released program completes the reversible lifecycle

- **WHEN** native acceptance runs on a supported operating system
- **THEN** the installed older program updates from a verified newer portable archive
- **AND** the installed program reports the newer version
- **AND** rollback restores the older version
- **AND** a second verified update restores the newer version
- **AND** uninstall removes the executable, its single rollback copy, and all staging residue
- **AND** retained configuration and credentials remain available as declared.

#### Scenario: Lifecycle evidence uses the product command plane

- **WHEN** native acceptance exercises update, rollback, forward recovery, or uninstall
- **THEN** it SHALL invoke the public AIGW command for that transition
- **AND** SHALL NOT substitute a package-level helper, platform-specific script state machine, Forge API, or external-gateway lifecycle.

### Requirement: Native release targets are independent of the build host

The macOS ARM64 artifact SHALL retain the declared macOS 13 deployment
floor supported by the locked Go toolchain and selected native APIs. A newer
build host or SDK SHALL NOT silently raise that floor. Native artifact evidence
SHALL inspect the linked executable, not infer compatibility from configuration
or successful execution on the build host.

#### Scenario: A newer SDK links the native credential implementation

- **WHEN** macOS construction uses external linking for the native credential API
- **THEN** the macOS archive SHALL declare macOS 13.0 as its minimum
- **AND** cached CGO objects compiled for a newer target SHALL NOT cause deployment warnings
- **AND** native signature and deterministic archive validation SHALL remain required
- **AND** linked-header acceptance SHALL NOT imply execution on an older OS.

#### Scenario: Native CGO cache contains a newer deployment target

- **WHEN** a private empty cache is warmed at a newer deployment target
- **THEN** the matching host architecture SHALL rebuild through the declared native release configuration
- **AND** its actual CPU, deployment floor and signature SHALL be verified without deployment warnings
- **AND** warmup and rebuild SHALL retain one original bounded deadline and no operator signing or Forge credentials
- **AND** the independent full-matrix journey SHALL retain the macOS archive and its deterministic signature.
