## Context

Node.js publishes 26.13.1 as a security release of the 26 LTS line, past the repository's release age. Node runs
Prettier, the OpenSpec CLI and npm audit here; the shipped `aigw` program does not contain it.

## Goals / Non-Goals

**Goals:** take the security release without other changes, so the update lands quickly.

**Non-Goals:** Mise 2026.11.7, whose Windows runner binary is pinned by digest and needs a guest window, and the
other routine updates of the next supply refresh.

## Decisions

### Take Node alone

A narrow change keeps the review and the hosted graph focused on one supply input. The lock entries for all six
platforms come from `mise lock`, which records each official archive's checksum.

## Risks / Trade-offs

- [A patch release changes tool behavior] → the full local check and the hosted graph run the same tools.
