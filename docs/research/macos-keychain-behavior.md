---
subject: aigw:macos-keychain-behavior
role: research
state: active
relations: {}
---

# macOS Keychain Behavior for a Noninteractive Credential Worker

These measurements of 2026-11-15 record what the macOS Keychain answers to a program that reads,
writes and deletes generic-password items without user interaction, as AIGW's credential worker
does. They informed the retirement of the predecessor Keychain path and the
[Token backend decision](../decisions/dr-0011-single-portable-token-backend.md). They establish no
product support by themselves, and they hold for the versions and samples named under
[Limits](#limits).

## Questions

1. What does macOS answer for an item that another program stored, and can that answer be told
   from a locked keychain's?
2. Which keychain's lock state explains a refusal when several keychains are searched?
3. What happens to an item reference that one process holds while another process changes the
   item?
4. What do several concurrent writers of one slot get, through an item reference and by query?
5. Does a signed release read and replace the items of another signed release, in both
   directions of an upgrade?

## Method

- **Keychains.** Every keychain was a private file that `security create-keychain` made in a
  temporary folder and that was deleted afterwards. No measurement named the login keychain. The
  values were synthetic.
- **Tests.** The private-keychain tests of the worker's package,
  [`internal/secret/native`](../../internal/secret/native/foreign_item_darwin_test.go), ran on
  macOS 27.2.1 and, in the hosted runs of the change, on macOS 26. A finding marked _test_ is
  pinned by one of them.
  Tests of concurrency force their order through one pause point between a lookup and the
  operation on what it found.
- **Probe.** A C program called the Security framework directly on macOS 27.2.1, with
  `SecKeychainSetUserInteractionAllowed(false)`. A finding marked _probe_ comes from it. Its
  source and output were working evidence of the change and are not kept in the repository; the
  calls are named below so that a measurement can be repeated.
- **Another program's item.** `/usr/bin/security add-generic-password` stored it, as releases up
  to 0.3.5 stored every Token.
- **Signed builds.** The package's test binary was built from the worker of this change and from
  the worker of 0.7.2, each from an export of its commit, and signed with the release's Developer
  ID identity and the identifier `aigw`; one copy was signed ad hoc.

No Apple documentation was consulted. Each statement below is a measurement, and no mechanism
behind it was investigated.

## Findings

### An item that another program stored

| Operation by the measuring program                           | Answer                          | Source      |
| ------------------------------------------------------------ | ------------------------------- | ----------- |
| Look the item up by its attributes                           | found                           | test, probe |
| Read its value (`SecItemCopyMatching` with `kSecReturnData`) | -25293 (`errSecAuthFailed`)     | test, probe |
| Replace its value through the found reference                | accepted; the read stays -25293 | probe       |
| Replace its value by query (`SecItemUpdate`)                 | accepted; the read stays -25293 | probe       |
| Delete it, with the keychain unlocked or locked              | deleted                         | test        |

macOS therefore refuses the read and nothing else. A program that replaces such an item stores a
value that it cannot read back. The worker reads an item before it replaces it; without that
read, the test of a refused replacement fails with an accepted one.

### A locked keychain

A read answers -25293 for the program's own item and for another program's. A write answers
-25293 for both and for an absent slot. A lookup by attributes, a listing and a deletion succeed.
(_Test._)

The status is the one that an item of another program gives. The two cases differ only in the
lock state of the keychain.

### Which keychain's lock state

With two keychains in the search list, the first unlocked and empty, the second locked and
holding the item:

- the read answers -25293;
- the lookup still finds the item, `SecKeychainItemCopyKeychain` names the second keychain, and
  `SecKeychainGetStatus` reports it locked;
- for a slot without an item, the lookup answers -25300 (`errSecItemNotFound`).

After another process unlocks the second keychain, the same process reads its status as unlocked
and its next read succeeds. (_Test, probe._)

### Creating an item

| Creation                                                    | Label attribute | Access description | Creator code |
| ----------------------------------------------------------- | --------------- | ------------------ | ------------ |
| `SecKeychainAddGenericPassword`, then a label change        | the label       | the service name   | `aapl`       |
| `SecKeychainItemCreateFromContent` with the label attribute | the label       | the label          | none         |

The access description is what `security dump-keychain -a` prints for the item's access lists,
and what macOS names when it asks a person for access. A second creation of the same slot
answers -25299 (`errSecDuplicateItem`). Of three processes that create one new slot at once,
exactly one succeeded in each of 150 rounds. (_Test for the description; probe for the rest._)

The two-step creation lost its label on a hosted macOS 26 runner when two writers created one
slot at once: both writers reported success, and the item kept the service name as its label.
The order that produced it was not reproduced. A forced order in which the second writer
replaces the value between the first writer's two steps loses the label too, and the first
writer then fails. The same test had passed 120 of 120 runs on the development host.

### A reference after another process changed the item

A process looks an item up and holds the reference. Another process then acts. The first
process operates through its reference. (_Probe; the worker's tests force the same orders._)

| The other process                    | Label, replace or delete through the held reference |
| ------------------------------------ | --------------------------------------------------- |
| replaced the value                   | -25300                                              |
| deleted the item and created it anew | -67701 (`errSecInvalidRecord`)                      |
| changed only the label               | succeeds, and the label stays                       |

After a replacement by another process, a new lookup in the first process returns the same stale
reference while that reference is held, and the operation fails again. Once the reference is
released, a new lookup returns a usable one. An update by query fails the same way while the
process holds the stale reference, and succeeds after its release.

### Several writers of one slot

Each round started its writers together; each writer replaced the value once.

| Writers per round               | Rounds | 0   | -25299 | -25300 | -67701 | Successes in a round |
| ------------------------------- | ------ | --- | ------ | ------ | ------ | -------------------- |
| 3, through a reference          | 150    | 205 | 221    | 9      | 15     | not counted          |
| 3, through a reference          | 200    |     |        |        |        | 1, 2 or 3            |
| 5, through a reference          | 150    |     |        |        |        | 1 to 4               |
| 3, by query (`SecItemUpdate`)   | 150    | 150 | 297    | 1      | 2      | 1                    |
| 1 by query and 1 by a reference | 150    | 150 | 149    | 1      | 0      | not counted          |

No round of the per-round runs ended without a success: 119, 79 and 2 of the 200 rounds had one,
two and three successes, and 29, 97, 19 and 5 of the 150 rounds with five writers had one to
four. After each run the slot held one item with its label and a readable value. (_Probe._)

An update by query collides as an update through a reference does, with the same three statuses,
and it gave fewer successes in a round. It removes no retry.

### A reader during another process's write

A process read one slot in a loop while a child process wrote it, for 200 rounds each.
(_Probe._)

| The child                 | Reads while the child ran                 | Reads after the child |
| ------------------------- | ----------------------------------------- | --------------------- |
| created the item, 1 call  | 246,464 not found, 599 succeeded          | 200 of 200 succeeded  |
| replaced the item's value | 5,770 succeeded, 9 not found, 6 at -67701 | 200 of 200 succeeded  |

A reader can see a slot missing or invalid in the instant of another process's replacement: 15
of 5,785 reads here.

### Signed builds

(_Probe binaries as described under Method; macOS 27.2.1._)

- An item that one signed build of the new worker created was read and replaced by a second
  signed build of different bytes, and the first then read the second's value.
- A signed build of the 0.7.2 worker read that item.
- An item that the 0.7.2 worker created was read and replaced by the new worker, and the 0.7.2
  worker read the replaced value. The label stayed.
- The ad hoc build was refused the read of a signed build's item with -25293 in an unlocked
  keychain, and a signed build was refused the read of the ad hoc build's item.
- The item's access list showed one trusted application with its designated requirement: the
  identifier `aigw`, Apple's Developer ID certificates and the team. The second build, at
  another path and with other bytes, was admitted, so the requirement decides and not the path
  or the bytes.

An [earlier measurement](../../openspec/changes/archive/2026-10-13-macos-keychain-item-migration/design.md)
of 2026-10-13 on macOS 27.0.1 gave the same binding to identifier and team.

## What the findings decided

| Finding                                                          | Decision in the worker                                                                            |
| ---------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| One status for another program's item and for a locked keychain  | The worker reports whether the keychain was unlocked, read before the operation and after it      |
| The lock state belongs to the keychain that holds the item       | The worker asks that keychain, and for an absent slot the one that a creation would write to      |
| macOS accepts a replacement of an item that it does not let read | The worker reads before it replaces, and replaces nothing that it cannot read                     |
| A creation in two steps can lose its label                       | The worker creates an item with its label in one call                                             |
| A changed item makes a held reference stale, with three statuses | A write or a delete releases its reference and looks the slot up again, with five lookups at most |
| By query, writers collide alike                                  | The worker keeps the lookup and the reference; no retry would be saved                            |
| Signed builds read and replace each other's items, both ways     | No release takes over an item, and a program rollback moves no credential                         |

That AIGW does not delete another program's item for the operator, although macOS would let it,
is a decision about ownership and not a finding.

## Limits

- **Versions.** The probe ran on macOS 27.2.1 only. The tests ran there and, in one hosted run
  per change, on macOS 26.
- **Keychains.** Private file keychains only. The login keychain of a real account was not
  measured; the release's system-store journey covers it on a disposable host. The
  data-protection keychain, which AIGW does not use, was not measured.
- **Samples.** The counts above are single runs of 150 or 200 rounds on one host under its load
  of that hour. They bound nothing; the worker's bound of five lookups is a choice that the tests
  pin with two creators and three replacers.
- **Not measured.** Whether the user-interaction switch belongs to the process or to the thread;
  the worker keeps one operation on one thread instead. What a writer gets while a person locks
  and unlocks a keychain within one operation.
- **Mechanism.** Why a replacement collides as a duplicate, and why a replaced item invalidates
  a reference, were not investigated.
