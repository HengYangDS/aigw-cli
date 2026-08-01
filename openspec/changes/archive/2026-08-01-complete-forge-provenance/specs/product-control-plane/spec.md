## MODIFIED Requirements

### Requirement: Portable source

Product source SHALL NOT encode a personal identity, home directory, private
Forge coordinate, local checkout path, credential, signing key, signing
program, or trust anchor. Publication context SHALL remain external to reusable
source.

#### Scenario: Build in another team environment

- **WHEN** the repository is cloned under a different user, directory, or Forge
- **THEN** ordinary build and source verification SHALL not require the original
  contributor's machine, path, account, key, fingerprint, or remote

### Requirement: Complete Forge commit provenance

Each Forge's protected publication context SHALL provide its actor, signer,
trust anchor, and remote coordinates. Every commit reachable from a published
branch tip MUST store that Forge actor as both author and committer and MUST
verify with the Forge's explicit trust input. A commit floor, mailmap, or
suffix-only exception MUST NOT weaken this invariant.

#### Scenario: Reachable history contains invalid provenance

- **WHEN** any commit reachable from the selected revision has a different
  author or committer, lacks a trusted signature, or is hidden behind a floor or
  mailmap
- **THEN** commit-provenance verification SHALL fail and publication SHALL stop

## ADDED Requirements

### Requirement: Semantic Forge history projection

A Forge-specific history projection MUST preserve every source commit's tree,
exact message bytes, author and committer timestamps, ordered parents, and
merge topology while replacing only author identity, committer identity,
signature, and parent object references required by the target graph. The
projection MUST be constructed and verified in an isolated object database
before a canonical or remote ref is changed.

#### Scenario: Replay complete history into another Forge identity

- **WHEN** an authorized publication operation projects a source graph into a
  target Forge identity domain
- **THEN** the target graph SHALL have one mapped commit per source commit, the
  same ordered semantic history, and a trusted target-Forge signature on every
  mapped commit

#### Scenario: Projection cannot prove exact semantics

- **WHEN** a source commit lacks a mapped parent, message bytes or timestamps
  differ, parent order or merge arity changes, object storage is shared, or a
  generated signature does not verify
- **THEN** projection SHALL fail before changing any canonical or remote ref

### Requirement: Recoverable published-history repair

An explicitly authorized repair of published history MUST capture immutable
recovery evidence before mutation. Local ref replacement MUST compare the
expected old object, and remote replacement MUST use the captured remote tip as
a lease.

#### Scenario: A remote advances during prepared repair

- **WHEN** a branch or tag no longer equals the old object captured by the
  recovery record
- **THEN** replacement SHALL stop without overwriting that ref and the repair
  SHALL remain incomplete

### Requirement: Atomic published-history replacement

An authorized repair MUST treat all affected branches, provider-native
annotated tags, releases, hosted CI, release assets, integrity records, and
active commit-bound evidence as one fail-closed operation. Completion MUST NOT
be claimed while an affected Forge still exposes invalid or mixed provenance.

#### Scenario: Both Forge graphs have been replaced

- **WHEN** every affected ref maps to a verified Forge-specific graph
- **THEN** completion SHALL additionally require exact-tip hosted CI, rebuilt
  provider-native releases, matching cross-Forge asset digests, refreshed
  active evidence bindings, and a verified recovery record
