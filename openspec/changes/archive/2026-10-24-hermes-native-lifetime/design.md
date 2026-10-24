# Design

## Context

`aigw verify --for hermes` runs `hermes --version` and a one-shot `hermes chat`
in a disposable home that `hermesconfig.CreateVerificationHome` creates. A
chat that finds no gateway for its home starts a detached
`gateway run --idle-exit`, which ends only after
`gateway.unmanaged_idle_exit_seconds` (600 by default) without clients or
work. The running gateway holds an exclusive `flock` on `<home>/gateway.lock`;
Hermes itself treats that lock as the authority on whether a gateway runs.
Host-wide locks live in `HERMES_GATEWAY_LOCK_DIR`, which the verification
inherited from the operator environment. `verifyIsolated` deleted the home in
a deferred call as soon as the probes returned.

`process.DiagnosticFailure` decides whether a zero-exit client wrote an
explicit warning or error. It matches words such as `warning` and `error`, and
many callers share it: client verification, CI checks and release evidence.

Hermes 0.22.5 freezes a session's launch policy. `chat --resume` refuses any
creation option whose value differs from the launch, such as `--max-turns` or
`--ignore-rules`.

## Goals / Non-Goals

**Goals:** no disposable home is deleted while its gateway runs; the gateway
of a verification ends within seconds and never touches operator locks;
explicit warning symbols fail qualification; the release journey follows the
native session contract.

**Non-Goals:** a new daemon, controller or signal-based termination; changes
to Hermes; changes to the operator's Hermes home or its gateway; accepting
labels as client values.

## Decisions

### The verification policy bounds the gateway's life

The verification configuration sets `gateway.unmanaged_idle_exit_seconds` to
2, beside the other verification policies, and the verification environment
points `HERMES_GATEWAY_LOCK_DIR` into the home. Codex observed the gateway's
absence about 3.5 seconds after its last request with this setting. A short
idle setting alone does not authorize deletion, because Hermes keeps a gateway
alive whenever a probe cannot read its state.

### The per-home gateway lock decides deletion

`hermesconfig.GatewayAbsent` tries a non-blocking exclusive `flock` on
`<home>/gateway.lock`, the same probe Hermes uses, and releases it at once. A
missing lock file means no gateway started. Verification polls it for up to
15 seconds; if the lock stays held, it runs `hermes gateway stop` once with
closed input for that home, which refuses an active turn, and polls for up to
15 seconds more. Signals and Hermes's private control verbs were rejected:
the native stop already refuses active work. The home's remover checks the
lock again and retains the home with a named error when it is still held, so
no caller can delete a home under a live gateway.

### Warning symbols count only at the start of a line

A line whose first visible character is `⚠`, `✗`, `✖`, `✘` or `❌` is a
diagnostic. A symbol in the middle of a line, check marks and progress glyphs
such as `↻` are not. Matching the whole Hermes message was rejected because
the classifier is shared and the defect is the symbol class, not one message.

### The release journey creates once and resumes by identifier

The first lifecycle turn creates a titled session with
`--continue <title> --create-if-missing` and `--oneshot`, and reads the
`session_id:` line Hermes prints. Hermes creates a titled session first and
then treats the turn as its resume, so it refuses `--max-turns` and
`--ignore-rules` there as well; the journey passes no frozen option on any
turn. An untitled session was rejected: Hermes then asks the model for a
title in a background request that is not part of the turn. Later turns pass
`--resume <id>`. The journey stops its gateway natively and waits for the
lock before its temporary home is removed.

### A retained Hermes session keeps its creation route

Hermes 0.22.5 freezes the whole profile configuration of a CLI session when
it creates the session; only Bot Chat and migrated sessions follow the
profile. After `aigw use --forwarding-endpoint`, a new session uses the
forwarding endpoint, while the retained journey session keeps its original
route and credential reference. The journey therefore requires the retained
session to keep completing after the selection and its rollback, as the
forwarding requirement's retained-client scenario states, and no longer
requires it to switch endpoints. Rewriting the frozen session was rejected:
AIGW manages new-session defaults, not a native session's route.

The journey's native-selection step also still expected `aigw sync` to refuse
a model chosen in Hermes, although synchronization has adopted such a choice
since 0.4.2; the step now expects the dry run to report the adopted Route.

## Risks / Trade-offs

- [A slow gateway delays verification by up to 30 seconds] → the bound is
  explicit and the home is retained, not deleted, when it is exceeded.
- [A tool prints a leading warning symbol in normal output] → that output is
  a warning by definition; check marks and other glyphs remain ordinary.

## Migration Plan

None. Homes that earlier releases left behind contain only verification
state; the operator may delete them once no gateway holds their lock.

## Open Questions

- None.
