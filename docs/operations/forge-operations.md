<!--
---
subject: aigw:forge-operations
role: how-to
state: canonical
relations:
  canonical_for: independent Forge operation
---
-->

# Forge Operations

## Authority

Local Git is the only commit and annotated-tag authority. GitLab and GitHub are
independent optional publication peers. Each receives the same locally signed
object; neither peer is an input to the other.

| Concern                    | Authority                            |
| -------------------------- | ------------------------------------ |
| Commit and tag bytes       | Local Git                            |
| Product object trust       | Explicit allowed-signers file        |
| GitLab transport           | Git/SSH or GitLab credential context |
| GitHub transport           | Git/SSH or GitHub credential context |
| Hosted `Verified` display  | Each Forge account projection        |
| Release assets and records | Each selected peer, independently    |

Transport credentials never construct, rewrite, or sign product objects.

## GitLab Runner Admission

The CUE workflow selects Linux runners before jobs are created. `quality`,
`native-linux`, `linux-secret-service`, and `release-assets` consume the same
`AIGW_CI_LINUX_RUNNER_TAG`; no project variable chooses that tag.

| Ref context                        | Linux runner tag                     | Runner access   |
| ---------------------------------- | ------------------------------------ | --------------- |
| Same-project MR or unprotected ref | `ci-linux-arm64-container`           | `not_protected` |
| Accepted branch or release tag     | `ci-linux-arm64-container-protected` | `ref_protected` |
| Manual run on a protected ref      | `ci-linux-arm64-container-protected` | `ref_protected` |
| Manual run on an unprotected ref   | `ci-linux-arm64-container`           | `not_protected` |

Each registration is project-scoped, locked, tagged-only, and uses a disposable
container. Review and protected execution require distinct Runner registrations
and isolated caches. Public author and allowed-signers inputs remain available
to review; credentials and protected release variables remain restricted.
`release-assets` admits only explicit dispatch on a protected release tag.
Darwin retains control-plane object and tag verification; Windows and Darwin
native jobs retain their separate review and protected registrations.

CUE enables GitLab's [`FF_GIT_URLS_WITHOUT_TOKENS`](https://docs.gitlab.com/runner/configuration/feature-flags/)
for every job. The native Runner obtains the job Token from its environment
instead of embedding it in Git configuration or caching it in a credential
helper. This limits job-Token persistence; it does not isolate a persistent
Shell account from its Runner registration credential or other jobs.

The former `AIGW_GITLAB_LINUX_RUNNER_TAG` project variable has no consumer in
this workflow. Retire it after the updated workflow is admitted and both runner
paths have executed. Do not set a project variable named
`AIGW_CI_LINUX_RUNNER_TAG`, which would override the workflow's selection.

## Verify Local Objects

For change admission, record the target commit before integration as
`AIGW_REVIEW_BASE` and the reviewed candidate as `AIGW_REVIEW_HEAD`. Both values
must be exact commit IDs; the base must be an ancestor of the candidate. Supply
the product author email and trusted signers file through the corresponding
release variables below.

```sh
mise exec --locked -- go run ./tools/forge commits \
  --base "$AIGW_REVIEW_BASE" \
  --revision "$AIGW_REVIEW_HEAD" \
  --email "$AIGW_RELEASE_AUTHOR_EMAIL" \
  --allowed-signers "$AIGW_RELEASE_ALLOWED_SIGNERS_FILE"
```

This verifies every introduced commit in `base..head`, excluding the base.
Retain the recorded IDs after integration; a moving target branch may already
equal the candidate and would select an empty range. This command verifies
objects, not CI success or permission to publish.

Omitting `--base` deliberately audits the entire reachable history under the
selected revision's policy. Historical subjects, authors or signatures can
fail that separate audit; do not rewrite history merely to admit a valid new
change. `tags` likewise audits all local release tags and belongs to an explicit
release-history review:

```sh
mise exec --locked -- go run ./tools/forge tags \
  --allowed-signers "$AIGW_RELEASE_ALLOWED_SIGNERS_FILE"
```

## Publish a Branch

`main` publishes the same exact commit atomically to peer `main` and `dev`.
`proposal/*` publishes only its matching ref. No other branch is admissible.

```sh
mise exec --locked -- go run ./tools/forge project \
  --remote origin \
  --source main \
  --email "$AIGW_RELEASE_AUTHOR_EMAIL" \
  --allowed-signers "$AIGW_RELEASE_ALLOWED_SIGNERS_FILE"
```

A fast-forward or equal tip needs no destructive option. Every update carries
an exact lease for the observed remote tip, including expected absence. An
explicit `--expect-remote-tip` is checked even for a fast-forward or equal tip;
it is never ignored because the update appears safe. Concurrent remote drift
rejects the atomic publication instead of silently changing its admitted base.
A one-time divergent
cutover requires every fresh observed peer tip:

```sh
mise exec --locked -- go run ./tools/forge project \
  --remote github \
  --source main \
  --email "$AIGW_RELEASE_AUTHOR_EMAIL" \
  --allowed-signers "$AIGW_RELEASE_ALLOWED_SIGNERS_FILE" \
  --expect-remote-tip "main=$OLD_MAIN" \
  --expect-remote-tip "dev=$OLD_DEV"
```

The operator temporarily authorizes protected-branch force push, runs the exact
compare-and-swap transaction, verifies the two remote OIDs, and immediately
restores force push to disabled. A changed tip invalidates the prepared command.

## macOS signing and notarization

Use an existing Developer ID Application identity and one explicitly selected
`notarytool` authentication mode on the authorized macOS host: a validated
Keychain profile, or an already protected App Store Connect API key file and its
key ID. Team API keys also require the issuer ID; Individual API keys must omit
it. Do not export the signing private key or copy the API key into the repository.
The signing identity and archive verification contract are defined in the
[release policy](../governance/change-and-release-policy.md#quality-and-platform-evidence).

Keep the signed candidate immutable while Apple processes it. Prepare one ZIP
containing the exact signed executables extracted from both macOS archives,
each in its own architecture directory. Preserve their executable permissions.
Use an explicit, new operation directory under the checkout's
`build/verification/`; retain the ZIP, candidate checksums and native responses
until the release decision is resolved. Never upload credentials or source-state
directories with the executables.

Set `NOTARY_ZIP` and `EVIDENCE_DIRECTORY` to those reviewed inputs. Select one
authentication argument list; never fall back silently after an authorization
failure. For a Team API key already held in a protected file:

```zsh
NOTARY_AUTH=(--key "$NOTARY_KEY_FILE" --key-id "$NOTARY_KEY_ID" --issuer "$NOTARY_ISSUER_ID")
```

For a validated Keychain profile instead, use
`NOTARY_AUTH=(--keychain-profile "$NOTARY_PROFILE")`. For an Individual API key,
omit `--issuer` and its value. Then submit once:

```zsh
mise run release:notary submit "$NOTARY_ZIP" \
  "${NOTARY_AUTH[@]}" --no-wait --output-format json \
  < /dev/null > "$EVIDENCE_DIRECTORY/submission.json"
```

Set `SUBMISSION_ID` to the returned Apple `id`. Query or resume that same
submission rather than uploading again:

```zsh
mise run release:notary info "$SUBMISSION_ID" \
  "${NOTARY_AUTH[@]}" --output-format json < /dev/null

mise run release:notary wait "$SUBMISSION_ID" \
  "${NOTARY_AUTH[@]}" --timeout 4m --output-format json \
  < /dev/null > "$EVIDENCE_DIRECTORY/wait.json"
```

The mise task forwards native arguments and bounds each invocation to five
minutes; the shorter native wait leaves time for command shutdown. A wait
timeout does not cancel Apple's submission. If upload is interrupted before
an ID is returned, inspect native `history` and recover the existing submission
before considering another upload. An authentication failure requires explicit
credential recovery, not repeated calls or interactive password prompts.

Only `Accepted` permits proceeding. Retrieve Apple's log, then run the
[final archive verifier](../governance/change-and-release-policy.md#quality-and-platform-evidence)
against the retained candidate before publication:

```zsh
mise run release:notary log "$SUBMISSION_ID" \
  "${NOTARY_AUTH[@]}" \
  "$EVIDENCE_DIRECTORY/notarization-log.json" < /dev/null
```

`Invalid` stops publication; use the log to diagnose the exact candidate.
Apple's [notarization workflow](https://developer.apple.com/documentation/security/customizing-the-notarization-workflow)
creates tickets for standalone binaries, but cannot staple tickets to those
binaries or ZIP files. Gatekeeper therefore needs online ticket discovery on a
fresh machine. Do not claim offline first-launch approval for the portable
archive. An independently stapled installer would be a separate distribution
channel, not a reason to alter these accepted executable bytes.

## Publish a Tag

Create and sign an annotated tag once in local Git. Publish that exact tag
object independently:

```sh
mise exec --locked -- go run ./tools/forge publish-tag \
  --remote origin \
  --tag "$TAG" \
  --allowed-signers "$AIGW_RELEASE_ALLOWED_SIGNERS_FILE"

mise exec --locked -- go run ./tools/forge publish-tag \
  --remote github \
  --tag "$TAG" \
  --allowed-signers "$AIGW_RELEASE_ALLOWED_SIGNERS_FILE"
```

An equal remote tag is idempotent. A different object fails closed unless its
exact OID is supplied through `--expect-remote-tag` for an explicitly approved
cutover.

## Completion

### Verify published bytes on a native host

The read-only GitHub Release workflow accepts an existing release tag and a
bounded native runner choice. Its default remains Linux artifact verification.
Set `native_lifecycle=true` to additionally run the existing release lifecycle
against that host's published archive, after checksum, signature and source
verification. This does not publish, replace a tag or rebuild the candidate.
Separate runner selections can execute independently.

Before downloading assets, the workflow resolves the selected tag to its
product commit and requires this peer's latest successful tag-push `Verify`
attempt at that SHA. Quality, macOS, Linux, Windows and version jobs must all
have passed in that same attempt. Branch or manual runs, an earlier attempt,
another peer, or incomplete GitHub Actions evidence do not qualify. The
workflow has read-only Actions access for this check; GitLab instead uses
same-pipeline `needs` for the same five obligations.

The bounded choices cover the six published OS/architecture pairs. Alongside
the ordinary Linux, macOS and Windows hosts, `ubuntu-24.04-arm`,
`macos-15-intel` and `windows-11-arm` execute the additional architectures.
Runner labels follow the [official image matrix](https://github.com/actions/runner-images#available-images);
the job must still demonstrate the actual host and candidate identity.

With `TAG` set to the exact published release, Windows ARM64 qualification uses:

```sh
gh workflow run release.yml --ref main \
  --field tag="$TAG" \
  --field runner=windows-11-arm \
  --field native_lifecycle=true
```

The selected tag and signed provenance identify the product bytes and their
build inputs. The selected workflow revision owns the verifier, tests and their
locked execution environment; it must declare the same product version, but
need not be the release commit. This permits stronger tests of an unchanged
published artifact without replacing its tag. The verifier checkout must be
clean, and artifact signature, checksum and tagged-source verification remain
mandatory. Native macOS and Windows journeys explicitly test
synthetic credentials on the disposable hosted machine. Linux's native Secret
Service still requires the separate isolated user-bus qualification. A queued
job, inspected archive or successful checksum does not prove native execution.
Real-client and live-Provider acceptance remain separate from this lifecycle.

### Supply a native job with public inputs

The existing GitLab native jobs can consume a reviewed generic package instead
of requiring an operator to stage a host or VM. Select `AIGW_NATIVE_PLATFORM`,
`AIGW_NATIVE_INPUT_PACKAGE`, the exact `AIGW_CANDIDATE_SOURCE`, and the archive's
`AIGW_NATIVE_INPUT_SHA256`. The package version is that source commit; its single
download is `public-inputs.tar`. Select `AIGW_BASELINE_TAG` and
`AIGW_NATIVE_CLIENTS=true` for retained-predecessor client acceptance with
explicitly supplied native clients.

The archive contains complete `candidate/` and `baseline/` matrices, native client
distributions and reviewed official Hermes source under `suppliers/`. The
portable release owner downloads once into private scratch, verifies the exact
archive SHA256, and extracts only the complete positive candidate/predecessor
matrix names. Signatures, source identity, published-predecessor admission and
lifecycle execution retain their existing release owners.

Windows client supply alone uses the existing job directory, managed Python
3.12 and frozen official Hermes dependencies. It forwards its already downloaded
archive through `--input-archive`; the release owner rechecks the same SHA256 and
matrices without another download. It never changes or deletes that caller-owned
file. Locked uv is an acceptance tool, not an AIGW runtime dependency. The job
cleans its supplies; release acceptance cleans its extracted inputs on success
or failure.

Trust comes from the independently configured source/artifact signer file
variables and signer principal, never from trust files inside the downloaded
archive. The checkout must contain the selected signed source and published
predecessor tag. Package inspection or successful preparation is not native
acceptance; original job execution must prove it. Full-quality and lock-refresh
requests retain their source gates. After the native supplier transition passes,
retire its superseded installer stages rather than keeping two preparation paths.

### Measure native candidate performance

The existing `accept-native` command also consumes the
[performance budgets](../governance/change-and-release-policy.md#performance-and-completion-claims).
Before tagging, use `mise run performance --artifacts "$CANDIDATE_DIRECTORY"
--candidate --performance "$OUTPUT_DIRECTORY"` with the approved artifact and
Git source trust inputs and `AIGW_ACCEPTANCE_BASELINE` pointing to the verified
published predecessor executable. After tagging, omit `--candidate` and select
`CI_COMMIT_TAG` to verify the signed release tag as well.

`--baseline-artifacts` with `--baseline-tag` instead admits the complete signed
predecessor matrix and extracts its native executable through the same owner.
`--input-package` with `--input-sha256`, `--candidate-source`, `--candidate`,
`--baseline-tag`, `--peer gitlab` and `--repository` admits the complete package
without staging a host. An absolute `--input-archive` instead consumes the same
signed input contract offline and is mutually exclusive with `--input-package`.
For published remote inputs, `--tag`, `--baseline-tag`, `--peer` and
`--repository` select that peer's native CLI with a bounded no-prompt download;
neither Forge is a transport fallback for the other. See
[native artifact acceptance](../../CONTRIBUTING.md#tagged-artifact-acceptance).

Pre-archive samples qualify only that candidate. Archive changes source identity;
measure the newly built, exact-source matrix again before tagging, rather than
carrying old samples into the final release verdict.

The output must be a new absolute directory. Locked Hyperfine supplies both
source diagnostic regressions and explicitly selected artifact measurements.
Selected measurements first qualify native preparation, exact arguments,
diagnostic streams and schema 2 through the existing focused fixture. A failed
preflight stops the operation before the full performance samples.
An explicit performance output selects only the matching measurements after
normal artifact, source and predecessor trust checks; it does not repeat core,
rollback or resource lifecycle acceptance. `--clients` additionally runs the
requested native-client journey. Neither selection replaces required lifecycle
evidence for release admission.
The same measurement owner records first setup and converged sync separately
from route changes. Setup preparation removes only fixture-owned configuration
and projection files before each timed command; it preserves the installed program
and credentials. Construction and CI costs remain tied to their original build
and job receipts, not these samples.

The GitHub Verify workflow accepts `performance=true` together with
`baseline_tag` and `candidate_tag`. GitLab's existing native jobs accept
`AIGW_NATIVE_PERFORMANCE=true` in a web or API pipeline with `AIGW_BASELINE_TAG`
and either `AIGW_CANDIDATE_TAG` or the signed `AIGW_CANDIDATE_ARTIFACTS` input.
Both peers reuse the same release acceptance and measurement owner, not a
second build or lifecycle. Standalone Secret Service qualification follows
source-quality admission on both peers: review, accepted-branch push, tag, and
explicit source checks retain it; artifact-only performance runs stay isolated.
Transport-only `--peer` and `--repository` inputs
do not repeat source lifecycle qualification. GitLab prepares Hyperfine for
selected prebuilt performance and GNU time on Linux; raw samples, warnings,
individual blocks and pooled p95 remain under `build/verification/performance`
in the existing always-retained verification artifact. GitHub retains the
corresponding native performance artifact, including on failure.
Each case uses five warmups and two reversed-order blocks of forty samples.
Hyperfine 2.0 invokes each workload directly with time metrics; preparation still
uses the platform shell. Schema 2 exports retain each run's wall-clock seconds and
exit code, not only aggregate summaries.
Setup uses an isolated synthetic endpoint; the timed helper executes its
actual projected command through the native shell. Client discovery is a
controlled fixture, not evidence of Provider inference.

For a failed native measurement, add `--performance-attribution` to the same
explicit signed-input command. It selects only component diagnosis: the original
shell helper, shell startup, source and copied-reader startup, direct invocation
of that exact projected reader, and keyring `Read`/`Exists` through each variant's
native source worker. Worker calls retain the product's restricted environment;
these API timings include the native subprocess but exclude Hyperfine and the
shell. Projection is environment-only; synchronization covers each selected
backend. Both cases retain the full measurement's declared prepare and command
argv, with five warmups and forty samples in each reversed-order block.
They record both completed prepare and sample processes:
exact PID, native exit, start/end, monotonic elapsed time, native CPU accounting
and available per-OS resource usage. These children inherit the public
`accept-native` suite's bounded process group or Windows Job; a manually invoked
test alone does not supply that outer cleanup boundary.

Workload process records are diagnostic, not Hyperfine-equivalent timing.
Each summary row includes preparation and evidence writes; compare individual
sample records, not that aggregate row, with a historical measured command.
Native CPU accounting follows `ProcessState`: Windows reports the completed
process itself, not its credential-worker descendants; Unix includes waited-for
descendants. Wall minus CPU does not distinguish scheduling, storage or IPC
waits. Darwin RSS is
bytes, Linux RSS is KiB, and unavailable Windows counters are explicitly null.
Zero counters do not prove no work occurred. Keep failed evidence, run one bounded
diagnosis, and either repair a demonstrated owner or disclose the unproved cause
before any separately declared full qualification. Do not remove durability
boundaries, drop samples or suppress native warnings to obtain a pass.

The receipt states
`qualification=false` and `scope=component-attribution`. It retains raw samples,
warnings and selected executable file identities without weakening or satisfying
the full-performance budgets. Selected file architecture does not prove the
architecture of a shell started after Windows x64-controller path redirection.
Ordinary acceptance explicitly disables inherited attribution scope.

Both peers project this scope from the same CUE owner. GitHub requires
`performance=true` with `performance_attribution=true`; GitLab requires
`AIGW_NATIVE_PERFORMANCE=true` with `AIGW_NATIVE_PERFORMANCE_ATTRIBUTION=true`.
Use one bounded diagnostic before choosing a product repair or repeating full
qualification; unchanged signed inputs may diagnose their own bytes, not a newer
source commit.

Environment credentials run on every measurement host. Linux additionally
measures its explicit file backend; native vault measurement requires the
existing isolated-credential opt-in. Never enable macOS vault tests on an
operator's login host. Native Hyperfine binaries are locked for Linux, macOS,
and Windows on both AMD64 and ARM64. Tool installation and product performance
acceptance remain separate evidence boundaries.

Memory acceptance runs separately from Hyperfine's timings. It measures the
configured `status` process with macOS wait accounting, GNU time at
`/usr/bin/time` on Linux, and a retained Windows process handle queried for its
peak working set. Linux measurement hosts therefore require the distribution's
GNU time package. The small native supervisor prevents the Go test parent's
address space from inflating Linux child accounting. Every performance run
first calibrates with a 128 MiB parent and 16/64/16 MiB child allocations;
uncalibrated, empty or incomplete observations fail admission.

The summary retains forty per-process byte observations for each candidate and
predecessor block after five warmups, with the same reverse-order design.
Compare each block's maximum against both declared growth thresholds. These
are native resident-set observations, not sampled RSS, virtual memory, managed
heap size or Hyperfine's cumulative child values. Retain statistical outliers
rather than deleting samples or changing budgets to obtain a pass.

### Observe publication and integration

Branch or tag publication is complete only when local Git and every selected
peer expose the same object OID. Hosted CI, Release records, assets, checksums,
installation, and runtime acceptance remain separate evidence boundaries.

For a review merge, read back the review state and exact target-ref OID. A
successful GitLab fast-forward may have no `merge_commit_sha` because no merge
commit was constructed. Require the target to equal the admitted signed object
and the merged proposal ref to be absent; a null merge-commit field alone is
neither failure nor completion.

Successful CI is distinct from enforced admission. Verify the native main/dev
rules require the intended checks and producer identity where supported, then
observe a real review blocked while checks are pending and admitted after they
pass. Preserve signature, history and force-push controls. An executed
maintainer merge alone does not prove the dependency-maintenance path; candidate
calculation, native lock refresh, checks and exact-object integration require
their own observations. That path does not require a scheduler.
