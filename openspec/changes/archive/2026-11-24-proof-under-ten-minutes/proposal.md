## Why

aigw-cli's full proof took about 4 minutes until 2026-11-23 (the 9 passing proofs from 11-19 to 11-23 11:08 spanned
217 to 390 s) and about 27 minutes since (the 7 passing proofs from 11-23 14:05 to 11-24 09:53 spanned 1,194 to
1,775 s). Every landing, archive
and release waits for it, and it holds a host slot that other repositories wait for. Read from the kept proof records
on 2026-11-24:

- `go-static-analysis` went from 65–83 s to 744–1,050 s. It runs `./tools/ci quality`, whose `selection-matrix` gate
  (added by `client-chosen-model`) runs the exhaustive selection tests; the eight light gates of the readiness tier
  start only when it ends.
- That gate's `-run` pattern also matches the 29 untagged `TestZZ11…` tests, which `go-behavior` runs as well: 162 s
  of serial tests run twice per proof (measured 14:17 to 14:39 in this lane).
- The matrix itself is 4,582 leaf subtests and uses every core of the host for about ten minutes. A CPU profile of one
  of its tests (`TestZZ11TheMatrixsCommandsInFurtherStatesHermes`, two workers) spends 21 % in `Config.ModelRefs`,
  called by `Config.AppendModel` for every model a configuration decodes, which sorts every model again on each append.
- `go-behavior` went from 94–123 s to 377–677 s. The acceptance package dominates (770 s of 1,724 s on hosted macOS
  on 11-24); of its 427 tests only 8 call `t.Parallel`, so it runs almost serially under the race detector.

The owner asked on 2026-11-18 for faster iteration with performance treated as part of quality; 「RW 加速与性能」
relayed on 11-24 a target of a full proof in ten minutes or less.

## What Changes

- The `selection-matrix` gate runs only the tests of its own build tag; the untagged `TestZZ11…` tests stay in
  `go-behavior` alone.
- `Config.AppendModel` keeps the display order without sorting every model again; decoding a configuration of n
  models no longer costs n sorts.
- The acceptance package's tests run in parallel unless they change process state (`t.Setenv`); a test of the package
  keeps every new test parallel or names why it is not.
- Further hot spots that a profile of the matrix names are fixed in the product where they are product cost.

No behavior of the product changes.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-quality`: a proof runs each test once, and the acceptance package's tests run in parallel unless they
  change process state.

## Impact

`tools/ci/quality.go` and its test, `internal/cli/acceptance` (the matrix's test names, `t.Parallel`, a structure
test), `internal/domain/ordering.go` and its test. Acceptance: a full proof of this lane's head held 600 s or less on
this host, measured, with the same tests and the coverage floor.
