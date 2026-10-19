# Tasks

## 1. Codex Adoption

- [x] 1.1 Make Codex package tests expect a model-only root change to be
      observed, a comment-only rewrite and an unchanged target to report no
      change, and provider, provider table, scheduler or catalogue drift not
      to be adoptable; they fail before observation exists.
- [x] 1.2 Make reconciliation tests expect a root model that already equals the
      model being projected to pass ordinary synchronization while a foreign
      value still refuses; they fail before the transition accepts it.
- [x] 1.3 Make adapter and synchronization tests expect one matching Route to
      be adopted and reported, none or several to name `aigw use --for codex`
      commands, disagreeing targets to refuse, and a following dry run to
      report no change; they fail before adoption exists.
- [x] 1.4 Implement observation, matching, adoption in synchronization and the
      transition rule; the tests pass.

## 2. Documentation

- [x] 2.1 Describe the adoption in `docs/concepts/product-concepts.md` and in
      `CHANGELOG.md`.

## 3. Verification

- [x] 3.1 `mise run check` passes on the change set.
- [x] 3.2 In an isolated home, the candidate adopts a Codex root model written
      the way Codex writes it, repeats an unchanged dry run, and refuses a
      provider edit
      (`build/evidence/codex-selection-adoption/acceptance-candidate-1.txt`).
