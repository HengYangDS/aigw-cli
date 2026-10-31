## Why

Each complete local verification leaves twelve `aigw-git-*` directories and one
`testscript-main*` directory, which holds two 12.7 MB copies of the test
program, in the temporary directory. By 2026-10-31 the operator Mac held more
than 1,300 of them, 2.8 GB. Tests that run their own test binary again as a
helper process repeat the setup of the binary's test entry point, and the
helper exits inside a test, so the removal that follows the tests never runs.

## What Changes

- A helper process inherits the Git isolation of the test process that started
  it instead of creating its own, so a helper that exits inside a test leaves
  no directory behind.
- The program's entry-point test runs the program through the command that
  the acceptance-script harness installs, which the harness dispatches by name
  to the entry point without its setup.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `repository-organization`: local verification leaves no temporary directory
  of its own behind, including when a helper process exits inside a test.

## Impact

Tests change; no product code, release artifact, configuration or credential
changes, so no release follows. Out of scope: a process that the host
terminates, which no test can clean up after, and other repositories' tests.
