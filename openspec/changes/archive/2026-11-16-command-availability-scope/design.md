## Context

`secret.ObserveAvailability` starts a scope in which the presence and the emptiness of a slot are asked once;
a scope forgets a slot that the operation changes. Seven commands open such a scope for themselves:
`config rollback`, `setup`, `use`, `sync`, `check`, `client enable` and `client edit`. `provider add`,
`provider edit`, `provider remove`, `provider rotate` and the `model` commands open none. This was a finding
that the Change `keychain-predecessor-retirement` kept for a Change of its own.

## Goals / Non-Goals

**Goals:**

- Every command asks the credential backend for each fact of a Token once.
- A command that is added later gets that without knowing of it.

**Non-Goals:**

- A cache of Token values, or any cache that outlives one command.
- A change of the native operation's bound.

## Decisions

### One availability scope for each command, opened where the command starts

- **Rests on:** the third review of `keychain-predecessor-retirement`, which ran
  `aigw provider edit one --endpoint …` at `ce808d89` against the acceptance tests' worker: it was asked
  `exists one` twice and `empty one` twice, once for the preview and once for the commit. The seven sites
  that open a scope, and the commands without one, were read at `dbcded17`. The acceptance tests record every
  operation that reaches the worker, so a count is a test's assertion and not an estimate.
- **Considered:** a scope in each command that lacks one; one scope where every command's context is built; an
  observation cache in the store for the life of the process.
- **Chosen because:** a scope in each command is the same rule at every command and is missing at the next one.
  A cache in the store has no end: the tests run many commands in one process, and a scope that outlived a
  command would answer the next one with the last one's reading. A command is the unit in which one reading
  is meant to hold, and the place where its context is built is the one site that every command passes.
- **Applies to:** the presence and the emptiness of a provider's Token, and the presence of its balance
  credentials, in every command: both views share the one scope, so `provider remove`, which asked twice
  whether balance credentials exist, asks once. It does not apply to a credential's value, which is read from
  the backend each time, nor to the credential command, which reads a value and observes nothing. Nor does it
  apply to naming the variables that the read-only environment backend sees set, for the line that tells a
  person which to unset (`provider remove`, `provider balance`): that reads the process's own environment, in
  memory, and is no operation of a credential service.

### A reading holds for the command, and the command's own change ends it

- **Rests on:** `TestAScopeForgetsTheEmptinessOfASlotThatItChanges` and
  `TestAvailabilityObservationInvalidatesBeforeMutation` in `internal/secret`: a scope drops what it observed of a slot when the operation sets or deletes that slot.
  AIGW's commands that change configuration hold the configuration lock, so two of them do not run at once
  over one configuration.
- **Considered:** observe again before every decision; hold one reading for the command.
- **Chosen because:** a command decides its admission, its preview and its change from one state; two
  readings could disagree and plan a client that the change then leaves out. What another program does to a
  slot while a command runs is seen by the next command, as it is today inside the seven scopes.
- **Applies to:** a command that waits for a person too: guided setup observes before it asks for a Token and
  observes again after it has stored one. It applies to a command's undo as to its change: the undo of a
  stored Token wrote through a view outside the scope when AIGW had selected the backend itself, so the scope
  kept the Token as present and `aigw use` did not say that the Token was not kept. The command-level test of
  task 1.2 found it; the undo now writes through the scope.

## Risks / Trade-offs

- A command that runs for minutes acts on a reading from its start → the same holds inside today's scopes;
  the reader check before a client file is written reads the Token itself and does not use a scope.
- The acceptance tests' expected operation lists change where a command asked twice → each changed list is
  shorter, never longer; a longer list is a defect.

## Migration Plan

One commit of code and tests. Reverting it restores the per-command scopes.
