## Why

The owner chose on 2026-11-22 at 19:43 to land `client-chosen-model` with two root causes open and to fix each in its
own Change from `dev`, with a fresh review, releasing nothing until both land. This is the first: whether a command
keeps or drops one of a client's files is decided in two ways. Four places compare the files' paths as strings
(`admitTargets` in `aigw client edit`, `withdrawnClients` in synchronization, Codex's `reconciliationInputs` and
`hermesPlans`), while Codex's projection compares the files themselves (`codexTargetUnion` through `sameTargetFile`).
A file named by a symbolic link in one configuration and by its resolved path in the other is dropped by the first
judgement and kept by the second. The twelfth review of `client-chosen-model` found two results: a rollback without
the Token strips AIGW's settings and record from the file that Codex still uses, erasing a pick there (its finding 1),
and `aigw client edit --target` of that file in another spelling succeeds or is refused by whether the Token is stored
(its finding 4).

## What Changes

- One judgement of whether two configured paths name one client file, owned in one place and used by every caller:
  paths that both exist are one file when the operating system reports one file, which covers a symbolic link and a
  volume that ignores case; otherwise they are one file when their cleaned spellings are equal. This is the rule that
  Codex's projection applies today.
- `aigw client edit`, synchronization's choice of the clients whose files are withdrawn, Codex's reconciliation and
  Hermes's plans decide which files a command keeps and which it drops by that judgement, so a file in another
  spelling is kept, with the Token and without it.
- What AIGW writes does not change, and a target list that names one file twice is refused as before.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `client-chosen-model`: one requirement is added, that a client's file is one file however its path is spelled.

## Impact

`internal/platform` (the judgement's one owner), `internal/cli/client`, `internal/app/synchronization`,
`internal/adapter/codex`, `internal/adapter/hermes` and their tests. No configuration, schema or report field changes;
not a breaking change.
