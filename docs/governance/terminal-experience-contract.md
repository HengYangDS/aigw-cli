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
that an external compatibility layer exists. It does not reveal a port, name a
specific proxy, or create an ownership claim. AIGW neither requires nor manages
that process.
