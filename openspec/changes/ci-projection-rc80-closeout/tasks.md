# Tasks

- [x] 1.1 Define one declarative CI graph while keeping release targets, native evidence, and developer-host claims semantically distinct.
- [x] 1.2 Generate GitLab and GitHub files as deterministic projections.
- [x] 1.3 Fail source verification on projection drift or an invalid evidence graph.
- [x] 1.4 Delete the handwritten CI contract parser and duplicate policy.
- [x] 2.1 Run the complete local source and native macOS gates.
- [x] 2.2 Verify the GitLab projection with server-side lint and bind each native evidence class to a Forge-supplied runner selector.

## Delivery boundary

Exact-HEAD proof, Change archive, candidate integration, accepted-head CI,
independent Forge publication, installation, runtime acceptance, and lane
retirement are post-Change lifecycle effects. Public ETHOS receipts and the
active delivery Goal own those facts; duplicating them as pre-archive tasks
would make this Change depend on its own archive operation.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `product-quality:one complete quality graph` | `1.1` | `.config/ci/pipeline.cue`; CUE validation |
| `product-quality:one complete quality graph` | `1.2` | exact generation of `.gitlab-ci.yml` and both GitHub workflows |
| `product-quality:one complete quality graph` | `1.3` | `tools/ci project --check`; focused drift tests |
| `product-quality:one complete quality graph` | `1.4` | deleted `verify-gates.toml` and handwritten contract parser |
| `product-quality:one complete quality graph` | `2.1` | full source graph and native macOS install lifecycle |
| `product-quality:one complete quality graph` | `2.2` | GitLab CI lint; Forge-supplied runner selectors |
| `product-quality:complete delivery evidence` | post-Change lifecycle | exact-HEAD, hosted CI, release, install, runtime, and retirement receipts |
