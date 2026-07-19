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
- **User-visible evidence:** a reply in the original failing Codex conversation
  is distinct from configuration or HTTP health.

Do not claim historical-session recovery merely because a new thread works or
because a config file is syntactically valid. Keep 429, 477, and upstream SSE
failures separately classified from configuration and payload-schema defects.

For a loopback endpoint, AIGW can show only that an external compatibility
layer is selected and that the client route uses its listener. This is an
endpoint classification, not proof that a listener is running or that the
transport is healthy; those claims require the transport owner's manifest and
service evidence.
