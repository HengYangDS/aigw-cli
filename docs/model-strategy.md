# Model Strategy

## Principle

AIGW exposes a small set of admitted, purpose-labelled profiles rather than an
unbounded provider catalog. An account owns endpoints and one token; a profile
is an admitted `account + client + model` selection; `purpose` is human guidance
only. Catalog discovery is read-only and never creates profiles or routes.

## Current profile set

| Capability | Preferred profile | Boundary |
| --- | --- | --- |
| Default agent | `claude-fable-5` | Claude baseline |
| Codex engineering | `gpt-5.6-terra` | Codex baseline |
| Deep reasoning | `claude-opus-4-8-thinking` | Explicit, on demand |
| Balanced alternative | `claude-sonnet-5` | Explicit, on demand |

These are team-template examples, not baked-in provider defaults. A model ID
becomes a local profile only after account catalog, protocol, permission, cost,
and adapter evidence are accepted.

## Admission order

1. Define an isolated client adapter and its uninstall boundary.
2. Verify the client's actual protocol, authentication, streaming, tools, and
   required multimodal/context behavior.
3. Assign an independent account and token boundary.
4. Run one user-authorized, bounded real verification and prove rollback.
5. Choose one preferred profile per capability; keep alternatives explicit.

Any failed stage leaves existing routes unchanged and creates no half-configured
profile. See [Adapter admission](adapter-admission.md) for the evidence record.

## Operator path

```bash
aigw
aigw use
aigw check
```

A maintainer adds an already-admitted profile explicitly:

```bash
aigw profile add <profile> \
  --account <account> --for <client> --model <model-id> \
  --purpose "One-line operating guidance"
```
