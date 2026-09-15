# DR-0011: Select One Portable Token Backend

- Status: accepted
- Date: 2026-08-23
- Last amended: 2026-09-14

## Context

AIGW originally assumed that a native credential service was usable on every
supported host. Headless Linux and constrained macOS sessions can lack that
service even though the filesystem provides a sound local security boundary.
Searching both a keyring and a file store would make Token authority ambiguous.

## Decision

Each installation selects one Account Token backend. Automatic resolution
honors an existing choice; only an unrecorded choice probes native-service
metadata and selects a fallback if that probe fails. The probe neither reads
Tokens nor proves future read/write permission or freedom from interaction.
The fallback is owner-only files on macOS and Linux, or current-user
DPAPI-protected files on Windows. The automatic choice is persisted
before the first credential mutation changes Token state. Read-only commands and
credential reads may use the resolved backend for that invocation but do not
persist a previously unrecorded choice. Explicit `keyring`, `file`, and read-only
`env` selections never fall through to another backend.

The Unix file backend accepts only current-user-owned directories and regular
files, requires modes `0700` and `0600`, rejects symbolic and multiply linked
files, and commits replacements atomically within the owning directory. The
Windows file backend binds encryption to the current Windows user through DPAPI
and uses bounded directory handles and same-directory replacement. Neither
implementation reads or migrates another product's credential state.

## Consequences

macOS metadata queries and value reads use the existing file-based Keychain's native API in a
same-executable worker. `purego` supplies the supported C-call bridge while
keeping the portable build independent of Cgo. The worker disables interaction,
preserves the existing service, slot, search-list and stored-value grammar, and
returns bytes only after a successful read. Its parent enforces a five-second
deadline with bounded process cleanup. This does not grant access: a locked or
unauthorized item fails without changing ACLs, unlocking, migrating or falling
back. The legacy API remains necessary for the existing file-based store; no
Data Protection Keychain migration is implied.

The system `security` command has no value-read no-interaction option. A timeout
around it could still permit a password dialog, so it is not the read boundary.
A separate installed helper or a new credential framework would introduce
another deployment owner without removing the authorization requirement.
The private native test demonstrates that `security`-created items can deny a
different reader even when metadata is visible. An actual reader-identity
acceptance gate therefore precedes deployment; preserving item bytes alone is
not sufficient compatibility evidence.

One provider Token is sufficient on a supported workstation even when no
native credential service is available. Existing keyring Tokens remain in
their selected backend; there is no dual-read compatibility period. Operators
must make any intentional backend change explicitly.

## Revisit Trigger

Revisit only if a supported operating system offers a stronger native
credential API that is available without user-session assumptions and can
preserve the single-backend authority model.
