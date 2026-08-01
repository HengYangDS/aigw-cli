## Why

AIGW had accumulated product-bound provider assumptions, overlapping package
owners, and release identity coupled to repository history. It must instead be
a portable control plane whose configuration, projections, quality floor, and
publication trust remain explicit and independently verifiable.

## What Changes

- **BREAKING**: remove proxy lifecycle, IDE recovery, compatibility shims, and
  implicit provider topology from AIGW's product boundary.
- Make Account, Profile, Route, endpoint, storage policy, credential, and client
  projection the complete control-plane model.
- Accept arbitrary operator-selected endpoints through token-free manifests;
  keep provider-native diagnostics optional and leaf-owned.
- Make client projection transactional, inspectable, and unable to modify
  existing conversation history or model metadata.
- Replace broad command and domain buckets with cohesive semantic packages and
  enforce portability, dependency direction, structure, and coverage above 95%.
- Supply authors and signing trust from each Forge's protected publication
  context rather than from product source or a local path.

## Capabilities

### New Capabilities

- `product-control-plane`: subject=AIGW provider control plane; reuse=new;
  change=add; provider-neutral configuration, credential, routing, projection,
  portability, and ownership boundaries;
  facet:lifecycle=configuration,projection,validation,publication;
  facet:surface=cli,manifest,client-projection,docs,ci,openspec;
  facet:authority=source,configuration,credential,projection,claim,evidence.

### Modified Capabilities

None.

## Out of Scope

- Proxying or adapting provider wire traffic, supervising an external gateway,
  or adding a gateway-specific setup surface.
- Controlling PyCharm, Air, Codex session history, item records, JSONL, SQLite,
  per-conversation models, or model metadata.
- Encoding a contributor, machine, checkout, private Forge, credential, or
  signing key as product source.
- Preserving old packages or commands through shims, wrappers, aliases, or
  parallel authorities.

## Impact

The CLI command tree, configuration schema, client projections, package layout,
quality gates, release workflows, contributor guidance, and product docs are
converged together. External Responses services remain independent HTTP
endpoints and require no AIGW-specific integration.
