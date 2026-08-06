# rc80 terminal forward fix

## Why

The rc.80 candidate passed local product checks but failed hosted Windows and
Linux verification. Its setup documentation also omitted the small public
environment contract used by automation and the read-only secret backend.

## What changes

- make the Windows Claude launcher fixture exercise the intended unreadable
  path contract;
- raise `internal/codex` package coverage above the strict 95% floor;
- keep portable installation independent of a pre-existing user profile;
- document only environment variables that are part of AIGW's public product
  behavior.
- refresh applicable stable Go dependencies and verify the complete repository
  against the resulting module graph.

## Out of scope

- adding clients, providers, or compatibility layers;
- changing Proxy runtime behavior;
- changing Forge identity or release policy.
