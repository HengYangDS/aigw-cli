## Context

ETHOS v0.2.0-beta.3 shipped `peer-address-boundary`. It resolves a peer's repository from the clone's local `remote.<name>.forgeRepository` before tracked configuration, so `forge_repository` can leave `.ethos/release.toml`. This clone already sets the key for `gitlab`.

## Goals / Non-Goals

**Goals:**

- No tracked content names the GitLab peer's host, port, namespace, project path or project ID.
- Tests keep exercising private-forge handling with fixture values.

**Non-Goals:**

- Rewriting history, which the ETHOS rebuild does.
- Deleting old Release assets, which the owner does in the web UI from a prepared list.

## Decisions

### Fixtures instead of the operator's values

A test that needs a private forge uses an RFC 1918 fixture host or a `.invalid` host, and project ID 101. The mirror-prelude test asserted that the prelude does not contain the operator's host and project ID. It now refuses any IPv4 literal and any `projects/<digits>` path, which is stricter and names nothing.

### The link index keeps GitHub only

GitHub is the public peer. Its compare links stay, and the GitLab links go. Nothing validates the GitLab links, so no tool changes.

## Risks / Trade-offs

- A clone without `remote.gitlab.forgeRepository` can no longer publish to GitLab. That is the intended boundary: each operator configures the private peer locally.
