## Context

`client-chosen-model` landed in `dev` at `06edce9a4` on 2026-11-23. Its twelfth review (2026-11-22; the record is
`build/evidence/selection-states-20261121/README.md` in the canonical checkout, section "Version twelve and the
twelfth review", and the probes `build/runtime/work/review-12-chosen/src/internal/cli/acceptance/zz12_review_*_test.go`)
named two facts that each have two sources. This Change removes the second; `one-file-identity` removes the first.

## Goals / Non-Goals

**Goals:** one owner of "is this provider's Token usable, absent, or not observable"; every caller treating "not
observable" as "absent" for the clients that use that Token; findings 2, 3 and 5 failing first and passing after.

**Non-Goals:** the twelfth review's finding 6 (a file added without a Token has no record until the Token is stored),
which converges once the Token is there and contradicts no requirement; how a keychain or file store reports its own
failure, which stays in `internal/secret`; the file-identity judgement, which is `one-file-identity`'s.

## Decisions

### The judgement stays with `secret.ObserveToken`, and its callers stop interpreting its error

- **Rests on:** `ObserveToken` is already the one observation; the two sources differ only in what their callers do
  with its error (`tokenUsable` waits, `SecretAvailable`'s callers fail).
- **Considered:** a new type in `internal/adapter`; leaving `SecretAvailable` and fixing its two callers.
- **Chosen because:** an answer with three values returned by the owner of the observation leaves no caller to decide
  what an error means; `tokenUsable` and `SecretAvailable` become one call, or go.

### A dropping edit withdraws without the Token

- **Rests on:** synchronization already withdraws a client's settings from a file without its Token
  (`withdrawnClients`); the refusal in `admitTargets` existed because the edit would otherwise leave AIGW's settings
  in the dropped file.
- **Considered:** keeping the refusal (the specification's wording until now), which makes the edit's outcome depend
  on the Token, against the rule of `client-chosen-model`.
- **Chosen because:** the edit then ends the same way with the Token and without it, and nothing is left in a
  dropped file.

### The fresh review of `034296053`

- **Rests on:** its findings: `aigw sync` said "aligned" while a client waited for an absent Token, though not for
  one that could not be observed; a Token file refused for its mode was given the fix "unlock the store", because the
  callers asked `NamesTokenFolder` before `ObservationFailure`, whose own fix for that file they never reached; the
  model catalog judged a Token itself (`Exists` and `Get`), so an empty Token file was a backend failure there and
  an absent Token everywhere else.
- **Chosen because:** "aligned" now requires that no selected client waits, by either answer; the predicate is
  `NamesItsFix`, true for every failure that `ObservationFailure` gives its own fix, refused files included; the
  catalog asks `ObserveToken` and reads a Token's value only when it is usable. Each has a test that fails on
  `034296053`.
- **Left as it is:** `aigw use`, `aigw client enable` and guided setup still refuse, for one client, a Token that cannot
  be observed, as they refuse an absent one; the requirement now says that a command ends as it does for an absent
  Token, which these do. When an edit drops the file that holds a model chosen in the client and the Token is not
  usable, the chosen model is recorded and the kept file is written for it once the Token is there.

## Risks / Trade-offs

- **`one-file-identity` touches `admitTargets` too:** it lands first, and this lane merges `dev` before it archives.
- **A store that cannot tell for a long time** leaves its clients waiting rather than failing a command; the result
  names the failure and its fix each time, so it is not silent.
