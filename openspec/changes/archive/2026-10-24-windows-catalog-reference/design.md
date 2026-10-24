# Design

## Context

`rebaseUserCatalogReference` in `internal/codex/catalog.go` rewrites a relative
user-owned `model_catalog_json` in a verification copy to
`filepath.Join(<configuration directory>, <reference>)`. On Windows,
`filepath.IsAbs` requires a drive with a root or a UNC prefix, so `\x` and `C:x`
both reach that join, and Go's `filepath.Join` appends them to the directory.
Rust's `Path::join`, which Codex uses to resolve the reference, keeps only the
base's prefix for a rooted path and returns a path that has a prefix but no
root unchanged.

## Goals / Non-Goals

**Goals:** the verification copy names the file that the operator's Codex
resolves, for every reference shape, on every platform.

**Non-Goals:** `~`, profile or command-line references; other Windows path
semantics; running the real Codex on Windows with such references.

## Decisions

### Platform files own reference resolution

`resolveCatalogReference(dir, reference)` lives in `catalog_reference_unix.go`,
which joins the directory, and in `catalog_reference_windows.go`, which keeps a
drive-relative reference, gives a rooted one the directory's volume and joins
any other. Platform files keep code that one platform cannot reach out of the
other platforms' coverage, as the repository's coverage policy prefers. A
shared function with a `runtime.GOOS` branch was rejected for that reason.

### A drive-relative reference stays unchanged

Windows resolves `D:x` from the process's current directory on drive `D:`, which
the configuration does not record. AIGW therefore cannot name that file by an
absolute path, and the copy keeps the value. Whether Codex resolves it to the
same file inside the private home is unproved.

## Risks / Trade-offs

- [Codex resolves a rooted Windows reference differently from Rust's documented
  `Path::join`] → the copy names another file and verification fails there,
  which no real Codex run on Windows has ruled out yet.
- [A drive-relative reference depends on the process's current directory on
  its drive] → the copy keeps it, and verification may read another file.

## Migration Plan

0.4.16 is not released. After the archive, the 0.4.16 release continues from
this lane's head. The hosted Native Windows acceptance of its pull request runs
the new Windows tests before `dev` moves. Release, notarization, the Homebrew
cutover and client verification then follow the 0.4.16 plan.

## Open Questions

- None.
