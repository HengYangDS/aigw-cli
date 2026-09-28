# DR-0011: Select One Portable Token Backend

- Status: accepted
- Date: 2026-08-23
- Last amended: 2026-09-28

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
policy, without asking for password data. A private item created by the old
`/usr/bin/security` writer can deny the new AIGW identity. The new writer
refuses to replace an item it cannot first read; it does not change an ACL,
migrate a Token, retry or select another backend. New native items pass
write/read/update/delete in an isolated Keychain, but retained operator-item
authorization and signed-successor acceptance remain unproved.

The legacy `/usr/bin/security` item and the new native item have different
physical Keychain account names. The latter uses `native@` before each
logical slot, including `diagnostic@<account>` for optional provider
diagnostics; service and envelope remain unchanged. This is a
one-time authorization boundary, not a second Token backend or a dual-read
fallback. An operator must supply the selected Token and any configured
provider-diagnostic credential explicitly to the candidate before claiming
those capabilities survive cutover. The predecessor keeps its old items and command;
the candidate reads only its native item. Missing or denied native access
blocks the switch, while rollback leaves the old item intact. Neither binary
copies a Token from the other item's address.

The independent host-local helper cutover was rejected and rolled back. It is
not the product's credential reader or Token backend. This decision does not
forbid an AIGW-owned copy of the same executable to keep the existing
`credential` command reachable during package-manager replacement; that
delivery path remains subject to
[post-archive acceptance](../../openspec/changes/archive/2026-09-25-inference-readiness-claude-override/design.md#post-archive-delivery-acceptance).

A successor must execute every retained original credential command before
client projection refresh, then prove update, rollback and re-upgrade against
the same item. Source-only worker tests do not admit deployment.

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

The product reader is the existing `aigw credential` command and one selected
backend. On macOS, its bounded credential subprocess uses the existing
logical Account grammar and envelope with noninteractive Security.framework
operations at its own physical Keychain slot.
The proposed user-private executable copy changes the command's installed
location without adding a reader or Token
backend; preservation of native item authorization still requires the retained-
item journey. No independent helper reader, host script, service, or second
Token backend is admitted.

| Path                                          | Disposition                      | Reason                                                                                                       |
| --------------------------------------------- | -------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| Bounded AIGW credential worker                | Selected                         | Keeps one command, item grammar and backend; native macOS reads fail without authorization UI.               |
| Independent host-local credential helper      | Rejected                         | Adds another reader and caller boundary.                                                                     |
| Content-addressed AIGW executable copies      | Selected design; cutover pending | Keeps each projected command's executable bytes available across package replacement.                        |
| Security.framework same-executable operations | Selected for macOS               | Keeps new writes readable by AIGW and suppresses legacy-item prompts; retained-item access remains unproved. |
| Silent backend migration                      | Rejected                         | Changes credential authority without the operator's decision.                                                |

The existing optional `credential_command` configuration is an explicit
external-integration contract, not permission to install an independent helper
or automatic Keychain recovery path. Its presence does not establish an
approved deployment consumer. Product defaults continue to use AIGW itself.

### Credential command continuity

The 0.3.1 predecessor and original source created one fixed-path AIGW copy.
The current Change derives a SHA-256-addressed path from executable bytes, but
that source behavior is not yet an installed-product guarantee. Installed
0.3.1 clients still invoke Homebrew's public binary link, whose lifetime is
owned by Homebrew, not AIGW.

- **Package-manager path: rejected.** The installer owns its public link and
  may withdraw it before the successor is installed. AIGW cannot promise that
  cached callers retain that path.
- **Fixed private copy: rejected for upgrades.** It preserves old bytes, but
  replacement changes the pathname's identity. Go does not promise atomic
  `os.Rename` on Windows; open-handle sharing may also reject replacement.
- **Stable indirection: rejected as a portable default.** A Unix symlink can
  switch targets, but unprivileged Windows symlink creation requires Developer
  Mode. A forwarding executable adds another launcher contract.
- **Direct immutable versioned AIGW paths: selected for implementation.**
  Prepare and verify successor bytes before projection. Old commands keep their
  old bytes; rollback reselects them without replacing an in-use file. Native
  acceptance remains mandatory.

The selected path is another installation of the same `aigw credential`
reader, not an independent helper or Token store. It lives in an owner-only
AIGW data namespace resolved on the destination host. Preparation is
create-if-absent and byte-verified; projection switches only after the new
command passes its selected backend. Old copies are not removed merely because
the current configuration points elsewhere: explicit, cached, rollback and
unknown consumers must be accounted for before exact deletion.

Explicit portable uninstall withdraws managed projections and removes the
selected installation executable and its single rollback copy. It retains
every versioned reader and receipt byte-for-byte: the content-addressed path
may also serve another installation, and a cached caller may still invoke it.
Keeping the executable does not authorize a withdrawn Client Binding. Absent
readers are not created. Exact reader retirement requires separate
evidence that no cached, rollback, explicit, or other-installation consumer
remains; uninstall alone cannot establish that fact.

This choice does not retroactively protect cached 0.3.1 commands that name the
Homebrew link. Reprojecting client files cannot prove that an existing session
reloads its command. For that one-time bridge, prepare and verify the private
successor, project managed clients before package replacement, and prefetch the
verified Homebrew artifact. Upgrade in a bounded quiet window, measure any
public-link gap, then exercise the captured old commands and real clients
immediately; a failed candidate restores the working predecessor. A cached
public-link call during the gap can still fail once. Report that residual risk
and measured interval rather than claiming zero interruption or seizing
Homebrew's link. Later versioned-path upgrades must keep old commands callable
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
3. Before switching a client, stage its selected Token and any configured
   diagnostic credential in their native slots through explicit input. Prove
   captured predecessor access and signed-successor access independently for
   each claimed capability. Environment-backed fixtures and
   new-client runs do not qualify a Keychain transition or establish recovery
   of every existing session. A Token rotated in the legacy item after rollback
   makes earlier candidate staging stale; restage and verify it before
   re-upgrade rather than inferring freshness from a readable old command.
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

## Revisit Trigger

Revisit the native reader when an operating-system contract changes or exact
native evidence contradicts its security or lifecycle assumptions. A replacement
requires an explicit product decision, preserves one backend authority, and
removes the superseded implementation. This trigger does not reactivate the
rejected host-local helper.
