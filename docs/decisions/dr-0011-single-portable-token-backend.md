# DR-0011: Select One Portable Token Backend

- Status: accepted
- Date: 2026-08-23
- Last amended: 2026-09-16

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
the credential store owns authorization to each retained item. The publisher's
release operator supplies the approved signing key through protected signing
infrastructure.
Its custody, recovery and rotation policy is not an AIGW Account or Token backend.
No private signing material belongs in the repository or client configuration.
Installing or using published AIGW requires neither developer membership nor the
publisher's private key. An individual may also be the publisher; that is a
separate role, not a prerequisite imposed on every workstation user.

[Apple's Code Signing Tasks](https://developer.apple.com/library/archive/documentation/Security/Conceptual/CodeSigningGuide/Procedures/Procedures.html)
distinguishes signing from certificate-authority trust. Local signing need not
use a CA-issued certificate, but that does not establish public distribution
trust or authorization to a retained Keychain item. The product obligations are:

| Boundary                | Required outcome                                                                                                   |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------ |
| Native signing          | Verify the exact executable and declared code identity.                                                            |
| Public distribution     | The publisher supplies Developer ID signing and notarization under the release policy.                             |
| Initial item enrollment | Obtain exact reader authorization through a separately authorized setup or migration; never silently widen access. |
| Routine access          | Read and mutate only authorized exact slots without prompts, fallback readers or store changes.                    |
| Reader replacement      | Prove retained-item access after update and rollback, independently of a main CLI update.                          |

One-time enrollment is not routine access and is not authorized by this decision.
Existing-item denial remains noninteractive until its owner chooses and validates
the disposition. Requiring enrollment after every routine update would fail the
retained-credential journey rather than satisfy it.

The private native credential regression uses
[rcodesign](https://gregoryszorc.com/docs/apple-codesign/stable/apple_codesign_rcodesign_signing.html)
with a disposable self-signed certificate loaded from files. The repository locks
this tool for release construction on each supported build host; it is never
shipped in AIGW. The current same-executable system-Keychain qualification requires
publisher-supplied signing inputs before fixture construction; missing inputs fail explicitly instead of
creating a disposable identity that cannot prove upgrade continuity. This
preflight checks input presence, not certificate trust or retained-item access.
Those remain obligations of the native release journey.

The private fixture must create a partitioned Keychain, assert database
version `0x200`, and observe the item's partition ACL before testing access.
A byte-identical copy can read the retained item. A changed self-signed image
can satisfy the same certificate-bound designated requirement yet still be
denied by its different code-hash partition. This is an authorization regression,
not evidence that a self-signed identity supports production upgrades.

The fixture lives under its own temporary `Library/Keychains` directory, never
the operator's home. This path has a security meaning: Apple's
[database format selection](https://github.com/apple-oss-distributions/Security/blob/db15acbe6a7f257a859ad9a3bb86097bfe0679d9/OSX/libsecurityd/lib/ssblob.cpp#L58-L86)
creates old-format `0x100` Keychains elsewhere. Those databases skip
[partition authorization](https://github.com/apple-oss-distributions/Security/blob/db15acbe6a7f257a859ad9a3bb86097bfe0679d9/securityd/src/acls.cpp#L147-L184).
The previous plain-temporary-directory fixture therefore proved a weaker
authorization contract than the system Keychain. Its apparent cross-code-hash
success does not establish current platform support.

Apple derives [application partition identity](https://github.com/apple-oss-distributions/Security/blob/db15acbe6a7f257a859ad9a3bb86097bfe0679d9/securityd/src/clientid.cpp#L187-L287)
independently from the designated requirement. Recognized Apple-issued developer
certificate chains use a team partition; other signed code uses `cdhash:`.
A self-signed certificate, fixed identifier, or matching designated requirement
alone does not preserve this partition when the reader's code hash changes.
That establishes the current same-executable reader's failed upgrade path, not
the impossibility of every alternative architecture or a requirement for the
end user to buy membership. With the current reader, qualification must use an
approved Apple-recognized publisher identity and prove the exact retained-item
transition; a certificate alone is not that proof.

The existing GoReleaser build signs macOS binaries before archiving. An encrypted
PKCS#12 identity, password file and compiled designated requirement are explicit
publisher release inputs; missing inputs stop construction without prompting or
creating an identity. Signing time and post-signing file time use the release epoch, not
wall time. Native Linux and Windows acceptance selects only its own operating
system and needs no macOS credential. Full release construction still emits and
verifies every declared target. The native archive test verifies both macOS
architectures and their Hardened Runtime flag with Apple's verifier, then
compares two complete matrices across a wall-clock boundary. It uses a
disposable signing identity, not production trust. The
[release policy](../governance/change-and-release-policy.md#reproducible-assets)
owns Developer ID provisioning and the remaining secure-timestamp and
notarization boundary; these are not credential-store responsibilities.

Following [Apple's subsystem-specific trust model](https://developer.apple.com/library/archive/technotes/tn2206/_index.html),
Keychain identity continuity, distribution trust and notarization remain separate
acceptance decisions. The private self-signed fixture proves refusal, not a
substitute for the production signing authority. A certificate-leaf requirement does not survive key rotation
automatically: rotation needs an explicitly admitted successor requirement and
retained-item transition. Introducing a stable signer also cannot retroactively
authorize items created by an older ad-hoc identity or another program. Their
explicit authorization or re-enrollment must precede deployment; denial remains
noninteractive until that decision is made.

### Reader architecture alternatives

The retained-item outcome is fixed; the same-executable worker is the current
implementation, not an immutable product requirement. Compare these paths before
making signing procurement the only remaining action:

| Path                                          | Benefit                                                                                      | Cost and unproved boundary                                                                                                                                                              |
| --------------------------------------------- | -------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Publisher-signed AIGW with the current worker | One delivered program and existing lifecycle; users install without developer membership.    | Publisher signing/public distribution remains to be qualified; foreign-created items and signer rotation still need exact authorization evidence.                                       |
| Narrowly owned stable credential adapter      | Unchanged reader bytes could decouple ordinary CLI rebuilds from Keychain code-hash changes. | Adds a delivered executable, caller boundary and update lifecycle; changed adapter bytes face the same retained-item problem. No end-to-end acceptance currently proves this candidate. |

Prefer the publisher-signed current worker for the public product: it introduces
no additional executable and reuses the existing lifecycle. The adapter remains
a candidate only if an isolated proof establishes a net reduction in complexity
or enables an explicitly required deployment that this path cannot serve. An
unchanged vulnerable reader is not a valid stability strategy. An adapter would
replace the current native reader, not become a second fallback path or backend.

Before adopting an adapter, require all of the following at its existing owner:

- Define whether the trust boundary is the OS user or an authenticated calling
  application. A fixed service, executable path, parent PID or environment
  marker alone does not authenticate a caller. An application-bound IPC design
  must validate OS-supplied caller identity and the admitted code requirement.
- Keep the exact service/slot contract, bounded stdin/stdout, native interaction
  suppression and process cleanup. Expose no arbitrary Keychain reader, loader
  override, plugin mechanism or plaintext Token log.
- Verify new installation and explicit enrollment, CLI-only update with unchanged
  reader, actual reader-byte update, rollback and interrupted replacement against
  retained items. Verify owned cleanup separately from retained user credentials.
- Admit security updates and signer rotation without permanent old-reader
  retention, silent store switching, expanded ACLs or a second read authority.

None of this authorizes a host experiment, production enrollment, access-policy
change, installation or restart. Native qualification uses the existing explicit
disposable-host admission; source-only work proceeds independently.

The system `security` command has no value-read no-interaction option. A timeout
around it could still permit a password dialog, so it is not the read boundary.
The private native test demonstrates that `security`-created items can deny a
different reader even when metadata is visible. An actual reader-identity
acceptance gate therefore precedes deployment; preserving item bytes alone is
not sufficient compatibility evidence.

One provider Token is sufficient on a supported workstation even when no
native credential service is available. Existing keyring Tokens remain in
their selected backend; there is no dual-read compatibility period. Operators
must make any intentional backend change explicitly.

## Revisit Trigger

Revisit the reader architecture when isolated native evidence proves that a
maintained adapter lowers lifecycle cost while preserving the caller and
retained-item contracts, or when a supported operating system offers a stronger
credential API without additional session assumptions. Any replacement must
preserve the single-backend authority model and remove the superseded reader.
