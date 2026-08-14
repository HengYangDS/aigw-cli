# Design

## Authority

`.config/ci/pipeline.cue` is the sole CI topology authority. It owns logical
jobs, platform availability, dependencies, repository commands, and release
stages. CUE is locked through `mise` and is used only for validation and
deterministic rendering; it does not execute product tests.

The existing `tools/ci` command remains the sole owner of executable quality
behavior. Generated `.gitlab-ci.yml` and `.github/workflows/*.yml` files are
thin Forge-native projections required by the hosting platforms.

## Projection

```text
.config/ci/pipeline.cue
          │
          ├── .gitlab-ci.yml
          ├── .github/workflows/verify.yml
          └── .github/workflows/release.yml
```

The source gate validates the CUE model, renders all outputs to a temporary
directory, and compares exact bytes with tracked projections. Any manual edit
to a projection fails before expensive tests. Generation and verification use
the same public repository command; no parser or parallel policy remains.

## Portability model

The model keeps four meanings separate:

| Surface | Meaning | Proof |
| --- | --- | --- |
| Product targets | Final user-facing OS and architecture support | Installed release behavior |
| Release assets | Signed archives produced for each product target | Manifest, checksum, and signature |
| Native acceptance | Real execution on an operating system and architecture | Native CI result |
| Developer hosts | Hosts able to build and run repository gates | Documented DX workflow |

Cross-compilation proves only that an asset can be produced. It never proves
native execution or developer-host compatibility. GitLab and GitHub may use
different runner architectures while projecting the same logical evidence
graph. Each Forge remains an independent publication plane.

## Verification

1. Validate the CUE model and exact projections.
2. Run the complete local source graph and native macOS acceptance.
3. Execute exact-HEAD proof and integrate the owner lane.
4. Require available native jobs independently on each Forge.
5. Publish signed rc.80 tags and immutable assets independently.
