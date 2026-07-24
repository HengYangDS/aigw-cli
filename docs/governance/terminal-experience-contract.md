# Terminal Experience Contract

Status: canonical.

AIGW is a local control plane. Its terminal surface serves the person operating
it before the configuration objects implementing it. Every human-readable
command therefore answers one of three questions:

1. What is selected now?
2. Is it ready?
3. What is the one safe next action?

## Task-first navigation

The root help surface has four ordered sections:

| Section | User intent | Commands |
| --- | --- | --- |
| Connect | Establish the first service | `setup` |
| Use every day | Inspect, select, check, or rotate a route | `status`, `use`, `check`, `rotate` |
| Recover | Diagnose and reconcile bounded local drift | `doctor`, `repair`, `sync`, `rollback`, `update` |
| Advanced | Manage declared configuration objects or explicit diagnostics | `account`, `adapter`, `add`, `balance`, `catalog`, `config`, `models`, `profile`, `route`, `test`, `verify` |

## Interactive and scripting behavior

Renaming commands (`aigw profile rename`, `aigw account rename`) support 0, 1, or 2 positional arguments:

- **0 arguments**: In an interactive environment, AIGW prompts for a source object through a sorted selection list and then asks for the new target ID.
- **1 argument**: AIGW treats the argument as the source ID and prompts only for the new target ID.
- **2 arguments**: AIGW renames the source to the target immediately, providing a stable path for scripts and automation.
- **Non-interactive failure**: If required arguments are missing in a non-interactive environment, the command fails with a clear error rather than hanging for input.

Renaming commands also support `--dry-run` and `--json`. A dry-run must not acquire a mutation lock or write configuration, `.bak`, credentials, or client state. Rename JSON contains neither secret values nor local filesystem paths.

`aigw account rename <old> <new> --finalize` requires both IDs explicitly; it never prompts for either one. A rotation confirmation flag is required only when its corresponding old and new credential slots differ. With the `env` backend, old variables must be unset outside AIGW; until they are absent, finalization exits non-zero and remains incomplete, and the command must be retried. After successful finalization, dry-run JSON reports `already-finalized`.

No command alias is introduced merely to improve presentation. The existing
command grammar remains the stable automation surface.

## Layout

Human output uses an aligned layout when the available terminal width fits the
row. Labels then share a value column. On a narrow terminal, labels occupy one
line and their values follow on separately indented, word-wrapped lines. AIGW
does not truncate output or split a word to fit the display.

A positive `COLUMNS` environment value is a portable width override. Otherwise
AIGW uses the output terminal width only when it can read one; a redirected or
unknown output stream keeps the unbounded layout. JSON output does not consult
terminal width and remains a machine contract.

## Recovery language

Errors use the ordered form **Problem**, **Evidence**, **Impact**, and
**Recommended action**. The final section contains one safe command. It does
not encourage broad configuration edits, client restarts, proxy lifecycle
changes, or Codex conversation mutation.

## Boundary language

If a selected Account points at a loopback endpoint, the status view may say
that an external compatibility layer exists and that the client route uses its
listener. It does not reveal a port, name a specific proxy, or create an
ownership claim. AIGW does not manage that process or diagnose its lifecycle.

## Host-route commands

`aigw route doctor` is a read-only ownership diagnostic. It may inspect local
configuration attribution, but it must not run Codex, Junie, or an IDE; read a
credential; contact a provider; expose configuration bodies or paths; or claim
that authentication, session metadata, endpoint hops, terminal outcomes, or
billing were proven. Junie is reported as not probed. A reported conflict is a
state to investigate, not authority to repair an external host.

For an ordinary route-ownership conflict, the safe next command is
`aigw repair --dry-run`, not a direct write and not an Air fallback restore.
The repair preview is read-only: it does not take the mutation lock, enable a
shim, bind authentication, expose configuration paths, or execute a client. It
maps every planned Codex transition to a stable surface ID, then leaves host
idleness and any later apply as separate operator responsibilities. The exact
Air state `recoverable-stale-full-selection` is different: use
`aigw route recover air --dry-run`. It can remove only a complete AIGW full
selection paired with a mismatched recognized fallback sidecar; it does not
write a JetBrains selection or claim runtime authentication.

An `external-host-mirror` is healthy external management and receives no
mutation guidance. A sidecar-absent `orphaned-exact-full-selection` instead
uses `aigw route recover-orphan air --dry-run --json`. Doctor next actions are
therefore ordered: ADR-0003 recovery for its recognized mismatch, exact-orphan
recovery for its admitted shape, and ordinary `aigw repair --dry-run` for other
conflicts.

`aigw route attest air` is credential-free, lock-free, and read-only. Its
human wording must say bounded forwarding evidence, never authentication,
billing, quota, terminal, or reply proof. `recover-orphan` preview is also
read-only. Apply requires the exact case, an operator idleness attestation, and
the explicit unset-selection acknowledgement; it never supports `--force`.
`settle` preview is read-only, and settle apply mutates only the private ledger
and quarantine, never Air.

`aigw route fallback air` and `aigw route restore air` are explicit mutation
commands, not generic recovery actions. Their `--dry-run` variants are
credential-free previews that acquire no mutation lock and perform no native
authentication binding. Their apply variants require `--confirm-host-idle` and
must state that the flag is an operator attestation rather than an Air process
probe. They never start, stop, restart, or reload Air, and they must fail
closed if Air's top-level selection is already AIGW rather than JetBrains AI.

`aigw route recover air` is an explicit mutation command for the single stale
full-selection/fallback-sidecar mismatch reported by route doctor. Its preview
is credential-free and lock-free; apply requires `--confirm-host-idle` and
removes only AIGW-owned markers, sidecar, and AIGW target membership. It never
starts, stops, restarts, reloads, or authenticates Air, and it returns Air to
an unselected external baseline rather than fabricating a JetBrains setting.

An `orphaned-aigw-marker`, or `partial-or-foreign-residue` whose disk selection
remains `aigw-managed`, is unbound Air residue. It is a diagnostic boundary,
not proof that AIGW can safely remove the marked text. Route doctor therefore
states that no AIGW mutation is admitted and recommends only another read-only
`aigw route doctor --json` report; it must not suggest generic repair or Air
recovery for either state.

Human guidance must distinguish a persistent routing policy from runtime proof:
the desired state is standalone Codex/AIGW and JetBrains-owned PyCharm, Air,
and Junie; a route, endpoint, terminal response, or billing assertion remains
unverified until its separately requested live evidence exists.
