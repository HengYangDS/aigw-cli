## Context

See `proposal.md`. `internal/secrets` already owns Token persistence and
`internal/platform` owns portable AIGW paths. The existing selector chooses a
keyring without proving that the service can be used.

## Goals / Non-Goals

**Goals:**

- Preserve one storage authority per installation.
- Make the default usable on supported non-Windows hosts without weakening
  file ownership and permission checks.
- Keep selection testable from explicit platform, environment, path, and probe
  inputs.

**Non-Goals:**

- Migrating or probing Codex, Claude, Proxy, browser, or IDE credentials.
- Dual-read, dual-write, implicit Token migration, or cross-provider routing.
- Replacing Windows Credential Manager with file storage.

## Decisions

### Selection is an explicit context

Replace positional selector inputs with a context containing the target OS,
AIGW data path, environment lookup, and a native-store availability probe.
This keeps host discovery at composition boundaries and makes all branches
deterministic in tests. A global platform probe was rejected because it would
hide authority and make cross-platform tests dependent on the executing host.

### Availability uses a disposable service-scoped probe

Automatic macOS/Linux selection reads one reserved, intentionally absent probe
key. `not found` proves the service is reachable; service errors mean it is
unavailable. The probe never writes or inspects user Tokens and therefore does
not create a credential or request access to one. Guessing from environment
variables or desktop-session presence was rejected because neither proves the
service.

### Selection is persisted before normal use

The chosen backend name is stored as an owner-only marker below the AIGW data
directory. Once present, it is authoritative until the operator explicitly
changes it. Searching multiple stores was rejected as a parallel source of
truth.

### File storage uses one directory of per-Account files

Each validated Account identifier maps to one file beneath the secrets
directory. Writes use a same-directory temporary file, sync, rename, and
directory sync. A single JSON map was rejected because unrelated Account
updates would share one failure and contention boundary.

## Risks / Trade-offs

- **A previously selected keyring later becomes unavailable** → fail clearly;
  do not switch stores and conceal where Tokens live.
- **Portable permission APIs differ** → file fallback is admitted only on
  macOS/Linux; Windows retains Credential Manager.
- **Interrupted first selection** → write the marker atomically only after the
  chosen backend has passed its availability check.

## Migration Plan

Existing installations with a usable native credential service select and pin
`keyring`; no Token bytes move. Operators who explicitly select `keyring` or
`env` preserve that exact behavior. Rollback removes only this release before
any file-backend Token is created; there is no compatibility reader.
