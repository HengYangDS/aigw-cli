# DR-0011: Select One Portable Token Backend

- Status: accepted
- Date: 2026-08-23

## Context

AIGW originally assumed that a native credential service was usable on every
supported host. Headless Linux and constrained macOS sessions can lack that
service even though the filesystem provides a sound local security boundary.
Searching both a keyring and a file store would make Token authority ambiguous.

## Decision

Each installation uses one Account Token backend. Windows defaults to
Credential Manager. macOS and Linux prove native-service availability on first
Token access and otherwise select an AIGW-owned file store. The automatic
choice is persisted before a successful Token operation returns. Explicit
`keyring`, `file`, and read-only `env` selections never fall through to another
backend.

The file backend accepts only current-user-owned directories and regular files,
requires modes `0700` and `0600`, rejects symbolic and multiply linked files,
and commits replacements atomically within the owning directory. It neither
reads nor migrates another product's credential state.

## Consequences

One provider Token is sufficient on a supported workstation even when no
native credential service is available. Existing keyring Tokens remain in
their selected backend; there is no dual-read compatibility period. Operators
must make any intentional backend change explicitly.

## Revisit Trigger

Revisit only if a supported operating system offers a stronger native
credential API that is available without user-session assumptions and can
preserve the single-backend authority model.
