# Compact Model Catalog Design

## Purpose

Keep `aigw catalog` useful on Accounts that expose hundreds of model IDs. The
default human view must answer which configured models are available without
turning routine discovery into an unscannable inventory dump.

## Contract

- `aigw catalog` remains read-only and fetches the complete authenticated
  `/v1/models` inventory for every eligible Account.
- Its default human view shows, per Account: total discovered model count,
  configured model count, every configured model with its Profile IDs, and the
  number of remaining unconfigured IDs.
- `aigw catalog --all` explicitly renders every discovered model. Each model
  uses a readable two-line record rather than a fragile fixed-width table.
- `aigw catalog --json` keeps the existing complete, stable JSON contract.
  `--all` is unnecessary in JSON mode and is rejected to avoid ambiguous use.
- Failed, unavailable, and empty Account results retain their current
  diagnostic behavior.

## Acceptance

1. The default human view omits unconfigured model IDs while retaining correct
   counts and all configured models.
2. `--all` includes both configured and unconfigured IDs without wrapped or
   run-together status text.
3. JSON remains complete, sorted, secret-free, and read-only.
4. Help and team documentation make the compact/default versus full/explicit
   distinction discoverable.
