# Design

## Context

See [the proposal](proposal.md). On macOS the bounded credential worker
(`internal/secrets/native`) runs each operation in a private subprocess of the same
AIGW executable, with a five-second deadline. Since `6f8e25cf` it reads, writes and
deletes Keychain items in-process through Security.framework with Keychain UI
disabled.

Releases up to 0.3.5 created the same items through go-keyring, which runs
`add-generic-password` inside `/usr/bin/security -i`. Their decrypt access list
therefore trusts only that tool. The access list names a trusted application by
its designated requirement. In the probe of 2026-10-13 on macOS 27.0.1, an item
created in-process by a Developer ID-signed build was bound to `identifier aigw`
and team 893KZJ9HDW, not to that build's bytes or path.

## Goals / Non-Goals

**Goals:**

- After an upgrade from a `/usr/bin/security` release, every client keeps reading
  its Token without entering it again, authorization UI or access-list changes.
- Later upgrades signed by the same team need nothing more.
- A portable rollback still works without interaction.
- An interruption never loses a Token.

**Non-Goals:** items that do not trust `/usr/bin/security`; Homebrew downgrades;
Linux and Windows stores; the `terminal-product-convergence` Change's records.

## Decisions

1. **Trigger and eligibility.** A migration starts in only two cases: an
   in-process read or write of the exact slot is denied, or a read finds the slot
   absent while its staging item exists. In both cases the item's keychain must
   be unlocked. A denial also requires the item's decrypt access list to name
   `/usr/bin/security`. Both checks read item metadata under the no-UI policy. A
   locked keychain, or an item that trusts no application or another one, keeps
   today's denial, so `/usr/bin/security` never runs where it could prompt. A
   read that is not eligible repeats its lookup once, because a concurrent
   worker may have just completed the migration.
2. **Staged migration.** The worker takes the migration lock and re-reads the
   slot, because another worker may have finished. Then it:
   1. reads the Token with `/usr/bin/security find-generic-password -w`;
   2. writes it to the AIGW-owned staging slot `migration@<slot>` and verifies
      the staged value;
   3. deletes the retained item with `/usr/bin/security delete-generic-password`;
   4. recreates the exact slot in-process and verifies it;
   5. deletes the staging slot.

   At every point the retained item, the staging item or both hold the Token, and
   the Token never enters argv.

3. **Recovery.** A read that finds the exact slot absent checks for its staging
   slot. If one exists, the read recreates the exact slot from the staged Token
   under the lock. A crash before the retained item is deleted simply repeats the
   migration on the next operation. A staging item left after a successful
   migration is removed by the next write or deletion of the slot, or by a
   rollback conversion. Reads that succeed do not look for it, so the cost of the
   common read path does not change.
4. **Serialization.** An exclusive `flock` on a per-user lock file serializes
   migrations. The file lives in `confstr(_CS_DARWIN_USER_TEMP_DIR)` for the user
   search list, and beside the keychain for an explicit test keychain. Waiting is
   bounded inside the worker deadline, and on timeout the worker returns the
   original denial. The kernel releases the lock when a worker dies.
5. **Writes and deletions.** A write denied on an eligible retained item first
   migrates it and then replaces the owned item, so an interrupted rotation
   cannot lose the previous Token. A deletion denied on an eligible retained item
   deletes it through `/usr/bin/security`. Both also remove a stale staging slot.
6. **Rollback conversion.** Before `aigw update --rollback` activates the
   retained program, AIGW converts the API Token and provider-diagnostic slots of
   each configured Account:
   1. stage the owned Token;
   2. delete the owned item;
   3. write the Token through `/usr/bin/security -i add-generic-password` with
      go-keyring's envelope;
   4. verify it through `/usr/bin/security`;
   5. remove the staging slot.

   The conversion runs for every rollback, because a successor migrates again on
   first use. If it fails, the active program stays in place.

7. **Other platforms.** Outside macOS the conversion is a no-op, because those
   stores authorize the user or session. No Linux or Windows code path changes.

## Risks / Trade-offs

- [Only a converted item is readable by the predecessor's tool] → Homebrew
  downgrades bypass AIGW, so the predecessor then asks macOS for authorization.
  This is a documented non-goal.
- [Ad-hoc local builds and Developer ID builds cannot read each other's items] →
  Source tests use private keychains, and operators install signed releases.
- [A team or identifier change would need another migration] → This design
  records it as a release-identity constraint.
- [A staging item can remain after a crash] → It is AIGW-owned like the slot, and
  the next write, deletion or rollback conversion removes it.
- [An item from a non-default keychain is recreated in the default keychain] →
  Both keychains stay in the user search list, and the slot identity does not
  change.

## Migration Plan

No configuration or schema change. The first credential operation after an
upgrade migrates each retained slot, and a portable rollback converts them back.
Proof, candidate landing, archive, closeout and publication are governed effects
recorded in ETHOS Attestations.

## Local Qualification

The failing evidence is the operator journey of 2026-10-13. A Developer
ID-signed candidate built from `8a8f3479` was denied every retained item
(OSStatus -25293), while installed 0.3.5 read them. The new private-keychain
regressions first failed for the expected reasons: the predecessor item was
denied, its replacement was refused, and a staging item was left behind. They
now pass with the worker's own executable, with no prompt, and with the search
list unchanged. They cover:

- migration on first read;
- six concurrent first reads;
- resumption from each interrupted state;
- locked, foreign and undecodable items that keep their denial;
- a held migration lock;
- rotation and deletion of retained items;
- rollback conversion from migrated, interrupted, retained and absent slots,
  with refusal of foreign and oversized ones.

Vet and lint pass for the darwin, linux and windows targets, and vet also
passes for darwin without cgo. `mise run native` passed on macOS with 95.18%
aggregate statement coverage (15,268 of 16,042 statements). `mise run check`
passed on the staged tree. The operator's real items are migrated only in the
separate confirmed acceptance step.
