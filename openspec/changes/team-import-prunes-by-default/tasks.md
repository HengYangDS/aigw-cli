## 1. The merge keeps only what something uses

- [x] 1.1 Change `MergeTeamFile` to take the keep-local choice instead of a prune switch, so that an import
      removes the local-only models that no client selects and no recommendation names, and then the families that
      no remaining model or the file names. Done when the domain tests cover: an unused older family beside its
      successor removed; a selected local-only model kept; a recommended local-only model kept; a family removed
      only when none of its models remains; and `keep-local` leaving every local entry.
- [x] 1.2 Add `--keep-local` and remove `--prune` from `aigw setup`, with the argument check that `--keep-local`
      needs `--from`. Done when `aigw setup --prune` fails as an unknown flag with nothing written, and
      `aigw setup --keep-local` without `--from` fails in the words of the other team flags.
- [x] 1.3 Report the removal in every preview and write, and what `--keep-local` kept. Done when the human section
      and the JSON document name each removed model and family, and with `--keep-local` each kept one; a preview
      of the second import after a first reports no removal.

## 2. The surfaces that name the old behavior

- [x] 2.1 Update `cmd/aigw/testdata/script/setup.txtar` and the acceptance test that lists setup's flags. Done
      when the script's prune cases run under the new default and its `--keep-local` case keeps what the old
      `--prune` case removed, and the flag list names `--keep-local` and not `--prune`.
- [x] 2.2 Update `docs/concepts/product-concepts.md` and `docs/guides/team-rollout.md`, and state the breaking
      change under `CHANGELOG.md` Unreleased. Done when no document says that local-only entries stay unless
      `--prune`, and the guide shows `--keep-local` where it shows the old behavior.

## 3. Verification

- [ ] 3.1 Compile and vet the changed packages and their tests for Windows and Linux on the lane head.
- [ ] 3.2 Pass `mise run check` on the lane head.
- [ ] 3.3 Apply the shipped team file on this workstation with the built program from the lane, in a preview
      first, and read the report. Done when the preview names exactly the 30 models and 11 families that the
      current host holds as local-only, and nothing else.
