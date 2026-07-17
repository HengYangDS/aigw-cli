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

`aigw route fallback air` and `aigw route restore air` are explicit mutation
commands, not generic recovery actions. Their `--dry-run` variants are
credential-free previews that acquire no mutation lock and perform no native
authentication binding. Their apply variants require `--confirm-host-idle` and
must state that the flag is an operator attestation rather than an Air process
probe. They never start, stop, restart, or reload Air, and they must fail
closed if Air's top-level selection is already AIGW rather than JetBrains AI.

Human guidance must distinguish a persistent routing policy from runtime proof:
the desired state is standalone Codex/AIGW and JetBrains-owned PyCharm, Air,
and Junie; a route, endpoint, terminal response, or billing assertion remains
unverified until its separately requested live evidence exists.
