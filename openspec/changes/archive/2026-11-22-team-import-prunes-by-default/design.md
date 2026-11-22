## Context

`aigw setup --from` merges a reviewed team file into `aigw.toml`. Since the command surface's end state (Change
`command-surface-end-state`, archived 2026-11-12) the merge kept every local-only model and family unless
`--prune` was given, because an import must not silently take away a model that a person added for their own use.
The cost showed up on the owner's workstation on 2026-11-22: 128 models, 30 of them families the file no longer
names, and his reading that an imported profile should leave only the profile's configuration.

## Goals / Non-Goals

**Goals:** an import leaves a configuration that holds what the file names plus whatever a client selects or a
recommendation names, and nothing else; the preview and the result name what was removed; a person who wants the
old result has one flag to ask for it.

**Non-Goals:** changing what a team file may declare; touching any client selection, provider endpoint or Token;
any compatibility surface for `--prune`; pruning anything a selection or a recommendation names.

## Decisions

### An import keeps only what something uses, and says what it removed

- **Rests on:** the owner's words of 2026-11-22 (「导入 team profile 后应该只有遵循范式的配置方式啊」, 「按照你建议的方式」) and
  the measurement that not one of the 30 local-only models on his machine was selected by a client or named by a
  recommendation. The merge already computed exactly this set for `--prune`; the default and the flag were the
  only thing wrong.
- **Considered:** keep the flag and document it better (leaves every existing machine redundant, which is the
  defect the owner reported); make the import ask before removing (an import is run by automations and by team
  members applying a reviewed file, and a question in the middle of a transaction is a worse surface than a
  default); warn and keep (leaves the redundancy in place).
- **Chosen because:** a team file is a reviewed statement of what the team runs, so an import that leaves
  everything else is the surprise, not the removal. Nothing that is in use can be lost: the removal is defined by
  "no client selects it and no recommendation names it", and every provider, endpoint, Token and client selection
  is untouched. The one real cost — a local model added for one experiment disappears — is met by `--keep-local`
  and by `aigw config rollback --last-change`, which restores the configuration as it was.
- **Applies to:** `MergeTeamFile` and `pruneLocalOnly` in `internal/domain/team_merge.go`; the report's pruned
  entries in `internal/cli/setup/team_report.go`.

### The old flag is removed, not deprecated

- **Rests on:** the repository's rule that no compatibility shim survives a breaking change, and the owner's
  standing preference for the ideal end state over compatibility code.
- **Considered:** keep `--prune` as a no-op so old scripts keep parsing (a flag that means "do what I now do
  anyway" teaches a wrong model of the command); keep it as an alias of the default and add `--keep-local`
  (two names for one behavior, and the deprecated name says the opposite of what it does).
- **Chosen because:** the command surface is the contract, and a flag whose name describes the removed behavior is
  worse than its absence. `aigw setup --prune` now fails as an unknown flag, which is what a script written
  against the old surface should see.
- **Applies to:** `internal/cli/setup/setup.go` and the acceptance test that lists the flags setup admits.

### The report names both outcomes

- **Rests on:** the existing report's job — a preview and a result say each difference that stays local — and the
  owner's need to see what an import took away before it is taken away.
- **Considered:** report removals only on request (silent removal is the failure mode this Change is fixing);
  report nothing and let `aigw model list` show the result (the person cannot tell a removal from a typo in the
  file).
- **Chosen because:** what an import removes is the most consequential part of its result, so it is reported in
  every preview and every write, and what `--keep-local` kept is reported beside it.
- **Applies to:** `team_report.go`; the human section "Pruned"/"Kept" and the JSON `pruned`/`kept_local` fields.

## Risks / Trade-offs

- **Silent loss on another machine.** A team member with an unselected local model loses it on the next import.
  Mitigation: the preview names it first, the result names it, the rollback restores it, and the guides say it in
  one line before the command example.
- **A test that pins the old default.** Several tests build a configuration with local-only models and import the
  file; each now has to say whether it keeps them, which is the point — the tests that meant "kept" get
  `--keep-local`.

## Migration

No schema change. The behavior applies from the release that carries it: the first `aigw setup --from` on a
machine removes its unused local entries. The `CHANGELOG` names it as a breaking change, and the rollout guide's
example gains `--keep-local` for readers who want the old result.
