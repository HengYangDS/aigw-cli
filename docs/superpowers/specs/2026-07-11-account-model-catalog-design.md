# Authenticated Account Model Catalog — Design

## Purpose

Expose the authenticated `/v1/models` inventory for every configured AIGW
Account without turning discovered model IDs into Profiles or changing any
client route.  This lets routing decisions be grounded in the account's real
catalog rather than in the small, hand-maintained Profile list.

## Non-goals

- Do not create, modify, select, repair, sync, or remove Accounts, Profiles,
  routes, adapters, or client configuration.
- Do not issue a model generation request or imply that a listed model has
  passed a Codex, Claude, or capability test.
- Do not print, persist, log, or include Account tokens in JSON or errors.
- Do not classify model capability from the model name.  Protocol and task
  qualification belong to the later admission process.

## Chosen interface

Add a top-level read-only command:

```text
aigw catalog [--json]
```

It has no positional arguments and is intentionally distinct from `aigw
models`, whose stable role remains reporting the reachability of configured
Profiles.

For each configured Account with an OpenAI Responses endpoint and an available
system secret, the command performs one bounded authenticated `GET /v1/models`
request.  It displays one deterministic row per returned model ID with:

- Account ID;
- source protocol label `openai_responses`;
- discovered model ID;
- Profile relationship: `configured` (one or more Profile IDs) or
  `unconfigured`.

The JSON result is a stable object with an `accounts` collection.  Each account
contains only public configuration metadata and discovery results; endpoint
URLs and `secret_available` may be included, but token values and Authorization
headers are never represented.

## Data flow and errors

The command loads the existing config, obtains each usable Account token only
through `app.Secrets.Get(accountID)`, and reuses the existing bounded
`fetchModelSet` transport path.  The token is placed only in the outgoing
Authorization header.  The command converts returned IDs to sorted discovery
rows and compares them with configured Profile models from the same Account.

Accounts without the OpenAI Responses endpoint, without a usable secret, or
with an endpoint failure remain visible with a non-secret status and no rows.
One account's failure does not suppress another account's discovery.  The
command exits successfully when configuration was read and renders the
per-account diagnostic status; malformed successful payloads are reported for
that account without leaking response content.

`openai_responses` is the source endpoint, not a statement that every listed
ID supports Responses, Codex, Claude, vision, tools, or reasoning.

## Verification

Tests precede implementation and prove:

1. the command calls the correct `/v1/models` endpoint with authentication;
2. IDs are sorted and include unconfigured IDs;
3. configured Profile relationships are shown accurately;
4. JSON is parseable and does not contain the test token or Authorization
   header;
5. unavailable/failed Accounts are reported without blocking a healthy Account;
6. no config write is made.

Acceptance is `go test ./...`, followed by a live `aigw catalog --json` read
against the current Account.  The live result is saved only to the terminal or
a deliberately user-chosen location; it is never committed because account
inventory can be sensitive operational metadata.

## Follow-on admission boundary

Catalog discovery is not route admission.  Each candidate proceeds only after:

1. account visibility;
2. a user-authorized minimal request for the matching protocol/client;
3. price and quota visibility;
4. fixed workload benchmark evidence.

Only candidates passing all four gates may be added to the AIGW team catalog
and then projected into a client route.
