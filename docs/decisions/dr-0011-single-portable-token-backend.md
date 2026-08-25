# DR-0011: Select One Portable Token Backend

- Status: accepted
- Date: 2026-08-23

## Context

AIGW originally assumed that a native credential service was usable on every
supported host. Headless Linux and constrained macOS sessions can lack that
service even though the filesystem provides a sound local security boundary.
Searching both a keyring and a file store would make Token authority ambiguous.

## Decision

Each installation uses one Account Token backend. Every supported platform
proves native-service availability on first Token access. If unavailable, AIGW
selects one AIGW-owned fallback store: owner-only files on macOS and Linux, or
current-user DPAPI-protected files on Windows. The automatic choice is persisted
before a successful Token operation returns. Explicit `keyring`, `file`, and
read-only `env` selections never fall through to another backend.

The Unix file backend accepts only current-user-owned directories and regular
files, requires modes `0700` and `0600`, rejects symbolic and multiply linked
files, and commits replacements atomically within the owning directory. The
Windows file backend binds encryption to the current Windows user through DPAPI
and uses bounded directory handles and same-directory replacement. Neither
implementation reads or migrates another product's credential state.

## Consequences

One provider Token is sufficient on a supported workstation even when no
native credential service is available. Existing keyring Tokens remain in
their selected backend; there is no dual-read compatibility period. Operators
must make any intentional backend change explicitly.

## Revisit Trigger

Revisit only if a supported operating system offers a stronger native
credential API that is available without user-session assumptions and can
preserve the single-backend authority model.
