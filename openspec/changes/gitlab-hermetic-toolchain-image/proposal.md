## Why

GitLab Linux jobs currently download mise from GitHub during every job. The
same 35 MiB bootstrap is duplicated three times and fails before repository
verification when the peer Forge is slow. The official mise image is also
behind the repository minimum, so merely deleting the bootstrap would run an
unsupported toolchain.

## What Changes

- Mirror the verified latest-stable Linux mise binaries into AIGW's GitLab
  Generic Package Registry.
- Model one hidden GitLab Linux toolchain job and let all Linux jobs inherit it.
- Read the version only from `mise.toml`, verify the architecture-specific
  package digest, and remove peer-Forge bootstrap traffic from GitLab CI.
- Regenerate `.gitlab-ci.yml` from the CUE SSOT and replace the old bootstrap
  regression with positive inheritance and provenance checks.

## Non-goals

- GitHub Actions remains an independent projection.
- Windows runner host repair and release publication are operational follow-up,
  not part of this source Change.
