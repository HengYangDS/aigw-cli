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
      at hand, delete the item with the system tool, then store the credential again. `provider add` and `setup`
      name their own line as the last step, and `config rollback` does for a provider that only the restored
      configuration holds. `aigw check`, the credential command and the general presentation of a store failure
      give it; a locked keychain keeps its own fix. Tests read the fix on each surface and follow each kind of
      last step as printed.
- [x] 2.3 Show the same title and fix from `model search`, `model add`, `use` for a model that is not added yet
      and `config rollback`, and stop guided setup at the refusal before it asks for a model. Tests read each
      surface, in text and in JSON where the command has JSON.
- [x] 2.4 Add to the system-store journey a step that stores an item through the system tool in the default
      keychain of its disposable host and requires the refusal from the candidate's store; the step is not
      reached where that scope is not set.

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

- [ ] 5.1 `mise run check` passes on the lane head.
- [ ] 5.2 The hosted graph passes at the proposal head on both forges.
- [ ] 5.3 Each finding of a fresh-context review of the lane is fixed on the lane head or answered in the design.
