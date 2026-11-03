## Context

Mise declares the developer and CI tools and locks each one by checksum for six
platforms. The repository's Renovate policy holds a new npm release for three
days, which covers npm's initial unpublish window, and admits Go modules, Mise
tools, digest-pinned images and commit-pinned Actions without that delay. The
native client pins follow the npm age, because the clients ship as npm
packages.

## Goals / Non-Goals

**Goals:** every supply input on the latest stable release that the policy
admits, with each digest or checksum read from its publisher.

**Non-Goals:** a release, a change to the AIGW program, or overriding the
release age.

## Decisions

- The refresh lands the updates that the policy admits now and takes the two
  held npm releases when their ages end, in a later lane of this Change;
  admitted updates do not wait for them.
- The Renovate image's index digest is read from ghcr.io and matches the
  candidate calculation's; `mise lock` reads glab's checksums from its GitLab
  release for every locked platform.
- The npm lock moves within the declared ranges with npm's `--before` set three
  days back, so a transitive release inside the age stays out, and the
  registry signatures of every package are verified.
- The supply-chain specification states which sources skip the release age, as
  the Renovate policy already does, so a fresh digest-pinned image or
  checksum-locked tool is not held by mistake.

## Risks / Trade-offs

- glab and the Renovate image were published hours before the refresh. Their
  locks bind the publisher's checksum and digest, and the hosted graph and the
  GitLab pipelines exercise them before the forges integrate them.
