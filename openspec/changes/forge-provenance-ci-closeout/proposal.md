## Why

The complete Forge-provenance mechanism is now published, but exact-ref hosted
CI exposed two portability defects: GitHub passes allowed-signers content where
the checker requires a path, and the architecture policy delegates portable
path syntax to the runner operating system. The release boundary must project
trust content into a temporary file and validate repository-relative paths with
the same grammar on macOS, Linux, and Windows.

## What Changes

- Project the GitHub allowed-signers variable into a runner-temporary file
  before invoking any checker that requires a file path.
- Apply the same projection to Verify and Release jobs without logging trust
  material or introducing repository-local trust files.
- Replace host-dependent absolute-path detection with a provider-neutral path
  grammar that rejects POSIX roots, Windows drive/UNC/device roots,
  backslashes, and parent traversal on every operating system.
- Add contract and unit tests that fail on the current hosted-CI defects.
- Restore `~/.codex/config.toml` as the shared Codex Home target for CLI and
  Desktop, keep Desktop-only GUI state outside AIGW, and leave absent clients
  untouched.
- Keep Claude Code and Codex as the current admitted clients; Hermes and future
  clients require independent Adapter admission.
- Advance the repository to the current stable Go dependency graph and immutable
  latest-stable GitHub Actions revisions; keep `go.mod` and the CI gate policy
  as their respective owners rather than preserving obsolete versions.
- Keep GitLab's file-variable contract and all publication actors, trust
  anchors, keys, remote coordinates, and recovery paths outside product
  defaults.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: subject=AIGW hosted verification portability;
  reuse=extend; change=modify; define ephemeral trust-input projection and a
  host-independent repository-relative path grammar;
  facet:lifecycle=verification,release;
  facet:surface=ci,architecture-policy;
  facet:authority=source,publication-context.

## Out of Scope

- Rewriting Codex JSONL, SQLite, historical messages, Responses item records,
  conversation models, or model metadata.
- Storing an allowed-signers file in the repository or logging its contents.
- Encoding an operator identity, machine path, key, fingerprint, or Forge URL
  as a product default.
- Redesigning the already validated complete-history replay mechanism.
- Adding a Hermes adapter or any other new client integration.
- Claiming hosted CI, release, deployment, runtime acceptance, or housekeeping
  before those external transactions are freshly verified.

## Impact

The GitHub Verify and Release workflows, their contract tests, architecture
policy, policy tests, release documentation, and current evidence binding must
converge on one portable implementation. Hosted reruns remain external proof.
