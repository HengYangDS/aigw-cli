## 1. Real Codex Verification Authority

- [x] 1.1 Add RED domain tests proving Codex verification must build a
      non-persistent real-client process plan, use one synchronized target, and
      reject missing model, target, executable, capture capability, or response
      marker; verify the focused tests fail for the absent behavior
- [x] 1.2 Add RED CLI acceptance tests proving `aigw verify --for codex`
      invokes the configured Codex executable rather than `runtime.HTTP`, reports
      client version and SHA-256 without Token or response content, and performs
      only one request for multiple targets; verify the focused tests fail for the
      current HTTP-only implementation
- [x] 1.3 Implement the shared Codex executable identity and verification plan,
      route the public verify command through `process.CaptureRunner`, and verify
      the domain and CLI acceptance tests pass
- [x] 1.4 Delete the superseded Codex HTTP request and response parser from the
      verification package, then verify repository search and focused tests show a
      single Codex live-verification owner

## 2. Composition Contract and Real Journey

- [x] 2.1 Update contribution guidance to name `aigw verify` as the real-client
      evidence command and verify Markdown format, lint, and link checks pass
- [x] 2.2 Build the exact candidate and run an isolated-home UCloud direct HTTPS
      journey through setup, sync, authentication, check, and real Codex verify;
      verify the operator home and running Proxy service remain unchanged
- [x] 2.3 Run focused Go tests for Codex, verification, CLI verification, and
      acceptance packages, then run `mise exec --locked -- go run ./tools/ci source`
      with no warnings or errors

## 3. Source Readiness

- [x] 3.1 Freeze the implementation overlay and run one full ETHOS proof with
      all required gates passing
- [x] 3.2 Verify the canonical `product-control-plane` delta contains the
      endpoint-neutral and real-client requirements and that the intended commit
      introduces no parallel verifier or gateway lifecycle owner

The signed implementation commit, exact-HEAD proof, official archive transition,
post-archive proof, landing, Forge publication, release, installation acceptance,
and Work Lane retirement are lifecycle effects performed after this editable
Change is complete.
