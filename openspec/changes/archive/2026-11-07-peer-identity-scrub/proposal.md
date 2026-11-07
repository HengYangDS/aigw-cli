## Why

The owner ruled that publication peers stay independent: a peer must not learn another's address from tracked content, and nothing may expose the private GitLab peer, whether its address, port, namespace, project path or numeric project ID. ETHOS's decided paradigm retires every tracked peer coordinate, GitHub's included: peers resolve from the clone's Git configuration, and a changelog stores no peer URL. The tip of `dev` still names peers in seven files, mostly in CHANGELOG's link index. A history rebuild, prepared with ETHOS, removes the private identity from history. It starts only after the tip no longer holds it.

## What Changes

- `.ethos/release.toml` names each peer by its Git remote only. ETHOS resolves the repository from the clone's local `remote.<name>.forgeRepository`, or else the remote's URL.
- CHANGELOG keeps its headings and text and drops every History row and compare link.
- Two archived task records cite GitLab pipelines by number, without a link.
- Tests that need a private forge use fixture identities. The mirror test refuses any IPv4 literal or numeric project path instead of naming the operator's values.

**Authority boundary:** tracked content only. Local Git configuration, the GitLab project and its CI stay as they are.

**Non-goals:** the history rebuild, the old Release assets, the proxy's one test line, and public-distribution references to GitHub such as install instructions and package metadata, whose scope ETHOS's `peer-neutral-outputs` decides. Each has its own owner and step.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `repository-organization`: the changelog and the publication declaration name no peer, and tracked content never names the private peer.

## Impact

Seven files change, and no product behavior does.
