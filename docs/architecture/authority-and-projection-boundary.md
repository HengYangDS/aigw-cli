# Authority and Projection Boundary

Status: canonical.

AIGW resolves local Account, Profile, Route, and Adapter configuration. It
manages only admitted client surfaces; it does not proxy API traffic or own
client lifecycle.

```text
AIGW configuration
  Account -> selected endpoint -> external service
  Route   -> Claude -> process-only environment
          -> Codex  -> admitted target -> guarded projection -> compensation
Client runtime -> sessions, model selection, transcripts, and lifecycle
```

| Surface | Authority | AIGW contract |
| --- | --- | --- |
| Standalone Claude Code | AIGW-owned launcher | Inject the selected Account Token only into the launched process. |
| Standalone Codex home | AIGW projection | Manage the marked provider/model block, sidecar, and native credential binding. |
| Existing Codex conversation | Client runtime | Never edit its model, transcript, JSONL, SQLite, or metadata. |
| IDE or other client integration | Owning client | Never discover, diagnose, repair, or control it. |
| Responses compatibility service | Service operator | Treat it as an ordinary HTTP endpoint; never install or manage it. |

AIGW neither imports nor manages a Responses compatibility service, and such a
service needs no AIGW installation. The products compose only when an operator
selects the service's HTTP endpoint for an Account; that creates an ordinary
request-path dependency, not lifecycle or ownership coupling.

## Source topology

The physical packages follow the behavior they own:

| Package | Semantic owner |
| --- | --- |
| `internal/configuration` | Account, Profile, Route, Adapter schema, validation, persistence, and token-free manifests. |
| `internal/secrets` | Account Token storage backends. |
| `internal/account` | Optional provider-native diagnostic credentials and results; not Account configuration. |
| `internal/credential` | Provider-neutral endpoint authentication validation. |
| `internal/providers` | Explicitly bundled provider-native diagnostics. |
| `internal/claude` | Claude process plans and the AIGW-owned launcher. |
| `internal/codex` | Standalone Codex projection planning, inspection, reconciliation, and native login plans. |
| `internal/synchronization` | Configuration, Codex projection, and Codex authentication convergence. |
| `internal/discovery` and `internal/surface` | Read-only host discovery and stable surface authority. |
| `internal/cli` | Cobra composition and root workflows; semantic command groups live in its subpackages. |
| `internal/selfupdate` | Dual-Forge update resolution, artifact verification, installation, and portable rollback. |
| `internal/process` and `internal/transaction` | Bounded process execution and guarded filesystem mutation. |

A Codex projection comprises `configuration.toml` plus its
`.aigw-state.json` sidecar. The sidecar records the projection mode, writer ID,
and transaction ID. A legacy sidecar with none of those fields is adoptable
only as a full-selection legacy projection; partial, foreign, and
mode-mismatched attribution fails closed.

Each reconciliation captures byte-exact pre-state for both artifacts, including
sidecar absence, and validates every target before its first write. Commits use
guarded preimage checks. On failure, compensation runs in reverse order only
while an artifact still equals this transaction's postimage, so a newer writer
is never overwritten. This is process-local coordination, not a cross-process
or crash-safe compare-and-swap.

`aigw sync --dry-run --json` exposes the target/action plan without reading
credentials, binding authentication, starting a client, or changing files.
Generic setup and repair discover only the ordinary standalone Codex CLI home.
An unknown path is admitted only when the operator explicitly configures it as
a standalone Codex target.

A loopback endpoint is reported as an external compatibility layer. AIGW does
not reveal the port, infer a proxy product, or claim transport ownership.
Availability, service configuration, health diagnosis, watchdogs, deployment,
and rollback remain the service operator's responsibility.
