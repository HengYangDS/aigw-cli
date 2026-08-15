## Decision

Use one source-specific GitLab template. Mise installs only Go, Node, and
OpenSpec. CUE, actionlint, and lychee come from the current project's existing
Generic Package Registry because their locked upstream artifacts are hosted by
the peer Forge.

The package version is the SHA-256 of `mise.lock`. Mise's URL replacement
redirects only locked GitHub release artifact URLs to that package. Mise still
owns checksum verification, extraction, and installation; CI does not duplicate
those mechanisms.

| Concern | Authority |
| --- | --- |
| Tool versions and upstream checksums | `mise.toml` and `mise.lock` |
| CI topology and job closures | `.config/ci/pipeline.cue` |
| GitLab mirror bytes | Generic Package Registry, addressed by lock digest |
| Generated workflow | `.gitlab-ci.yml` |

## Rejected

| Alternative | Reason |
| --- | --- |
| Increase mise HTTP timeout | Retains peer-Forge coupling and release-only installs. |
| Add a project container image | Creates another build and publication surface. |
| Commit binaries | Pollutes source history with replaceable assets. |
