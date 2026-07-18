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

Human guidance must distinguish a persistent routing policy from runtime proof:
the desired state is standalone Codex/AIGW and JetBrains-owned PyCharm, Air,
and Junie; a route, endpoint, terminal response, or billing assertion remains
unverified until its separately requested live evidence exists.
