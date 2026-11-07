## Why

The owner ruled that publication peers stay independent and that no tracked content may expose the private GitLab peer: not its address, its port, its namespace, its project path or its numeric project ID. One public GitHub identity is allowed. The tip of `dev` still names the GitLab peer in seven files, mostly in CHANGELOG's link index. A history rebuild, prepared with ETHOS, removes the same identity from history. It starts only after the tip no longer holds it.

## What Changes

- `.ethos/release.toml` names the GitLab peer by its Git remote only. Each clone keeps the address in its local `remote.gitlab.forgeRepository`, which ETHOS reads first.
- CHANGELOG keeps only the GitHub compare links.
- Two archived task records cite GitLab pipelines by number, without a link.
- Tests that need a private forge use fixture identities. The mirror test refuses any IPv4 literal or numeric project path instead of naming the operator's values.

**Authority boundary:** tracked content only. Local Git configuration, the GitLab project and its CI stay as they are.

**Non-goals:** the history rebuild, the old Release assets and the proxy's one test line. Each has its own owner and step.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `repository-organization`: tracked content names no publication peer other than the public GitHub repository.

## Impact

Seven files change, and no product behavior does.
