# AI Access Tooling: Needs, Architectures, and Competitive Value

## Research thesis

**This is research into how people and organizations obtain dependable access to AI through heterogeneous clients—not a contest between repositories, and not a defense of AIGW or Proxy.** Its purpose is to explain why this tooling exists, distinguish competing solution paradigms, identify where value is created or merely relocated, and help the reader make better product and engineering decisions.

The central tension is **choice versus coordination**. More clients, providers, models and accounts give users options, but also create configuration, authentication, compatibility and operating work. A useful tool absorbs some of that work. A poor integration adds another state store, process or failure boundary without removing enough work elsewhere.

Three conclusions organize the evidence:

1. **There is no single homogeneous “AI switcher/gateway market.”** Configuration managers, launchers, compatibility proxies, shared gateways and integrated workbenches overlap, substitute and complement one another at different boundaries. Comparing feature totals across them obscures the actual choices.
2. **Many entry-level capabilities are already supplied by multiple projects.** Local deployment, endpoint switching, a model catalog and an OpenAI-compatible interface do not alone establish distinctive value. This is a supply-side finding, not measured market commoditization or evidence that all implementations are equivalent.
3. **The more promising basis for differentiation is a complete workflow with a lower ongoing burden.** Ownership-preserving changes, usable credentials, task continuity and recovery matter when they produce observable user benefits. Whether any candidate delivers them better remains an empirical question.

**How to read:** [demand and causes](#1-the-demand-behind-the-tools) explain the problem; [solution paradigms](#2-solution-paradigms-and-competitive-relationships) map alternatives; [value movement](#3-where-value-may-move) examines evolution; [mechanism-based findings](#4-what-the-mechanisms-teach) explain trade-offs; [scenario comparisons](#5-compare-products-within-the-right-scenario) and [implications for AIGW and Proxy](#6-implications-for-aigw-and-proxy) apply the analysis. Extension and evaluation follow. The [reference catalog](#reference-the-broader-landscape) holds product detail.

The evidence is primarily official documentation and selected source inspection, not customer interviews, adoption statistics or competitor runtime tests. Strategic judgments below are reasoned hypotheses with reversal conditions. This report informs the user's decisions; it does not authorize migration, change the current delivery goal, or decide the two products' fate.

## 1. The demand behind the tools

Users do not ultimately need “provider switching.” They need to complete work using the service and account they intend, at an acceptable cost and risk, without repeatedly learning configuration internals or losing their ongoing task. Switching is one means; sometimes stable defaults or central administration serve that need better.

Different users therefore value different outcomes:

| User and situation                               | Desired outcome                                        | Burden they are trying to remove                                                 |
| ------------------------------------------------ | ------------------------------------------------------ | -------------------------------------------------------------------------------- |
| Individual using several coding clients          | Consistent, understandable access and easy recovery    | Repeated setup, wrong active accounts, conflicting settings and lost continuity. |
| Developer comparing accounts/models concurrently | Independent experiments without disturbing normal work | Global configuration contention and accidental cross-account use.                |
| Team lead distributing a recommended setup       | Members become productive with minimal assistance      | Inconsistent catalogs, secret sharing and machine-specific onboarding.           |
| Organization funding and governing access        | Controlled consumption with revocation and attribution | Credential sprawl, unmanaged budgets and unclear responsibility.                 |
| Platform or application developer                | A stable integration boundary                          | Provider-specific protocol, authentication and operational differences.          |

These are analytic segments derived from the workflows under review, not measured customer populations. A person can occupy several roles; their preferences can conflict. The cheapest individual setup is not necessarily the least costly organizational operating model.

### Why the problem recurs

Fragmentation exists at several independent seams: clients have different configuration and session contracts; upstreams expose different protocols and capabilities; accounts have different credential and renewal rules; operating systems deliver credentials and supervise processes differently. A route that works for one combination need not work for another.

This explains why adding model names is often easy while maintaining a dependable product is not. The hard work lives in the **relationships between components**, especially where one changes independently of another. A growing provider count measures catalog breadth, not the number of successfully maintained user journeys.

Separate four responsibilities before comparing products:

| Responsibility        | Core question                                      | Legitimate ownership boundary                                                                          |
| --------------------- | -------------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| Configuration control | What should this client use?                       | Catalogs, credential references, selected routes and explicitly owned settings.                        |
| Request execution     | How does this request reach a compatible service?  | Credential delivery, transport, protocol conversion, streams and errors.                               |
| Session continuity    | Does this ongoing task retain its meaning?         | Client-owned conversation/tool state; adapters preserve semantics without acquiring history ownership. |
| Organizational access | Who may consume which resources, at whose expense? | Membership, downstream credentials, quotas, revocation, policy and audit.                              |

These are responsibility boundaries, not a proposal to create four products. One product may cover several; separate binaries may still be tightly coupled if they compete to own the same state.

## 2. Solution paradigms and competitive relationships

A useful map starts with **how a solution removes work**, not the language it uses or whether it has a GUI.

| Paradigm                            | Mechanism and benefit                                              | Obligation introduced or retained                                                    | Representative alternatives                      |
| ----------------------------------- | ------------------------------------------------------------------ | ------------------------------------------------------------------------------------ | ------------------------------------------------ |
| Native/direct setup                 | Use the client's existing surface; no extra manager or traffic hop | User or administrator still coordinates settings and verifies upstream compatibility | The baseline to test before adding a product     |
| Per-launch isolation                | Scope account and configuration to a child process                 | Different launch workflow and possibly different history/plugin discovery            | MuxLM; CC Switch CLI launch mode                 |
| Persistent configuration management | Maintain reusable intent and project it to client settings         | Conflict detection, field ownership, credential delivery and safe withdrawal         | CC Switch Desktop/CLI, ccman, AIGW               |
| Local compatibility proxy           | Adapt requests or routing at one endpoint                          | Semantic fidelity, ports, service availability, privacy and recovery                 | CLIProxyAPI, CCR, OpenCodex, Proxy               |
| Shared access gateway               | Centralize authorization, routing, consumption and operations      | Server administration, member isolation and a shared failure/data boundary           | One API, New API, LiteLLM, Portkey, Bifrost      |
| Integrated workbench                | Bundle several repeated tasks into one interaction surface         | Broader permission/state scope and coordination across features                      | CC Switch family, CCS, ZCF with different scopes |
| Reusable component                  | Embed existing behavior behind an application boundary             | API integration, upgrade compatibility and host responsibilities                     | CC Switch Core; CLIProxyAPI SDK                  |

Sources and version qualifications are in the [catalog](#reference-the-broader-landscape). These are mechanisms, not mutually exclusive product buckets. CC Switch's optional proxy does not make it a mandatory traffic gateway; a gateway's client instructions do not make it a full configuration manager.

Local/self-hosted versus hosted, GUI versus CLI, personal versus shared, and API key versus OAuth are **separate axes**. A local proxy still sends inference data upstream. A hosted aggregator may also change the billing and trust relationship; it is not merely another installation format. This report does not establish comparable hosted pricing or subscription eligibility.

### Competition is a graph, not a league table

| Relationship            | Example                                                     | What a meaningful comparison asks                                                        |
| ----------------------- | ----------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| Direct substitutes      | AIGW and CC Switch CLI for persistent configuration         | Can they complete the same onboarding, switching and withdrawal journey?                 |
| Conditional substitutes | A direct native endpoint and a compatibility proxy          | Is the proxy solving a still-present incompatibility or another required operating need? |
| Complements             | A local configuration manager and a shared gateway          | Is there a clean endpoint/credential handoff with no competing state owner?              |
| Components and hosts    | CLIProxyAPI SDK and an integrated application               | Does embedding transfer enough implementation and maintenance work?                      |
| Platform absorption     | An upstream or client adds a previously external capability | Which intermediary responsibility disappears, and which remains?                         |

Thus One API is neither irrelevant nor a drop-in replacement for every local workflow. CC Switch is neither merely a file editor nor necessarily a heavyweight mandatory gateway. An integrated tool can win by reducing coordination across products; a narrow tool can win by solving one boundary exceptionally well. Either must be assessed in its relevant mode, not its largest possible deployment.

## 3. Where value may move

This section makes **conditional strategic inferences**, not market forecasts. Repository popularity, a feature announcement and a release asset do not establish adoption, willingness to pay or operational quality.

### Native capabilities can absorb intermediary work

The pinned [AWS Bedrock Access Gateway README][aws-gateway] deprecates that sample in favor of native compatible APIs. Separately, an [AWS enterprise deployment reference][aws-native-adoption] enters maintenance mode and recommends Claude Apps Gateway for new deployments, while listing remaining complementary reference patterns. These are two concrete examples of functionality moving into upstream/client-associated offerings, not proof that all independent gateways are obsolete.

The implication is to distinguish a **temporary compatibility gap** from a **continuing coordination need**. A translator may lose its reason to exist when native protocol support arrives. Cross-provider administration or organization-specific policy may remain—but native offerings can absorb those too. Reassess each responsibility, not the continued existence of its repository.

The opposite scenario is also plausible: richer tools and session semantics create new incompatibilities faster than protocols converge. Then specialized compatibility work can remain valuable. A single deprecation case cannot settle which scenario dominates.

### Bundling competes with both duplication and narrowness

CC Switch combines configuration with optional proxy, MCP/skills and session-related features; CCS composes a gateway underneath a concentrated interface. [CC Switch][cc-desktop], [CCS][ccs]. Their potential advantage is less coordination for users, not merely more checkboxes.

Bundling can also widen authority, state and incident scope. A narrow product is preferable when its limited responsibility improves predictability or safe composition. But “small” is not automatically better: five individually small tools can impose more user work than one coherent suite. Compare the complete workflow's burden, including the support work shifted to the user.

### Durable value requires a recurring problem and a credible way to solve it

Safe ownership, reliable session semantics, cross-platform recovery and organizational controls are candidate sources of recurring value. None is an exclusive moat by definition. Competitors can implement them; native platforms can absorb them; users may value convenience more in some scenarios.

The test is two-sided: **does the problem persist, and does this solution measurably reduce its burden better than alternatives?** A reusable compatibility corpus or well-defined extension boundary can make maintenance more efficient, but test volume alone proves neither superiority nor product demand.

A practical implication is to keep an exit path: reversible configuration, exportable intent and replaceable endpoints. High switching costs caused by opaque state are not the same as user value. The goal is to deserve continued use, not make removal difficult.

## 4. What the mechanisms teach

The findings below separate observation, mechanism and decision implication. Reversal conditions make them falsifiable rather than slogans.

### Configuration convenience creates an ownership obligation

**Observation.** CC Switch provides persistent configuration and optional proxy modes. MuxLM instead launches Codex with a temporary home containing generated configuration and a key-bearing auth file. CC Switch Lite/Core expose owned-field and conditional-write mechanisms, while Lite warns about concurrent writers without a shared lock. [Desktop][cc-desktop], [MuxLM launcher][muxlm-launch], [Lite][cc-lite], [Core executor][core-executor].

**Mechanism.** Persistent settings preserve the normal launch surface but introduce shared-write coordination with the client, the user, and other managers. Per-launch isolation reduces that shared-write problem by changing the scope of configuration. It may also change session, plugin and Desktop discovery. Complexity is relocated, not automatically removed.

**Implication.** The central comparison is not GUI versus CLI. It is **which process owns each setting, for how long, and how it relinquishes ownership**. Two products can both be easy individually yet unsafe together if they write the same field. Prefer one writer per surface; compose through explicit endpoint or import boundaries rather than concurrent reconciliation.

**Reversal condition.** If isolated CLI launches are acceptable, a launcher may remove the need for persistent integration. If shared CLI/Desktop state is required, isolation must pass a continuity test. AIGW's ownership contract is valuable only if it handles the required cases better in practice, not because its documentation uses the word “atomic.”

### Protocol breadth and semantic fidelity are different axes

**Observation.** CLIProxyAPI's inspected generic compatible executor sends ordinary upstream requests to Chat Completions; compact handling and specialized executors differ. CC Switch's release notes include native Responses and tool/multi-agent fixes. Neither is adequately described by a single “Responses supported” cell. [Executor][cpa-executor], [release notes][desktop-release].

**Mechanism.** Translation can preserve only information the destination contract can represent or the adapter can safely retain. When a source protocol includes state references, tool associations or reasoning structures absent from the target, field renaming cannot establish equivalence. A syntactically accepted request can still change the task. An HTTP success that silently discards required content is not compatibility.

**Implication.** Compare a concrete path: **client version + inbound protocol + executor + outbound protocol + upstream + conversation behavior**. Test native direct access before translation. Where translation is necessary, reject unsupported semantics explicitly rather than masking them with apparently successful text output.

**Reversal condition.** If a general gateway preserves the actual workloads and has acceptable lifecycle cost, a dedicated Proxy has no established correctness advantage. If the provider gains the needed native contract, even a successful adapter may become unnecessary. Proxy's possible value is a narrow, evidenced compatibility guarantee, not a large translation surface.

### Team distribution and access governance solve different problems

**Observation.** CC Switch Desktop and CLI database exports preserve provider configuration, which can contain keys. CC Switch describes a local single-user trust model. AIGW describes a token-free catalog, whereas One API, New API and LiteLLM document shared users, tokens, quotas or virtual keys. [Provider storage][desktop-provider-storage], [Desktop export][desktop-export], [CLI export][cli-export], [security boundary][desktop-security], [One API][one-api], [New API][new-api], [LiteLLM][litellm].

**Mechanism.** Personal backup reproduces personal state; team distribution publishes common intent; access governance authorizes consumption. The same export file cannot be assumed to serve all three safely. Removing keys from a manifest avoids one sharing risk, but does not create central revocation or enforce budgets.

**Implication.** For members bringing their own keys, assess a small catalog plus local credential binding. For organization-funded access, assess a governed endpoint plus the smallest necessary client configuration. Do not grow AIGW into a gateway merely because the audience is a company, or advertise a team manifest as enterprise authorization.

**Reversal condition.** A team whose actual need is personal multi-device synchronization may prefer an existing database-sync product. A team requiring central access controls may rationally accept a server and database. “Enterprise” is a set of operating requirements, not a synonym for more features.

### Cross-platform support is a lifecycle property, not an asset count

**Observation.** CC Switch CLI publishes Windows artifacts but its stable README distinguishes foreground proxy serving from Unix-only daemon management. MuxLM's credential defaults differ by platform. Its Codex launcher writes a temporary credential file even when storage uses a native backend. [CLI stable README][cli-stable-readme], [MuxLM storage][muxlm-storage], [key handling][muxlm-keys], [launcher][muxlm-launch].

**Mechanism.** A distributable executable proves packaging, not equivalent credential delivery, background operation or cleanup. Storage at rest and delivery to a GUI, service or child process are separate boundaries. A Linux container without a user session does not establish a desktop credential-service path.

**Implication.** Evaluate the whole required journey on each platform: install, configure with one credential, execute, recover, upgrade, withdraw integration and uninstall. Avoiding an unnecessary resident service eliminates its service obligation; it does not prove the remaining journeys correct. Environment credentials need inheritance tests, not an assumption that every process sees the shell's environment.

**Reversal condition.** Foreground use or environment credentials may fully meet a declared deployment. They are not defects merely because another product offers daemons or a vault. They become gaps only when the user's required journey needs the missing behavior.

### Reuse reduces code only when it reduces owned obligations

**Observation.** CLIProxyAPI exposes a Go SDK and CCS composes it. CC Switch Core exposes reusable configuration behavior, but its adapter trait is sealed and its host still implements I/O, locks and platform security. The inspected CLIProxyAPI SDK documentation also drifts from the module's public import paths. [CCS][ccs], [SDK][cpa-sdk-doc], [module][cpa-module], [builder][cpa-builder], [Core adapter][core-adapter], [Core executor][core-executor].

**Mechanism.** Depending on a library transfers implementation, not automatically compatibility ownership. A wrapper that repairs undocumented behavior, duplicates an upstream state machine or requires a permanent fork can cost more than a narrower implementation. Conversely, a feature-rich product may remove more operational work than a small bespoke binary.

**Implication.** Judge adoption by the responsibilities, tests and support work it lets us delete. Prefer using a product unchanged, then configuration or a public extension, before considering a fork. Reuse is attractive when its public boundary matches the required responsibility and its upgrade path is supportable.

**Reversal condition.** A small missing seam that can be contributed upstream may favor adoption. A deep, unstable fork may not. Lines of code and repository size are maintenance signals, not a total-cost model or an excuse to retain custom code.

## 5. Compare products within the right scenario

A shortlist is an investigation order, not a claim that a candidate passed acceptance. Start with the native baseline and at most two contenders for the actual scenario; the broader catalog is a reserve, not a demand to test everything.

| Actual journey                                     | First comparison                                                           | Decisive trade-off                                                                                |
| -------------------------------------------------- | -------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| Manage persistent Claude/Codex settings visually   | Native settings; CC Switch Desktop; current AIGW journey                   | Daily usability and ownership-preserving withdrawal, rather than GUI feature count.               |
| Script configuration and team onboarding           | CC Switch CLI; current AIGW journey                                        | One-key setup, secret-free distribution, deferred client installation and diagnosable automation. |
| Run concurrent accounts in separate terminals      | CC Switch CLI per-launch mode; MuxLM                                       | Isolation convenience versus continuity with normal Desktop, history and plugin locations.        |
| Fix a specific Codex Responses incompatibility     | Direct upstream; CC Switch optional proxy; CLIProxyAPI's relevant executor | Replay/tool fidelity; substitute OpenCodex if the executor contract rules CLIProxyAPI out.        |
| Distribute paid access with budgets and revocation | One API/New API; LiteLLM                                                   | Shared access authority and operating cost, not local settings convenience.                       |
| Keep a custom UX while replacing plumbing          | CLIProxyAPI SDK or CC Switch Core, according to boundary                   | Stable usable public APIs, integration burden and actual custom code removed.                     |

### CC Switch CLI versus AIGW: what the user would actually notice

The [CLI][cli-stable-readme] combines CLI/TUI, global and per-launch switching, account management, MCP/skills and optional proxy operation across seven declared clients. AIGW's [documented boundary](../architecture/authority-and-projection-boundary.md) is narrower: Claude/Codex configuration, Accounts, Profiles, Routes and credential integration, without owning sessions or a traffic service.

CC Switch CLI's breadth is a real benefit if it replaces several tools the user needs. AIGW's restraint is useful only if it makes setup, diagnosis, preservation and removal materially more predictable. Optional competitor features need not be enabled; comparing AIGW's minimal mode to a competitor's maximal deployment would bias the result. Conversely, the CLI's Windows foreground-proxy boundary must not be hidden by a generic platform checkmark.

### One API: relevant, but at a different operating boundary

[One API][one-api] already addresses aggregation and downstream API distribution. Dismissing it because it does not resemble AIGW's CLI would miss an alternative architecture. It becomes a leading candidate when an administrator should hold upstream credentials and distribute governed access to members.

It does not thereby solve every client's local setup or preserve a particular Codex conversation. A shared gateway and a small local configuration surface can be complementary. Their composition should have one handoff: a documented endpoint and credential contract, not mutual installation and private-state dependencies. New API extends the same area, but its observed RC and license terms require version-specific review before adoption.

## 6. Implications for AIGW and Proxy

**“Keep both” and “replace both” are not the only choices.** Configuration and request compatibility can be replaced independently. The counterfactual matters: what user-visible outcome gets worse if this component is removed and the best existing alternative is used?

### AIGW: a configuration contract, not another universal workbench

A defensible value hypothesis is: **a team member with any one supported provider credential can reach a usable client configuration, understand it, and safely withdraw it across required platforms, with less recurring work than the alternatives**. This combines onboarding and ownership into an outcome; it is not established by a manifest, a keyring abstraction or a passing unit test alone.

The competing hypothesis is that an existing switcher plus a small team catalog already does this. If true, custom account stores, generic client registries and switching mechanics may be unnecessary maintenance. A thin team distribution artifact could remain useful without preserving AIGW as a complete product. If the only advantage is preferred naming or UI arrangement, measure its importance rather than treating it as architectural necessity.

### Proxy: preserve task semantics only where adaptation is necessary

A defensible value hypothesis is: **a narrowly scoped compatibility service preserves the required long-running Codex tool workflow and survives its own lifecycle without disrupting the task, where a direct route or existing service does not**. Evidence must include replay, compaction, cancellation, interruption and recovery—not just fresh text generation.

The competing hypothesis is that direct native endpoints or a maintained gateway cover the same workload. In that case, a custom service adds a port, supervisor, credential-delivery path and upgrade obligation without a corresponding user benefit. A library contribution or small upstream adapter may better preserve any useful work. A past failure in our implementation is not evidence that competitors fail too.

### Compare future cost, not sunk effort

For each viable option, record the same costs: remaining delivery work, migration and rollback, recurring client/provider compatibility changes, service operations, incident recovery and eventual removal. No comparable measurements currently support a numeric ranking.

Keep three outcomes open:

- **Adopt:** an existing product meets the non-negotiable journeys; local customization stays configuration-level.
- **Compose:** distinct products meet distinct responsibilities through supported interfaces, with one owner per state surface.
- **Build narrowly:** a material unmet behavior remains after native/product/public-extension options are tested; owning it is more supportable than a fork or workaround.

The recommendation raises the evidence bar for custom plumbing. It does not stop an in-flight safe delivery or replace a healthy service on an untested assumption.

## 7. Extension should follow the changed contract

Vendor count is a poor proxy for extensibility. One new model can be a data change; one new authentication or session contract can require substantial behavior.

| Change                                            | Smallest plausible owner                  | Evidence of genuinely low-cost extension                                                         |
| ------------------------------------------------- | ----------------------------------------- | ------------------------------------------------------------------------------------------------ |
| Another endpoint/model using an admitted protocol | Catalog or configuration                  | No core branch or binary release; real client authentication, streaming and tools work.          |
| New signing or credential renewal                 | Native client/provider SDK when available | Credential lifecycle works in the target process; no redundant local credential authority.       |
| New wire or replay semantics                      | Maintained protocol adapter               | Conformance tests preserve task meaning, errors and cancellation; no silent field dropping.      |
| New client such as Hermes, OpenCode or Pi         | Client-specific projection/launch adapter | Correct path, precedence, credential delivery, conflict handling and removal in the real client. |

CC Switch CLI declares Hermes, OpenCode and Pi support; AIGW currently admits Claude Code and Codex. That is a breadth difference, not a reason to call a future AIGW integration free. Equivalent Qoder evidence was not established in the assessed leading candidates. “Endpoint configured,” “model visible,” “tool loop works” and “survives upgrade” are separate acceptance levels. [CLI][cc-cli], [AIGW](../../README.md).

### AWS illustrates why adapters need an expiry condition

The pinned [Bedrock Access Gateway README][aws-gateway] marks the sample deprecated and recommends native OpenAI-compatible and Anthropic-compatible Bedrock APIs instead. This is evidence of that sample's rationale, not proof that every model, region or client can use every AWS surface. Direct official-document retrievals failed in the original collection; no AWS request was run.

For an AWS evaluation, bind the exact client, model, region, API and authentication contract; check native support and maintained SDKs first. The extension question follows the value-movement analysis: which responsibility still requires an intermediary? Do not introduce a signer or translator solely because an earlier integration needed one.

## 8. From research to practice

The practical objective is better decisions and less work for users and maintainers—not a larger report or a larger test program. The following recommendations distinguish improvements within the approved delivery scope from experiments and product decisions that remain separate.

| When                              | Recommended action                                                                                                                                                                                                  | Observable result or decision boundary                                                                                                                                                          |
| --------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Current delivery                  | Complete AIGW first, then Proxy. Use the existing Change and tests to close one-key onboarding, owned-setting preservation, credential delivery and recovery; do not add a parallel research-driven implementation. | Required user journeys work from the delivered artifact. Source checks, hosted CI and installed-product evidence remain separate.                                                               |
| Next configuration evaluation     | Compare the current AIGW journey with native setup and CC Switch CLI; use Desktop instead when GUI operation is the actual need. Begin with one provider and one required client.                                   | Record setup effort, route clarity, unrelated-state preservation and withdrawal. Expand the pilot only if the result could change the choice.                                                   |
| Next compatibility evaluation     | Reproduce the required Codex workflow directly against its upstream first; evaluate an existing proxy only for a demonstrated remaining need. Keep this isolated from the serving installation.                     | A passing direct path removes that proxy obligation from the proposed architecture. Otherwise, retained tool/replay semantics and recovery determine whether an existing service is sufficient. |
| Each proposed extension           | Identify the changed contract; try configuration, native capability or a maintained public extension before new product code.                                                                                       | Document the behavior gained and the custom responsibility removed. A dependency without deleted work or a necessary new capability has not yet shown net benefit.                              |
| After the user's product decision | Translate the selected option into one bounded migration or improvement plan, with retained state, rollback and exact retirement targets.                                                                           | The real journey remains usable; replacement mechanics and obsolete owned artifacts are removed only after their successors are verified.                                                       |

These are conditional recommendations, not completed experiments or migration authorization. The next pilot should answer one unresolved decision; it must not become an indefinite prerequisite for current delivery.

### Measure whether the recommendation helped

Compare the same task before and after the change: member setup steps and assistance needed, successful configuration and recovery, preservation of unrelated state, maintenance changes required by an upstream/client update, and remaining components or custom behavior to support. Keep the observations instead of inventing a weighted score. If the intervention merely moves work from code to operators, creates a fragile fork, or makes recovery harder, revisit it even if the feature checklist improves.

Accepted choices belong in the existing decision records; implementation and acceptance belong in the relevant OpenSpec Change. Research retains the evidence, reasoning and rejected alternatives—it does not become another task tracker or policy authority.

### Establish hard constraints before scoring preferences

Apply the same constraints to our products and competitors: preserve user settings and history; respect per-conversation model selection; accept the intended one-provider setup; expose the actual route; avoid unwanted credential prompts and leakage; satisfy the required platform and recovery journey. Central RBAC is conditional, not automatically required. A safety failure cannot be compensated by more features.

Then compare usability, automation, client breadth, maintenance burden and measured performance. Do not invent weights or a single score before the user identifies trade-offs. An unfamiliar or untested feature is not a “no.”

### Run the cheapest decisive experiment first

| Order                        | Experiment                                                                                                                                                    | Decision it enables                                                                              |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| 1. Establish necessity       | Pin a client and upstream; run the native direct text/tool/stream journey. Replay the relevant sanitized failure separately.                                  | If direct access meets the required behavior, remove the proxy from that candidate architecture. |
| 2. Test daily value          | Compare native setup and two scenario contenders: one key, missing other keys, client installed later, inspect route, switch and withdraw.                    | Eliminate tools that cannot deliver the primary journey; retain the simplest workable options.   |
| 3. Test ownership            | Seed unrelated settings, plugins and session metadata; exercise update, conflicts, disable and uninstall. Compare exact owned/unowned effects.                | Distinguish acceptable integration from a merely successful configuration write.                 |
| 4. Test protocol necessity   | Only for paths still needing a proxy: identical client/upstream fixtures for parallel tools, images, reasoning, replay, compaction, cancellation and failure. | Decide whether a general gateway suffices or a narrow missing behavior remains.                  |
| 5. Test operational survival | On required native platforms, test environment and vault delivery separately; exercise upgrade, rollback and interrupted cleanup.                             | Establish deployability and the platform-specific operating burden.                              |
| 6. Test economics and exit   | Perform one provider/client update and reversal; measure manual steps, patch surface, failure recovery and removal. Benchmark equivalent routes if needed.    | Compare adopt/compose/build on future cost; verify that the apparent simplification is real.     |

Move a protocol test earlier if it is the known decisive requirement. Platform obligations follow the selected deployment, not a demand to run every optional feature everywhere. Docker Linux establishes container behavior; it does not replace Windows native service or desktop credential evidence.

### Make outcomes comparable and stop when the decision is resolved

For each experiment retain the exact version/configuration, intended route, expected invariant, observed result, secret-free evidence and recovery outcome. Classify failure at its owning boundary: client, configuration, adapter, upstream or infrastructure. An upstream quota failure is not a switcher defect; a network outage is not proof of a protocol bug.

For proxy paths, first-token success is insufficient. Compare tool associations and terminal results, explicit unsupported behavior, cancellation, duplicate side effects, and retained task meaning. For performance, separate startup/steady-state overhead from upstream latency; keep workload, cache conditions and concurrency comparable. No performance numbers are asserted here.

Stop evaluating a candidate when a hard constraint has a reproducible failure without an acceptable supported remedy. Stop broadening the search when a candidate clears the constraints at acceptable cost and more research is unlikely to change the choice. If a small upstream contribution closes the only gap, evaluate that before a permanent fork. If no candidate clears the constraints, the failing invariants define the smallest legitimate custom scope.

This source-based research can explain alternatives and guide evaluation; it cannot establish customer demand or declare an operational winner. Interviews, adoption evidence and competitor runtime acceptance remain distinct work if the next decision requires them. They are not new dependencies of the existing AIGW/Proxy delivery plan.

## 9. What the earlier investigation missed

AIGW's [DR-0001](../decisions/dr-0001-control-plane-data-plane-boundary.md), dated July 14 and amended August 16, compares architectural categories rather than named products and equivalent journeys. Snapshots at July 14, 2026, 23:59:59 UTC already describe multi-client configuration, optional proxies and related functionality in [CC Switch Desktop][historical-desktop], [CLI][historical-cli], [CLIProxyAPI][historical-cpa] and [CCR][historical-ccr]. The DR has no timestamp, so these snapshots cannot order every same-day change against the decision. Lite/Core were created later, on August 27, and must not be retroactively treated as available July alternatives.

The justified criticism is **insufficient recorded evidence for excluding existing alternatives**, not proof that development was necessarily pointless. The decision record does not bridge a sensible boundary principle—separate configuration from traffic—to the need for custom products. That is a gap in the argument, not proof of the original authors' motives or every investigation they performed. A clean architecture can still duplicate a product offering the same architecture or an acceptable optional mode.

This report's earlier inventory repeated part of that error: it collected facts without making them resolve the decision. A better standard is that every major finding explains a mechanism, an implication and what would overturn it. Before substantial custom development, document the native baseline, the nearest real substitutes, a decisive journey comparison and the residual responsibility worth owning. Keep adopted decisions separate from research; revisit them when client/provider contracts or operating costs materially change.

## Evidence and limits

- **Collection:** September 9, 2026; synthesis revised September 10. Scope covers 26 competitor repositories plus two AWS reference cases, not an exhaustive internet census. Agent Reach's read-only GitHub/gh API route was used; selected pinned source identities were rechecked during revision.
- **Evidence levels:** documented capability, inspected implementation, published artifact, unreplicated issue report and unverified behavior remain distinct. Source at HEAD is not automatically released capability. No competitor runtime, throughput, security-audit or migration success is claimed.
- **Internal comparison:** AIGW's original declared baseline was HEAD c357a2cae88408e09d3a2b2360c03f55d00df8d3; Proxy's was 210a108ec7a90ca2774dd702c70326cf1b240e1c. AIGW's responsibility document was also reread at 95ca0800b11ee754c876655e65c131714a0149fe. These are contract comparisons, not current release/install attestations. Direct OpenAI documentation retrieval during revision returned a timeout and HTTP 403; no new native-client capability was inferred from those failed requests.
- **Balanced reliability evidence:** CC Switch's observed [tool-message issue][desktop-issue-tools] and [Desktop-routing issue][desktop-issue-route] identify useful regression scenarios, not a comparative failure rate. Our products must face the same tests. The [CLI Windows issue][cli-windows-issue] being closed does not override its stable README's daemon restriction.
- **Compatibility precision:** CLIProxyAPI's generic executor finding does not apply to every specialized executor. CC Switch CLI v5.10.4's [schema compatibility note][cli-release-note] names Desktop v3.20.1, not arbitrary version combinations. Shared storage is not proof of safe concurrent operation.
- **Security precision:** local-first describes where control runs, not where inference data travels. A relay adds a party to the path. OAuth, API keys and subscription allowances are different contracts; login or a successful request does not establish entitlement to unrestricted relay use. No private credential store was examined.
- **Commercial boundary:** licenses, upstream access terms, hosted features, enterprise components and maintenance cost need separate assessment. The inventory records source observations, not legal clearance. No comparable total-cost data was collected.

## Reference: detailed source qualifications

These qualifications matter when a shortlisted option moves from inspection to a pilot:

- **Distribution:** CC Switch Desktop/CLI, MuxLM and CLIProxyAPI publish multiple platform assets; those assets do not establish native lifecycle acceptance. CC Switch CLI's Windows daemon limitation is explicit. Only Windows assets were observed for U-Pool. Container, npm, SDK and standalone forms have different operating obligations.
- **Credential handling:** MuxLM's Linux backend detects Secret Service and requests consent for automatic file fallback; macOS/Windows default to files, with macOS Keychain opt-in. Its temporary Codex auth file contains the selected key; interrupted cleanup was not tested. AIGW documents [keyring, file/DPAPI and environment choices](../architecture/security-model.md#credential-storage), not equivalent competitor behavior.
- **Release labels:** New API's observed RC is not a stable release merely because metadata says non-prerelease. Bifrost's observed enterprise-component tag is not an OSS runtime version. Portkey's README discusses prerelease 2.0 capabilities beyond the observed 1.15.2 release. Lite/Core have no observed release.
- **Reuse terms:** component exceptions matter. LiteLLM distinguishes enterprise code; [LLM Gateway's license][llmgateway-license] separates enterprise exceptions from AGPLv3 code. New API declares AGPLv3 and additional attribution terms. ccman declares MIT and [OpenCils package metadata][opencils-package] declares ISC, but complete standalone notices were not found. U-Pool explicitly says no license is declared; code-agent-switch's inspected files did not establish one. Public visibility is not reuse permission.

## Reference: the broader landscape

### Client configuration, launch, and initialization

Capabilities below derive from the linked source snapshots and their READMEs. Relative strengths and trade-offs are analysis of those mechanisms, not measured performance or reliability scores.

| Project and observed release                            | Main capabilities                                                                                                                    | Relative strengths                                                                               | Limitations or trade-offs                                                                                                                              |
| ------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| [CC Switch Desktop][cc-desktop] · `v3.20.2`             | Multi-client providers, GUI/tray, MCP, skills, prompts, sessions, usage, sync, optional proxy and failover                           | Configuration and visual daily management in one product; switching need not start a proxy       | Local single-user model, not organizational authorization; optional session operations need review; recent compatibility reports exist                 |
| [SaladDay/cc-switch-cli][cc-cli] · `v5.10.4`            | CLI/TUI, global and per-launch switching, seven client types, accounts, import/export, WebDAV, MCP/skills, usage, optional proxy     | Directly relevant to terminal and automation workflows; more than an endpoint editor             | No equivalent managed proxy daemon on Windows; shared databases require compatible versions                                                            |
| [MuxLM][muxlm] · `v2.6.0`                               | Codex, Claude Code, and OpenCode launchers; provider/model catalog; isolated launch settings; platform-dependent credential backends | Single Go binary without a persistent proxy; fewer global configuration writes                   | Launch isolation differs from persistent Desktop configuration; Codex still receives a temporary key-bearing file; no Windows native vault established |
| [ZCF][zcf] · `zcf@3.7.3`                                | Client initialization, API/CCR setup, MCP, workflow installation/updates, noninteractive setup                                       | Addresses onboarding a machine with no existing tools                                            | Broader environment changes; ownership, idempotence, and uninstall behavior in an existing team environment need testing                               |
| [ccman][ccman] · `v3.3.31`                              | Codex, Claude Code, Gemini, OpenCode; CLI/Desktop; MCP; WebDAV                                                                       | Existing multi-client configuration and synchronization                                          | Export scope and full lifecycle remain unverified; README declares MIT, but no standalone license file was found in the inspected tree                 |
| [aisw][aisw] · `v0.3.8`                                 | Account profiles, cross-client contexts, repository binding, isolated Codex homes, rollback                                          | Directly addresses work/personal account separation and project context                          | Isolated homes differ from a shared CLI/Desktop home; equivalent behavior cannot be assumed                                                            |
| [OpenCils/cc-switch-cli][opencils] · `v1.2.14`          | Separate project; terminal UI, native configuration writes, Windows/WSL discovery, optional proxy takeover                           | Explicitly discovers distinct Windows and WSL installations                                      | Separate from SaladDay's project; package declares ISC, but no standalone license file was found; multi-environment writes need testing                |
| [AI Provider Switcher][vs-switcher] · `v0.5.5`          | VS Code extension; providers, model catalog projection, Claude/Codex/Desktop settings, optional history migration                    | Editor-centered interaction; addresses configuration overrides and reconnection issues           | Editor-dependent; optional history migration must be compared with the no-history-rewrite requirement                                                  |
| [U-Pool][u-pool] · `v0.8.0`                             | Desktop multi-client configuration, Cursor account pool, owned-field writes, Windows environment integration                         | Visual management with Windows user-environment integration                                      | Only Windows assets observed; README explicitly says no license is declared; reuse permission is not established                                       |
| [cc-api-switcher-cli][ccsw] · `v0.1.0`                  | Single Go binary, Claude/Codex providers, templates, configuration import                                                            | Narrow responsibility and scriptable interface                                                   | Early release; insufficient evidence for team governance and complete lifecycle support                                                                |
| [Cursedpotential/ccswitch][small-ccswitch] · No release | Claude-focused provider, environment, and launch switching                                                                           | Smaller client-specific problem scope                                                            | No published release observed; multi-client management must not be inferred                                                                            |
| [code-agent-switch][agent-switch] · No release          | Claude/Codex/OpenCode configuration overlays, local hot-switching proxy, MCP                                                         | Explicit focus on relevant fields rather than whole-file replacement                             | Early project; implemented behavior, roadmap, and proposed UI need separation                                                                          |
| [CC Switch Lite][cc-lite] · No release                  | Focused provider/MCP/skill selection, shared database, ownership-checked projection                                                  | Narrower UI and writes with shared underlying capabilities                                       | Explicitly pre-alpha; concurrent writes with the full application remain restricted                                                                    |
| [CC Switch Core][cc-core] · No release                  | Rust application registry, native import/projection, deterministic writes, CAS/rollback, separate Store                              | Reusable configuration semantics already exist; complete reimplementation is not the only option | Sealed built-in adapter trait; host still owns exact I/O, locks, resource identity, and platform security; pre-1.0 integration needs assessment        |

### Local proxies and composed products

| Project and observed release            | Main capabilities                                                                                                                                 | Relative strengths                                                                   | Limitations or trade-offs                                                                                                             |
| --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------- |
| [CLIProxyAPI][cpa] · `v7.2.155`         | Go relay, multiple inbound protocols, API keys/OAuth, multiple accounts, SDK, native assets                                                       | Existing data plane and reuse ecosystem across authentication models                 | Generic compatible execution uses Chat Completions; example configuration binds all interfaces; SDK documentation has version drift   |
| [EasyCLIProxyAPI][easy-cpa] · `v0.2.81` | Tauri GUI, configuration, updates, tray, CLIProxyAPI management                                                                                   | Concrete example of a product UI reusing an existing data plane                      | Host UI and underlying runtime require separate version, update, and failure-boundary verification                                    |
| [Claude Code Router][ccr] · `v3.0.22`   | Multi-agent configuration, stable local endpoint, protocols, model routing, credential pools, retries/fallback, observability; Desktop/npm/Docker | Broader than a Claude-only tool; configuration and traffic management                | Adds a gateway to requests; translation and post-failover session semantics need testing; HEAD and release are distinct               |
| [CCS][ccs] · `v8.9.0`                   | CLI/dashboard, API/OAuth profiles, routing, accounts, CLIProxyAPI integration                                                                     | Uses composition instead of rebuilding every capability; concentrated user interface | Compatibility with the underlying proxy and optional fork must be verified; not all capabilities belong to one independent kernel     |
| [OpenCodex][opencodex] · `v2.48.0`      | Responses translation, streaming/tools/reasoning/images, multiple clients, background service, account pools, session affinity                    | Direct overlap with Proxy, not generic HTTP forwarding                               | Long histories, compaction, built-in tools, and lifecycle behavior need real tests; npm/Bun distribution still has runtime complexity |
| [9router][nine-router] · `v0.5.35`      | Multiple providers, local relay, accounts, dashboard                                                                                              | Another existing visual local aggregation product                                    | Not runtime-tested here; marketing claims of unlimited or free access do not establish quotas, authorization, or reliability          |

### Shared gateways and API distribution

| Project and observed release                        | Main capabilities                                                                                | Relative strengths                                                                              | Limitations or trade-offs                                                                                                                  |
| --------------------------------------------------- | ------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| [LiteLLM][litellm] · `v1.100.0`                     | SDK/gateway, providers including Bedrock, virtual keys, spend, routing, logs, client integration | Library and service reuse; organizational API access beyond local switching                     | Enterprise directory has separate licensing; deployment, database, and authorization requirements depend on enabled features               |
| [One API][one-api] · `v0.6.10`                      | Unified API, channels, load balancing, users/tokens/quotas, management API, Docker               | Already addresses third-party aggregation and downstream distribution; single-executable server | Observed release is older; modern Responses fidelity is not established; does not manage every local client's configuration                |
| [New API][new-api] · `v1.0.0-rc.36`                 | One API-derived multi-protocol aggregation, distribution, users/tokens/metering                  | Continued development of shared administration and protocol coverage                            | RC tag is not a stable-version claim; AGPLv3 and additional attribution terms require review                                               |
| [Portkey Gateway][portkey] · `v1.15.2`              | Open-source routing, fallback, retries, load balancing, observability integration                | Reusable unified API data plane with hosted-platform integration                                | OSS gateway and hosted platform differ; README's 2.0 prerelease features are not automatically delivered in 1.15.2                         |
| [Bifrost][bifrost] · API returned `ent-v2.1.0-base` | Go gateway, providers including Bedrock, routing, governance, extension interfaces               | Go data plane and reusable core for shared governance                                           | Observed tag names an enterprise component, not a directly comparable OSS runtime release; speed claims were not independently benchmarked |
| [LLM Gateway][llmgateway] · `v1.16.0`               | Unified model API, provider routing, management interface                                        | Covers parts of shared access and observability                                                 | LICENSE distinguishes AGPLv3 outside `ee/` from commercial enterprise code; complete self-hosting requirements remain unverified           |

Kong AI Gateway belongs to the adjacent infrastructure category. Its specific AI plugin versions and licensing were not examined to the same depth, so it is not presented as an equally assessed twenty-seventh candidate. Hosted aggregation APIs also differ from local-first configuration tools: they may change billing relationships and the parties handling request data.

## Source index

Repository links are pinned to the inspected snapshots. Release pages and documentation are interpreted at the collection date. Original retrievals remain in this Work Lane's `build/verification/c357a2cae88408e09d3a2b2360c03f55d00df8d3/competitive-analysis/`; the additional AWS reference and revision checks are under `build/verification/95ca0800b11ee754c876655e65c131714a0149fe/research-revision/`. These are verification outputs, not a second product or governance state store.

[cc-desktop]: https://github.com/farion1231/cc-switch/tree/f3b18df12007d0fd79fd8ad8d310880664015197
[cc-cli]: https://github.com/SaladDay/cc-switch-cli/tree/8a5614db0f582cea36268389a98ea3abe4eaa418
[muxlm]: https://github.com/Neo-Isshin/MuxLM/tree/72440581778996e0361b60eeaa16dc06b46651cf
[zcf]: https://github.com/UfoMiao/zcf/tree/63cb2d07a0fad3a9def37118ba056bafae780d44
[ccman]: https://github.com/2ue/ccman/tree/b8d58895f04460d7fcde7418584985c4ba9040d7
[aisw]: https://github.com/burakdede/aisw/tree/457b0093e25285c622045b94cd22c093e2f0912a
[opencils]: https://github.com/OpenCils/cc-switch-cli/tree/d8da75b2f96417cac686afac79d381ce4663bff7
[vs-switcher]: https://github.com/Silver-Zhang/ai-provider-switcher/tree/d1dbed86236a47dd9ec140f903e7b1c8f134f30d
[u-pool]: https://github.com/U-C4N/U-Pool/tree/eee309a16f76ad073c8f8df04c551157dfe249f4
[ccsw]: https://github.com/zhouyeyu/cc-api-switcher-cli/tree/3c0adaaab6001c564e7f2e0bd0e832277d9f083e
[small-ccswitch]: https://github.com/Cursedpotential/ccswitch/tree/211c4b7708e1d5954400676a4428b18ce772b0a5
[agent-switch]: https://github.com/IvanLark/code-agent-switch/tree/c73af9bd425983d4c59608e35e471a92a3922159
[cc-lite]: https://github.com/SaladDay/cc-switch-lite/tree/f65ad1b38fb8286f648c8390ff79cc578c8fa060
[cc-core]: https://github.com/SaladDay/cc-switch-core/tree/f5b6b4cd21c0207aea82ab21b8ca9281c2ff0569
[cpa]: https://github.com/router-for-me/CLIProxyAPI/tree/7fac6b15bcfe5ea55c18c9eaec8e5b7e6457d974
[easy-cpa]: https://github.com/router-for-me/EasyCLIProxyAPI/tree/895bff0d2c1e31e2e89ab63250b72256d0e7930e
[ccr]: https://github.com/musistudio/claude-code-router/tree/5ad5083b4eca8e0fe04f69f03b2e674772305425
[ccs]: https://github.com/kaitranntt/ccs/tree/03ff7d840001d504642c2fa25dc0c29d661be2fd
[opencodex]: https://github.com/lidge-jun/opencodex/tree/9a27e86992d7a014e0aa92c046199b9fac148201
[nine-router]: https://github.com/decolua/9router/tree/eb712ca821f0ba6bc41043fbd14494c5af5daba5
[litellm]: https://github.com/BerriAI/litellm/tree/ee7c7e14f3dd7c4c3930a423440ec26427e2c554
[one-api]: https://github.com/songquanpeng/one-api/tree/8df4a2670b98266bd287c698243fff327d9748cf
[new-api]: https://github.com/QuantumNous/new-api/tree/4fc9d1f1fa77c0cfdd9719cb59ff9ecc9885d66a
[portkey]: https://github.com/Portkey-AI/gateway/tree/669825cbe89ee51569918b8f78a9db486fd69dd4
[bifrost]: https://github.com/maximhq/bifrost/tree/86779fcfa89567caed4a1bfdd32e30b42bd4192c
[llmgateway]: https://github.com/theopenco/llmgateway/tree/2eeccaf513802bf8eac0c84f19355e8fdbd2eb3c
[desktop-security]: https://github.com/farion1231/cc-switch/blob/f3b18df12007d0fd79fd8ad8d310880664015197/SECURITY.md
[desktop-provider-storage]: https://github.com/farion1231/cc-switch/blob/f3b18df12007d0fd79fd8ad8d310880664015197/src-tauri/src/database/dao/providers.rs
[desktop-export]: https://github.com/farion1231/cc-switch/blob/f3b18df12007d0fd79fd8ad8d310880664015197/src-tauri/src/database/backup.rs
[cli-export]: https://github.com/SaladDay/cc-switch-cli/blob/8a5614db0f582cea36268389a98ea3abe4eaa418/src-tauri/src/database/backup.rs
[muxlm-storage]: https://github.com/Neo-Isshin/MuxLM/blob/72440581778996e0361b60eeaa16dc06b46651cf/storage.go
[muxlm-keys]: https://github.com/Neo-Isshin/MuxLM/blob/72440581778996e0361b60eeaa16dc06b46651cf/keys.go
[muxlm-launch]: https://github.com/Neo-Isshin/MuxLM/blob/72440581778996e0361b60eeaa16dc06b46651cf/launch.go
[core-adapter]: https://github.com/SaladDay/cc-switch-core/blob/f5b6b4cd21c0207aea82ab21b8ca9281c2ff0569/src/adapter.rs
[core-executor]: https://github.com/SaladDay/cc-switch-core/blob/f5b6b4cd21c0207aea82ab21b8ca9281c2ff0569/src/executor.rs
[opencils-package]: https://github.com/OpenCils/cc-switch-cli/blob/d8da75b2f96417cac686afac79d381ce4663bff7/package.json
[llmgateway-license]: https://github.com/theopenco/llmgateway/blob/2eeccaf513802bf8eac0c84f19355e8fdbd2eb3c/LICENSE
[desktop-release]: https://github.com/farion1231/cc-switch/releases/tag/v3.20.2
[desktop-issue-tools]: https://github.com/farion1231/cc-switch/issues/7230
[desktop-issue-route]: https://github.com/farion1231/cc-switch/issues/7217
[cli-stable-readme]: https://github.com/SaladDay/cc-switch-cli/blob/v5.10.4/README.md
[cli-release-note]: https://github.com/SaladDay/cc-switch-cli/blob/v5.10.4/docs/releases/v5.10.4.md
[cli-windows-issue]: https://github.com/SaladDay/cc-switch-cli/issues/294
[cpa-executor]: https://github.com/router-for-me/CLIProxyAPI/blob/7fac6b15bcfe5ea55c18c9eaec8e5b7e6457d974/internal/runtime/executor/openai_compat_executor.go
[cpa-sdk-doc]: https://github.com/router-for-me/CLIProxyAPI/blob/7fac6b15bcfe5ea55c18c9eaec8e5b7e6457d974/docs/sdk-usage.md
[cpa-module]: https://github.com/router-for-me/CLIProxyAPI/blob/7fac6b15bcfe5ea55c18c9eaec8e5b7e6457d974/go.mod
[cpa-builder]: https://github.com/router-for-me/CLIProxyAPI/blob/7fac6b15bcfe5ea55c18c9eaec8e5b7e6457d974/sdk/cliproxy/builder.go
[historical-desktop]: https://github.com/farion1231/cc-switch/tree/1cc52c7e736105b2765f7d51c38c67f909454912
[historical-cli]: https://github.com/SaladDay/cc-switch-cli/tree/2def7b847bb8bedd693373ff0771ca35e4c95874
[historical-cpa]: https://github.com/router-for-me/CLIProxyAPI/tree/c8803713c972af0076f55933fdeed4db81d72d24
[historical-ccr]: https://github.com/musistudio/claude-code-router/tree/deff4859b57f2d0822a3d1fb3f0504d34ac2b06c
[aws-gateway]: https://github.com/aws-samples/bedrock-access-gateway/blob/144c4bcb866d247dc1cef5b7c5d4cbea8bc7d633/README.md
[aws-native-adoption]: https://github.com/aws-solutions-library-samples/guidance-for-claude-code-with-amazon-bedrock/blob/014d7abe7cc8fb1820ffe1e3b26df4ac30d23fd9/README.md
