## 1. The macOS worker uses the Security framework alone

- [ ] 1.1 Read, write and delete a slot directly; remove the migration, the staging item, every call of
      `/usr/bin/security`, the lock, and the restore and reclaim commands on every platform. Private-keychain
      tests show that an item of another program stays denied and unchanged on read and replacement, that its
      removal removes it, and no product source runs `/usr/bin/security`.
- [ ] 1.2 Remove the worker's role, `program.InstalledProgram` and `credentialRole`; every worker command takes the
      same path whatever executable runs it.
- [ ] 1.3 Answer a duplicate at creation by updating the item: two concurrent writers of one new slot both
      succeed, the slot holds one of the two values, and no second item exists.

## 2. An item of another program names its fix

- [ ] 2.1 Tell another program's item from a locked keychain by the keychain's lock state, which the worker reads
      after a refusal and reports in its typed diagnostic; the status is -25293 for both, measured in private
      keychains.
- [ ] 2.2 Give a refusal with an unlocked keychain one typed failure whose fix has one owner: delete the item with
      the system tool, then store the credential again. `aigw check`, the credential command and the general
      presentation of a store failure give it; a locked keychain keeps its own fix. Tests read the fix on each
      surface.

## 3. A rollback moves no credential

- [ ] 3.1 Remove credential custody from `program.Updater.Rollback` and delete the predecessor conversion; the
      portable rollback journeys pass unchanged.
- [ ] 3.2 Refuse `aigw self rollback` on a Homebrew installation as the other program commands are refused, and
      drop the rollback advice from the Homebrew ownership error; help and JSON tests follow.

## 4. Documents

- [ ] 4.1 Amend the Token backend decision record and the 0.7.0 decision record, and correct the README, the
      security model, the authority boundary, the contribution guide and the changelog.

## 5. Verification

- [ ] 5.1 `mise run check` passes on the lane head.
- [ ] 5.2 The hosted graph passes at the proposal head on both forges.
- [ ] 5.3 Each finding of a fresh-context review of the lane is fixed on the lane head or answered in the design.
