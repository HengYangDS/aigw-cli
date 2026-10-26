# Design

## Context

The Go command treats every directory below the module root as a possible
package, except directories named `testdata` and those whose names start with
`.` or `_`. `build/` is ignored by Git but present in the canonical checkout.
That checkout keeps retained evidence there, including captured Go source.

ETHOS's static analysis runs `go vet ./...` in the checkout it proves, using
`GOTOOLCHAIN=local` and `GOPROXY=off`. In that same failed proof, the
repository's own quality steps passed, so only this `./...` pattern reached
`build/`.

Mise declares the developer and CI tools, and `mise lock` owns their lock
entries. The CUE authority `.config/ci/pipeline.cue` pins GitHub Actions by
commit, and the projector generates both Forge files from it.

## Goals / Non-Goals

**Goals:**

- Make every Go package pattern in every checkout match product and tool
  source only.
- Take the tool updates that the candidate calculation admits.

**Non-Goals:**

- Moving, renaming or deleting retained evidence. Other holders own some of
  it.
- The Mise image and its Windows runner.
- The Windows native client pins.
- Any change to AIGW programs, credentials, configuration or client
  projections.

## Decisions

### The module declares its own boundary

`go.mod` gains `ignore ./build`. Go 1.25 introduced the directive, and the
locked Go 1.27.3 honors it. It removes `build/` from `./...` and `all` for
every consumer: vet, list, test, coverage and golangci-lint.

Two alternatives were rejected:

- Renaming evidence files in place: other sessions' retained evidence would
  have to change, and the defect would return with the next capture.
- Narrowing ETHOS's pattern: that would patch the governor for a fact the
  module owns.

### Tools advance through their declarations

Each tool moves in its single declaration: gh and the Renovate image in Mise,
mise-action in the CUE authority. Then:

- `mise lock gh` regenerates gh's entries for all six locked platforms.
- `tools/ci project` regenerates the GitHub workflows.

The Renovate image is pinned by its index digest
`sha256:4f5a2621afd6629fda621e5c66221d1000f0ef3330ec14c828d7e8d357f03522`, read
from `ghcr.io` with `docker buildx imagetools inspect`. mise-action v5.2.3 is
pinned by commit `82a612c5ab8bcda7cc5db6b7e81a8e7584c3dc6e`, the target of the
upstream `v5.2.3` tag.

### The Mise image waits for its runner

`miseVersion` comes from the Mise image tag, and the GitLab Windows job refuses
when its runner-owned Mise has a different version. The GitLab catch-up waiting
for GitLab to become reachable publishes tags whose pipelines require
2026.10.15.

Under "A runner-owned tool advances with its declaration", the runner must be
replaced before any job consumes a new declaration. The image therefore moves
in the batch that replaces the runner's Mise, after the catch-up.

## Risks / Trade-offs

- **A package under `build/` might become a real dependency.** `build/` is
  generated output by the repository artifact contract, and the source size
  and architecture gates do not admit Go packages there.
- **A tool release might change behavior.** The dependency tasks,
  `mise run check` and the full proof run before integration, and the GitHub
  native jobs run on every platform.

## Migration Plan

Mise installs the new tools on the next bootstrap of each checkout and runner.
Nothing migrates, and installed AIGW is unaffected. After this lands, the
canonical checkout proves in place under alpha.14.

## Open Questions

- None.
