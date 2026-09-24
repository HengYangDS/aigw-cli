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
      `diagnostics.Probe`, `ProbeStable`, and `Result`. Verify: focused
      diagnostics tests demonstrate the same scope in request and result.
- [x] 3.2 Make one inference request that receives `503 no available channel`
      classify as retryable `ModelUnavailable` without a second request.
      Fail unchanged 401/403 without repeated probes. Verify: request-count
      and classification tests in `./internal/diagnostics/...`.
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
- [ ] 4.3 Keep status and doctor locally truthful, and make check use endpoint
      scope for a proven Claude override with
      `aigw verify --for claude` as continuation. Verify: focused human and
      JSON acceptance tests plus a candidate-bound real nonpersistent native
      Claude request in an owned isolated context with noninteractive
      credentials. Preserve the user's current host settings and sidecar bytes.
      The prior `opus[1m]` result is historical: at the 2026-09-24 12:11 UTC
      read, the host settings had no top-level model after a later edit. If an
      isolated request cannot authenticate without UI, report that limit and
      leave native override acceptance open.

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

- [ ] 6.1 Run `mise run check` and `mise run native` on the same committed
      product HEAD, with all focused regressions green.
- [ ] 6.2 Exercise the actual packaged candidate and shipped team manifest,
      binding every selected native journey to its candidate digest. Verify
      explicit UCloud GPT-6 Sol inference, Claude Code's native override,
      endpoint-only mode, and fixture-backed distributor 503 classification.
- [ ] 6.3 Obtain native macOS, Linux, and Windows evidence for the same
      product tree and verify exact commit signatures and the introduced
      range under repository policy. Linux ordinary native acceptance must
      exercise secure-file fallback without a session bus; qualify real Secret
      Service separately in an ephemeral D-Bus/keyring environment.

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
      GPT-6 Sol as the Codex and Hermes primary with AIHubMix as the same-model
      alternative and DMXAPI Luna as the fallback for a sole connected DMXAPI
      Account; a current DMXAPI direct GPT-6 Sol request returned HTTP 503.
      Keep DMXAPI direct Responses acceptance distinct from its locally
      configured Proxy transport; omit the currently failing Sol Route from
      the shipped manifest while preserving any explicit local binding.
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
      and two AIHubMix Chat Routes for Cohere and Poolside cover all 59 Routes
      in the `db724161` manifest at the product-shaped 512-token cap. Retain
      the source and Grok observations under
      `build/verification/765cb24c77bc15bed815d576387ea7c71a5f5ed0/catalog-*`,
      and the new vendor calls under the `00f6aac3` verification directory.
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
      catalogue across AIHubMix, DMXAPI, and UCloud. Compare Model IDs,
      Account endpoint/probe metadata, and Route Account/protocol/wire IDs;
      separately read back explicit Client Bindings and native projections.
      Require only GPT-6 Astra/Sol/Luna, Claude Fable 5.1/Opus 5.5/Sonnet 5
      identities with Routes only where verified, one qualified general Model
      per other vendor globally, DMXAPI variants mapped to base Models, and a
      7.4 inference observation for every admitted Route. Preserve personal
      recommendations, credentials, native preferences, and foreign fields;
      any lost Account coverage requires an explicit reviewed Route retirement.
- [ ] 7.6 Preserve the existing GitLab Darwin tag variable as the single
      selector for native Darwin, quality, accepted-ref parity, release-version,
      and release-assets. Keep the current host binding while re-running source,
      native, packaged-manifest, exact-HEAD, and hosted evidence for changed
      release inputs. Before any VM98 cutover, prove its eligible-ref access and
      actual execution of the declared native and control jobs on the exact
      candidate HEAD; verify job SHA, runner ID, system ID, tag, status, and
      work. An older protected-branch job or an unassigned proposal job is not
      proof. Tag-only jobs need their own exact-tag evidence. Decide whether
      changing the existing variable suffices only after those observations;
      do not add a separate host-release CI lane.

## 8. Publish and install

- [ ] 8.1 Archive the completed Change through the official OpenSpec and
      ETHOS transition after checking merged canonical requirements and
      inherited scenarios.
- [ ] 8.2 Publish one signed stable release with identical assets and
      checksums through the existing GitHub and GitLab workflows. Verify
      remote refs, exact-tag CI job runners, release objects, and every
      published asset byte; separately retain host signing and accepted Apple
      notarization evidence for the exact macOS candidate.
- [ ] 8.3 Publish and install the matching Homebrew Cask, exercise a bounded
      rollback and forward restoration, then verify the installed executable,
      UCloud routes, Claude preference preservation, and both check scopes.
- [ ] 8.4 Preserve recovery material and foreign state, retire only proven
      owned Lane residue, and verify exact worktree, ref, lease, and artifact
      outcomes.
