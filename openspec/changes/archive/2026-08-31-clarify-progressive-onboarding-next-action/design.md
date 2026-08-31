## Context

The existing setup transaction already imports all capability while connecting
only locally available Accounts. The defect is confined to continuation text:
the environment backend emits one instruction per missing Account and points to
`check`, although `sync` owns later discovery and activation.

## Goals / Non-Goals

**Goals:**

- Make the next action truthful, singular, and executable.
- Preserve the existing progressive Account and client model.
- Derive human and JSON guidance from the same result.

**Non-Goals:**

- Add a default Account or provider priority.
- Add a new command, credential backend, or compatibility fallback.
- Change Route selection, token validation, or projection transactions.

## Decisions

### Recommend one choice, not every credential

A catalogue contains alternatives. When no Account is connected, setup will use
the first stable Account ID only as an example operand for the existing
account-scoped connection mechanism. It will explicitly state that any one
compatible Account is sufficient.

### Keep activation with synchronization

`sync` already owns rediscovery and projection. Guidance after supplying an
environment Token therefore points to `aigw sync`; `check` remains a read-only
health check for enabled Routes.

## Risks / Trade-offs

- A lexical example may not be the operator's preferred Account. The wording
  presents it as one choice, not a default or recommendation.
- Human guidance becomes two short lines while JSON retains one `next_action`;
  both still derive from the same semantic result.
