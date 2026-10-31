## Context

Two test entry points prepare shared state before their tests and remove it
after them: the Git isolation of the repository tools, and the acceptance-script
harness of the program. A test that needs a separate process often runs its
test binary again, which repeats that preparation. A helper that reports a
status through an immediate exit inside a test skips the removal.

## Goals / Non-Goals

**Goals:** a complete local verification leaves no directory of its own in the
temporary directory, and every helper keeps its behavior and exit status.

**Non-Goals:** changing how helpers exit, cleaning up after a process that the
host terminates, or changing the vendored harness.

## Decisions

- The Git isolation owner recognizes an isolation that its parent process
  created, from the two variables it sets, and reuses it. The parent removes
  its directory only after its tests, which wait for every helper. Moving each
  helper's dispatch ahead of the preparation, as the supply-interruption helper
  already does, would change eight helper sites and leave the leak one
  forgotten site away.
- The program's entry-point test runs the `aigw` command that the harness
  installs on the test's search path. The harness dispatches that command to
  the entry point without preparing anything, as it does for every acceptance
  script, so the vendored harness stays unchanged.

## Risks / Trade-offs

- A helper started with Git variables that its test replaced does not match
  the parent's isolation and creates its own, as before.
