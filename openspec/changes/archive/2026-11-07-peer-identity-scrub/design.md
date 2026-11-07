## Context

ETHOS v0.2.0-beta.3 shipped `peer-address-boundary`. It resolves a peer's repository from the clone's local `remote.<name>.forgeRepository`, or else the remote's URL, before tracked configuration, so `forge_repository` can leave `.ethos/release.toml`. ETHOS's `peer-resolution` retires the field for every peer, and its loader will refuse it.

## Goals / Non-Goals

**Goals:**

- No tracked content names the GitLab peer's host, port, namespace, project path or project ID.
- The changelog and the publication declaration name no peer at all.
- Tests keep exercising private-forge handling with fixture values.

**Non-Goals:**

- Rewriting history, which the ETHOS rebuild does.
- Deleting old Release assets, which the owner does in the web UI from a prepared list.

## Decisions

### Fixtures instead of the operator's values

A test that needs a private forge uses an RFC 1918 fixture host or a `.invalid` host, and project ID 101. The mirror-prelude test asserted that the prelude does not contain the operator's host and project ID. It now refuses any IPv4 literal and any `projects/<digits>` path, which is stricter and names nothing.

### The changelog stores no peer link

A comparison link names a peer's repository, so a peer's copy of the changelog would expose the other peer. Every History row and compare link goes, and the headings and text stay. Release-time links belong to ETHOS's `peer-neutral-outputs`. Nothing in aigw reads the History rows or `forge_repository`, so no tool changes.

## Risks / Trade-offs

- A clone without `remote.gitlab.forgeRepository` can no longer publish to GitLab. That is the intended boundary: each operator configures the private peer locally.
