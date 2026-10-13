# Proposal

## Why

On 2026-10-13, a Developer ID-signed candidate built from accepted `dev`
`8a8f3479` could not read any of the operator's retained macOS Keychain Tokens
(OSStatus -25293), while installed 0.3.5 read them without interaction. Since
`6f8e25cf`, the bounded credential worker reads Keychain items in-process through
Security.framework with authorization UI disabled. Releases up to 0.3.5 wrote and
read the same items through go-keyring's `/usr/bin/security`, so every retained
item trusts only that tool. A Homebrew upgrade would therefore silently break the
credential reads of Codex, Claude and Hermes until each Token is entered again or
authorized by hand. The operator rejected both as upgrade experience and chose a
one-time migration.

A private-keychain probe of AIGW's own Keychain code on macOS 27.0.1 showed three
things:

- an item created by one Developer ID build is readable without UI by another
  build with different bytes and path from the same team;
- an ad-hoc build is denied;
- reading, deleting and recreating a `/usr/bin/security` item needs no
  interaction.

## What Changes

- On macOS, migrate a retained item once when the worker's in-process read or
  write of its exact slot is denied, the item's keychain is unlocked and the item
  trusts `/usr/bin/security` for decryption. The migration reads the
  Token through `/usr/bin/security` and stages it in an AIGW-owned
  `migration@<slot>` item. It then deletes the retained item through
  `/usr/bin/security`, recreates the exact slot as an AIGW-owned item, verifies
  it and removes the staging item.
- Serialize migrations with an exclusive per-user lock, so concurrent workers
  migrate an item once and never call `/usr/bin/security` on an item that no
  longer trusts it.
- Complete an interrupted migration from its staging item on the next read of
  that slot. A write or deletion of the slot also removes its stale staging
  item.
- Migrate a denied retained item before replacing or deleting it, so rotation
  and removal keep working after an upgrade.
- Before `aigw update --rollback` activates the retained predecessor, convert
  each configured Account's items back to `/usr/bin/security` ownership, so the
  predecessor reads them without interaction. Abort the rollback if any
  conversion fails.
- Amend DR-0011, whose rule that a retained-item refusal blocks the cutover this
  migration replaces.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `secret-storage`: adds "Retained macOS Keychain items migrate to AIGW
  ownership". "Native credential qualification owns real store access", "Native
  credential operations are bounded" and "Upgrade recovery preserves credential
  ownership" admit that migration and the rollback conversion, instead of
  requiring `/usr/bin/security` for every version.

## Impact

Product code changes in `internal/secrets/native` (macOS only), in
`internal/secrets` and in the rollback path of `internal/cli/update`, with
private-keychain tests. Configuration, schemas and client projections are
unchanged. Linux Secret Service and Windows Credential Manager authorize the user
or session rather than the executable, so their behavior is unchanged.

Authority boundary: AIGW changes only the exact `AIGW_TOKEN` slots that it reads,
writes, deletes or rolls back, and only while `/usr/bin/security` already has
decrypt access to them. It never edits an access list, prompts or touches another
item.

Reuse stance: the migration reuses the bounded worker, its no-UI Security.framework
operations and go-keyring's storage envelope. `/usr/bin/security` is the same tool
the predecessor used.

Non-goals:

- migrating items that do not trust `/usr/bin/security`;
- Homebrew downgrades below this version, which AIGW cannot intercept, so the
  predecessor's reader then asks macOS for authorization;
- the `terminal-product-convergence` Change's own delta and task 5.4.

Migrating the operator's real items is a separate acceptance step that runs only
after explicit confirmation.
