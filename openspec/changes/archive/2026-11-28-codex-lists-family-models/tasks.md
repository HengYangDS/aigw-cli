## 1. The listed models

- [x] 1.1 Add `Listed` to `domain.Runtime`, filled from `Config.FamilyModels` when a selection resolves, each with its
      model ID, family, name and declared window; a test with several families, a variant and two protocols passes.
- [x] 1.2 Make `modelcatalog.Document.Project` take the listed models: each described one listed with its window, its
      own entry or an alias under a unique name, every other listed entry hidden, hidden entries and every other field
      kept; tests of the scenarios of both delta specifications pass.
- [x] 1.3 Make the Codex projection, its reused copy and the window check use the runtime's listed models; the Codex
      adapter's tests pass.

## 2. Real-client proof and record

- [x] 2.1 Add to `catalogcheck` the claim that the effective catalog lists the verified entry alone and hides every
      entry that the projection hid; its tests pass, and runs against the installed Codex pass for an alias and for a
      model's own entry, recording the client's version and digest. Passed on 2026-11-28 against codex-cli
      0.172.0 (SHA-256 `f30a9971…`) for `gpt-6.2-sol-cdx` and `gpt-6.2-sol`.
- [x] 2.2 Record the change in `CHANGELOG.md`'s unreleased section as breaking; the changelog and prose gates pass.
