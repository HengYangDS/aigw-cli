## Context

See [proposal.md](proposal.md). Production already fails closed when the native
Secret Service connector cannot be created; only Linux-native evidence for that
branch is missing.

## Goals / Non-Goals

**Goal:** exercise the real Linux connection failure deterministically.

**Non-goal:** introduce a production test seam, fallback backend, prompt, or
platform abstraction.

## Decisions

Run the existing observer with `DBUS_SESSION_BUS_ADDRESS` directed at a missing
Unix socket and assert its narrowed connection error. This uses the platform's
public D-Bus contract and keeps the test Linux-only.

Rejected alternatives:

- injecting a connector factory would add production surface solely for a test;
- lowering the coverage floor would hide the unproved branch;
- mocking the keyring package would not prove the real Linux integration.

## Risks / Trade-offs

- **D-Bus client caching could mask the address override** → keep the test
  isolated from successful session-bus initialization and verify it in the same
  Linux container used to reproduce the gate.
