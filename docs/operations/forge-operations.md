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

Use an existing Developer ID Application identity and an already validated
`notarytool` Keychain profile on the authorized macOS host. Do not export the
signing private key to enable this workflow. The signing identity and archive
verification contract are defined in the [release policy](../governance/change-and-release-policy.md#quality-and-platform-evidence).

Keep the signed candidate immutable while Apple processes it. Prepare one ZIP
containing the exact signed executables extracted from both macOS archives,
each in its own architecture directory. Preserve their executable permissions.
Use an explicit, new operation directory under the checkout's
`build/verification/`; retain the ZIP, candidate checksums and native responses
until the release decision is resolved. Never upload credentials or source-state
directories with the executables.

With `NOTARY_ZIP`, `NOTARY_PROFILE`, and `EVIDENCE_DIRECTORY` set to those
reviewed inputs, submit once:

```sh
mise run release:notary submit "$NOTARY_ZIP" \
  --keychain-profile "$NOTARY_PROFILE" --no-wait --output-format json \
  < /dev/null > "$EVIDENCE_DIRECTORY/submission.json"
```

Set `SUBMISSION_ID` to the returned Apple `id`. Query or resume that same
submission rather than uploading again:

```sh
mise run release:notary info "$SUBMISSION_ID" \
  --keychain-profile "$NOTARY_PROFILE" --output-format json < /dev/null

mise run release:notary wait "$SUBMISSION_ID" \
  --keychain-profile "$NOTARY_PROFILE" --timeout 4m --output-format json \
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

```sh
mise run release:notary log "$SUBMISSION_ID" \
  --keychain-profile "$NOTARY_PROFILE" \
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

### Measure published native performance

The existing `accept-native` command also consumes the
[performance budgets](../governance/change-and-release-policy.md#performance-and-completion-claims).
Use `mise run performance --artifacts "$CANDIDATE_DIRECTORY" --performance
"$OUTPUT_DIRECTORY"` with the same signature inputs as artifact verification
and `AIGW_ACCEPTANCE_BASELINE` pointing to the verified predecessor executable.
The output must be a new absolute directory. The task installs its locked
Hyperfine tool only when requested; ordinary checks do not require it.

The GitHub Verify workflow accepts `performance=true` together with
`baseline_tag` and `candidate_tag`. It reuses historical release acceptance,
not a second build or lifecycle. Raw samples, warnings, individual blocks and
pooled p95 are retained as native job artifacts, including on failure.
Each case uses five warmups and two reversed-order blocks of forty samples.
Setup uses an isolated synthetic endpoint; the timed helper executes its
actual projected command through the native shell. Client discovery is a
controlled fixture, not evidence of Provider inference.

Environment credentials run on every measurement host. Linux additionally
measures its explicit file backend; native vault measurement requires the
existing isolated-credential opt-in. Never enable macOS vault tests on an
operator's login host. Native Hyperfine binaries are locked for Linux/macOS
AMD64 and ARM64, and Windows AMD64. Windows ARM64 product acceptance remains
independent of this measurement tool's missing native binary.

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
