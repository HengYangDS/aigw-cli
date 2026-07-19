# Evidence Policy

Status: canonical.

Each claim must name its scope, verifier, evidence, and limit.

## Local Git-object housekeeping

Signed release tags, reachable commits, active worktrees, and tracked evidence are
never cleanup targets. After ref and worktree inventory is recorded, a canonical
checkout may remove only objects reported by `git prune --dry-run --verbose`.
Record the pre/post ref sets, dry-run object counts, final `git fsck --full`,
Git version, exact command, and the limit that this proves local object-database
hygiene only. It neither changes forge history nor proves remote publication.

- **Projection evidence:** dry-run plan, all-target transaction tests,
  byte-exact rollback tests, and `aigw doctor` validation.
- **Transport evidence:** proxy manifest, verified listener identity, and its
  own service tests; these are outside AIGW lifecycle ownership.
- **Authentication-stability evidence:** `aigw check` treats an initial 401 as
  transient only after three healthy recovery observations, and treats it as a
  persistent invalid Token only after three further 401 responses. Mixed or
  canceled recovery is retryable instability and cannot recommend rotation.
- **User-visible evidence:** a reply in the original failing Codex conversation
  is distinct from configuration or HTTP health.

Do not claim historical-session recovery merely because a new thread works or
because a config file is syntactically valid. Keep 429, 477, and upstream SSE
failures separately classified from configuration and payload-schema defects.

Authentication stability is scoped to the same configured endpoint and the
same in-memory Token during one bounded command. It is not a direct-upstream
probe, account or billing proof, transport-health proof, or guarantee about a
later request. Only bounded redacted detail may be rendered; raw response
bodies and credentials are not persisted, and no credential or configuration
mutation is evidence collection.

For a loopback endpoint, AIGW can show only that an external compatibility
layer is selected and that the client route uses its listener. This is an
endpoint classification, not proof that a listener is running or that the
transport is healthy; those claims require the transport owner's manifest and
service evidence.
