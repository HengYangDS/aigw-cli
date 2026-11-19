## Why

A user applied the shipped team file with `aigw setup --from` holding only an AIHubMix Token. Setup asked for the
DMXAPI Token alone, because every client's first recommendation is on DMXAPI. An empty answer was refused by the
prompt, and the AIHubMix Token typed instead was rejected by DMXAPI; each rejection discarded the whole team file. The
error named Codex ("Codex authentication was rejected"), which reads as AIGW misjudging a good Token.

The owner's rule (2026-11-19): a user may hold any subset of the providers' Tokens; a missing, skipped or rejected
Token must never block setup as a whole.

## What Changes

- **BREAKING** `aigw setup --from` always applies the team file. A Token that its provider does not accept leaves that
  provider without a Token and is reported with the provider's name, the reason and the step that stores one; the
  other providers and the clients they can serve are set up in the same write.
- In a terminal without `--provider`, setup first asks which of the team file's providers without a Token the person
  has a Token for, and allows none; it then asks only for those Tokens. A Token that is not accepted can be entered
  again or skipped.
- A piped Token (`--provider <id> --token-stdin`) that is not accepted no longer prevents the write; the command exits
  1, so a script still sees that its Token was not stored.
- A Token rejection names the provider that rejected it, in every command that validates a Token.
- The shipped team file recommends, for each client, a model on every provider that serves the client's recommended
  family, so a Token of any one provider can configure every client it can serve.

Out of scope: guided setup without `--from`, `aigw provider add` and `rotate` (one provider each, already scoped to
the Token they ask for), and adding Suhe.AI to the team file, which its own Change does.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `progressive-team-onboarding`: team setup connects Tokens without requiring them; the shipped team file's
  recommendations cover every provider that serves a recommended family.

## Impact

`internal/cli/setup` (team Token flow and report), `internal/terminal` (a multiple-choice prompt),
`internal/cli/invocation` (the prompt interface), `internal/secret/probe` (the rejection's wording),
`manifests/team.toml`, their tests and `CHANGELOG.md`. A script that relied on a rejected piped Token leaving the
configuration untouched now finds the team file applied; the exit status is unchanged.
