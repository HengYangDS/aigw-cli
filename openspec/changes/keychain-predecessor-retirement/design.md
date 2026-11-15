## Context

`internal/secret/native/migration_darwin.go` wraps every macOS read, write and delete. A denied operation on an
item whose access list trusts `/usr/bin/security` is answered by reading the Token through that tool, staging it
in a `migration@<slot>` item, deleting the old item through the tool and recreating the slot. Only the installed
program does this; any other copy reads through the tool and changes nothing, so each worker asks its own
installation which it is. `aigw self rollback` converts every configured slot back for a program older than
0.4.0, and on a Homebrew installation that conversion is all it does.

A Keychain item that a signed AIGW release creates trusts the signing identifier and team, not the file. A later
signed release reads it without a prompt. Measured on 2026-10-13, and again on 2026-11-15 for an item that this
Change's worker creates: of three probes signed with the release identity, two built from this worker and one from
the 0.7.2 worker, each reads the item and the new ones replace it, an ad-hoc build is refused, and the item's
access list names the identifier and the team.

## Goals / Non-Goals

**Goals:**

- The macOS worker uses one authority, the Security framework with user interaction off.
- A person whose item AIGW may not use learns the steps that replace it, from the first command that fails.
- No command of the product changes the owner of a credential item.

**Non-Goals:**

- A way back to 0.3.5 or earlier. No release older than 0.7.0 reads `aigw.toml`.
- Changing the storage envelope, the service or the account names, or the item labels.
- Changing how Linux and Windows store credentials.

## Decisions

### The worker reads, writes and deletes directly

`readCredential`, `writeCredential` and `deleteCredential` call the Keychain functions that they wrapped. The
migration file, the access-list and unlock probes, the staging label, the lock and the restore and reclaim
commands go, on every platform. The envelope stays: it is the storage format, not a predecessor's.

Alternative: keep the one-way migration and drop only the conversion back. It keeps the tool's one-second bound,
the role, the lock and half of the fragile tests for people who skip from 0.3.5 to this release.

### An item of another program is refused with its fix

Measured in private keychains on macOS 27.2.1: macOS refuses AIGW the read and the replacement of an item that
another program stored with OSStatus -25293, the status that a locked keychain gives for every item, and it lets
AIGW delete either. The status alone does not tell the two apart. The worker therefore reads the lock state of the
keychain that holds the item, metadata that needs no prompt, before the operation and again after such a refusal.
Its typed diagnostic says that the keychain was unlocked only when both readings say so. A refusal with an
unlocked keychain is an item that macOS does not let this program use: another program stored it, or its access
list no longer admits AIGW.

Two cases would otherwise read as another program's item, whose fix deletes it:

- **A locked keychain behind an unlocked default one.** A lookup searches every keychain of the search list, so
  the reading asks the keychain that holds the item, and the default keychain only for an absent slot, which a
  creation writes to. Measured with two private keychains.
- **A keychain that someone unlocks in the instant after the refusal.** The reading before the operation says
  locked, so the failure stays a locked keychain's. The worker does not run the operation again: an authorization
  failure ends it.

The store turns that refusal into one typed failure, which owns the title, the evidence and the fix: have the
credential at hand, delete the item with `security delete-generic-password -s AIGW_TOKEN -a <slot>`, then store
the credential again with its command, `aigw provider rotate <provider>` for a Token and `aigw provider edit
<provider> --balance-token-stdin --balance-user-id <user-id>` for balance credentials. The deletion discards the
only stored copy, so the credential comes first.

A command names itself as the last step where the credential's own command would not work. `aigw provider add`
and `aigw setup` always do: they store the Token themselves, and `aigw provider rotate` does not know a provider
before one of them adds it. `aigw config rollback` does for a provider that only the configuration it restores
holds: without the item the rollback restores that provider and leaves its client for the Token. Acceptance tests
follow each printed fix: the fixture removes the item that the printed deletion names, and the test runs the
printed last step word for word.

Every command that reads or stores the credential shows that title and fix. `aigw check`, the credential command,
`provider balance` and `model search` present a store failure in their own rows and take both from the typed
failure. Every other command returns the refusal, and the general presentation of a store failure renders it and
adds that the deletion takes the item from the program that stored it: `provider add`, `provider rotate`,
`provider edit`, `setup`, `use`, `sync`, `model add` and `config rollback`. Guided setup reads a stored Token
before it asks for a model, so it stops at the refusal. A locked keychain keeps its own fix. `aigw provider show`
reads no Token and reports such an item as stored.

A batch that stored another provider's Token before it met the refusal undoes that Token. If the undo fails, the
store reports it as a failed undo, which every presentation states beside the refusal.

AIGW does not replace such an item itself, although macOS would let it delete the item. A write that failed
after the deletion would lose a Token that the other program may still use, and the operator's own deletion is
the explicit decision. `aigw provider remove` deletes the item as it deletes AIGW's own, which is the operator's
decision too.

### No role

The role existed to keep a reader copy from migrating. Without a migration every copy does the same thing, so
`Role`, `program.InstalledProgram` and the worker's role callback go.

### No lock; a mutation looks its slot up again

The lock serialized migrations. AIGW's own commands change a credential under the configuration lock, so two of
them that share a configuration do not write one slot at once. Another program can, and so can two AIGW homes
over one keychain or a caller that drives the store directly. Measured in private keychains on macOS 27.2.1:

- Replacing an item's value replaces its record. A reference that another process looked up before is stale
  afterwards: an operation through it answers -25300, or -67701 once the slot was deleted and created again.
- Of 450 attempts by three processes that replace one slot at once, 205 succeed; the others answer -25299, -25300
  or -67701 and nothing else. In every round one writer at least succeeds (200 rounds of three writers, 150 of
  five).
- A creation in two steps, the item and then its label, loses the label when another writer replaces the value
  in between. It did on a hosted macOS runner on 2026-11-15.

So the worker creates an item with its label in one call, which also makes the label the name that macOS shows
when it asks for access. A write or a delete that meets one of the three statuses releases its reference and
looks the slot up again, with at most five lookups in all. Five is a bound, not a promise for any number of
writers: the suite pins two creators and three replacers of one slot, which leaves two lookups of room, and a
probe with five writers passed 120 rounds. A slot that changes at every lookup ends the operation with the last
status as a failure, and a delete reports an absent item only when its own lookup found none. Tests force each
order through a pause between the lookup and the operation.

A reader that looks at the slot in the instant of another process's replacement can see it missing or invalid: 15
of 5,770 polls in the measurement. That window is the platform's and older than this Change; the worker's read
answers what it saw.

### Rollback moves no credential

`program.Updater.Rollback` loses its custody parameter. On a Homebrew installation `aigw self rollback` is
refused as `aigw self update` is. Its fix is a sentence, because Homebrew has no one command that installs an
earlier release of a Cask. The update document keeps its schema version: no field changes, and the `prepared`
state was a value of the Homebrew branch that no longer answers.

## Risks / Trade-offs

- Someone who upgrades directly from 0.3.5 or earlier, with a provider named as the old Account was, must enter
  the Token again. → The failure names the steps. Whether anyone is in that state is not known.
- A release from 0.4.1 to 0.7.2 took an item over only when its installed program read or replaced it. Balance
  credentials and Tokens that no command read, a portable copy with no retained predecessor beside it, and a
  Homebrew installation after a 0.7.x `aigw self rollback` can still hold items of the system tool. → The same
  failure and fix.
- An interrupted migration of 0.4.1 to 0.7.2 keeps its Token only in a `migration@<slot>` item, which this
  release neither reads, lists nor removes; the slot then reads as holding no Token. → The Token is stored again,
  and the changelog names the command that removes the leftover item.
- A 0.7.2 parent that meets this release's worker during a package replacement, same path and new bytes, refuses
  the diagnostic's new field and reports the plain failure. → Only a refused item in that moment is affected.
- Signed releases must keep the identifier `aigw` and the team. A change of either makes every item foreign.
  → Unchanged by this Change; the release policy already binds the identity.
- An ad-hoc or development build cannot read a signed release's items, and its failure names the deletion.
  → Such a build uses a private home, and the impact says that the deletion takes the item from the program that
  stored it.

## Review findings kept

- The general presentation shows a failed undo in its title and in its impact, as it does for every failed undo.
  The names of the two undos of a Token batch are corrected here; the presentation is not this Change's.
- `program-lifecycle` describes a program that AIGW owns and has no Homebrew qualifier for a rollback preview.
  `release-distribution` owns the refusal of replacement, rollback and removal on a Homebrew installation,
  previews included, and gains the rollback here.
- "No Keychain item SHALL change" on a portable rollback has no Keychain test: `program.Updater.Rollback` holds
  no credential store any more.
- A read and a write look the slot up once before they start, for the lock state: 0.4 ms more for a read and
  1.2 ms more for a replacement, measured in a private keychain.

## Migration Plan

This release takes no item over. A command that meets an item that still trusts only `/usr/bin/security` fails
with the fix above. There is no way back that converts items: a release from 0.7.0 on reads AIGW's own items as
they are.

## Verification after the tag

The system-store journey needs a signed candidate and a published predecessor, so it runs on a hosted disposable
macOS runner after the tag. There it stores an item through the system tool in the runner's default keychain and
requires the refusal from the candidate's store, which no private-keychain test reaches.
