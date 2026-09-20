# DR-0011: Select One Portable Token Backend

- Status: accepted
- Date: 2026-08-23
- Last amended: 2026-09-20

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

The accepted implementation delegates macOS credential operations to
go-keyring `v0.2.8`, whose provider invokes `/usr/bin/security`. A private AIGW
worker preserves that provider and service/slot grammar. The worker adds a
five-second process deadline and bounded cleanup without
introducing a helper executable, a second backend or a credential migration.

Writes carry the logical Token through standard input, never argv or the
environment; go-keyring applies its storage envelope exactly once. Metadata
observation remains value-free. Failure returns no Token, changes no ACL and
does not retry through another backend. The deadline bounds AIGW's worker, but
cannot prove that macOS itself will never present authorization UI.

The host-local external-helper cutover was rejected and rolled back. It is not
the product's credential solution. A successor must execute every retained
original credential command before client projection refresh, then prove update,
rollback and re-upgrade against the same item. Source-only worker tests do not
admit deployment.

### Release identity and credential authorization

Release construction owns distribution signing; the credential store owns
authorization to each retained item. Signing-key custody, recovery and rotation
are not an AIGW Account or Token backend. No private signing material belongs in
the repository or client configuration.
Installing or using published AIGW requires neither developer membership nor the
publisher's private key. An individual may also be the publisher; that is a
separate role, not a prerequisite imposed on every workstation user.

Distribution and credential authorization retain separate acceptance
boundaries:

| Boundary                     | Required outcome                                                                                      |
| ---------------------------- | ----------------------------------------------------------------------------------------------------- |
| Reproducible construction    | Verify deterministic archives, checksums, provenance, and ad-hoc native signatures where required.    |
| Public macOS distribution    | Verify Developer ID signing and accepted Apple notarization before publishing final bytes.            |
| Routine credential access    | Use the selected backend and exact slots without fallback, migration, or access-control modification. |
| Installed release transition | Prove retained-item access through update, rollback, and re-upgrade using original client commands.   |

The release owner applies the signature required by the selected distribution
mode before final inventory and checksums. Reproducible construction, publisher
trust, notarization, and retained-item authorization remain distinct claims. The
[release policy](../governance/change-and-release-policy.md#reproducible-assets)
owns their executable acceptance and current channel requirements.

Following [Apple's subsystem-specific trust model](https://developer.apple.com/library/archive/technotes/tn2206/_index.html),
credential authorization, distribution trust and notarization remain separate
acceptance decisions. A successful signing check does not substitute for the
retained-item journey, and a retained-item journey does not establish public
distribution trust.

### Product reader and migration boundary

The product path is the existing `aigw credential` command and one selected
backend. On macOS, a bounded credential subprocess invokes the same go-keyring
provider used by the published predecessor. No helper binary, host script,
service, or permanently retained predecessor is part of this path.

| Path                                      | Disposition | Reason                                                                                      |
| ----------------------------------------- | ----------- | ------------------------------------------------------------------------------------------- |
| Bounded go-keyring worker                 | Selected    | Preserves the published provider while bounding the AIGW-owned process and transport.       |
| Host-local stable credential helper       | Rejected    | Adds an unapproved runtime and caller boundary.                                             |
| Security.framework same-executable reader | Rejected    | Changes reader identity and requires unnecessary native bridge and authorization machinery. |
| Silent backend migration                  | Rejected    | Changes credential authority without the operator's decision.                               |

The existing optional `credential_command` configuration is an explicit
integration contract, not permission to install a helper or an automatic
Keychain recovery path. Its presence does not establish an approved deployment
consumer. Product defaults continue to use AIGW itself.

Before a successor can replace a working installation, the existing release
journeys must establish all of these boundaries:

1. Identify exact predecessor and candidate artifacts, selected backend and
   retained item. Matching paths or service names alone do not prove access.
2. Capture each client's original executable, arguments and environment before
   replacement. Execute those snapshots before sync or client refresh after
   update, rollback and re-upgrade. Capturing a later command must not overwrite
   an earlier snapshot.
3. Prove retained-item access and real-client inference separately.
   Environment-backed fixtures and new-client runs do not qualify a Keychain
   transition or establish recovery of every existing session.
4. Verify bounded failure, interrupted replacement, exact rollback and uninstall
   preservation. Do not claim that process timeout suppresses operating-system UI.

These are source and release acceptance requirements, not authorization to read
operator credentials, enroll items, alter ACLs, install software or restart
clients. The working predecessor remains installed until the exact transition
has evidence and the operator admits the cutover. A denied candidate blocks
that transition, not unrelated source, documentation or platform work.

One provider Token is sufficient on a supported workstation even when no
native credential service is available. Existing keyring Tokens remain in
their selected backend; there is no dual-read compatibility period. Operators
must make any intentional backend change explicitly.

## Revisit Trigger

Revisit the native reader when an operating-system contract changes or exact
native evidence contradicts its security or lifecycle assumptions. A replacement
requires an explicit product decision, preserves one backend authority, and
removes the superseded implementation. This trigger does not reactivate the
rejected host-local helper.
