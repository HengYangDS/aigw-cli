## Decision

Keep Go's native module and `mise` toolchain. Add no Python-style wrapper or compatibility alias. The version carrier is read by one repository package used by CLI and release checks; CI supplies only protected Forge/signing inputs.

ETHOS remains lifecycle authority: `dev` is accepted integration, `main` is release, and `candidate/dev` plus `work/*` are local-only. GitLab and GitHub remain independent peers.

## Ownership

| Concern | Owner |
|---|---|
| Product version | tracked repository version carrier |
| Changelog ordering | `tools/repository` |
| Artifact names | `tools/release` |
| Branch transition | ETHOS public command |
| Provider publication | Forge-native workflows |
| Client projections | AIGW runtime packages |

## Rejected

- version inferred only from CI tags;
- a second build system or compatibility reader;
- direct Git ref edits to bypass ETHOS;
- coupling to Proxy, Workstation, JetBrains, or Codex session storage.

## Requirement To Task To Proof

| Requirement | Task | Proof |
| --- | --- | --- |
| `repository-organization:One repository version source` | `1.1` | `focused-tests` |
| `repository-organization:Governed release-branch convergence` | `2.1` | `ethos-closeout-receipt` |
| `repository-organization:Portable repository quality surface` | `3.1` | `repository-quality-gate` |
