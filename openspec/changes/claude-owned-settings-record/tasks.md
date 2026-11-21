## 1. Records

- [x] 1.1 Return the first record of Claude Code's settings to the fields and digest inputs that 0.8.3 reads. Done
      when a test decodes a written record with unknown fields refused into the fields that 0.8.3 declares and
      recomputes its digest from 0.8.3's inputs, with a map in the settings.
- [x] 1.2 Add the record of owned settings keys and plan, write, restore and withdraw it with the first record.
      Done when the adapter tests cover each row of the design's table, a withdrawal without the first record, an
      entry that this program does not know (kept, its key untouched), a re-indented file, and a refusal that
      writes nothing; and the adapter contract test lists the record among the plan's effects.
- [x] 1.3 Report a map that differs from the selected one in the inspection, and carry the record in the
      verification copy. Done when the inspection and copy tests pass.

## 2. Acceptance and documents

- [ ] 2.1 Make the release journey's manifest list a provider's own ID of a Claude model, and assert the map after
      the update, after the rollback's synchronization, after the re-upgrade and its absence after the uninstall,
      with the new record among the files that the uninstall removes. Done when the journey passes with the
      source-built predecessor.
- [ ] 2.2 Correct the changelog's entry for 0.9.0 and the documents that describe the map and its record. Done when
      the changelog validation and the documentation gates pass.

## 3. Verification

- [ ] 3.1 Pass `mise run check` on the lane head.
- [ ] 3.2 Pass the proposal's hosted checks on both forges, the Windows jobs included.
- [ ] 3.3 Pass the candidate's acceptance against the published `v0.8.3` on the lane head.
