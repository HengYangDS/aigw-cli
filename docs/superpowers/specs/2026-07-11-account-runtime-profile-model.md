# Account and Runtime Profile Contract

## Purpose

AIGW separates upstream service identity from model choice. This lets users select GPT and Claude models on the same Account without duplicating URLs or Tokens; provider-native diagnostics remain optional and explicit.

## Decision

AIGW uses three layers:

1. **Account**: one upstream provider account boundary with protocol endpoints, optional Provider Diagnostics declaration, and exactly one Token at `AIGW_TOKEN/<account-id>`.
2. **Runtime Profile**: one selectable runtime choice that references an Account and may define a client scope and model names.
3. **Route**: maps default/Claude/Codex usage to Runtime Profiles.

Users switch Runtime Profiles with `aigw use`. Users rotate Account Tokens with `aigw rotate <account>`. Generic health diagnostics work for every Account; exact provider-native diagnostics are available only when explicitly declared and bundled in the installed AIGW version.

## Data model

```toml
[accounts."team-gateway"]
label = "Team Gateway"

[accounts."team-gateway".endpoints]
openai_responses = "https://gateway.example/v1"
anthropic = "https://gateway.example"

[profiles."gpt-5.6-terra-cdx"]
label = "GPT-5.6 Terra Codex"
purpose = "Codex 代码与工程"
account = "team-gateway"
client = "codex"

[profiles."gpt-5.6-terra-cdx".models]
codex = "gpt-5.6-terra-cdx"

[profiles."claude-fable-5"]
label = "Claude Fable"
purpose = "默认 Agent"
account = "team-gateway"
client = "claude"

[profiles."claude-fable-5".models]
claude = "claude-fable-5"
```

## Routing rules

- `routes.default` points to a Runtime Profile.
- `routes.overrides.claude` and `routes.overrides.codex` point to Runtime Profiles.
- `Config.ResolveRuntime(client, explicitProfile)` returns a resolved Runtime: profile id, profile label, account id, account label, endpoint, and model.
- If a Runtime Profile declares `client`, it may only be used for that client. Empty client means both clients may use it if endpoints exist.

## Adapter projection

- Claude receives `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_BASE_URL`, `AIGW_ACCOUNT`, `AIGW_PROFILE`, and, when set, `ANTHROPIC_MODEL`.
- Codex receives the Account OpenAI Responses URL and a model field when a Codex model is set. Token still goes through `codex login --with-api-key`.

## UX

Daily commands stay simple:

```bash
aigw setup
aigw use gpt-5.6-terra-cdx --for codex
aigw use claude-fable-5 --for claude
aigw rotate team-gateway
aigw check
```

When the team manifest explicitly enables a bundled Provider Diagnostics integration, users may also run `aigw balance <account>`.

`aigw status` shows both the selected Runtime Profile and backing Account.
