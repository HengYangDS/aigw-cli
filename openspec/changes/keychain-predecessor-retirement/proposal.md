## Why

Releases up to 0.3.5 stored each macOS Token through `/usr/bin/security`, so those Keychain items trust that tool
alone. Since 0.4.1 the macOS credential worker carries four behaviors for such items: a one-time migration to an
item of its own, deletion through the tool, a conversion back before a rollback, and a role that tells the
installed program from every other copy, with a lock around each write and delete.

None of it serves a transition that the product still supports. 0.7.0 carries no importer and no rollback across
it, by the owner's ruling that a release holds no code for an earlier one; this path is the exception that stayed.
An item that one signed release writes is readable by every later signed release, so no release from 0.4.1 on
needs an item converted, and the conversion before a Homebrew downgrade has no reader left to serve.

The path also costs. Each call of the tool has one second inside the worker's five, and five of the seven tests
that failed on a loaded macOS guest are in it. A plain write waits on the migration lock. And `aigw status`,
`sync`, `use` and `setup` can migrate an item, which the specification allows only to `aigw check`.

## What Changes

- **BREAKING**: AIGW no longer takes over a Keychain item that 0.3.5 or earlier stored. It can neither read nor
  replace such an item; the command that meets one says that another program stored it and names the steps that
  replace it: have the Token at hand, delete the item, then store the Token again. Nothing changes for an item
  that the installed program of 0.4.1 to 0.7.2 read or replaced, which made it AIGW's own.
- **BREAKING**: `aigw self rollback` moves no credential. On a Homebrew installation it is refused like the other
  program commands, and Homebrew installs the earlier release; its `prepared` result is gone.
- The macOS worker reads, writes and deletes only through the Security framework. No product code runs
  `/usr/bin/security`, and no command migrates an item.
- A write or a delete no longer waits on a lock. An item is created with its label in one call, and a write or
  a delete looks its slot up again when another writer changed it, so it completes on the slot as it then is.
  The worker no longer learns a role from its installation.
- The one-time cutover for a session that still calls Homebrew's public credential-command link leaves the
  README, the decision record and the specification. It was written for the first upgrade from 0.3.5 and rested
  on reading that release's Keychain items; since 0.7.0 such an upgrade is a new setup.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `credential-continuity`: no migration and no rollback custody; an item of another program is refused with its
  fix; no cutover for a caller of a package manager's link.
- `secret-storage`: the native bound has no retained-item exception; a retained item is reached only by its own
  signing identity; a mutation looks its slot up again when another writer changed it.
- `cli-readiness`: `aigw check` changes no credential.
- `release-distribution`: Homebrew's ownership of the program covers a rollback.

## Impact

Code: `internal/secret/native` (the migration file, the access-list and lock helpers, the worker's commands and
role), `internal/secret` (predecessor custody), `internal/program` (rollback custody, the installed-program
test), `internal/cli/self` (the Homebrew rollback), `cmd/aigw`, and their tests. The decision record on the Token
backend and the README, security model and release policy follow.

Linux and Windows authorize the user or the session, never the executable, and never had this path; they lose
only the two unused worker commands.

Evidence: unit tests with private keychains cover the worker, and the hosted graph covers the three platforms.
That one signed release reads another's Keychain items cannot be tested before the release: the system-store
journey needs a disposable macOS host and two published, signed releases, so it is dispatched after the tag as
release acceptance (`macos_keychain=true`). Until then the claim rests on the operator host, where the installed
0.7.2 read all three stored Tokens without a prompt on 2026-11-15 and their creation dates, from releases before
it, stayed as they were. The release therefore meets no item of another program there.

Not addressed: the bound of the tests' own Keychain fixtures, which still create private keychains through
`/usr/bin/security`.
