# Concepts

## Profile

A Profile is one provider account boundary: a name, display label, supported protocol endpoints, and exactly one logical Token. The secret is stored at `AIGW_TOKEN/<profile>` in the operating-system credential store; it is never embedded in the Profile document.

Create separate Profiles when the same provider gives you multiple Tokens:

```text
dmx-main
dmx-backup
company-gateway
```

## Endpoint

A Profile may provide an Anthropic endpoint, an OpenAI Responses endpoint, or both. Claude consumes the Anthropic endpoint; Codex consumes the OpenAI Responses endpoint. HTTP is accepted only for loopback compatibility tools such as a separately managed local proxy.

## Route

The default route is the normal Profile. Claude and Codex inherit it unless a client-specific override exists:

```bash
aigw use dmx-main
aigw use company-gateway --for claude
aigw route reset claude
```

The last command removes the override; it does not copy the current default into a second setting.

## Adapter

An Adapter projects a resolved Profile into one client boundary:

- Claude receives `ANTHROPIC_AUTH_TOKEN` and `ANTHROPIC_BASE_URL` only in the launched process.
- Codex receives an AIGW-marked provider block plus credentials through its official `login --with-api-key` command.

Adapters never own provider secrets and never write into one another's directories.
