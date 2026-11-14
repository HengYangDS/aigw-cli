## Context

`internal/secret/native/migration_darwin.go` wraps every macOS read, write and delete. A denied operation on an
item whose access list trusts `/usr/bin/security` is answered by reading the Token through that tool, staging it
in a `migration@<slot>` item, deleting the old item through the tool and recreating the slot. Only the installed
program does this; any other copy reads through the tool and changes nothing, so each worker asks its own
installation which it is. `aigw self rollback` converts every configured slot back for a program older than
0.4.0, and on a Homebrew installation that conversion is all it does.

A Keychain item that a signed AIGW release creates trusts the signing identifier and team, not the file. A later
signed release reads it without a prompt.

## Goals / Non-Goals

**Goals:**

- The macOS worker uses one authority, the Security framework with user interaction off.
- A person whose item AIGW may not use learns the two steps that replace it, from the first command that fails.
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
AIGW delete either. The status alone does not tell the two apart. After such a refusal the worker therefore reads
the lock state of the keychain that it writes to, metadata that needs no prompt, and its typed diagnostic says
whether that keychain was unlocked. A refusal with an unlocked keychain is an item that macOS does not let this
program use: another program stored it, or its access list no longer admits AIGW. The worker asks the default
keychain, where AIGW and the releases before it store every item; an item that someone moved to another keychain
is outside this reading.

The store turns that refusal into one typed failure, which owns the title, the evidence and the fix: delete the
item with `security delete-generic-password -s AIGW_TOKEN -a <slot>`, then store the credential again with its
command, `aigw provider rotate <provider>` for a Token and `aigw provider edit <provider> --balance-token-stdin
--balance-user-id <user-id>` for balance credentials. The general presentation of a store failure gives it, which
serves `provider add`, `provider rotate`, `setup`, `use` and `sync`. `aigw check`, the credential command,
`provider balance` and `provider edit` present a store failure themselves and take the title and the fix from the
typed failure. A locked keychain keeps its own fix.

A batch that stored another provider's Token before it met the refusal undoes that Token. If the undo fails, the
store reports it as a failed undo, which every presentation states beside the refusal.

AIGW does not replace such an item itself, although macOS would let it delete the item. A write that failed
after the deletion would lose a Token that the other program may still use, and the operator's own deletion is
the explicit decision. `aigw provider remove` deletes the item as it deletes AIGW's own, which is the operator's
decision too.

### No role

The role existed to keep a reader copy from migrating. Without a migration every copy does the same thing, so
`Role`, `program.InstalledProgram` and the worker's role callback go.

### No lock

The lock serialized migrations. Without it, two writers that create one slot at once collide: the second adds an
item that the first has just added, and macOS answers with a duplicate. The writer answers a duplicate by updating
the item it now finds, so both succeed and the slot holds one of the two values. A test with two concurrent
writers of a new slot pins it.

### Rollback moves no credential

`program.Updater.Rollback` loses its custody parameter. On a Homebrew installation `aigw self rollback` is
refused as `aigw self update` is. Its fix is a sentence, because Homebrew has no one command that installs an
earlier release of a Cask. The update document keeps its schema version: no field changes, and the `prepared`
state was a value of the Homebrew branch that no longer answers.

## Risks / Trade-offs

- Someone who upgrades directly from 0.3.5 or earlier, with a provider named as the old Account was, must enter
  the Token again. → The failure names both steps. Whether anyone is in that state is not known.
- Signed releases must keep the identifier `aigw` and the team. A change of either makes every item foreign.
  → Unchanged by this Change; the release policy already binds the identity.
- An ad-hoc or development build cannot read a signed release's items. → As before; such a build belongs to no
  installation and uses a private home.

## Migration Plan

Before the release, each stored Token must have been used once by 0.4.1 or later; `aigw check` on 0.7.2 does it.
After the release an item that still trusts only `/usr/bin/security` is replaced by the two steps of its failure.
There is no way back that converts items: a release from 0.7.0 on reads them as they are.
