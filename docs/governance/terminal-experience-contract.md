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

## Client and transport boundaries

Route commands manage only AIGW's default and per-client Profile selections.
They do not inspect or control IDEs, desktop clients, external proxy processes,
or conversation state. `aigw repair --dry-run` is the safe preview for a local
projection problem; it takes no mutation lock, binds no authentication, exposes
no configuration body, and executes no client.

When an Account selects a loopback endpoint, human guidance calls it an external
compatibility layer and leaves its service lifecycle unverified. Authentication,
endpoint passage, terminal output, billing, and a visible reply require separate
live evidence; configuration alone proves none of them.
