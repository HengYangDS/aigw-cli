# Converge CI projection and rc.80 delivery

## Why

GitHub exposes native platform jobs while GitLab collapses verification into
one opaque job. Maintaining each Forge file independently would create two
authorities and recurring drift. The repository already names `0.1.0-rc.80`,
but neither Forge has published it.

## What changes

- define the CI graph, platform matrix, commands, and dependencies once;
- generate GitLab and GitHub configuration as deterministic projections;
- retain repository-owned Go commands as the executable quality authority;
- delete the handwritten CI parser, duplicated policy, and its tests;
- publish and verify `v0.1.0-rc.80` independently on both Forges.

## Out of scope

- a second task runner or heavyweight CI execution framework;
- treating cross-compilation as native acceptance;
- coupling credentials, runs, tags, releases, or availability across Forges;
- changing provider behavior, client projections, or conversation state.
