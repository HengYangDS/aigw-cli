# Design

## Context

`retireClients` refuses a supply that a live process still uses. On Linux and
macOS it lists processes with `ps` and looks for command lines that name a path
inside the supply. On Windows a runtime check skips that inventory. The skipped
branch and `supplyProcesses` still compile into the Windows build, so the
Windows native job counts their 15 statements as unexecuted.

The repository already splits host-specific tooling by build constraint:
`tools/ci/execution_unix.go` builds with `//go:build !windows`, and
`tools/ci/execution_windows.go` holds the Windows counterpart.

## Goals / Non-Goals

**Goals:** each host builds the retirement check that it runs, and the Windows
coverage floor holds with the per-host supply in place.

**Non-Goals:** changing what retirement checks on any host, adding a Windows
process inventory, or changing the coverage floor or its gate.

## Decisions

### Split the inventory by build constraint, not by a runtime check

`supplyInUse` is defined once per host family, and retirement calls it on every
host. The POSIX file holds the `ps` inventory and its refusals unchanged. The
Windows file makes no inventory and refuses nothing, as the runtime check did.

Excluding the file from coverage, or a lower floor for Windows, was rejected.
The floor measures the code that each host builds; the defect was building
POSIX-only code for Windows, not the measurement.

### Cover the refusals that no host reached

A client without an official distribution and a Hermes supply whose private
home already exists are refusals that no test reached on any host. Each gets a
portable test, so every host runs it, and each test fails when its refusal is
removed.

## Risks / Trade-offs

- **The Windows margin stays narrow.** The PR's Windows profile predicts 95.05%,
  above the 95.03% of the base. A later change with Windows-only gaps can cross
  the floor again; the hosted Windows job remains the authority.

## Migration Plan

Publish the landed head to the native-client supply proposal, so the checks of
PR 207 run again.

## Open Questions

- None.
