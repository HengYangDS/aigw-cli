## Context

`followed_selections` is a list of entries with `client`, `model` and `earlier` (`internal/cli/invocation/followed.go`);
`model` is the model reference as `provider/model`, the same string that `adopted_selections` carried per client.

## Goals / Non-Goals

**Goals:**

- The journey asserts the same fact as before: the preview names the model chosen in Hermes's file as the one AIGW
  follows for Hermes.

**Non-Goals:**

- Any assertion of `earlier`, `returns_with` or the other fields; the acceptance suite covers them.

## Decisions

### Read the Hermes entry of `followed_selections`

- **Rests on:** the candidate's failure output and the field's definition above.
- **Considered:** asserting the whole entry; asserting the model of the Hermes entry.
- **Chosen because:** the journey's purpose is the client's own choice reaching AIGW with real clients; the model is
  that fact, and the rest is owned by the acceptance tests.
- **Applies to:** `tools/release/clients_hermes_test.go`.

## Risks / Trade-offs

- The journey runs only with the native clients, so the fix is proven by the 0.10.0 candidate's acceptance against
  `v0.9.0` on the release head that carries it, a step of the release route after this Change's archive and proof, not
  by the check; a later step of the same journey could still fail there.
