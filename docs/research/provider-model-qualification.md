<!--
---
subject: aigw:provider-model-qualification
role: research
state: active
relations: {}
---
-->

# Provider Model Qualification Evidence

These dated observations informed the reviewed September 27, 2026 team
manifest. They do not maintain a live catalogue or override the
[current Route inventory](../../manifests/team.toml). For member setup and
model selection, use the [team rollout guide](../guides/team-rollout.md#reviewed-model-defaults).
A public listing, bounded inference call, and native-client tool loop prove
different claims.

## Provider catalogue and route evidence

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
Microsoft documents MAI Thinking 1 as preview. Step 5 Preview is also outside
the stable-model selection. These listings are not admitted Routes.

AIHubMix's `agnes-3.0-flash` returned Chat text, but the [model card](https://huggingface.co/Agnes-AI/Agnes-3.0-Flash)
calls it a preview, so it is not admitted. `intern-s2-free` also returned Chat
text, but that aggregator alias does not identify a specific release in
[InternLM's Intern-S2 model collection](https://huggingface.co/collections/internlm/intern-s2).

On September 29, 2026, the public AIHubMix catalogue also listed
`claude-sonnet-5-5`, while [Anthropic's model overview](https://docs.anthropic.com/en/docs/about-claude/models/overview)
named `claude-sonnet-5` as its current Sonnet API ID. The aggregator listing is
discovery evidence, not grounds to replace the admitted Sonnet 5 Route without
authenticated inference and native-client qualification.

Specialized, small, preview, or unidentified public catalogue entries from
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
September 27, 2026 manifest restored it as a setup alternative, not a claim of
sustained availability or runtime failover. AIHubMix and DMXAPI GPT-6 Luna also
completed exact Responses requests; all three Accounts have evidenced Luna
Routes. Existing local Route and Client Binding state remains separate and is
not rewritten by this catalogue change.

The earlier two rejected UCloud Opus IDs remain historical observations.
Catalogue membership and one successful text call remain narrower than complete tool, streaming,
long-context, cost, or latency qualification.

DMXAPI's retained CC, SSVIP, and CDX channels remain separate Routes within
the Claude and GPT families. Each Route retains its exact wire ID while its
`model` names the base logical Model; a channel is not another logical model.
Other Accounts use their ordinary model identifiers. Channel names are not
substitutes for the native model selected in an existing Codex conversation.

The reviewed [DMXAPI public catalogue](https://rmb.dmxapi.cn/) listed ordinary
and CC Fable 5.1, ordinary/CC/SSVIP Sonnet 5, and ordinary/CDX/SSVIP GPT-6
Astra. The September 27, 2026 manifest kept those previously qualified channels. Opus 5
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

Baidu [positions ERNIE 5.1](https://ernie.baidu.com/blog/posts/ernie-5.1-0508-release/)
as its current general reasoning model; the exact AIHubMix Responses ID
completed with text at a 512-token cap. Mistral calls
[Large 3](https://mistral.ai/news/mistral-3/) its most capable general model;
the exact AIHubMix Chat Completions ID completed with text. The same configured
AIHubMix Chat endpoint produced text for NVIDIA's
[Nemotron 3 Ultra](https://huggingface.co/nvidia/NVIDIA-Nemotron-3-Ultra-550B-A55B-BF16),
Upstage's [Solar Pro 4](https://www.upstage.ai/blog/en/solar-pro-4),
Inception's [Mercury 2.5](https://www.inceptionlabs.ai/blog/introducing-mercury-2-5),
Meituan's [LongCat 2.0](https://huggingface.co/meituan-longcat/LongCat-2.0),
Tencent's [HY3](https://huggingface.co/tencent/Hy3),
StepFun's [Step 3.7 Flash](https://huggingface.co/stepfun-ai/Step-3.7-Flash),
and InclusionAI's [Ling 3.0 Flash](https://huggingface.co/inclusionAI/Ling-3.0-flash).
The Nemotron wire ID carries AIHubMix's `-free` channel suffix; it is one
canonical NVIDIA Model, not a second logical model.

Cohere's [Command A guide](https://docs.cohere.com/docs/command-a) identifies
`command-a-03-2025` as a general agent model. AIHubMix completed Chat text
inference for that exact ID. Its newer listed Command A+ returned HTTP 400
on both tested protocols, so Command A is the strongest completed Cohere Route
on this configured Account, not a claim about Cohere's overall strongest model.
Poolside [positions Laguna S 2.1](https://poolside.ai/blog/introducing-laguna-s-2-1)
for long-horizon agentic coding; its [model card](https://huggingface.co/poolside/Laguna-S-2.1)
also documents text-to-text Chat use. It is the strongest S/XS model listed
by AIHubMix, and its exact `laguna-s-2.1` Chat Route completed text inference.

xAI's [model guide](https://docs.x.ai/developers/models) consulted in September 2026 calls Grok
4.7 its flagship for code and other general tasks. The exact `grok-4.7`
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
