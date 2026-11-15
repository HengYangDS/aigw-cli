## 1. The macOS worker uses the Security framework alone

- [x] 1.1 Read, write and delete a slot directly; remove the migration, the staging item, every call of
      `/usr/bin/security`, the lock, and the restore and reclaim commands on every platform. Private-keychain
      tests show that an item of another program stays denied and unchanged on read and replacement, that its
      removal removes it, and no product source runs `/usr/bin/security`.
- [x] 1.2 Remove the worker's role, `program.InstalledProgram` and `credentialRole`; every worker command takes the
      same path whatever executable runs it.
- [x] 1.3 Create an item with its label in one call, and let a write or a delete look its slot up again, with at
      most five lookups in all, when another writer changed the slot (-25299, -25300, -67701). Tests force each
      order through a pause between lookup and operation; two concurrent creators and three concurrent replacers
      of one slot all succeed; a slot that changes at every lookup ends with its last status.

## 2. An item of another program names its fix

- [x] 2.1 Tell another program's item from a locked keychain by the lock state of the keychain that holds the
      item, read before the operation and after a refusal; the typed diagnostic says unlocked only when both
      readings do. Tests show a locked keychain for a locked second keychain behind an unlocked first one, and for
      a keychain unlocked between the refusal and the second reading.
- [x] 2.2 Give a refusal with an unlocked keychain one typed failure whose fix has one owner: have the credential
      at hand, delete the item with the system tool, then store the credential again. For a provider that the
      configuration does not hold, `provider add` and `setup` name their own line as the last step, with
      `--token-stdin` where `setup --from` could not ask. `aigw check`, the credential command and
      the general presentation of a store failure give the fix; a locked keychain keeps its own. Tests read the
      fix on each surface and follow every printed step of each kind of fix until the credential is stored.
- [x] 2.3 Show the same title and fix from `model search`, `model add` and `use` for a model that is not added
      yet, and stop guided setup at the refusal before it asks for a model. Tests read each surface, in text and
      in JSON where the command has JSON.
- [x] 2.4 Add to the system-store journey a step that stores an item through the system tool in the default
      keychain of its disposable host and requires the refusal from the candidate's store; the step is not
      reached where that scope is not set.
- [x] 2.5 Keep a refusal in the observation of whether a Token is stored: `aigw check --depth local`,
      `aigw status` and `aigw setup` report the refusal with its fix and do not count the item as a stored Token.
      `setup --from` ends with it for a provider that it names and lets any other provider wait with the fix.
      Tests read each in text and, where the command has it, in JSON, and follow the fix that
      `check --depth local`, `status` and a waiting provider's row print.
- [x] 2.6 Decide in the one owner of readiness that a client whose Token slot is empty or holds an item that
      macOS refuses is not written: `sync`, `config rollback` and a team setup complete for the other clients
      and name the fix, a preview plans the same clients as its apply, and `use` and `client enable` end with the
      refusal and change nothing. The reader check's failure is the reader's again. Tests cover the transaction,
      each command in text and JSON, a second client that is written, and an empty Token on the file backend.
- [x] 2.7 Observe a Token through one owner, `secret.ObserveToken` (present, empty, refused), at every site that
      decides on one. `client edit` with configuration files refuses a client whose Token is unusable and changes
      nothing; an adapter leaves such a client's activation as it is, and Hermes lists no provider whose Token
      AIGW cannot use. Tests cover the owner's cases, `client edit` in text and JSON, and both adapter decisions
      for an empty and a refused slot.

## 3. A rollback moves no credential

- [x] 3.1 Remove credential custody from `program.Updater.Rollback` and delete the predecessor conversion; the
      portable rollback journeys pass unchanged.
- [x] 3.2 Refuse `aigw self rollback` on a Homebrew installation as the other program commands are refused, and
      drop the rollback advice from the Homebrew ownership error; help and JSON tests follow.

## 4. Documents

- [x] 4.1 Amend the Token backend decision record and the 0.7.0 decision record, and correct the README, the
      security model, the authority boundary, the release policy and the changelog. No document says that a
      Token used once is AIGW's own, or prescribes a cutover from 0.3.5.

## 5. Verification

- [x] 5.1 `mise run check` passes on the lane head.
- [x] 5.2 The hosted graph passes at the proposal head on both forges.
- [x] 5.3 Each finding of a fresh-context review of the lane is fixed on the lane head or answered in the design.
