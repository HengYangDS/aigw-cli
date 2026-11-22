## Why

The owner read `aigw model list` on his workstation on 2026-11-22 and found 128 models, 30 of them families that
the shipped team file no longer names — `gpt-6-astra` beside `gpt-6.2-astra`, `gpt-6.1-sol` beside `gpt-6.2-sol`,
`qwen3.8-max` beside `qwen4-max`, and previews beside their released models. His words: 「本机模型列表有很多冗余啊，比如有 gpt
6.2 的情况下，还保留对应的低版本干嘛？这是 bug 么？」and「导入 team profile 后应该只有遵循范式的配置方式啊」.

Nothing was broken: the accepted specification kept every local-only model unless `--prune` was given, so a team
member who applied the file got the file's models plus every older model the machine had ever added. He asked
for that default to change, and chose the form on 2026-11-22 about 17:2x: 「按照你建议的方式」 — importing a team
file removes the local-only models that nothing uses, and an explicit option keeps them instead.

## What Changes

- **BREAKING:** `aigw setup --from` removes the local-only models that no client selects and no remaining
  recommendation names, and the families that no remaining model uses, in the same transaction as the merge.
- **BREAKING:** `--prune` is removed, not deprecated; a flag that no longer means anything is worse than none.
  `--keep-local` is added and keeps the local-only entries, which is what `--from` did before this Change.
- The preview and the result report what the import removed, always, and what it kept when `--keep-local` is
  given.
- The shipped team file and its documentation say what an import now leaves.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `progressive-team-onboarding`: a team file's import keeps only what the file and the clients' selections and
  recommendations name; local-only entries stay on request.

## Impact

`internal/domain/team_merge.go` (the merge takes the keep-local choice instead of a prune switch),
`internal/cli/setup/setup.go`, `team.go` and `team_report.go` (the flag, the request, the report),
`cmd/aigw/testdata/script/setup.txtar`, the domain and setup tests, `docs/concepts/product-concepts.md` and
`docs/guides/team-rollout.md`. No configuration schema change. A machine that applies a team file loses its
unused local models; `aigw config rollback --last-change` restores them, and every client's selection, every
provider's endpoints and every Token stay untouched. Out of scope: changing what a team file may declare, and
pruning anything that a selection or a recommendation names.

## Authority boundary

The owner set the default and its option on 2026-11-22 (「按照你建议的方式」, after my recommendation that the
import prune by default and offer `--keep-local`). The accepted specification owns the merge's behavior; this
Change replaces the sentence that kept local-only entries and the scenario that spelled `--prune`.

## Non-goals

- Not touching `aigw sync`, `aigw use`, `aigw model add` or any other command's effect on local entries.
- Not pruning a model that a client selects, that a recommendation names, or that the file declares.
- No compatibility shim for `--prune`: an unknown flag is refused by the command surface as any other.
