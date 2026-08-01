# ADR-0002: Accept Signed GitHub Provenance on the Free Private Peer

- Status: accepted
- Date: 2026-07-17
- Owner: Release maintainers

## Context

The current GitHub publication peer is private and uses a plan that does not
admit repository-ruleset tag protection for this combination. The available
alternatives are a paid plan or making the repository public; neither is part
of the current publication policy.

## Decision

Keep the repository private and do not upgrade the GitHub account for tag
rulesets. GitHub release tags are accepted as signed, independently verified
provenance records, not as host-enforced immutable refs. AIGW automation must
continue to avoid copying, overwriting, deleting, or regenerating a
provider-native release tag.

## Consequences

Release acceptance requires the current remote tag to verify against the
protected GitHub trust input and the complete GitHub artifact matrix to pass checksum
and byte-for-byte comparison with GitLab. A manual GitHub tag mutation remains
possible at the hosting layer and is treated as a detected provenance failure,
not as an impossible state. No release or host-route claim may state that the
GitHub tag is host-enforced immutable.

## Revisit Trigger

Revisit this decision only if the project requires host-enforced GitHub tag
protection, changes repository visibility, or adopts a GitHub plan that admits
private-repository rulesets.
