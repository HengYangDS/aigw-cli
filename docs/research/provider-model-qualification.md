<!--
---
subject: aigw:provider-model-qualification
role: research
state: active
relations: {}
---
-->

# Provider Model Qualification Evidence

These dated observations inform the reviewed September 30, 2026 team
manifest. They do not maintain a live catalogue or override the
[current Route inventory](../../manifests/team.toml). For member setup and
model selection, use the [team rollout guide](../guides/team-rollout.md#reviewed-model-defaults).
A public listing, bounded inference call, and native-client tool loop prove
different claims.

## Curated model choices

Select one general-model option per vendor from exact Account/protocol Routes
with completed inference and compatible native-client evidence. Vendor
positioning informs this choice; a version suffix, catalogue listing or
vendor benchmark does not establish an independent quality ranking.

The October 1, 2026 primary-source review closes four missing rationales:

- Google's [Gemini 3.1 Pro Preview](https://ai.google.dev/gemini-api/docs/models/gemini-3.1-pro-preview)
  improves the Gemini 3 Pro series for thinking, software engineering and
  precise multi-step tool use. Retain this explicitly labelled Preview as the
  qualified Pro-class option, not a stable-release claim. Google's newer
  [Gemini 3.8 Flash](https://ai.google.dev/gemini-api/docs/models/gemini-3.8-flash)
  targets long-horizon agents and is listed by AIHubMix, but has no current
  AIGW inference/client qualification. Neither `Flash` nor a newer number
  proves superiority over the selected Pro Route; qualify that exact channel
  before replacing it.
- Z.ai's [GLM 5.3 model card](https://huggingface.co/zai-org/GLM-5.3)
  identifies post-training improvements over GLM 5.2 for complex coding and
  long-horizon tasks. This supports GLM 5.3 as the reviewed general coding
  option; its vendor-reported cross-model scores are not AIGW benchmarks.
- Moonshot's [Kimi K3 source](https://github.com/MoonshotAI/Kimi-K3)
  calls K3 its most capable model to date for long-horizon coding, knowledge
  work and reasoning. Keep the exact `kimi-k3` Routes rather than retaining
  another older general Kimi slot.
- Qwen's [3.8 model card](https://huggingface.co/Qwen/Qwen3.8-2.4T-A95B)
  calls 3.8 its most capable open generation and explicitly identifies
  Qwen3.8-Max as its managed version. This supports `qwen3.8-max`; it does not
  transfer the vendor's one-million-token or built-in-tool claims to an
  aggregator without separate evidence.

Stable-only supply-chain admission does not make every inference model GA.
A Preview may remain a non-default, explicitly named Route when its general
agent role and exact provider/client behavior are qualified. An unqualified
Preview remains discovery-only; do not conceal that distinction or silently
replace an explicit local selection.

## Provider catalogue and route evidence

At an earlier September 30, 2026 read, AIGW observed 417 AIHubMix, 565 DMXAPI,
and 276 UCloud IDs across eight Account/protocol catalogue surfaces. All 60
Routes shipped at that time had wire IDs on their declared surfaces; the current
manifest has 56 after the Solar Route, three GPT-6 Sol Routes, the DMXAPI
Fable 5.1 CC channel, and unavailable AIHubMix Fable 5.1 and MiMo Routes were removed,
with AIHubMix, DMXAPI, and UCloud GPT-6.1
Sol Routes added. The earlier DMXAPI Responses observation used the locally configured
`127.0.0.1:8792` Proxy,
whereas the team manifest declares direct `https://www.dmxapi.cn/v1`.
Catalogue membership therefore does not qualify that direct endpoint or
prove the newly preferred Route can sustain inference.

The DMXAPI `claude-fable-5-1-cc` wire ID remained listed, but two bounded
Claude Code verifications each reached the 60-second client deadline and one
independent direct Messages request reached a 28-second transport deadline
without an HTTP response on September 30. The curated manifest therefore
withdraws this channel while retaining the base Fable 5.1 Route. These failures
do not establish permanent provider unavailability; a new real-client inference
result is required before readmission.

A later isolated public `check` and one direct Anthropic Messages request
both rejected AIHubMix `claude-fable-5-1` with HTTP 400 and the explicit
message that the model cannot be served at the moment. The curated manifest
withdraws that Account's Route until new inference qualifies it. DMXAPI and
UCloud Fable 5.1 requests and real Claude Code sessions succeeded; their
Routes and the shared logical Model remain. A provider's model refusal is
separate from Account authentication and does not change explicit local bindings.

On September 30, 2026,
[OpenAI's GPT-6.1 Sol model page](https://developers.openai.com/api/docs/models/gpt-6.1-sol)
identified `gpt-6.1-sol` and Responses tool calling. AIHubMix's
[public catalogue](https://api.inferera.com/v1/models)
initially listed only `gpt-6-sol`, but a later September 30 read returned 418 IDs
and included `gpt-6.1-sol`. A later authenticated DMXAPI listing also included
that exact ID; UCloud's did not. Direct Responses text requests completed on
AIHubMix and DMXAPI, while UCloud rejected the model. An isolated Codex tool
loop then succeeded on AIHubMix but an earlier DMXAPI tool request was rejected
with `missing_required_parameter` for `tools[4].tools`. A later authenticated
DMXAPI listing returned the exact `gpt-6.1-sol` ID; direct Responses text and a
strict function-call request both completed. The installed official Codex CLI
0.159.2 then completed two isolated direct-DMXAPI tool loops. Its bundled
catalogue includes GPT-6.1 Sol. AIGW's earlier base-model projection also
omitted `model_catalog_json`: real Codex tool use succeeded, but its model
manager logged two errors decoding DMXAPI's standard `/v1/models` response as
Codex metadata. An isolated comparison using the client's unmodified bundled
catalog eliminated those errors. The existing AIGW catalog owner now pins that
same table for known base models on its custom Provider; final-product warning
acceptance remains open.

The official Hermes v0.21.5 release source completed two named-session turns
against direct DMXAPI GPT-6.1 Sol. AIGW 0.3.3 then completed an isolated
`verify --for hermes --route dmxapi-gpt-6.1-sol` against that endpoint after
its versioned credential reader was staged. This distinguishes a missing test
reader from a product failure. The public AIHubMix catalogue also listed the
model, while an earlier UCloud request rejected it. Later on September 30, a
direct UCloud Responses request completed with exact wire model `gpt-6.1-sol`.
Installed AIGW 0.3.3 then completed bounded `verify --for codex` and
`verify --for hermes` sessions for an explicitly imported UCloud Route using
Codex CLI 0.159.2 and Hermes Agent v0.21.5. That supersedes the earlier
UCloud exclusion for bounded direct and native-client inference, not sustained
availability, final release bytes, or this host's optional Proxy path. The
team retains its declared DMXAPI-first order, followed by UCloud and AIHubMix;
existing explicit local selections are not silently rewritten. Final-artifact
admission remains open.

At the September 25, 2026 read, the public
[AIHubMix](https://api.inferera.com/v1/models) and
[UCloud](https://api.modelverse.cn/v1/models) endpoints returned 416 and 132
IDs respectively. This is a dated provider listing, not Route inference or a
claim that private catalogues contain no further models. OpenAI/Openai,
InclusionAI/Inclusionai, and ByteDance/Doubao owner labels refer to the same
vendors; Llama entries belong to Meta, already represented by Muse Spark.
Those labels do not create extra general-model slots.

The public
AIHubMix catalogue also lists Cohere Command A+ and Microsoft's MAI Thinking 1.
Command A+ returned HTTP 400 on the tested Responses and Chat endpoints;
MAI Thinking 1 and Step 5 Preview lack the completed general-agent
provider/client qualification required above. These listings are not admitted
Routes; their Preview labels alone do not establish unavailability.

AIHubMix's `agnes-3.0-flash` returned Chat text, but its
[preview model card](https://huggingface.co/Agnes-AI/Agnes-3.0-Flash) and that
single response do not qualify a general-agent client Route. `intern-s2-free`
also returned Chat text, but that aggregator alias does not identify a specific release in
[InternLM's Intern-S2 model collection](https://huggingface.co/collections/internlm/intern-s2).

On September 29, 2026, [Anthropic's model overview](https://platform.claude.com/docs/en/models/overview)
identified `claude-sonnet-5-5`. Authenticated catalogues and one direct
Anthropic Messages request per exact wire ID qualified AIHubMix and UCloud's
ordinary Sonnet 5.5 Routes and DMXAPI's ordinary, CC, and SSVIP Routes. Each
request returned HTTP 200, model `claude-sonnet-5-5`, and nonempty text; these
bounded calls do not establish streaming, tools, latency, or native-client use.
The previous Sonnet 5 Routes were retired from the shipped manifest.

Specialized, small, unidentified or unqualified public catalogue entries from
Jina AI, Liquid, Dots Studio, Sao10k, and Stealth are not general-model Routes.

MiniMax M3 has AIHubMix and UCloud Routes; its DMXAPI candidate timed out.
Muse Spark 1.3 has an AIHubMix Route only; DMXAPI's unrelated Spark IDs are
not Meta models, and no UCloud Muse Route was observed.

Model entries carry identity only; client-scoped
recommendations express preference without a global benchmark or cost claim.

All three configured Accounts listed the retained September 21 set in
authenticated catalogue observations, and their selected protocols completed
minimal inference calls at that time. On September 23, 2026, UCloud also
listed `gpt-6-sol` and `gpt-6-luna`; both completed minimal OpenAI Responses
requests. On September 25, all three Accounts completed minimal authenticated
Opus 5.5 requests; AIHubMix and UCloud also completed GPT-6 Sol requests.
DMXAPI GPT-6 Sol previously completed through the locally configured Proxy
transport and once through its direct Responses endpoint. Repeated direct
requests on September 25 returned HTTP 503, so release 0.3.1 omitted that
Route. Two later direct Responses requests completed with text, so the
September 27, 2026 manifest restored it. The September 30 manifest then
retired the GPT-6 Sol Routes and recommends DMXAPI GPT-6.1 Sol for unselected
Codex and Hermes clients; neither the earlier calls nor that recommendation
proves sustained availability or runtime failover.
AIHubMix and DMXAPI GPT-6 Luna also completed exact Responses requests; all
three Accounts have evidenced Luna
Routes. Existing local Route and Client Binding state remains separate and is
not rewritten by this catalogue change.

The earlier two rejected UCloud Opus IDs remain historical observations.
Catalogue membership and one successful text call remain narrower than
complete tool, streaming, long-context, cost, or latency qualification.

DMXAPI's retained CC, SSVIP, and CDX channels remain separate Routes within
the Claude and GPT families. Each Route retains its exact wire ID while its
`model` names the base logical Model; a channel is not another logical model.
Other Accounts use their ordinary model identifiers. Channel names are not
substitutes for the native model selected in an existing Codex conversation.

The September 27 [DMXAPI public catalogue](https://rmb.dmxapi.cn/) listed ordinary
and CC Fable 5.1, ordinary/CC/SSVIP Sonnet 5, and ordinary/CDX/SSVIP GPT-6
Astra. The Sonnet channels were superseded by the September 29 qualification
above; the other previously qualified channels remain. Opus 5
channels are outside the requested logical set. The ordinary DMXAPI Opus 5.5
Route was admitted after authenticated inference; no Opus 5.5 CC, SSVIP, or
CDX variant was admitted. An unauthenticated DMXAPI model request returning
401 establishes neither presence nor absence of an individual model.

The public AIHubMix `/v1/models` response observed on September 24, 2026,
listed Opus 5.5, all three GPT-6 IDs, `minimax-m3`,
`deepseek-v4.1-flash`, and Doubao Seed 2.1 Pro. UCloud's public
`/v1/models` response listed `MiniMax-M3`, `deepseek-v4.1-flash`, Doubao
Seed 2.1 Pro, and `MiniMax-H3-Max`. These listings are discovery evidence,
not newly qualified Routes.

A second read on September 24, 2026 at 10:28 UTC returned 416
[AIHubMix catalogue IDs](https://api.inferera.com/v1/models) and 132
[UCloud catalogue IDs](https://api.modelverse.cn/v1/models). AIHubMix also
listed `cc-minimax-m3`, `coding-minimax-m3`, and `grok-4.7`. The public UCloud
response listed no GPT-6 or Claude IDs; that omission does not contradict
prior authenticated UCloud inference. Neither catalogue identifies a model's
protocol, and a prefixed MiniMax wire ID is not a separate logical Model by
itself.

MiniMax's [H3 announcement](https://www.minimax.io/blog/minimax-h3)
describes a multimodal generation model that outputs video with sound. H3
family names in a provider catalogue are not evidence of a general text
model. Its [M3 announcement](https://www.minimax.io/blog/minimax-m3)
positions M3 as an LLM for coding and agentic work. AIHubMix's `minimax-m3`
and UCloud's `MiniMax-M3` completed bounded text inference; DMXAPI's
`MiniMax-M3` timed out and remains unadmitted.

DeepSeek's [V4.1 Flash announcement](https://www.deepseek.com/en/news/deepseek-v4-1-flash/)
claims benchmark results ahead of V4 Pro and says `deepseek-v4-pro` requests
on DeepSeek's own API now route to V4.1 Flash. That does not establish what
an aggregator serves for `deepseek-v4-pro-0813`. The exact
`deepseek-v4.1-flash` ID completed bounded text inference on all three
Accounts, so it replaced V4 Pro 0813 in the September 27, 2026 one-DeepSeek catalogue.
Existing explicit local selections of the old Route are not deleted by a
manifest import. Do not rank model quality from `Pro`, `Max`, or `Flash` in an ID.

ByteDance [positions Seed2.1](https://seed.bytedance.com/en/seed2_1) for
general agent and coding work. All three Accounts completed bounded text
inference for `doubao-seed-2-1-pro-260628`, the exact version listed in their
catalogues. A newer `260915` appears in the vendor's documentation but was
not listed by these Accounts, so the reviewed Route did not claim it.

Meta [positions Muse Spark 1.3](https://research.meta.ai/blog/introducing-muse-spark-1-3)
for general agentic and coding tasks. AIHubMix listed the exact
`muse-spark-1.3` ID. A 16-token Responses probe returned HTTP 200 but an
incomplete result without output. One earlier 128-token call completed, but
later short `ping` calls at that cap were incomplete; a 512-token explicit
short-answer request completed with text. The bounded AIGW probe was set to
512 output tokens and rejected an incomplete HTTP 200 response.

Xiaomi's [MiMo model guide](https://mimo.mi.com/docs/en-US/quick-start/summary/model)
recommends `mimo-v2.6-pro` for complex projects and long-running work; its
[V2.6 release](https://mimo.mi.com/docs/en-US/news/latest/v2-6) describes Pro
as the stronger model in that series. The exact lower-case ID returned
completed text through the configured AIHubMix and UCloud Responses endpoints
on September 25. A later AIHubMix `ping` call was incomplete even at 512
tokens, while the explicit short-answer request completed. No DMXAPI MiMo
Route was inferred from those observations.
On October 1, the exact AIHubMix Route returned HTTP 400
`The model mimo-v2.6-pro cannot be served at the moment` through both AIGW and
one official OpenAI SDK 2.24.0 request with retries disabled. Withdraw only
that Account's Route; retain the canonical Model and qualified UCloud Route.
The older completed request does not prove current availability.

Baidu [positions ERNIE 5.1](https://ernie.baidu.com/blog/posts/ernie-5.1-0508-release/)
as its current general reasoning model; the exact AIHubMix Responses ID
completed with text at a 512-token cap. Mistral calls
[Large 3](https://mistral.ai/news/mistral-3/) its most capable general model;
the exact AIHubMix Chat Completions ID completed with text. The same configured
AIHubMix Chat endpoint produced text for
Inception's [Mercury 2.5](https://www.inceptionlabs.ai/blog/introducing-mercury-2-5),
Meituan's [LongCat 2.0](https://huggingface.co/meituan-longcat/LongCat-2.0),
Tencent's [HY3](https://huggingface.co/tencent/Hy3),
StepFun's [Step 3.7 Flash](https://huggingface.co/stepfun-ai/Step-3.7-Flash),
and InclusionAI's [Ling 3.0 Flash](https://huggingface.co/inclusionAI/Ling-3.0-flash).

NVIDIA identifies [Nemotron 3 Ultra](https://research.nvidia.com/labs/nemotron/Nemotron-3-Ultra/)
as the family's final and strongest model; [Super](https://research.nvidia.com/labs/nemotron/Nemotron-3-Super/)
is smaller. The listed AIHubMix Ultra `-free` channel once returned text, but
at signed AIGW source `8891b56c` two isolated, unmodified official Hermes
verifications each timed out after 60 seconds despite a successful AIGW
`check`. In the same source build with a temporary manifest, the exact Super
`nemotron-3-super-120b-a12b-free` channel returned direct Chat text and
completed AIGW `use`/`check` plus two official Hermes verifications. The team
manifest therefore selects Super as the currently qualified NVIDIA option on
this Account, **not** as NVIDIA's strongest model. Ultra remains discoverable
through the provider catalogue but is not an admitted team Route. Neither two
successes nor two timeouts establish long-term channel behavior; final release
bytes and cross-platform real-client admission remain open.

The same endpoint previously produced text for Upstage's
[Solar Pro 4](https://www.upstage.ai/blog/en/solar-pro-4). A September 30,
2026 direct Chat Completions recheck of the exact `solar-pro4` wire ID at
`https://api.inferera.com/v1` returned HTTP 400 with `no_available_channel`.
The public catalogue still listed that ID, but listing is not service
availability. The AIHubMix Route and its now-unreferenced logical Model were
removed from the shipped manifest; this does not claim Solar Pro 4 is
unavailable from Upstage or that the aggregator can never restore its channel.

Cohere's [Command A guide](https://docs.cohere.com/docs/command-a) identifies
`command-a-03-2025` as a general agent model. AIHubMix completed Chat text
inference for that exact ID. Its newer listed Command A+ returned HTTP 400
on both tested protocols, so Command A is the strongest completed Cohere Route
on this configured Account, not a claim about Cohere's overall strongest model.
Poolside [positions Laguna S 2.1](https://poolside.ai/blog/introducing-laguna-s-2-1)
for long-horizon agentic coding; its [model card](https://huggingface.co/poolside/Laguna-S-2.1)
also documents text-to-text Chat use. It is the strongest S/XS model listed
by AIHubMix, and its exact `laguna-s-2.1` Chat Route completed text inference.

xAI's [model guide](https://docs.x.ai/developers/models) still recommends
Grok 4.6, but its later [September 21 Grok 4.7 announcement](https://x.ai/news/grok-4-7)
calls 4.7 its most capable model for coding and knowledge work. The exact `grok-4.7`
Responses ID completed text inference on AIHubMix, DMXAPI, and UCloud, so it
replaces Grok 4.6 without losing Account coverage. Existing explicit local
bindings to Grok 4.6 are not silently rewritten by the team manifest.

UCloud returned a 200 response with nonstandard `response` and `type` fields
for `glm-5.3` on `/v1/responses`, without a completed Responses output.
Its `/v1/chat/completions` endpoint returned standard assistant text at 512
tokens, so the UCloud GLM Route declares Chat Completions only.

AIHubMix Gemini 3.1 Pro Preview timed out once at a 30-second transport limit,
then returned completed text in 48 seconds with a longer bounded deadline.
`aigw check` keeps endpoint-only attempts at five seconds and allows one
60-second inference attempt; neither a timeout nor an HTTP 200 without usable
output is reported as healthy.

The public UCloud response is not a complete substitute for the earlier
authenticated GPT and Claude observations. Admit any new Route only after a
bounded noninteractive call to its exact Account, protocol, and wire model
succeeds.

## Codex native chooser

Codex can execute a non-OpenAI Responses Route without listing it in its
[native `/model` chooser](https://developers.openai.com/codex/cli/slash-commands).
On 2026-09-27, an isolated Codex CLI 0.157.1 session ran Grok 4.7 but its
chooser listed only bundled GPT models. Separately, the AIGW-projected Grok
Route completed an authenticated tool loop against a controlled endpoint.
Select such Routes with [`aigw use --for codex`](../../README.md#use-it-every-day)
and verify them through Codex. AIGW does not invent missing Codex catalogue
metadata, alter an existing Desktop conversation's model, or infer that the
upstream Account is currently available from this local-client result.

Every Claude Route in the [shipped team manifest](../../manifests/team.toml)
currently declares only an Anthropic interface, whereas Codex's
[custom-provider contract](https://learn.chatgpt.com/docs/config-file/config-reference#configtoml)
supports the Responses wire API. No shipped AIGW Claude Route is therefore
admitted for Codex. A provider-native Responses endpoint or an explicitly
selected external adapter would need real Codex text, tool, streaming, and
replay qualification before such a Route could be offered. AIGW does not
translate inference traffic or configure ordinary ChatGPT conversations.
