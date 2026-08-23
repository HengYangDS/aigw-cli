# Product Concepts

AIGW has four configuration entities and two external boundaries.

```mermaid
flowchart LR
    A["Account"] --> P["Profile"]
    P --> R["Route"]
    R --> D["Adapter"]
    A --> K["Selected Token slot"]
    D --> C["Native client"]
```

## Core entities

| Entity | Meaning | Cardinal rule |
| --- | --- | --- |
| Account | One provider endpoint and logical Token boundary | Token belongs to the Account, not a Profile |
| Profile | One `account + client + model` choice | Client scope is explicit |
| Route | Default or client-specific Profile selection | No hidden provider fallback |
| Adapter | Projection into one native client | Never writes another client's surface |

## Account

An Account contains:

- a human label;
- an OpenAI Responses endpoint, an Anthropic endpoint, or both;
- one logical Token stored by the selected local backend;
- an optional provider-native diagnostic declaration.

Configuration and manifests never contain the Token.
The reviewed distribution is [`manifests/team.toml`](../../manifests/team.toml);
it is the sole tracked team configuration and is directly consumable by
`aigw setup --from`.

## Profile

A Profile is the daily model choice for one client. The team manifest carries
the reviewed profile IDs and model IDs; operators select those IDs directly
instead of copying a second illustrative configuration.

Profile and model IDs are transparent operator-defined strings. AIGW does not
infer a provider, capability, or version policy from their spelling.

## Route

```bash
aigw use dmxapi-gpt-5.6-sol --for codex
aigw use dmxapi-claude-fable-5 --for claude
aigw route reset claude
```

A client-specific route overrides the default. Reset removes only that override.
AIGW selects before the request; it does not retry traffic through another
service or model.

## Adapter

| Adapter | Projection |
| --- | --- |
| Codex | AIGW-marked provider/model configuration and native credential binding |
| Claude Code | Official user-settings endpoint/model projection and credential helper |

Adapters do not own provider behavior. Missing clients remain untouched.

## Endpoint

- Claude Code consumes the Account's Anthropic endpoint.
- Codex consumes the Account's OpenAI Responses endpoint.
- HTTPS is required except for an explicit loopback Account.
- A loopback process remains external to AIGW lifecycle ownership.

## Provider diagnostics

Routing and endpoint checks are provider-neutral. Exact balance or account state
is an optional leaf capability declared by `account_probe` and implemented by a
bundled diagnostic provider.

An unknown diagnostic kind does not invalidate the Account or Route. It makes
only the optional diagnostic unavailable.

## Manifest import

A token-free team manifest adds or reconciles public metadata. Same-named
Accounts and Profiles must match or import stops before mutation. Explicit
replacement changes metadata only; it never changes the Token slot.

## Rename

| Operation | Changes | Preserves |
| --- | --- | --- |
| `profile rename` | Profile ID and Route references | Account and Token |
| `account rename` | Account ID and Profile references | Token through a two-phase migration |
| `account rename --finalize` | Removes verified old credential slots | Current configuration and checkpoint |

Finalize fails closed if credential equality or checkpoint proof is incomplete.

## Installation lifecycle

All platforms use one archive and one CLI-owned lifecycle: `aigw install`,
`aigw update`, `aigw update --rollback`, and `aigw uninstall`. Updates replace
the binary atomically and retain exactly one immediate predecessor. There is no
parallel package-manager channel.
