---
subject: aigw:decision:0011
role: decision
state: canonical
decision_id: DR-0011
decision_status: accepted
relations:
  canonical_for: Select One Portable Token Backend
---

# DR-0011: Select One Portable Token Backend

- Date: 2026-08-23
- Last amended: 2026-10-13

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

## Alternatives Rejected

- **Searching both a keyring and a file store:** rejected because Token
  authority would be ambiguous.
- **Independent host-local credential helper:** rejected and rolled back,
  because it adds another reader and caller boundary.
- **Silent backend migration:** rejected because it changes credential
  authority without the operator's decision.
- **Package-manager path for credential commands:** rejected because the
  installer owns its public link and may withdraw it before the successor is
  installed. AIGW cannot promise that cached callers retain that path.
- **Fixed private copy:** rejected for upgrades. It preserves old bytes, but
  replacement changes the pathname's identity. Go does not promise atomic
  `os.Rename` on Windows; open-handle sharing may also reject replacement.
- **Stable indirection:** rejected as a portable default. A Unix symlink can
  switch targets, but unprivileged Windows symlink creation requires Developer
  Mode. A forwarding executable adds another launcher contract.

## Consequences and Boundary

The private AIGW worker preserves go-keyring `v0.2.8`'s service, logical
Account slots and storage envelope. On macOS, the same AIGW executable now owns native
read, write and delete through Security.framework. An isolated denied
legacy-Keychain fixture showed that `LAContext` plus the per-query UI-fail
flag still invoked SecurityAgent. Its short-lived, single-operation worker
therefore disallows optional Keychain UI for each value operation, then
restores the process setting. macOS release binaries require cgo on both CPU
targets.

Writes carry the logical Token through standard input, never argv or the
environment; the native writer applies the same base64 envelope exactly once.
Native metadata observation requests attributes only, under the same no-UI
policy, without asking for password data.

An item created by the old
`/usr/bin/security` writer denies the new AIGW identity. On 2026-10-13 a
Developer ID-signed successor was denied every retained operator item with
OSStatus `-25293`, and the operator rejected re-entering or hand-authorizing
each Token as upgrade experience.

Such an item is therefore migrated once:
when its keychain is unlocked and its access list trusts `/usr/bin/security`,
the worker reads it through that tool and stages the Token in an AIGW-owned
`migration@<slot>` item. It then deletes the retained item through the tool,
recreates the exact slot as an AIGW-owned item and removes the staging item.
Every intermediate state keeps the Token. A per-user lock serializes
migrations, and an interrupted one resumes from its staging item. Items that
trust no application, or another one, keep their denial; the worker never
changes an ACL, prompts or selects another backend.

Only the installed program migrates: an executable that Homebrew's receipt
owns, or a portable one beside the predecessor AIGW retained for its rollback.
Any other AIGW executable cannot tell whether its installation still runs the
predecessor: the one-time Homebrew cutover stages and verifies the successor's
reader, and preprojects clients through the extracted successor, while 0.3.5
still owns the public link; a rollback leaves cached successor commands in
client settings; and a development build belongs to no installation. Such a
program therefore reads a retained item through `/usr/bin/security`, or the
staged Token of an interrupted migration, and never migrates, overwrites or
converts it. The worker learns its role from its own executable's installation
evidence, not from a flag or marker its caller could omit.

Native Keychain failures retain a typed numeric OSStatus through the bounded
worker's stderr and the credential command's safe evidence. Standard output
remains Token-only on success and empty on failure. The worker sends no item
name, path, raw backend text or secret data; its parent rejects extra diagnostic
fields. This exposes the actual native failure without guessing that the store
is locked, retrying a denied read or changing authorization.

The successor uses the predecessor's exact physical Account name, including
`diagnostic@<account>` for the separate optional provider-diagnostic credential.
Changing the reader implementation must not change storage identity or require
Token enrollment again. A migrated item's access list names the AIGW signing
identifier and team rather than one executable's bytes, so later signed
releases read it without migration, while ad-hoc builds are denied.

A portable
rollback first admits the retained program and current configuration; if that
program predates AIGW-owned items, AIGW then converts each configured Account's
items back to `/usr/bin/security` ownership with the same envelope before
activating it, and takes them back if activation fails. On a Homebrew
installation, the update command's rollback mode performs only that conversion
and leaves the program files to Homebrew. A Homebrew downgrade without it
leaves the older reader asking macOS for authorization.

Earlier unreleased candidates created
`native@` shadow items.
Preserve an exact item until its retained consumer and rollback owner release
it; then retire it through its existing owner, never by prefix-based deletion.

The independent host-local helper cutover was rejected and rolled back. It is
not the product's credential reader or Token backend. This decision does not
forbid an AIGW-owned copy of the same executable to keep the existing
credential command reachable during package-manager replacement; that
delivery path remains subject to
[installed-transition acceptance](../governance/change-and-release-policy.md#native-platform-evidence).

A successor must execute every retained original credential command before
client projection refresh, then prove update, rollback and re-upgrade against
the same item. Source-only worker tests do not admit deployment.

**Release identity and credential authorization.** Release construction owns
distribution signing; the credential store owns authorization to each retained
item. Signing-key custody, recovery and rotation are not an AIGW Account or
Token backend. No private signing material belongs in the repository or client
configuration. Installing or using published AIGW requires neither developer
membership nor the publisher's private key. An individual may also be the
publisher; that is a separate role, not a prerequisite imposed on every
workstation user.

Distribution and credential authorization retain separate acceptance
boundaries:

| Boundary                     | Required outcome                                                                                                                                                   |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Reproducible construction    | Verify deterministic archives, checksums, provenance, and ad-hoc native signatures where required.                                                                 |
| Public macOS distribution    | Verify Developer ID signing and accepted Apple notarization before publishing final bytes.                                                                         |
| Routine credential access    | Use the selected backend and exact slots without fallback or access-control modification; only the macOS retained-item custody rules below change an item's owner. |
| Installed release transition | Prove retained-item access through update, rollback, and re-upgrade using original client commands.                                                                |

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

Pre-archive acceptance uses isolated synthetic native items or explicit
environment credentials. An untagged Developer ID candidate may prove stable
designated requirements across signed versions; an ad-hoc cdhash is a different
identity. Neither identity equality nor signing migrates a legacy physical item.
After archive and final signing, the exact copied production reader must prove
authorized access to each selected operator Account and diagnostic slot before
any host projection or package link changes. Retain original commands and items;
denial blocks that cutover, not unrelated source or platform acceptance.

**Product reader and migration boundary.** The product reader is AIGW's
existing credential command and one selected backend. On macOS, its bounded
credential subprocess uses the existing logical Account grammar and envelope
with noninteractive Security.framework operations at its own physical Keychain
slot. The proposed user-private executable copy changes the command's installed
location without adding a reader or Token backend; preservation of native item
authorization still requires the retained-item journey. No independent helper
reader, host script, service, or second Token backend is admitted.

| Path                                          | Disposition                      | Reason                                                                                         |
| --------------------------------------------- | -------------------------------- | ---------------------------------------------------------------------------------------------- |
| Bounded AIGW credential worker                | Selected                         | Keeps one command, item grammar and backend; native macOS reads fail without authorization UI. |
| Independent host-local credential helper      | Rejected                         | Adds another reader and caller boundary.                                                       |
| Content-addressed AIGW executable copies      | Selected design; cutover pending | Keeps each projected command's executable bytes available across package replacement.          |
| Security.framework same-executable operations | Selected for macOS               | Keeps new writes readable by AIGW and suppresses legacy-item prompts.                          |
| One-time retained-item migration              | Selected for macOS               | Upgrades from `/usr/bin/security` items need neither Token entry nor manual authorization.     |
| Silent backend migration                      | Rejected                         | Changes credential authority without the operator's decision.                                  |

The existing optional `credential_command` configuration is an explicit
external-integration contract, not permission to install an independent helper
or automatic Keychain recovery path. Its presence does not establish an
approved deployment consumer. Product defaults continue to use AIGW itself.

**Credential command continuity.** Releases up to 0.3.5 project the CLI
executable itself; an unreleased early form of this Change created one
fixed-path AIGW copy. The current Change derives a SHA-256-addressed path from
executable bytes, but that source behavior is not yet an installed-product
guarantee. Installed clients of 0.3.5 and earlier still invoke Homebrew's
public binary link, whose lifetime is owned by Homebrew, not AIGW.

Direct immutable versioned AIGW paths are selected for implementation. Prepare
and verify successor bytes before projection. Old commands keep their old
bytes; rollback reselects them without replacing an in-use file. Native
acceptance remains mandatory.

The selected path is another installation of the same AIGW credential
reader, not an independent helper or Token store. It lives in an owner-only
AIGW data namespace resolved on the destination host. Preparation is
create-if-absent and byte-verified; projection switches only after the new
command passes its selected backend. Old copies are not removed merely because
the current configuration points elsewhere: explicit, cached, rollback and
unknown consumers must be accounted for before exact deletion.

Windows file creation and shell execution have different path limits. A deep
canonical reader can exist while `cmd.exe` cannot invoke it. Shell projection
uses only an OS-provided short ancestor name, checked against the same native
file identity. Parsing preserves that exact command spelling; the retained
reader owner checks native namespace identity without expanding paths. It neither
creates aliases nor changes short-name policy. The chosen spelling must remain
stable through reader preparation; no usable native name means no projection
commit. An extended path prefix alone does not qualify a shell invocation.

Explicit portable uninstall withdraws managed projections and removes the
selected installation executable and its single rollback copy. It retains
every versioned reader and receipt byte-for-byte: the content-addressed path
may also serve another installation, and a cached caller may still invoke it.
Keeping the executable does not authorize a withdrawn Client Binding. Absent
readers are not created. Exact reader retirement requires separate
evidence that no cached, rollback, explicit, or other-installation consumer
remains; uninstall alone cannot establish that fact.

This choice does not retroactively protect cached commands of 0.3.5 and earlier that name the
Homebrew link. Reprojecting client files cannot prove that an existing session
reloads its command. For that one-time bridge, prepare and verify the private
successor, project managed clients before package replacement, and prefetch the
verified Homebrew artifact. Upgrade in a bounded quiet window, measure any
public-link gap, then exercise the captured old commands and real clients
immediately; a failed candidate restores the working predecessor. A cached
public-link call during the gap can still fail once. Report that residual risk
and measured interval rather than claiming zero interruption or seizing
Homebrew's link.

Later versioned-path upgrades must keep old commands callable
throughout. Native macOS Keychain, Linux backend and Windows credential tests,
including retained state and interruption, remain release gates.

The platform constraints above follow [Go's `os.Rename` contract](https://pkg.go.dev/os#Rename),
[Windows symbolic-link requirements](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-createsymboliclinkw),
and [Windows file-sharing rules](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfile3).

Before a successor can replace a working installation, the existing release
journeys must establish all of these boundaries:

1. Identify exact predecessor and candidate artifacts, selected backend and
   retained item. Matching paths or service names alone do not prove access.
2. Capture each client's original executable, arguments and environment before
   replacement. Execute those snapshots before sync or client refresh after
   update, rollback and re-upgrade. Capturing a later command must not overwrite
   an earlier snapshot.
3. Before switching a client, prove captured predecessor and signed-successor
   access to the same selected Token and each configured diagnostic credential.
   Update, rollback and re-upgrade preserve that identity. A predecessor Token
   rotation after rollback must be immediately visible to the successor without
   restaging. Environment-backed fixtures and new-client runs do not qualify a
   Keychain transition or recovery of every existing session.
4. Verify bounded failure, interrupted replacement, exact rollback and uninstall
   preservation. Test the native no-UI policy separately; process timeout alone
   cannot suppress an operating-system prompt.

These are source and release acceptance requirements, not authorization to read
operator credentials, enroll items, alter ACLs, install software or restart
clients. The working predecessor remains installed until the exact transition
has evidence and the operator admits the cutover. A denied candidate blocks
that transition, not unrelated source, documentation or platform work.

One provider Token is sufficient on a supported workstation even when no
native credential service is available. Existing keyring Tokens remain in
their selected backend; there is no dual-read compatibility period. Operators
must make any intentional backend change explicitly.

## Evidence and Revisit

Revisit the native reader when an operating-system contract changes or exact
native evidence contradicts its security or lifecycle assumptions. A replacement
requires an explicit product decision, preserves one backend authority, and
removes the superseded implementation. This trigger does not reactivate the
rejected host-local helper.
