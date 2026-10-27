## Why

The native-client supply proposal
([HengYangDS/aigw-cli#207](https://github.com/HengYangDS/aigw-cli/pull/207))
failed its Native Windows acceptance job: statement coverage was 94.96% (16418
of 17290 statements) against the 95% floor. Linux and macOS passed. The supply
added 17 statements that no Windows test runs, all in
`tools/ci/native_clients.go`:

- 15 are the POSIX process inventory of native client retirement. Windows
  builds it but never runs it, because retirement checks for processes only on
  Linux and macOS.
- 2 are refusals that no test on any host reaches: a client without an official
  distribution, and a Hermes supply whose private home already exists.

The local check measures macOS coverage, and the dispatched journey runs
measure none, so only the hosted Windows job saw the gap.

## What Changes

- The POSIX process inventory moves into `tools/ci/native_clients_unix.go`,
  which builds only for Linux and macOS. `tools/ci/native_clients_windows.go`
  holds the Windows counterpart, which makes no inventory. Retirement behaves
  as before on every host.
- The retirement test that starts a real process inside the supply moves into
  the POSIX-only test file.
- Two portable tests cover the refusal of a client without an official
  distribution and of a Hermes supply whose private home already exists.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-quality`: tooling that only one family of hosts runs builds only for
  those hosts, so each native job's coverage counts the code that its host runs.

## Impact

Only `tools/ci` changes. No product, pipeline or dependency behavior changes.
From the Windows coverage profile of PR 207, the expected Windows coverage is
16421 of 17277 statements (95.05%); the base `8de42bbf` had 95.03%.
