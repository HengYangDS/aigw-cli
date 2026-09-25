# Tasks

## 1. Freeze the behavior contract

- [x] 1.1 Reconcile proposal.md, the cli-readiness, product-control-plane,
      and projection-format deltas, and design.md with the current canonical
      requirements. Verify:
      `openspec validate inference-readiness-claude-override --strict --json`.
- [x] 1.2 Inspect the complete locked OpenSpec merged result and confirm every
      inherited scenario remains present and the Claude override does not turn
      a native alias into an upstream Route model. Verify: compare all
      requirement and scenario names with the canonical specs before coding.

## 2. Construct model-carrying requests

- [x] 2.1 Add an inference request for Anthropic Messages, OpenAI Responses,
      and OpenAI Chat Completions using each selected endpoint, protocol,
      credential header, exact Route wire model, and capped output. Verify:
      focused request-shape tests in `./internal/credential/...` fail before
      implementation and pass afterward.
- [x] 2.2 Reject an empty or unresolvable wire model without falling back to
      the catalogue request, and bound response reads without retaining a
      conversation. Verify: focused negative and size-bound tests in
      `./internal/credential/...`.
- [x] 2.3 Preserve the existing model-free catalogue request for endpoint-only
      checks, setup, rotation, and `aigw test`. Verify:
      `go test ./internal/credential/... ./internal/cli/...`.

## 3. Classify scoped observations

- [x] 3.1 Pass an explicit endpoint or inference scope through
      `diagnostics.Probe`, `ProbeBounded`, and `Result`. Verify: focused
      diagnostics tests demonstrate the same scope in request and result.
- [x] 3.2 Make one inference request that receives `503 no available channel`
      classify as retryable `ModelUnavailable` without a second request.
      Fail 401/403 after one request without repeated probes. Verify: request-count
      and classification tests in `./internal/diagnostics/...`. Consider both
      top-level and nested error text, not unrelated model metadata.
- [x] 3.3 Map a healthy inference observation to `inference_checked` and a
      healthy catalogue observation to `endpoint_checked`, preserving typed
      failure classes. Verify: `go test ./internal/readiness/...`.
- [x] 3.4 Require protocol-shaped assistant output before an HTTP 2xx inference
      observation is healthy. Reject incomplete Responses, empty outputs, and
      200 error bodies without retrying or disclosing credentials; use an
      explicit short-answer prompt, a 512-token output cap, and separate
      five-second endpoint versus 60-second inference attempt bounds. Verify: RED/GREEN
      diagnostics tests for all three protocols, affected CLI acceptance, and
      the full quality and native gates.

## 4. Preserve Claude's native model preference

- [x] 4.1 Inspect the existing sidecar read-only and recognize only a
      model-only change reconstructed from the previous Route. Reject helper,
      endpoint, and managed-credential edits. Verify: RED/GREEN tests in
      `./internal/claude/...` and byte equality of settings and sidecar.
- [x] 4.2 Carry the proven native-model-override fact through Claude's
      Adapter without treating the alias as a Route wire model. Verify:
      `go test ./internal/client/...`.
- [x] 4.3 Keep status and doctor locally truthful, and make check use endpoint
      scope for a proven Claude override with
      `aigw verify --for claude` as continuation. Verify: focused human and
      JSON acceptance tests plus a candidate-bound real nonpersistent native
      Claude request in an owned isolated context with noninteractive
      credentials. Preserve the user's current host settings and sidecar bytes;
      a host model equal to the selected Route does not prove an override.
- [x] 4.4 Keep a sidecar-proven native model preference during an unchanged
      synchronization or an AIGW-owned helper path update. Reproject the Route
      model when the Account, endpoint, or Route model changes; reject foreign
      connection edits and preserve byte-exact rollback. Verify focused
      RED/GREEN Claude projection tests, the existing read-only inspection
      and Adapter suites, and the full source and native gates.

## 5. Expose the check scope

- [x] 5.1 Default `aigw check` to inference and add `--endpoint-only`.
      Keep client-native authentication local and skip inference for a proven
      Claude model override. Verify: RED/GREEN CLI tests on request count and
      model identity in `./internal/cli/readiness/...`.
- [x] 5.2 Render `diagnostic_scope`, `inference_checked`, failures, and
      native-model override consistently in human and JSON status, check, and
      doctor output. Verify: `go test ./internal/cli/...` and no secret or
      ambiguous `ready` claim in either projection.
- [x] 5.3 Update the terminal experience guidance for the two check scopes,
      metered request cost, time-bound inference result, and native Claude
      verification boundary. Verify: `mise run check`.
- [x] 5.4 Treat a reviewed catalogue with no enabled Client Binding as
      deferred activation. `check` must return a nonzero, single-document
      `ok: false` result without a probe; `status` and `doctor` must expose
      zero enabled clients and an Account-aware continuation. Keep `doctor`
      success scoped to local diagnostics rather than usability, and keep
      `sync` from recommending `check` after an empty selection. Verify a
      RED/GREEN public-command journey with the actual `manifests/team.toml`,
      an isolated environment credential backend, no Tokens or clients,
      closed stdin, and zero client or network calls. Preserve existing
      enabled-client outcomes and native preferences.

## 6. Prove the exact product

- [x] 6.1 Run `mise run check` and `mise run native` on the same committed
      product HEAD, with all focused regressions green.
- [x] 6.2 Exercise the actual packaged candidate and shipped team manifest,
      binding every selected native journey to its candidate digest. Verify
      explicit UCloud GPT-6 Sol inference, Claude Code's native override,
      endpoint-only mode, and fixture-backed distributor 503 classification.
- [x] 6.3 Obtain native macOS, Linux, and Windows evidence for the same
      product tree and verify exact commit signatures and the introduced
      range under repository policy. Linux ordinary native acceptance must
      exercise secure-file fallback without a session bus; qualify real Secret
      Service separately in an ephemeral D-Bus/keyring environment. At signed
      `6588bb09`, [GitHub dev run 36104586192](https://github.com/HengYangDS/aigw-cli/actions/runs/36104586192)
      passed all three native jobs and quality; its Linux log shows both
      `secure_file_fallback_without_session_bus` and the isolated
      `system_credential_store` subtest passing. GitLab [dev 8259](http://192.168.64.101:18086/dig/misc/tools/llm-third-party-api/aigw-cli/-/pipelines/8259)
      and [main 8260](http://192.168.64.101:18086/dig/misc/tools/llm-third-party-api/aigw-cli/-/pipelines/8260)
      succeeded at the same SHA. All six commits in `v0.3.0..6588bb09`
      pass `git verify-commit` and the declared subject policy.

## 7. Curate the three-provider team catalogue and local configuration

- [x] 7.1 Reconcile public provider catalogues, the current team manifest,
      local configuration, and prior inference evidence. Keep unauthenticated
      DMXAPI 401 and model listings distinct from unavailable or verified
      inference. Audit vendors missing from the curated manifest, not just
      newer IDs within its existing families. AIHubMix currently lists
      Cohere Command A+, Baidu ERNIE 5.1, Mistral Large 3, Xiaomi MiMo 2.6
      Pro, Step 5 Preview, NVIDIA Nemotron 3 Ultra, Upstage Solar Pro 4,
      Inception Mercury 2.5, Meituan LongCat 2.0, and InclusionAI Ling;
      UCloud also lists MiMo 2.6 Pro. The public owner audit additionally
      surfaced Cohere Command A, Poolside Laguna S 2.1, Agnes 3.0 Flash,
      and Intern S2 aliases: admit the first two on exact Chat text evidence,
      retain the Agnes preview and unresolved Intern alias as exclusions.
      Xiaomi's primary model guidance and
      completed text inference on both listed Accounts admit MiMo. Review
      primary positioning and exact inference for ERNIE 5.1, Mistral Large 3,
      Nemotron 3 Ultra, Solar Pro 4, Mercury 2.5, LongCat 2.0, HY3, stable
      Step 3.7 Flash, and Ling 3.0 Flash; keep the unsupported Command A+
      and preview MAI Thinking 1/Step 5 out. Preserve catalog observations
      and exclusions without mirroring every listed ID into `team.toml`.
- [x] 7.2 Curate `manifests/team.toml` as the shipped Account / canonical Model /
      provider Route / per-client Recommendation contract. Keep canonical and
      Route IDs lower-case, exact-case wire IDs, and one canonical Model per
      other vendor. Admit MiniMax M3 only on proven Account Routes;
      remove redundant ordinary Route labels and derive Account / Model names
      at presentation, preserving channel overrides and explicit client choices.
      Replace full-matrix and Route-ID-equals-wire tests with asymmetric-Account,
      exact-wire, channel-base, native-export, and public-command regressions.
      Do not add tier, purpose, proxy, or unproven Route data.
      Set Claude Opus 5.5 as the primary Claude model on DMXAPI. Set UCloud
      GPT-6 Sol as the Codex and Hermes primary with AIHubMix Sol then
      recovered DMXAPI Sol as same-model setup alternatives. Keep DMXAPI Luna
      as a separate manual Route, not automatic runtime recovery. The `v0.3.1`
      manifest omitted DMXAPI direct Sol after repeated HTTP 503 responses;
      two later exact-wire direct requests completed.
      Restore it as an explicit setup alternative, not runtime failover, and
      keep direct acceptance distinct from a local Proxy endpoint override.
      Preserve every existing explicit Account endpoint and Client Binding.
      Add AIHubMix and DMXAPI Luna Routes only after completed exact-wire
      Responses calls. Preserve explicit UCloud Client Bindings.
      Replace the one DeepSeek identity only after stronger vendor positioning
      and three-Account inference, and add one qualified Doubao Seed 2.1 Pro
      identity with exact provider wire IDs. Preserve any explicit binding to
      a retired incumbent Route until its owner explicitly changes it.
      Replace Grok 4.6 with vendor-positioned Grok 4.7 after complete Responses
      inference on all three Accounts, without losing Account coverage.
      Include Meta Muse Spark 1.3 only on AIHubMix after complete model-carrying
      output; do not confuse DMXAPI Spark IDs with Meta Muse. Include Xiaomi
      MiMo V2.6 Pro only on AIHubMix and UCloud after the vendor's general-model
      positioning and completed exact-wire Responses calls. Add each further
      qualified vendor only on its completed AIHubMix Responses or Chat Route,
      with one base NVIDIA Model for the `-free` channel. Correct UCloud GLM
      5.3 to its completed Chat Completions protocol. Keep Cohere Command A
      and Poolside Laguna S 2.1 as single canonical Models with their
      exact-wire AIHubMix Chat Routes; the newer Command A+ returned HTTP 400.
- [x] 7.3 Reconcile only team-owned local Models and Routes through the AIGW
      configuration owner after a complete dry-run. Preserve Accounts,
      credentials, client selections, native preferences, and foreign state.
      Read back every Route protocol key, including UCloud GLM 5.3 Chat
      Completions, and prove Hermes projection convergence. Preserve the
      locally configured DMXAPI GPT-6 Sol Proxy Route separately from the
      shipped direct-provider manifest.
- [x] 7.4 Prepare exact wire-ID probes from the public catalogues. For
      AIHubMix, try `gpt-6-sol`, `gpt-6-luna`, `claude-opus-5-5`,
      `deepseek-v4.1-flash`, `minimax-m3`, `cc-minimax-m3`, and
      `grok-4.7`; extend the qualified shortlist from 7.1 to new vendors,
      including `command-a-plus-05-2026`, `ernie-5.1`, `mistral-large-3`,
      and `mimo-v2.6-pro` on AIHubMix. For UCloud, try
      `deepseek-v4.1-flash`, `MiniMax-M3`, and `mimo-v2.6-pro`. Record Account,
      attempted protocol, and wire ID for each bounded noninteractive call.
      Verify every retained Route: 54 unchanged Route/Account tuples from the
      57-Route `2ad9c411` matrix plus three newly called Grok 4.7 Routes
      and two AIHubMix Chat Routes for Cohere and Poolside covered all 59 Routes
      in the `db724161` manifest at the product-shaped 512-token cap. One
      direct DMXAPI GPT-6 Sol Responses call at the same cap completed on
      2026-09-25: HTTP 200, completed status, and assistant text. Its probe
      used `https://www.dmxapi.cn/v1/responses`, not the local Proxy loopback.
      Reconciliation of that exact-wire record with the prior 59 observations
      matches all 60 Routes in the current manifest. The observation proves
      availability at that time, not future reliability.
      A listing or omission is not availability evidence:
      retain private UCloud GPT/Claude Routes, leave DMXAPI 401 unknown,
      and do not infer CC/coding prefix semantics or admit H3 as general
      text. A successful call proves availability, not comparative general
      strength. Replace a retained general Model only with current primary
      vendor evidence of stronger general-purpose positioning, successful
      exact inference, and the 7.5 Account-coverage and global-cardinality
      review. Grok 4.7 passed vendor-positioning and three-Account inference;
      a claim about the strongest Gemini Flash does not rank it above Gemini
      Pro. Stop if credentials require Keychain UI, reporting uncalled Routes
      as unverified.
- [x] 7.5 Accept the final candidate-bound `manifests/team.toml` and the
      installed profile exported by `aigw config export` as one semantic
      catalogue across AIHubMix, DMXAPI, and UCloud. Compare Model IDs and
      Route Account/protocol/wire IDs; compare Account endpoint/probe metadata
      while classifying the installed DMXAPI Responses loopback as an explicit
      local Proxy override of the team's direct endpoint, never replacing it
      merely to force parity. Separately read back explicit Client Bindings
      and native projections.
      Require only GPT-6 Astra/Sol/Luna, Claude Fable 5.1/Opus 5.5/Sonnet 5
      identities with Routes only where verified, one qualified general Model
      per other vendor globally, DMXAPI variants mapped to base Models, and a
      7.4 inference observation for every admitted Route. Preserve personal
      recommendations, credentials, native preferences, and foreign fields;
      any lost Account coverage requires an explicit reviewed Route retirement.
      On 2026-09-26, the 27-Model, 60-Route candidate and installed export
      agree on Model and Route definitions. The installed DMXAPI loopback is
      the sole Account endpoint exception; installed import rejects its
      conflict without changing config bytes. All four clients report
      unchanged or already-converged in `sync --dry-run --json`.
- [ ] 7.6 Derive both Forge pipelines from the one CUE product-native matrix;
      remove the separate native-capacity list and stale mirror-only metadata.
      Require each peer's macOS, Linux, and Windows jobs on review, accepted
      branch, and tag events, with peer-local release dependencies. Before
      archive, verify the generated event graph and exact-HEAD review jobs,
      including GitLab's Linux Docker runner and on-demand Windows ARM64 runner.
      Accepted-branch and tag job outcomes are separate post-archive delivery
      acceptance; do not infer them from the graph or review. Keep the current
      Darwin control selector unless a replacement proves every required job.
      Do not count a paused, pending, skipped, or other-peer job as GitLab
      acceptance.
      Separately prove that each peer consumes no AIGW source, policy, evidence,
      or assets from its sibling; disclose third-party tool-host availability
      separately rather than claiming global outage tolerance.
- [x] 7.7 Remove every native OpenSpec validation finding by tightening the
      five overlong canonical requirements without losing their scenarios.
      Make the existing quality gate reject future `INFO` findings as well as
      warnings and errors; verify the official report is clean.
- [ ] 7.8 Compare stable credential-entrypoint options under package-manager
      ownership, then implement the least complex cross-platform solution that
      keeps every retained client invocation working throughout replacement.
      Demonstrate a failing unlink-window test first; then prove concurrent
      invocation, candidate activation, rollback, and re-upgrade on macOS,
      Linux, and Windows without Token migration, ACL changes, credential
      prompts, or a second live selection authority. An active legacy caller
      still using the manager-owned path SHALL hold the first cutover before
      unlink, not be counted as migrated by a config rewrite. Verify owned
      cleanup and retain only artifacts with an active consumer.
