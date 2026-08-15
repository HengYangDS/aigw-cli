## Decision

Use the repository's existing GitLab Generic Package Registry as the immutable
bootstrap distribution surface. It is local to the GitLab publication plane,
requires no new service, and is available to jobs through `CI_JOB_TOKEN`.

The CUE model owns one hidden `.linux-toolchain` job with:

1. the digest-pinned upstream mise image as a minimal executor;
2. one `before_script` that reads `min_version` from `mise.toml`;
3. an architecture-to-SHA-256 mapping for the mirrored official binaries;
4. an authenticated package download from the current GitLab project; and
5. checksum verification before installing the executable.

`source-and-governance`, `native-linux`, and `release-readiness` inherit this
template. Their `script` arrays contain only product commands. GitHub keeps its
own hosted action projection and never consumes the GitLab package.

## Rejected alternatives

| Alternative | Reason |
| --- | --- |
| More curl retries against GitHub | Keeps Forge coupling and duplicated failure logic. |
| Lower `min_version` to the image runtime | Violates latest-stable policy. |
| Add an AIGW container image | The existing Generic Package Registry carries the two verified binaries without introducing another build and publication surface. |
| Commit mise binaries into Git | Pollutes source history with replaceable release assets. |

## Proof

- Official release asset digests must match before mirroring.
- Downloaded GitLab package bytes must match the same digests.
- Projection tests must prove one template, three consumers, no peer-Forge URL,
  no duplicated bootstrap body, and exact generated YAML.
- The full source and exact-HEAD proof gates remain mandatory.
