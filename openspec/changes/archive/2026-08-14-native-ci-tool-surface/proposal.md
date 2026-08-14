## Why

GitLab Linux starts from an immutable mise image whose embedded binary is older
than the repository declaration. Runtime self-update then queries the anonymous
GitHub API and fails when the shared runner exhausts its quota.

## What Changes

- Install the exact mise version declared by `mise.toml` from its official
  release asset without release discovery.
- Limit native jobs to the Go toolchain they execute.
- Keep both Forge workflows generated from the existing CUE model.

## Impact

- **Authority:** `mise.toml` remains the only version owner.
- **Reliability:** GitLab bootstrap no longer depends on GitHub API quota.
- **Surface:** one small Linux-container bootstrap replaces runtime self-update;
  no product command or compatibility path is added.
- **Non-goals:** Windows runner host repair and release publication.
