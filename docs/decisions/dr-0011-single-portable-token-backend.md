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

macOS metadata, read, write and delete operations share the existing file-based
Keychain's native API in a same-executable worker. `purego` supplies the C-call bridge while
keeping the portable build independent of Cgo. The worker disables interaction,
preserves the existing service, slot, search-list and stored-value grammar, and
returns bytes only after a successful read. Its parent enforces a five-second
deadline with bounded process cleanup. This does not grant access: a locked read
or unauthorized operation fails without changing ACLs, unlocking, migrating or falling
back. The legacy API remains necessary for the existing file-based store; no
Data Protection Keychain migration is implied.

The executable that creates a new item also reads it. Writes transport the
existing base64 storage envelope through bounded stdin, never argv or the
environment. Updating an item preserves its access policy; deleting an absent
item succeeds without recreating it. Native deletion authorization is distinct
from password-read authorization and may permit deletion while the store is
locked. This repair does not establish cross-release code-identity continuity
or authorize previously stored items.

Code identity is a release input, not a filename. The private native regression
reads one retained item from a new process and a byte-identical copy, then
re-signs only that disposable copy with the same identifier and path. The changed
ad-hoc identity is denied without a prompt. Thus keeping the installed path or
setting a fixed signing identifier does not establish upgrade continuity.
[Apple TN3127](https://developer.apple.com/documentation/technotes/tn3127-inside-code-signing-requirements)
explains that an ad-hoc designated requirement identifies one code version.
Release acceptance must prove the actual predecessor and successor identities
can access retained credentials; a detached SSH artifact signature does not
satisfy that native authorization contract. A stable authorized code-signing
identity is necessary for that release path, while existing foreign-created
items require their own explicit authorization disposition. Neither is solved
by widening item access, changing stores silently or retaining an old reader.

### Release identity and credential authorization

Release construction owns the native signing identity and designated requirement;
the credential store owns authorization to each retained item. Operators provide
the approved signing key through their existing protected signing infrastructure.
Its custody, recovery and rotation policy is not an AIGW Account or Token backend.
No private signing material belongs in the repository or client configuration.

An isolated feasibility probe used
[rcodesign](https://gregoryszorc.com/docs/apple-codesign/stable/apple_codesign_rcodesign_signing.html)
with a disposable self-signed certificate loaded from files. Two executables with
different code hashes and the same certificate-bound requirement read one retained
private Keychain item; an unrelated ad-hoc reader was denied. Native signature
verification and execution passed without importing that identity, changing host
trust or widening item access. This qualifies that isolated authorization boundary,
not a production signer, release artifact or supported upgrade path.

Following [Apple's subsystem-specific trust model](https://developer.apple.com/library/archive/technotes/tn2206/_index.html),
Keychain identity continuity, distribution trust and notarization remain separate
acceptance decisions. Developer ID is not a prerequisite for the demonstrated
private-item test. A certificate-leaf requirement does not survive key rotation
automatically: rotation needs an explicitly admitted successor requirement and
retained-item transition. Introducing a stable signer also cannot retroactively
authorize items created by an older ad-hoc identity or another program. Their
explicit authorization or re-enrollment must precede deployment; denial remains
noninteractive until that decision is made.

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
