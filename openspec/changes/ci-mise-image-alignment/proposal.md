## Why

The official mise container currently trails the repository's minimum runtime:
the image contains 2026.8.3 while `mise.toml` requires 2026.8.5. GitLab Linux
jobs therefore stop before the locked toolchain can run.

## What Changes

- Refresh the official mise runtime inside each GitLab container job before
  executing repository commands.
- Keep `mise.toml` as the only repository toolchain version authority.
- Generate the GitLab projection from the existing CUE model and cover the
  bootstrap ordering with the existing projection test suite.

## Impact

- **Authority:** `.config/ci/pipeline.cue` remains the CI topology SSOT.
- **Reuse:** use mise's public `self-update` command; add no installer or image.
- **Breaking changes:** none.
- **Non-goals:** Windows runner host repair and release publication remain
  delivery operations outside this source fix.
