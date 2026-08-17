# Deliver the rc.85 identity repair

## Why

The GitLab `dev` history contains one unsigned commit after rc.84. Because
published branches require every reachable commit to carry trusted publication
identity, rc.85 cannot be released from that history.

## Outcome

- replace only the local linear suffix after the published GitLab rc.84 tip;
- preserve every commit tree, message, author, timestamp, order, and parent
  relationship;
- sign each replacement commit with the trusted GitLab publication identity;
- move local lifecycle refs through one immutable exact-CAS receipt.

## Boundary

This delivery changes commit identity only. It does not change AIGW behavior,
release contents, rc.84 or earlier history, GitHub history, existing tags,
Releases, client configuration, or conversation state.
