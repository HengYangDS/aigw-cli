## 1. The declaration

- [x] 1.1 Give a model an optional `context_window` with one default constant of 1,000,000 in the domain, carry it
      into the resolved runtime, and refuse a value outside 64,000 to 100,000,000 tokens. Done when domain tests cover a model
      without a declaration, with one, and an invalid one.
- [x] 1.2 Store the field in schema 8 only when declared, in `aigw.toml`, the team file and `aigw config export`.
      Done when a store test shows that a model without a window encodes no such key, that a declared one round
      trips, and that a team file's value reaches the configuration.
- [x] 1.3 Add `--context-window` to `aigw model add` and `aigw model edit`, and `--default-context-window` to
      `aigw model edit`; report `context_window` in `aigw model list` and `aigw model show`, in JSON for every
      model and in text where it differs from the default. Done when acceptance tests read each and the refusals
      of the specification's fourth scenario.

## 2. Codex

- [x] 2.1 Write the declared window into the two window fields of the selected model's entry, for an alias and for
      a model whose ID is an entry of the table. Done when catalog tests compare every other field and every other
      entry with Codex's own table, cover a lower declared window, and fail when the bundled table lacks the two
      fields.
- [x] 2.2 Report in a passing check of `aigw check codex`, for each Codex target, the window that the selected
      model's entry carries in the table that AIGW wrote, or that Codex's fallback window applies to a model whose
      family the table does not hold. Done when tests read the check for a model with an entry and for one
      without, and its absence where AIGW wrote no table.
- [x] 2.3 Rewrite the table when the declared window of the selected model changes. Done when an adapter test
      changes the window, reads the rewritten table and reads a following plan that changes nothing; the command's
      path to a client's file is held by Hermes's acceptance test, and task 5.1 runs the command with the real
      Codex, because no fixture of the acceptance package can run a Codex that prints its table.
- [x] 2.4 Give the tracked catalog verifier the declared window: leave the two window fields out of its comparison
      with the bundled metadata, require them in the entry that the client rendered, and verify a model that is
      itself an entry. Done when its tests cover an alias and an entry with their refusals, and the contribution
      guide names what the command takes.

## 3. Hermes

- [x] 3.1 Read the models that a provider entry lists through one function that accepts the mapping and the list
      of IDs, and use it wherever an entry's models are read. Done when its table test covers both shapes and the
      choice-following tests pass over each.
- [x] 3.2 Write each model's declared window as `context_length` under its ID, in AIGW's order, and no window on
      an entry or in the `model` block. Done when projection tests read the mapping and its order, a file in the
      earlier shape with its record is accepted and rewritten, and a changed window plans a write.

## 4. Team file and documents

- [x] 4.1 Declare the vendor's documented window for the families below 1,000,000 in `manifests/team.toml`, and
      name each family's source in a dated section of the qualification evidence. Done when the team file's test
      holds the specification's scenario and the file still equals its native export.
- [x] 4.2 Describe the declaration and each client's use of it in the product concepts and the team rollout guide,
      and state the change under Unreleased in `CHANGELOG.md` with the new JSON field. Done when the documentation
      gates pass.

## 5. Verification

- [x] 5.1 Run the tracked catalog verifier with the installed Codex for an alias and for a model that is an entry
      and keep its records; run one short turn in Codex and one in Hermes, each in an isolated home with the real
      client, and read the window that the client reports for the selected model; switch the model once with
      Hermes's own command in that home and require the next synchronization to follow it.
- [ ] 5.2 Run the published previous release through the lifecycle journey after a rollback from this program,
      with Hermes and Codex enabled, and require both to synchronize without a refusal.
- [ ] 5.3 Pass `mise run check` on the lane head.
- [ ] 5.4 Pass the proposal's hosted checks on both forges.

## 6. The Review Of The Whole Lane

- [x] 6.1 Answer the review's findings 3, 5 and 7 and its design-principle items in code: Codex's check fails a
      table that does not give the declared window, a reused table takes it, a window that cannot reach a client is
      no changed projection of it, a window has one spelling (the default is stored as no declaration), and each
      of the two window fields, the reader of AIGW's table and the entry that describes a model has one owner.
      Done when every touched package and the acceptance package pass on the lane's tree.
- [x] 6.2 Answer findings 1, 2, 4, 6, 8 and 9 (the report is in `build/runtime/work/review-12/report.md` of the main
      checkout) and correct the specification and document text that the review found untrue. Done when each
      finding has a test or a stated decision in the design, and the CHANGELOG, the concepts and the team guide say
      what the code does.
