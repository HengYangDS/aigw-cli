## Context

See `proposal.md` for motivation and
`specs/product-control-plane/spec.md` for observable requirements. AIGW already
owns Codex projection and authentication plans, while the shared process runner
already owns bounded, cross-platform child execution. The remaining mismatch is
that the Codex branch of `aigw verify` bypasses both authorities and sends HTTP
itself.

## Goals / Non-Goals

**Goals:**

- Make the public `aigw verify` command the single live-verification authority.
- Exercise the configured Codex executable through an already synchronized
  target without mutating the operator's client state.
- Bind success to the exact executable identity and the expected response.

**Non-Goals:**

- Add another journey CLI, script, state file, or verification protocol.
- Manage Codex Responses Proxy or encode provider-specific behavior.
- Replace setup, sync, credential, or Codex projection ownership.

## Decisions

### Use the existing Codex adapter and process runner

Add a Codex execution-plan constructor beside the existing login plans. It
receives the executable, selected target home, and resolved runtime, and returns
one non-interactive `codex exec --ephemeral` process plan. Verification executes
that plan through `process.CaptureRunner`, inheriting its output ceiling,
timeout cancellation, and native Windows process handling.

A second HTTP verifier or a new external journey program was rejected because
it would preserve the current false proof and duplicate policy. Shell scripts
were rejected because they would weaken Windows portability and process
ownership.

### Select one already governed target

The verification command first requires an enabled Codex adapter with at least
one target and validates the chosen target against the selected runtime. It
uses the first target in the adapter's canonical order. This proves the product
path once without multiplying a quota-consuming request across equivalent
homes.

### Measure the executable used

The Codex package exposes one read-only identity operation that returns the
client's public version string and SHA-256 digest. Catalog verification and live
verification share that operation rather than maintaining duplicate hashing or
version probes. The CLI reports only this identity and the selected Profile;
credentials and response bodies remain private.

### Preserve command boundaries

`aigw verify` remains read-only with respect to AIGW configuration and client
projection. It does not authenticate, synchronize, or repair implicitly. Its
errors direct the operator to the existing owner command for the failed
precondition.

## Risks / Trade-offs

- **Codex command-line changes** -> Cover the exact argument and environment
  plan with unit tests and prove it against the repository-admitted real client.
- **A target can drift between readiness and execution** -> Validate immediately
  before invocation and fail without mutation.
- **A client can emit extra diagnostic output** -> Ignore captured diagnostics
  and accept only the bounded final-message file produced by the client.
- **Executable identity measurement adds one process and one file read** -> Keep
  it within the explicit quota-consuming verify command, where provenance is
  worth the small cost.

## Migration Plan

1. Add failing domain and CLI acceptance tests that require a Codex process
   invocation and reject HTTP-only success.
2. Add the shared Codex identity and execution plan, then route `aigw verify`
   through it.
3. Remove the obsolete Codex HTTP request/parser from the verification package.
4. Run focused tests, the source gate, and an isolated real-client direct HTTPS
   journey.
5. Run exact-HEAD proof once after the candidate is frozen; archive and publish
   through the existing ETHOS lifecycle.

Rollback is a single product commit revert; no persisted schema or external
service state changes.
