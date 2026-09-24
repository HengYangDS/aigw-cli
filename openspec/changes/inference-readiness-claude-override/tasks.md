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
      JSON acceptance tests plus a real nonpersistent native Claude request.

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

## 6. Prove the exact product

- [x] 6.1 Run `mise run check` and `mise run native` on the same committed
      product HEAD, with all focused regressions green.
- [x] 6.2 Exercise the actual packaged candidate and shipped team manifest,
      binding every selected native journey to its candidate digest. Verify
      explicit UCloud GPT-6 Sol inference, Claude Code's native override,
      endpoint-only mode, and fixture-backed distributor 503 classification.
- [x] 6.3 Obtain native macOS, Linux, and Windows evidence for the same
      product tree and verify exact commit signatures and the introduced
      range under repository policy.

## 7. Curate the three-provider team catalogue and local configuration

- [x] 7.1 Reconcile public provider catalogues, the current team manifest,
      local configuration, and prior inference evidence. Keep unauthenticated
      DMXAPI 401 and model listings distinct from unavailable or verified
      inference.
- [x] 7.2 Curate `manifests/team.toml` to the requested logical families,
      retaining only evidenced Route variants and exactly one previously
      qualified general model per other admitted vendor. Verify canonical
      native export and semantic Route/model tests.
- [x] 7.3 Reconcile only team-owned local Models and Routes through the AIGW
      configuration owner after a complete dry-run. Preserve Accounts,
      credentials, client selections, native preferences, and foreign state.
- [ ] 7.4 First probe AIHubMix/UCloud `deepseek-v4.1-flash` and AIHubMix
      `minimax-m3` / UCloud `MiniMax-M3` with bounded noninteractive inference,
      recording each exact Account, attempted protocol, and wire ID. Keep H3
      outside general-text Routes and DMXAPI candidates unknown on
      unauthenticated 401. Admit or replace Routes only after successful calls,
      then call each retained Route once. Stop at a credential boundary that
      requires Keychain UI; report such calls as unverified.
- [ ] 7.5 Re-run source, native, packaged-manifest, exact-HEAD, and hosted
      evidence for the changed release inputs before publication.

## 8. Publish and install

- [ ] 8.1 Archive the completed Change through the official OpenSpec and
      ETHOS transition after checking merged canonical requirements and
      inherited scenarios.
- [ ] 8.2 Publish one signed stable release with identical assets and
      checksums through the existing GitHub and GitLab workflows. Verify
      remote refs, hosted CI, release objects, and every published asset byte.
- [ ] 8.3 Publish and install the matching Homebrew Cask, exercise a bounded
      rollback and forward restoration, then verify the installed executable,
      UCloud routes, Claude preference preservation, and both check scopes.
- [ ] 8.4 Preserve recovery material and foreign state, retire only proven
      owned Lane residue, and verify exact worktree, ref, lease, and artifact
      outcomes.
