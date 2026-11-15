## Context

`internal/secret/native/migration_darwin.go` wraps every macOS read, write and delete. A denied operation on an
item whose access list trusts `/usr/bin/security` is answered by reading the Token through that tool, staging it
in a `migration@<slot>` item, deleting the old item through the tool and recreating the slot. Only the installed
program does this; any other copy reads through the tool and changes nothing, so each worker asks its own
installation which it is. `aigw self rollback` converts every configured slot back for a program older than
0.4.0, and on a Homebrew installation that conversion is all it does.

A Keychain item that a signed AIGW release creates trusts the signing identifier and team, not the file. A later
signed release reads it without a prompt. Measured on 2026-10-13, and again on 2026-11-15 for an item that this
Change's worker creates, with three probes signed with the release identity, two built from this worker and one
from the 0.7.2 worker: one creates the item, the second reads and replaces it, the first reads that value, and
the 0.7.2 worker reads it too. In the upgrade's direction, an item that the 0.7.2 worker creates is read and
replaced by this worker, and the 0.7.2 worker reads the replaced value, as after a program rollback. An ad-hoc
build is refused, and the item's access list names the identifier and the team.

Each statement about macOS below is pinned by a private-keychain test of `internal/secret/native`, or comes from
a probe of 2026-11-15, a C program run against private keychains on macOS 27.2.1. The probe gives the counts and
the two behaviors that no test reaches through the worker. `docs/research/macos-keychain-behavior.md` records
every measurement with its method, its counts and its limits, and the alternative that was measured and not
taken, an update by query.

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

macOS refuses AIGW the read of an item that another program stored with OSStatus -25293, the status that a
locked keychain gives for every item, and it lets AIGW delete either. It does not refuse a new value: the probe
replaced such an item's value through its reference, and the read stayed refused. So the worker reads an item
before it replaces it and replaces nothing that it cannot read, which is where the refusal of a replacement
comes from; a test shows the item, its value and its access list unchanged after one.

The status alone does not tell another program's item from a locked keychain. The worker therefore reads the
lock state of the keychain that holds the item, metadata that needs no prompt, before the operation and again
after such a refusal.
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

The credential's own command knows only the providers of the current configuration. So for a provider that the
configuration does not hold yet, the last step is the command that met the refusal: `aigw provider add`, and
`aigw setup` for the provider that it names or, in guided setup, that the person chose. Each stores the Token
itself. Where `setup --from` had no Token input and could not ask, its line gains `--token-stdin`, as its own
refusal of a missing Token does. For a provider that the configuration holds, the last step is the credential's
own command, whichever command met the refusal.

Acceptance tests follow each printed fix: the fixture removes the item that the printed deletion names, the test
runs every printed step word for word, and it requires the credential stored and the first command free of the
refusal. The first design of this fix asserted its sentence, and `aigw provider add` printed a last step that
answered "unknown provider".

### An item that AIGW may not read is no Token that AIGW can use

A client that reads its Token from AIGW's store can work only when the slot is present, holds a value, and is no
item that macOS refuses AIGW. The product already has one treatment for a client without that: it waits. A
missing Token never stops `sync` or a team setup for the other clients, and `status` and `check` name the step
that stores it. This Change gives a refused item the same treatment, and an empty stored Token with it, in the
one owner that decides which clients a transaction writes (`credentialReadyClients`).

- **A command that needs that Token for its own purpose ends with the refusal and its fix.** It stores the
  Token (`provider add`, `provider rotate`, `provider edit` for balance credentials, `setup` for the provider
  that it names), reads it (`model search`, `model add`, `provider balance`, `check` from the endpoint depth
  on, the credential command), or selects, enables or moves the one client that needs it (`use`,
  `client enable`, `client edit` with configuration files, guided setup). `client edit` already refused to move
  the files of a client without a Token, because the transaction then writes none and AIGW's settings would stay
  in the files that the edit drops.
- **A command that reconciles several clients leaves that client waiting and completes for the rest.** `sync`,
  `config rollback` and a team setup for a provider that it does not name do not write that client's files and
  name the item's fix as the next step: a team setup in that client's row, `sync` and `config rollback` as
  their one next step, for the first client that waits. Every other command that rewrites client files goes
  through the same owner and does not write that client's files either. A preview plans the same clients as
  its apply. The fix stores the Token and writes no client file, so a client that waited and whose files differ
  gets them from `aigw sync`, which the local check names until then.
- **A command that observes says what it saw.** `status` and `check --depth local` report the item with its
  title and fix. `provider show` promises to read no Token; it asks only whether an item exists and reports
  such an item as stored.

`aigw check`, the credential command, `provider balance` and `model search` present a store failure in their own
rows and take the fix from the typed failure, and all but the credential command its title; the credential
command, which a client runs, keeps its own title, which names the client. Every other command that ends with
the refusal returns it, and the general presentation of a store failure renders it and adds that the deletion
takes the item from the program that stored it. A locked keychain keeps its own fix.

One observation answers every one of these decisions: `secret.ObserveToken` reports a slot as present, empty or
refused, and a Token is usable when the slot is present and neither of the other two. Seven sites repeated its
two questions before and four asked only the first: `client edit`, the adapters' readiness, Hermes's list of
providers, which therefore listed a provider whose Token no reader could return, and a fallback of `status`
that no command reaches. A preview that presumes the Token that its command would store answers the
observation itself, so that the adapters plan what the apply writes.

What stays on presence alone decides nothing about a client's Token. `provider show` and the balance rows of
`status` promise to read no credential. The catalog read reads the Token itself next. Two decisions belong to
the environment backend, where an empty variable counts as unset and no Keychain item exists. Three sentences
only word the impact of a failed command. The price of the one owner is one more worker call for each present
slot that a command decided by presence before: a `sync` with two Hermes providers asks five times, four
before.

The observation asks the worker whether a present item is empty. The worker reads the item and
answers only that, so no value reaches the process, and one command asks once for each slot. It kept only the
answer before: a refused item counted as a stored Token. In a team setup a waiting provider's row shows the
refusal's title and fix, and its item in the document keeps the state `unobserved` with the fix as
`next_detail`, so the document's schema is unchanged.

Alternatives, with what each did when it was the lane's state or was worked through:

- **The transaction's reader check stops the command** (the lane's state after its first review). Before a
  transaction writes a client file it checks that the credential reader reads each Token, and that check met
  the item. One refused item then stopped `aigw sync` for every client. `aigw config rollback`, a recovery
  command, was stopped by a credential and needed a fix of three steps. A team setup's preview passed and
  listed a client file that its apply then refused to write.
- **End a team setup wherever its transaction would need the Token.** It keeps the reader check as the judge
  and makes preview and apply agree, at the price of giving the preview the transaction's change set, and it
  leaves `sync` and `config rollback` stopped.
- **Chosen: decide readiness once.** The price: `aigw sync` exits 0 while a client waits, as it does for a
  missing Token, and says why only through its next step, `status` and `check`.

It applies to every credential backend: an empty Token file counts as no Token too. On the system keyring an
empty stored Token failed `sync` and `use` as an unverified reader before; on the file backend, which has no
reader check, the client's file was written to name a reader that returns no Token. It does not apply to a
client that authenticates itself or runs its own credential command. The reader check stays for what it was made for, a reader that the store denies while it
admits the program, and its failure is the reader's again, not this item's.

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
writers: the suite pins two creators and three replacers of one slot, which leaves two lookups of room. A slot
that changes at every lookup ends the operation with the last status as a failure, and a delete reports an
absent item only when its own lookup found none. Tests force each order through a pause between the lookup and
the operation.

A reader that looks at the slot in the instant of another process's replacement can see it missing or invalid: 15
of 5,785 polls in the probe. That window is the platform's and older than this Change; the worker's read answers
what it saw.

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
- Both readings of the lock state are samples. A keychain that is locked after the first and unlocked before
  the second, around an operation of a few milliseconds, reads as unlocked, and AIGW's own item would then get
  the deletion fix. → Accepted: no person's lock and unlock fits that window, and the fix asks for the
  credential first.
- `aigw sync` and `aigw config rollback` complete while a client waits for a Token that AIGW may not read.
  → As for a missing Token: the next step is the item's fix, and `status` and `check` report the client.
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
- A read and a write look the slot up once more before they start, for the lock state. Its cost was not
  recorded.

## Migration Plan

This release takes no item over. A command that meets an item that still trusts only `/usr/bin/security` fails
with the fix above. There is no way back that converts items: a release from 0.7.0 on reads AIGW's own items as
they are.

## Verification after the tag

The system-store journey needs a signed candidate and a published predecessor, so it runs on a hosted disposable
macOS runner after the tag. There it stores an item through the system tool in the runner's default keychain and
requires the refusal from the candidate's store, which no private-keychain test reaches.
