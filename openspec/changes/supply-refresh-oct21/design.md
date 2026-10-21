# Design

## Context

Go is compiled into every AIGW program. Three places name its version: the
Mise declaration in `.config/mise/config.toml`, the `go` directive in `go.mod`
and the standard-library license override in
`.config/checks/dependencies/policy.toml`. OSV scans the standard library at the
`go.mod` version, and release construction requires every program in the
release SBOM to carry exactly that scanned version. The other tools are Mise
declarations whose lock entries `mise lock` owns.

## Goals / Non-Goals

**Goals:** build and release AIGW with a Go standard library outside the
vulnerable range of GO-2026-6809, and take the other admissible stable updates
of the same candidate calculation.

**Non-Goals:** Mise and the Windows native client pins, and any change to
credentials, configuration or client projections.

## Decisions

### Go advances everywhere it is named

The Mise declaration, the `go.mod` directive and the standard-library override
move to 1.27.3 together, so the build toolchain, the scanned standard library
and every program in the release SBOM name one version.

### Held updates keep their owners and next actions

Mise 2026.10.17 is pinned by executable digest on the GitLab Windows runner,
which must be replaced in a window without jobs and accepted by GitLab before a
job consumes the declaration; GitLab is unreachable from this workstation, so
the update waits for the GitLab catch-up. Codex 0.163.0 and Claude Code 2.1.302
change the qualified native client set and need the Windows native client
journey before admission; they follow in their own Change.

### The fix ships as AIGW 0.4.10

Released programs embed the standard library, so the fix reaches users only
through a new release built from this lock. The release follows the
established tag, build, notarization, publication and installed cutover steps
after this Change lands.

## Risks / Trade-offs

- [Go 1.27.3 changes behavior beyond the advisory] → the full proof and the
  native jobs on every platform run before integration.
- [A held update stays behind] → each has a named owner and next action, and
  the next candidate calculation lists it again.

## Migration Plan

Mise installs the new tools on the next bootstrap of each checkout and runner.
Homebrew replaces installed AIGW 0.4.9 with 0.4.10; no state migrates.

## Open Questions

- None.
