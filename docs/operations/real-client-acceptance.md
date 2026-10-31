---
subject: aigw:real-client-acceptance
role: how-to
state: canonical
relations:
  canonical_for: real-client release acceptance
---

# Real-Client Acceptance

Real clients qualify a candidate's projections, credentials and lifecycle with
the declared Codex, Claude Code and Hermes executables. This procedure belongs
to the [contribution workflow](../../CONTRIBUTING.md#development-and-verification);
source checks, the native artifact lifecycle and historical upgrades remain
separate observations.

## Client journey

The lower-level tagged test consumes the same explicit candidate and companion-tool
paths as `accept-native --clients` in
[tagged artifact acceptance](../../CONTRIBUTING.md#tagged-artifact-acceptance),
rather than searching the user's installation:

```bash
AIGW_ACCEPTANCE_RELEASE=/absolute/path/to/verified/native-directory \
AIGW_ACCEPTANCE_CODEX=/absolute/path/to/codex \
AIGW_ACCEPTANCE_CLAUDE=/absolute/path/to/claude \
AIGW_ACCEPTANCE_HERMES=/absolute/path/to/hermes \
AIGW_ACCEPTANCE_HERMES_HOME=/absolute/path/to/native/hermes-home \
AIGW_ACCEPTANCE_CLIENT_PATH=/usr/bin:/bin \
  mise exec --locked -- go test -tags=client_acceptance ./tools/release \
  -run '^TestNativeClientJourney$' -count=1 -v
```

Windows uses native executable paths and a semicolon-separated companion PATH.
Supply complete client distributions; a missing path is not a stub/download license.
On macOS and Linux, Hermes uses a disposable home outside its native data root.
It borrows committed dependencies without provisioning; boot records stay private.
Cleanup leaves native configuration, installation facts, boot records and launchers
unchanged. Windows verification refuses to start Hermes until native launcher
publication can be isolated.

The isolated Codex fixture explicitly enables the official Windows `unelevated`
sandbox, which uses a restricted token without administrator setup. The client
still runs in `read-only` mode; a permissions label without an active native
backend cannot qualify tool execution. This test-only setting neither changes
operator configuration nor chooses a production sandbox mode. Denied tool output
remains a failure even when Codex returns the final marker.

For focused retry-policy source acceptance, supply an explicit source-built
program, a portable predecessor fixture and the actual Codex distribution:

```bash
AIGW_ACCEPTANCE_RETRY_PROGRAM=/absolute/path/to/source-built/aigw \
AIGW_ACCEPTANCE_BASELINE=/absolute/path/to/retained/portable/aigw \
AIGW_ACCEPTANCE_CODEX=/absolute/path/to/codex \
AIGW_ACCEPTANCE_CLIENT_PATH=/usr/bin:/bin \
  mise exec --locked -- go test -tags=client_acceptance ./tools/release \
  -run '^TestNativeCodexConnectionRetriesSource$' -count=1 -timeout=2m -v
```

This exercises public preview, selection, cached predecessor credentials,
native budget admission, retained-reader rollback refusal, one unknown-outcome
request and native-default reset
in private homes with synthetic environment credentials. It does not qualify
package-manager cutover, release artifacts or Desktop behavior. The same case
runs as `codex-connection-retries` in the complete release journey.

The complete build-tagged journey requires all three declared executable paths:
`AIGW_ACCEPTANCE_CODEX`, `AIGW_ACCEPTANCE_CLAUDE` and `AIGW_ACCEPTANCE_HERMES`.
Hermes also requires `AIGW_ACCEPTANCE_HERMES_HOME` for its native installation.
Missing clients fail, not skip. See [Adapter admission](../governance/adapter-admission.md)
for independent Desktop qualification; CLI evidence cannot replace it.

A loopback protocol fixture consumes the shipped manifest and demands its exact
recommended model, effort, credentials and stream contract. Isolated homes and
synthetic environment Tokens replace operator state. With a published predecessor,
real clients run baseline → candidate → rollback → re-upgrade; without it Codex/
Claude use current-schema fixtures and Hermes proves first adoption only.

Before replacement the candidate stages its versioned helper; captured old/new
commands must remain callable. Verify retained authenticated inference before
active-program sync/check. The exact current candidate's per-client `verify`
observes every lifecycle state; it owns an isolated probe, not the retained
client or its settings. Original programs, byte checks, cached credential
commands, and independent native tool/session requests still qualify succession.
This does not prove the historical verifier works with a newer client:
0.3.3's Hermes verifier loses the installation binding in an arbitrary home.
Published 0.3.1 also cannot check the successor's projection until sync.
Uninstall preserves unowned settings and removes only the test installation.
No-token and actual Provider tests remain different observations.

Use the signed-candidate command of
[native upgrade acceptance](../../CONTRIBUTING.md#native-upgrade-acceptance) with `--clients`. Omitting artifacts
deliberately builds once; on macOS that explicit build needs
[release identity inputs](../decisions/dr-0011-single-portable-token-backend.md#consequences-and-boundary).
One retained candidate serves lifecycle and client tests; stop at first failure.
Quality jobs still run input/envelope/preservation regressions without real clients.
Homebrew's public-link replacement gap needs its own installed upgrade proof.

## Hosted Windows qualification

GitLab review/protected jobs use separate registrations selected by literal CUE
tags; proposals stay unprotected. Disposable execution and peer-local evidence
remain required. GitHub uses hosted runners, not self-hosted fallback.

GitLab `AIGW_CANDIDATE_TAG` selects a published matrix;
`AIGW_CANDIDATE_ARTIFACTS` selects local pre-tag bytes. `AIGW_BASELINE_TAG`
selects the predecessor and `AIGW_NATIVE_CLIENTS=true` adds provisioned clients.
CUE forwards these after `ci native --`; source checks do not inherit them.
`AIGW_CANDIDATE_SOURCE` forwards the exact signed pre-tag product commit to
`--candidate-source` when a newer verifier consumes existing candidate bytes.

Explicit prebuilt inputs without full quality or lock refresh select only native
artifact tools and the existing release journey; they do not repeat source-wide
tests or npm bootstrap. Ordinary source/review jobs and full qualification retain
their complete quality and security graph. A partial artifact run cannot satisfy
the failed quality job or release readiness. GitHub dispatches use `Manual`
check names, including tag dispatches and single-platform runs; only review and
accepted push events report canonical required checks. Release evidence requires
the exact tag push and canonical job set. GitLab retains its complete MR graph
and native MR-pipeline admission, not a manually dispatched pipeline substitute.

GitLab downloads use its explicit project and Job Token, not sibling credentials.
GitHub manual `candidate_tag`/`baseline_tag` selects that peer's same bytes, with
`native_clients` for real-client execution. A reviewed verifier may have a
different SHA but must declare the candidate version; signed provenance still
binds execution to the selected source.

`ci native-clients supply` installs the clients its host declares, at the releases pinned in `.config/ci/pipeline.cue`, into one
job-owned directory: npm platform packages, and on Linux and macOS Hermes through its release's checksum-pinned installer.
A failed or interrupted supply removes it; `retire` removes only that directory, proves it absent and fails while a process still uses it.

## Disposable Linux clients

Use Docker `--init`, a complete non-root account and owned tmpfs/output. Keep
the root filesystem read-only, drop capabilities and prohibit host mounts. Select
Mise at or above the repository minimum; the image must also supply `ssh-keygen`,
the native compiler and C runtime. Probe these inputs as the actual user before
starting product acceptance.

Docker tmpfs defaults to `noexec`. Explicitly allow execution only on private
Mise, work and test directories that contain tools, extracted product binaries or
Go test executables; retain `noexec` on artifact and trust input mounts. Check
actual mount flags and run the selected tools before a heavy gate. Synthetic
credentials never authorize host-store access.

Transfer inputs/results through `docker exec -i`. On macOS, create portable tar
streams with `--no-xattrs --no-acls --no-fflags --no-mac-metadata`; extract with
`--no-same-owner`. `docker cp` may miss live tmpfs. Verify hashes, process/resource
state and exact cleanup before stopping and removing only owned resources.

Codex requires working user namespaces and bubblewrap. Check both through the
actual client; containers are not interchangeable with native Linux hosts.
Never weaken an operator's security policy or silently remove the client's sandbox
to make acceptance pass. Use a test-owned Git repository where required rather
than global safe-directory changes. gh/glab probes use private config directories.

Retained-state acceptance executes projected helpers through the real native
shell and preserves configuration bytes before active-program sync. The baseline
must read the fixture's declared schema; older-schema inputs need the reviewed
migration journey, not disabled integration. Host, container, Provider and client
identities each have independent acceptance claims.
