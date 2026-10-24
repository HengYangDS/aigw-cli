# Windows Catalog Reference

## Why

Hosted Native Windows acceptance of the AIGW 0.4.16 pull request failed
`TestCodexVerificationUsesAnIsolatedProjectionForAnUnselectedRoute`. The
verification copy that 0.4.16 introduces treats every `model_catalog_json`
value that `filepath.IsAbs` rejects as relative, and joins it with the
directory of the configuration that names it. On Windows, a reference rooted
without a drive, such as `\catalogs\models.json`, is not absolute, so the copy
named a file inside that directory. Codex resolves the reference with Rust's
`Path::join`, which keeps only the directory's drive for such a path, so the
copy and the operator's Codex would read different files.

## What Changes

- On Windows, the verification copy gives a rooted user catalog reference the
  drive of the configuration's directory, and keeps a reference that names a
  drive without a root, such as `D:models.json`, unchanged. Plain relative
  references and every other platform keep the 0.4.16 behavior.
- The projection-preservation test names a catalog reference that is absolute
  on every platform, and Windows unit tests cover each reference shape.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `route-client-selection`: "Connectivity and verification use explicit Route
  semantics" gains a scenario for a rooted Windows catalog reference.

## Impact

Only Windows verification copies of a rooted or drive-relative user catalog
reference change. No command, flag, output, credential or operator file changes.
0.4.16 is not released yet, so the fix ships in it.
