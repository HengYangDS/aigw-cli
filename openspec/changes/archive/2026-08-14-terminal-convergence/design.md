# Design

## Product boundary

```mermaid
flowchart LR
    AIGW["AIGW configuration control plane"] --> Codex["Codex config"]
    AIGW --> Claude["Claude Code settings and credential helper"]
    AIGW --> Accounts["Provider Accounts and Routes"]
    Accounts --> Endpoint["Ordinary direct or loopback API endpoint"]
    AIGW -. "does not own" .-> Proxy["Responses data plane"]
    AIGW -. "does not own" .-> IDE["JetBrains or MCP state"]
```

Core configuration owns provider-neutral Accounts, credentials, Profiles, and
Routes. Each client adapter owns only its public projection contract. Provider
diagnostics are optional registrations, never client behavior.

## Lane absorption

For every historical lane:

1. Compare product paths with current `dev`.
2. Identify a concrete missing behavior and its existing semantic owner.
3. Rebuild it test-first in this lane only if it reduces the final gap.
4. Discard duplicate OpenSpec carriers, version noise, compatibility wrappers,
   and project-external governance.

No historical tree is merged wholesale.

The 2026-08-14 exact inventory leaves one current carrier:
`work/terminal-convergence`. Historical lanes divide as follows:

| Historical lane group | Disposition | Current authority |
| --- | --- | --- |
| 2026-07-31 reconstruction, decoupling, housekeeping, and Forge drafts | Discardable | Current source, decisions, and terminal Change already carry the accepted product boundary. |
| `work/20260808-ci-log-hygiene` | Superseded | Current CI contracts fail on warnings and traceback leakage without retaining the old `cicontract` surface. |
| `work/ci-log-hygiene` | Selectively absorbed | Current tree keeps the official Claude settings and credential-helper boundary and native Go quality owners; obsolete launcher, shell, and parallel release designs stay deleted. |
| `work/quality-contract-tightening` | Outside delivery critical path | Current hooks enforce message shape; destructive historical identity reconstruction is not required for the terminal product release. |
| `work/20260810-terminal-closeout` | Discardable carrier residue | It contains no product behavior beyond an obsolete Change carrier. |

No historical lane contributes a product delta that warrants wholesale replay.
Any later history-repair program must be an independently admitted change and
must not delay this product delivery.

## Structure and dependencies

| Concern | Owner | Rule |
| --- | --- | --- |
| Configuration model | `internal/configuration` | One provider-neutral SSOT. |
| Codex projection | `internal/codex` | CLI and Desktop shared-home semantics only. |
| Claude Code projection | `internal/claude` | Official settings and credential helper only. |
| Provider diagnostics | `internal/providers` | Optional registry entry with no client coupling. |
| CLI UX | `internal/cli` | Product commands; no shell wrapper or hidden Python dependency. |
| Release | `tools/release` | Same source, independently verified Forge assets. |

A mature dependency is admitted only when it deletes more ELOC, accidental
complexity, or platform risk than it introduces. No dual stack remains.

## Delivery boundary

Local proof and release assembly complete without a remote. GitLab and GitHub
are peer publication planes: either can publish the same signed revision and
neither is an input to the other. Runtime installation consumes formal assets,
not a source checkout or another product.

## Verification

Completion requires statement, branch, package, and aggregate coverage strictly
above 95%; native macOS, Linux, and Windows gates; cross-platform artifacts;
clean Codex and Claude Code projection; independent Forge publication; formal
installation; and owner-bound retirement of every superseded lane.
