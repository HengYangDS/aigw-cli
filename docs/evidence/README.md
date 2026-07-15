# Evidence Policy

Status: canonical.

Each claim must name its scope, verifier, evidence, and limit.

- **Projection evidence:** dry-run plan, all-target transaction tests,
  byte-exact rollback tests, and `aigw doctor` validation.
- **Transport evidence:** proxy manifest, verified listener identity, and its
  own service tests; these are outside AIGW lifecycle ownership.
- **User-visible evidence:** a reply in the original failing Codex conversation
  is distinct from configuration or HTTP health.

Do not claim historical-session recovery merely because a new thread works or
because a config file is syntactically valid. Keep 429, 477, and upstream SSE
failures separately classified from configuration and payload-schema defects.

For a loopback endpoint, AIGW can show only that an external compatibility
layer is selected. This is an endpoint classification, not proof that a
listener is running or that the transport is healthy; those claims require the
transport owner's manifest and service evidence.
