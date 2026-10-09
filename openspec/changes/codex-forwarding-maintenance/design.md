## Context

This is an operator-authorized maintenance release, not a shortcut to 0.4.0.
Its immutable source baseline is published AIGW 0.3.1. The original user
credential reader parses schema six with unknown-field rejection and derives
its identity from the selected Account upstream. Transport selection must not
change either contract.

## Decisions

Configuration owns one explicit Codex forwarding destination and its upstream
binding. Accounts remain authoritative for provider URLs and credential
identity. An additional owner-controlled destination component is justified
only because the original immutable reader cannot accept a new main-TOML field.
Store owns its parsing, validation, snapshots, backups and complete
postimage-checked compensation. Do not introduce another credential reader,
launcher, backend, daemon or independent transaction controller.

Selection prepares one proposed Client Binding for both credential-free
preview and guarded commit. Codex projects that effective destination while
retaining its original provider and auth command. The existing adapter,
transaction, checkpoint and native acceptance owners remain authoritative.
No generic client-registry or protocol-selection redesign is included.

## Migration Plan

Preserve the published predecessor and its original Homebrew stable entry.
Complete implementation and pre-publication candidate acceptance before
archive. After archive, qualify the exact final signed source and notarized
bytes, publish those objects and assets to selected peers, and update the
original Homebrew cask without portable-install duplication.

Before the production change, compare the exact managed configuration,
destination component, backups, checkpoint and Codex target preimages. Apply
only the selected Codex destination through the installed public command.
Prove forwarding, direct rollback and restored forwarding with the actual
client; preserve enabled Hermes and all user models, effort and sessions.
Retain the predecessor until no active consumer needs it. Retire the exact
maintenance Change, proposal and lane only after published-byte and installed
acceptance; do not retire the independent unfinished 0.4.0 work.

## Goals / Non-Goals

The destination component is the minimum addition needed for the immutable
predecessor reader. No service management, credential backend migration,
registry redesign, or independent helper is included.

## Risks / Trade-offs

- Multiple owned files are not globally atomic: preserve exact preimages,
  compare every postimage, and compensate each independent file.
- A cached predecessor cannot interpret forwarding checkpoint fields:
  restore direct mode and complete native verification before predecessor
  rollback; qualify its actual configuration and recovery readers.
- A historical maintenance base differs from current development:
  qualify and publish its exact source independently without rebasing to 0.4.0.
- Release chronology requires tags reachable from the maintenance source and
  preserves existing historical sections. All local tags still constrain version
  allocation and pending-version precedence without requiring unmerged changes.

## Implementation Ownership

Reuse `internal/configuration` for binding, resolution, Store snapshots and
checkpoint semantics; `internal/cli/selection` and `internal/synchronization`
for preview and guarded selection. Existing Codex rendering, credential,
transaction, release, and native acceptance owners remain unchanged.
