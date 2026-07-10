# Account and Runtime Profile Model Design

## Problem

AIGW currently treats a Profile as URL + Token + route target. That is not enough once users need to switch between model choices such as `gpt-5.5`, `gpt-5.5-ssvip`, and Claude-specific models on the same upstream gateway account.

Duplicating URL and Token per model would create secret sprawl, make rotation error-prone, and confuse balance diagnostics.

## Decision

AIGW uses three layers:

1. **Account**: one upstream provider account boundary with protocol endpoints, optional account probe, and exactly one Token at `AIGW_TOKEN/<account-id>`.
2. **Runtime Profile**: one selectable runtime choice that references an Account and may define a client scope and model names.
3. **Route**: maps default/Claude/Codex usage to Runtime Profiles.

Users switch Runtime Profiles with `aigw use`. Users rotate Account Tokens with `aigw rotate <account>`. Balance and exact account diagnostics belong to Accounts.

## Data model

```toml
[accounts.dmx]
label = "DMXAPI"

[accounts.dmx.endpoints]
openai_responses = "https://www.dmxapi.cn/v1"
anthropic = "https://www.dmxapi.cn"

[profiles.gpt-5_5]
label = "GPT-5.5"
account = "dmx"
client = "codex"

[profiles.gpt-5_5.models]
codex = "gpt-5.5"

[profiles.claude-sonnet]
label = "Claude Sonnet"
account = "dmx"
client = "claude"

[profiles.claude-sonnet.models]
claude = "claude-sonnet"
```

## Routing rules

- `routes.default` points to a Runtime Profile.
- `routes.overrides.claude` and `routes.overrides.codex` point to Runtime Profiles.
- `Config.Resolve(client, explicitProfile)` returns a resolved Runtime: profile id, profile label, account id, account label, endpoint, and model.
- If a Runtime Profile declares `client`, it may only be used for that client. Empty client means both clients may use it if endpoints exist.

## Adapter projection

- Claude receives `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_BASE_URL`, `AIGW_ACCOUNT`, `AIGW_PROFILE`, and, when set, `ANTHROPIC_MODEL`.
- Codex receives the Account OpenAI Responses URL and a model field when a Codex model is set. Token still goes through `codex login --with-api-key`.

## Compatibility

Legacy v1 configs with profile-owned endpoints are normalized in memory and on next save:

- Each legacy profile becomes an Account with the same id.
- Each legacy profile also remains as a Runtime Profile referencing that Account.
- The existing keyring slot `AIGW_TOKEN/<legacy-profile-id>` remains valid because Account id is unchanged.

New exports should use the Account + Runtime Profile shape. No tokens enter files.

## UX

Daily commands stay simple:

```bash
aigw setup
aigw use gpt-5.5
aigw use gpt-5.5-ssvip --for codex
aigw rotate dmx
aigw balance dmx
aigw check
```

`aigw status` shows both the selected Runtime Profile and backing Account.
