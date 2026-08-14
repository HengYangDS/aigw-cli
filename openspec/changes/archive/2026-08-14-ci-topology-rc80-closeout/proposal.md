# CI topology and RC 80 closeout

## Why

The accepted CI graph now exposes the intended macOS, Linux, and Windows
native gates on both Forges, but the first GitLab execution revealed three
environment defects: the mise container entrypoint consumed the runner shell
arguments, one unit test inherited Forge variables from its host, and the
Windows runner selected a shell absent from its host. These are delivery
defects, not reasons to weaken or remove native verification.

## What changes

- Project GitLab container jobs with an explicit empty image entrypoint.
- Make CI unit tests hermetic with respect to Forge-specific variables.
- Restore the registered Windows runner's native shell/toolchain outside the
  repository without encoding host identity in source.
- Prove the complete local graph, land one source SHA, and publish RC 80
  independently to GitLab and GitHub.

## Out of scope

- Adding another CI authority, workflow generator, or compatibility layer.
- Treating cross-compilation or a container as native macOS or Windows proof.
- Persisting runner names, host paths, credentials, or personal identity in the
  repository.
