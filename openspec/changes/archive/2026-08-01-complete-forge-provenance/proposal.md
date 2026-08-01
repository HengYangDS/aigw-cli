## Why

AIGW currently permits commit-provenance verification to begin after a tracked
floor. That exception can hide an invalid author or signature in reachable
history, so a green check does not prove the repository history that users can
fetch. The explicitly authorized identity repair must replace this partial
contract with complete, Forge-specific provenance and leave a reusable,
portable mechanism for future publication.

## What Changes

- **BREAKING**: delete the commit-floor exception and require every commit
  reachable from the selected revision to use the selected Forge actor and a
  trusted signature.
- Make a full-graph replay preserve each commit's tree, exact message bytes,
  author and committer timestamps, parent order, and merge topology while
  replacing only Forge-owned identity and signature fields.
- Keep GitLab and GitHub as separate provenance graphs. GitLab is the canonical
  source graph; GitHub is an independently signed identity projection with the
  same ordered semantic history.
- Require replay and verification in isolated object stores before any local or
  remote ref replacement.
- Treat branches, annotated tags, releases, CI, integrity records, and active
  evidence bindings as one fail-closed publication transaction.
- Keep publication actors, trust anchors, signing keys, signing programs,
  remote coordinates, and recovery-record paths outside product defaults.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: subject=AIGW publication provenance;
  reuse=extend; change=modify; remove partial commit-floor trust, define
  complete Forge-specific history verification and isolated semantic replay;
  facet:lifecycle=verification,recovery,replay,replacement,publication;
  facet:surface=git-history,branches,tags,releases,ci,evidence;
  facet:authority=source,publication-context,forge,record.

## Out of Scope

- Rewriting Codex JSONL, SQLite, historical messages, Responses item records,
  conversation models, or model metadata.
- Encoding the current operator's name, email, machine path, key, fingerprint,
  or Forge URL as a product default.
- Making two Forges share commit object IDs, tag objects, actors, signatures, or
  release records.
- Preserving an invalid object as reachable history for compatibility.
- Executing the current repository's authorized history replacement, remote
  publication, installation, runtime acceptance, or final lane retirement;
  those follow-up lifecycle transactions consume this mechanism and retain
  their own recovery and external-state evidence.

## Impact

The provenance checker, GitHub projection, release policy, tests, OpenSpec
contract, and current claim binding converge on whole-history verification.
The one-time authorized repair additionally rebuilds affected refs and
publication records only after recovery evidence and isolated verification are
complete.
