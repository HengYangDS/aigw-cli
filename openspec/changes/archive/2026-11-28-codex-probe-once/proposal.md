## Why

A command that plans and applies a Codex projection starts the same Codex program several times for one answer.
Codex's plan and its apply each prepare every Codex target, and each preparation reads the bundled model table by
starting the program twice, once with `--version` and once with `debug models --bundled`; a local check whose table
producer differs starts it twice more. Each start of an unchanged program prints the same thing.

In the repository's test processes the repetition dominates the selection matrix, a part of the static gate that every
proof waits for. On 2026-11-28 the matrix took 608 s with 5,207 s of system time; a profile put 92% of its mutex delay
on the fork lock, which a test process of 1 to 1.5 GB holds while it starts a probe. The same matrix with the probe
reused for an unchanged program, in an experiment that keyed the program by its file's metadata, took 267 s with
354 s of system time (`build/runtime/work/static-gate-20261128/` in the main checkout). For an operator the repetition costs a few program starts per command, and lets the plan and the apply
of one command act on different answers when the program's output changes between them.

## What Changes

- Within one AIGW process, the reader of Codex's bundled model table starts an unchanged Codex program once and gives
  every later read the same identity, table and failure. A program is unchanged while its configured path, the file
  that the path resolves to, the file's bytes and the version that its package manifest declares stay the same; a
  change of any of them starts the program again.
- Concurrent reads of one unchanged program wait for its one probe.
- The repository's verification of the Codex catalog projection and the release tools keep reading the client afresh.

Not breaking: no configuration, command, output or file changes. A command starts each unchanged Codex program at most
twice, one `--version` and one `debug models --bundled`, instead of twice for each preparation of each Codex target.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `client-programs`: an operation reads an unchanged Codex program once per process, and a changed one again.

## Impact

`internal/adapter/codex` (the bundled catalog reader that the adapter calls), its tests, `CHANGELOG.md` and the
accepted specification above.

Authority boundary: AIGW owns only its probe home and its reading of the program, as before; it writes nothing new and
keeps nothing beyond the process. Reuse: the program identity that AIGW already records, the SHA-256 of the program
file and the version that its package manifest declares, is the key; no second identity rule is added. Non-goals: a
cache on disk or across processes; the probes of `ReadEffectiveCatalog` and of a client-depth check's turn; the layout
of the selection matrix and its overlap with other gates, which a later Change takes up after the ETHOS rebind.
