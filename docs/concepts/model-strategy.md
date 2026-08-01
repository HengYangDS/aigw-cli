# Model Strategy

## Principle

AIGW exposes a small set of admitted, purpose-labelled profiles rather than an
unbounded provider catalog. An account owns endpoints and one token; a profile
is an admitted `account + client + model` selection; `purpose` is human guidance
only. Catalog discovery is read-only and never creates profiles or routes.

## Profile ownership

AIGW ships no current or preferred model set. Each adopting team owns the
minimum profile set its users need, including purpose labels and route
recommendations. A model ID becomes a local profile only after account catalog,
protocol, permission, cost, and client evidence are accepted.

## Admission order

1. Define an isolated client adapter and its uninstall boundary.
2. Verify the client's actual protocol, authentication, streaming, tools, and
   required multimodal/context behavior.
3. Assign an independent account and token boundary.
4. Run one user-authorized, bounded real verification and prove rollback.
5. Choose one preferred profile per capability; keep alternatives explicit.

Any failed stage leaves existing routes unchanged and creates no half-configured
profile. See [Adapter admission](../governance/adapter-admission.md) for the evidence record.

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
