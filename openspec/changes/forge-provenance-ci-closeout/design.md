## Context

Exact-ref hosted CI proved that source-level whole-history provenance can still
fail at the workflow boundary. GitHub variables contain allowed-signers text,
while the checker deliberately accepts only a file path. Separately,
`filepath.IsAbs` recognizes only the current host's path syntax, so a POSIX
absolute root passed on Windows can be misclassified as relative.

Current runtime evidence also exposed a client-boundary regression: Codex CLI
and Codex Desktop share `~/.codex/config.toml`, but the candidate discovered a
nonexistent `configuration.toml` and described Desktop as a separate surface.

## Goals / Non-Goals

**Goals:**

- Preserve the checker's file-path API and project GitHub variable content at
  the CI boundary.
- Keep trust material temporary, permission-restricted, and absent from logs.
- Validate repository-relative paths identically on every supported host.
- Restore one truthful Codex Home projection without acquiring authority over
  conversations or Desktop-only GUI settings.
- Prove all three defects with regression tests before implementation.

**Non-Goals:**

- Changing the complete-history replay or signature-verification protocol.
- Adding repository-local trust files or operator-specific defaults.
- Mutating application-managed Codex state.

## Decisions

### Trust content is materialized at the workflow boundary

Each GitHub job that runs provenance checks writes the protected variable to a
file below `RUNNER_TEMP` under `umask 077`, exports only the path through
`GITHUB_ENV`, and never prints the content. The checker remains provider-neutral
and file-oriented. GitLab continues to use its protected file-variable type.

Alternative: teach every checker to accept either content or a path. Rejected
because it expands the API, duplicates format detection, and makes accidental
secret logging more likely.

### Portable path syntax is lexical, not host-derived

Architecture configuration uses slash-separated repository-relative paths.
Validation rejects leading slash, drive prefixes, backslashes, empty or dot
segments, and parent traversal before any filesystem access. This grammar is
the same on macOS, Linux, and Windows.

Alternative: use `filepath.IsAbs` and add a Windows special case. Rejected
because the host library intentionally interprets only native syntax and would
leave foreign-host path forms inconsistent.

### Hosted results remain external evidence

Workflow contract tests prove source intent, not hosted execution. Exact-tip
Verify and Release runs remain required before publication or completion is
claimed.

### Reproducibility selects current stable inputs, not obsolete ones

`go.mod` owns the exact Go compiler and module graph. The CI gate policy owns
the immutable GitHub Action revisions projected into both workflows. A release
advances those owners to current stable inputs, validates the resolved graph,
and then reproduces that exact candidate; it does not retain an older version
merely because it was previously green. Indirect modules required by the graph
are upgraded when the selected direct modules and tools admit them; unrelated
modules outside the resolved build list are not product inputs.

### Codex Home is the shared configuration boundary

Codex CLI and Codex Desktop consume the same Codex Home configuration. The
default admitted target is `~/.codex/config.toml`; additional homes require
explicit operator configuration. AIGW owns only its marked provider/model block,
sidecar, and native credential binding. It does not create a Desktop adapter or
mutate conversations, session records, model metadata, or Desktop-only settings.

### Missing clients are a no-op, not a partial installation

The implemented Adapter registry contains Claude Code and Codex. First-time
setup enables only an Adapter whose required executable and configuration
surface are discoverable. An absent client remains unconfigured and AIGW does
not create foreign state. Hermes and future clients require an independent
admission record and Adapter implementation.

## Risks / Trade-offs

- **Environment files persist for the job** -> keep the path job-local below
  `RUNNER_TEMP`; hosted runners clean the workspace and temporary directory.
- **Lexical validation is stricter** -> policy paths are repository-relative
  identifiers, so accepting dot segments or platform syntax has no product use.

## Migration Plan

1. Add failing CI-contract and architecture-policy tests.
2. Materialize GitHub trust content in each provenance job and replace
   host-dependent absolute-path detection with the lexical grammar.
3. Restore the shared Codex Home projection and its product contract.
4. Prove that setup leaves absent clients untouched and document the admitted
   client registry.
5. Run focused and full local gates, commit and land through the governed lane.
6. Replay the commit into GitHub's identity domain, publish with compare-and-swap
   controls, and rerun exact-tip hosted gates.

Rollback before publication resets the work lane. After publication, restore
only through the existing Forge recovery transaction and never mix graphs.
