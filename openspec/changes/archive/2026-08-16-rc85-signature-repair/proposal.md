# Repair the rc.85 release identity

## Why

The unpublished rc.85 suffix contains two ETHOS-generated materialization
commits without a trusted GitLab signature. Every commit reachable from a
published AIGW release must carry the publication identity required by the
target Forge.

## What changes

- rebuild only the linear suffix after the published rc.84 GitLab commit;
- sign every replacement commit with the trusted GitLab release identity;
- preserve each commit's tree, message, author, timestamps, order, and parent
  topology;
- move local lifecycle refs only through one immutable, exact-CAS receipt.

## Boundaries

This change does not alter product source, provider or client behavior, release
contents, rc.84 or earlier history, GitHub history, remote refs, tags, Releases,
Codex conversation state, or JetBrains state. GitHub remains an independent
Forge projection after the repaired GitLab source history is accepted.
