## Why

A second review of this lane, before the 0.8.3 tag, found paths on which the new team setup still loses what a person
did, or reports it otherwise than the specification says:

- In accessible mode, three answers that name no choice at the "What now?" question after a refused Token end setup
  with "Nothing changed": the team file is not applied and the Tokens already accepted are dropped. The owner's rule
  of 2026-11-19 is that no Token's outcome blocks setup as a whole.
- A refused Token, "enter again", then Enter alone is reported as a provider that was never asked. A named provider
  skipped with Enter gets no next step. `--provider x --provider x` asks twice and can report a stored Token as
  rejected. A prompt cancelled after Tokens were accepted says "Nothing changed" without saying that those Tokens
  were not stored.
- `Prompt.OptionalSecret` has no test of its own, and the shipped file's recommendation order, which the guide
  states, is checked by no test.
- Four documents say what the code does not do, and the release notes do not state that AIHubMix's model catalog
  accepts any Token, which this release gives more weight by recommending AIHubMix for every client.

## What Changes

- A provider keeps the last verdict on its Token however the person then leaves its prompt: by choosing to skip, by
  Enter alone, or, in accessible mode, by answers that name no choice, which now skip the provider and no longer end
  setup; setup says that it skipped.
- A prompt that the person cancels after this run accepted a Token says that no such Token was stored.
- A provider named more than once is asked once. A named provider skipped with Enter is reported with its next step.
- A preview in a terminal says that the run asks for Tokens, which the preview never does.
- Tests: `OptionalSecret` in each of its forms; a script that drives the real prompts through a terminal; each
  client's recommendations name at most one model of a provider, in the
  file's provider order; a model that a client can use under any of its protocols counts as serving the family; the
  release test reads the family that Codex is recommended from the file.
- Records: the rollout guide, the route evidence, `README.md`, the qualification record and `CHANGELOG.md` say what
  the code does, and state that a provider whose model catalog answers any Token, AIHubMix on 2026-11-19, cannot have
  its Token proved by setup, `provider add`, `provider rotate` or `use`.

Out of scope: a validation that proves a Token on such a provider, which changes five commands and `secret-storage`
and is the next Change; a preview without a terminal that names a provider without a Token, whose next step the run
refuses; wiring an interrupt signal to the command's context, which no command does today; and the tests that still
repeat the file's values (the store's lists of families and hosts, the release test's protocol), which need a Change
of their own to read structure.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `progressive-team-onboarding`: the Token scenarios state the last verdict, the answers that skip, a cancelled
  prompt, a provider named in a terminal and what a preview assumes; the shipped file's recommendations keep the file's
  provider order.

## Impact

`internal/cli/setup` (the Token flow and its report), `internal/cli/invocation` (what a cancelled prompt says),
`internal/terminal` and `internal/store` tests, `tools/release` tests, `README.md`, `docs/guides/team-rollout.md`,
`docs/research/` and `CHANGELOG.md`. No JSON field or state is added; schema version 4 is unchanged.
