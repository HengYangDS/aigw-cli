# Design

## One graph, independent execution planes

`.config/ci/pipeline.cue` remains the sole CI topology model. GitLab and GitHub
workflow files are generated projections of the same logical gates; their
runner syntax may differ, but their evidence semantics do not.

| Concern | Authority | Decision |
|---|---|---|
| Logical jobs and dependencies | `.config/ci/pipeline.cue` | Keep one graph. |
| Release targets | `.config/release/goreleaser.yaml` | Do not duplicate them in CI. |
| GitLab container startup | GitLab image projection | Clear the image entrypoint so the runner-selected shell owns the script. |
| Test process environment | Test case | Explicitly remove Forge inputs unless the test is about Forge provenance. |
| Windows shell availability | Runner host | Repair the host registration/toolchain; do not encode the host in source. |

## Platform evidence

Native evidence means execution on the declared operating system. Additional
architecture archives may be cross-built and inspected, but are not described
as native runtime proof. A missing or broken required runner fails visibly; it
does not silently collapse into another job.

## Release sequence

1. Run focused projection and CI tests.
2. Run the complete local quality graph and exact-HEAD ETHOS proof.
3. Land the same tree through candidate, dev, and main.
4. Verify all GitLab and GitHub native jobs on that source SHA.
5. Create independent signed `v0.1.0-rc.80` tags and releases from that SHA.
6. Verify immutable assets, checksums, signatures, SBOMs, and installation.
