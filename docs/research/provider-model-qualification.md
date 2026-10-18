---
subject: aigw:provider-model-qualification
role: research
state: active
relations: {}
---

# Provider Model Qualification Evidence

These observations inform the October 16, 2026 team manifest; they neither
maintain a live catalogue nor override the [Route inventory](../../manifests/team.toml).
For setup/selection, use the [team rollout guide](../guides/team-rollout.md#reviewed-model-defaults).
Listings, bounded inference and native-client tool loops prove distinct claims.

## Curated model choices

Select one general model per vendor from exact Account/protocol Routes qualified
by inference and native-client evidence; positioning, suffixes, listings and vendor
benchmarks inform selection but do not prove independent quality rankings.

The October 1, 2026 primary-source review closes four missing rationales:

- Google's Gemini 3.8 Flash, selected over 3.1 Pro Preview on October 5, is replaced by the [October 16 admission](#october-16-gemini-4-argon-admission).
- Z.ai's [GLM 5.3 model card](https://huggingface.co/zai-org/GLM-5.3)
  reports coding/long-horizon post-training improvements over GLM 5.2, supporting
  general coding use. Vendor-reported cross-model scores are not AIGW benchmarks.
- Moonshot's [Kimi K3 source](https://github.com/MoonshotAI/Kimi-K3)
  calls K3 its most capable model for long-horizon coding, knowledge and reasoning;
  retain exact `kimi-k3` Routes, not a second older general Kimi slot.
- Qwen's [3.8 model card](https://huggingface.co/Qwen/Qwen3.8-2.4T-A95B)
  calls 3.8 its most capable open generation and Qwen3.8-Max its managed version,
  supporting `qwen3.8-max`. Million-token/built-in-tool claims need aggregator evidence.

The October 4, 2026 review did not admit speculative replacements. Anthropic's
[Fable page](https://www.anthropic.com/claude/fable) still identified Fable 5.1,
without a verified Fable 5.5 API identity, and Gemini 4 Argon was trusted-tester-only.
MiniMax's [language models](https://platform.minimax.io/docs/guides/models-intro) identified
M3 for coding; [H3](https://www.minimax.io/blog/minimax-h3) generates video and
sound, not text-agent inference. Step 5 Preview was discovery-only at the
September 25 read; its later [admission](#october-56-model-refresh) supersedes that observation.

Public catalogues returned 417 AIHubMix and 128 UCloud IDs; anonymous UCloud omitted
GPT/Claude, while DMXAPI required authentication. Neither omission nor anonymous
401/disabled-pricing 403 proves model absence. Bindings, credentials and models were unchanged.

Stable tool admission does not make inference models GA. Qualified general-agent
Previews may remain explicitly named, non-default Routes; unqualified ones remain
discovery-only. Preserve this distinction and explicit local selections.

## October 16–18 Claude Route observations

Claude Code 2.1.301 on UCloud `claude-opus-5-5`, with experimental betas lifted,
resolved a deferred MCP tool through `ToolSearch` (`tool_reference`) and called
it. Desktop's host shape keeps the other betas off and remains unqualified, so
no Route declares `tool_search` yet.

The October 18 cache-continuity step found that this workstation's October 11–18
Claude client requests on `claude-opus-5-5` read the cache in 92.6% of 52,214 pairs.
The 14,245 answered with a `msg_` ID and `inference_geo: "global"` hit in 79.5%, a
cost limitation (80.1% between two such responses), against 96.3–99.5% for five
other signatures. Five scripted journeys each on UCloud `claude-sonnet-5-5` and
`claude-haiku-5-5`, one signature per journey, hit in 19 of 20 pairs (the miss on
`msg_bdrk_`) and 20 of 20; both are too few to rank signatures. UCloud's guidance
asks for a stable `metadata.user_id`, which Claude Code 2.1.302 already sends.

## October 16 Gemini 4 Argon admission

Google's [changelog](https://ai.google.dev/gemini-api/docs/changelog) made its flagship reasoning model
[`gemini-4-argon`](https://ai.google.dev/gemini-api/docs/models/gemini-4-argon) stable on October 13. AIGW
0.4.2 then listed the exact ID on all eight authenticated surfaces, after one failed AIHubMix read.
Hermes v0.22.2 (local `2cde9522`, modified tree), using existing credential commands, read a private
file via `read_file` and recalled it tool-free in the resumed session on AIHubMix and direct DMXAPI
Responses and UCloud Chat (37.9/15.2, 60.5/11.8, 14.3/16.8 s). An earlier AIHubMix run stalled for
210 s; every run warned `install out of sync`. Argon replaces Gemini 3.8 Flash; other clients stay unqualified.

## October 10 Haiku 5.5 admission

Eight authenticated observations list `claude-haiku-5-5` at AIHubMix/UCloud and
`claude-haiku-5-5-cc` at DMXAPI, with 420/277/528 IDs respectively. This supersedes
only the dated Haiku absence, not the Fable 5.5 or Gemini 4 limits. The October 10
catalogue withdraws Fable 5.1 and Astra/Luna 6 under the Claude 5.5+/GPT 6.1+
floors. No qualified Fable, Astra, Terra, or Luna successor was observed.

Claude Code 2.1.296 at `xhigh` passed exact text, `Read` and same-process recall
for all three Routes; the second input omitted the marker. Probes used original
readers, private scratch and no session persistence; user state stayed unchanged.
Opus 5.5/Sol 6.1 recommendations remain; raw results bind `cc4ddf9ac8c264fc29795ae885964143a7959c3c` under the existing verification owner.

AIHubMix rejected `context_management` with HTTP 400; UCloud passed text but
rejected tool continuation with a duplicate-`tool_result` Bedrock error of
unproved origin. Both then passed in isolated [compatibility mode](../guides/team-rollout.md#client-compatibility)
with `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1`. Retain failures and this condition;
default betas, other clients/platforms, full-window and release bytes remain unqualified.

## October 5–6 model refresh

Eight sources returned 417/523/275 AIHubMix/DMXAPI/UCloud IDs; October 6 found
no changes. All listed Sol 6.1, Astra/Luna 6, Opus/Sonnet 5.5 and Fable 5.1. Dated
[model overview](https://platform.claude.com/docs/en/models/overview) and
[release notes](https://platform.claude.com/docs/en/release-notes/overview)
reads did not establish Fable or Haiku 5.5; Haiku 4.5 was not substituted.
Gemini 4 remained trusted-tester-only. [MiniMax M3.1 Flash Preview](https://platform.minimax.io/docs/guides/models-intro)
required M Plan/MiniMax Code and appeared in none of these Accounts; M3 stayed.
Installed AIGW 0.3.1 passed native Claude/Codex checks for UCloud Opus 5.5/Sol 6.1;
configuration stayed byte-identical, without import, sync, restart, full Route
requalification or independent ranking.

The following direct channels completed text inference, an actual Hermes
`read_file` call, a final marker and same-session recall with medium effort:

| Model               | Qualified Account / protocol                               | Selection rationale                                                                                                                                                              |
| ------------------- | ---------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Gemini 3.8 Flash    | AIHubMix and DMXAPI / Responses; UCloud / Chat Completions | Google's [current GA workhorse](https://deepmind.google/models/gemini/flash/) targets coding and agents.                                                                         |
| Step 5 Preview      | AIHubMix / Chat Completions; DMXAPI / Responses            | StepFun's [flagship agent release](https://www.stepfun.com/step-5-preview) replaces the earlier Step 3.7 option.                                                                 |
| HY4 Preview         | DMXAPI / Responses                                         | Tencent's [successor announcement](https://www.tencent.com/tencent-releases-and-open-sources-tencent-hy4-preview/) identifies improvements over HY3 for real productivity tasks. |
| LongCat 2.5 Preview | DMXAPI / Responses, exact wire `LongCat-2.5-Preview`       | Meituan's [September 25 release](https://longcat.chat/platform/docs/change-log) adds multimodal understanding and stronger coding.                                               |

The three Preview labels remain explicit, non-default model choices. Their
vendor claims are not independent benchmarks. UCloud Gemini 3.8 Responses
returned `convert_request_failed`; its admitted Chat route passed instead.
AIHubMix HY4 returned the file-tool output but exceeded the bounded completion
deadline, so that Account's Route remains unadmitted. Cohere Command A+ returned
`no_available_channel` on Chat Completions and stays discovery-only.
DeepSeek's [current announcement](https://www.deepseek.com/en/news/deepseek-v4-1-flash/)
phases out V4 Pro in favor of V4.1 Flash. Retain qualified V4.1 Flash rather than
restoring the older Pro name solely because it sounds stronger.

The isolated Hermes executable was version 0.21.5, upstream `af90026a`, local
`75e983671a840a1adcf249ab524d3c7847105489`, with OpenAI SDK 2.24.0. This proves
that installed client's direct file-tool/continuation journey, not an unmodified
upstream client, every client/OS, large-context behavior or final release bytes.
DMXAPI's operator-selected local Responses Proxy rejected these native inputs
with `unproved provider-portable structure`; the direct HTTPS channels passed.
Preserve that separate compatibility failure and the explicit local endpoint;
do not infer Proxy acceptance from direct-provider success.

This release's catalogue is frozen at these qualified identities. Import retains
explicit selected Routes, credentials and endpoint overrides. Retire an older
unselected Route through native import retirement only after its projections and
recovery consumer are checked; absence from the new manifest is not deletion.

The local retained-state import passed with 26 Models and 58 Routes. It
preserved Account endpoints, explicit client selections, the installed credential
executable and all credential commands. Hermes read the updated native catalogue
with exact wire IDs. Installed AIGW 0.3.1 still reports Hermes projection drift
because its catalogue checker compares logical IDs; the current source checker
passes. This data refresh is not a binary upgrade or final release acceptance.

## Provider catalogue and route evidence

The October 9, 2026 review matched all 57 shipped Routes to dated authenticated
inference: 48 unchanged October 1 matrix entries, seven October 5–6 entries
bound to post-acceptance source `de3982f3`, and the two isolated October 1
additions below. Each retained the same Account endpoints, wire ID and protocol.
This qualifies the frozen reviewed catalogue, not current service health,
an independent strongest-model ranking, every native tool/resume mode or final
signed artifacts.

At an earlier September 30, 2026 read, AIGW observed 417 AIHubMix, 565 DMXAPI,
and 276 UCloud IDs across eight Account/protocol catalogue surfaces. All 60
Routes shipped at that time had wire IDs on their declared surfaces. The later
September 30 curation yielded 56 after retiring the Solar Route, three GPT-6
Sol Routes, the DMXAPI Fable 5.1 CC channel, and unavailable AIHubMix Fable 5.1
and MiMo Routes, then adding AIHubMix, DMXAPI, and UCloud GPT-6.1 Sol Routes.
The earlier DMXAPI Responses observation used the locally configured
`127.0.0.1:8792` Proxy,
whereas the team manifest declares direct `https://www.dmxapi.cn/v1`.
Catalogue membership therefore does not qualify that direct endpoint or
prove the newly preferred Route can sustain inference.

The DMXAPI `claude-fable-5-1-cc` wire ID remained listed, but two bounded
Claude Code verifications each reached the 60-second client deadline and one
independent direct Messages request reached a 28-second transport deadline
without an HTTP response on September 30. The curated manifest therefore
withdrew this channel while retaining the then-qualified base Fable 5.1 Route,
now withdrawn under the October 10 version floor. These failures
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

A source-built AIGW 0.3.3 in an isolated installation completed bounded
`verify --for codex` and `verify --for hermes` sessions for an explicitly imported UCloud Route using
Codex CLI 0.159.2 and Hermes Agent v0.21.5. That supersedes the earlier
UCloud exclusion for bounded direct and native-client inference, not sustained
availability, final release bytes, or this host's optional Proxy path. The
team now declares DMXAPI first and UCloud as its sole automatic alternative;
AIHubMix Routes are manual-only. The October 2 correction supersedes the older
recommendation order without changing the dated inference observations or
silently rewriting explicit local selections. Final-artifact
admission remains open.

On October 1, 2026, DMXAPI's official
[Claude Code guide](https://doc.dmxapi.cn/claude-code-new.html) described `-cc`
as the Claude Code channel, and its
[Codex Desktop guide](https://doc.dmxapi.cn/cc_switch_to_codex_desktop.html)
described `-cdx` as the Codex channel. These are provider-specific wire IDs,
not different logical model generations or proof of client compatibility.

A later full nine-item native tool continuation failed on the direct plain
DMXAPI `gpt-6.1-sol` Route with Codex 0.159.2 and 0.159.3; UCloud accepted
the unchanged request. Omitting only the user message's `id` made DMXAPI
complete it, while omitting only the tool-call or tool-output `id` did not.
A first-turn message worked both with and without its `id`. This isolates
a continuation compatibility trigger, not a provider-internal root cause.
Earlier successful probes do not qualify this later failure, and AIGW does
not rewrite transport items to hide it.

An authenticated DMXAPI catalogue then listed `gpt-6.1-sol-cdx`. The exact
`9cf23cb8` candidate with one isolated Route addition passed public
setup/use/check/verify, official Codex 0.159.3 shell execution, and a two-turn
tool-context replay. Unmodified official Hermes `f97608f` separately passed
public setup/use/check/verify on that Route. The shipped manifest therefore
adds `dmxapi-gpt-6.1-sol-cdx` under the existing logical `gpt-6.1-sol` Model
and recommends it to unselected Codex clients, with UCloud as the sole
automatic alternative. AIHubMix remains an explicit manual choice.
Existing explicit selections remain unchanged. Native external-provider
Linux/Windows, final-artifact and installed-host acceptance remain separate;
the Hermes run conserved eleven original protected inputs and preserved one
concurrent operator-config change whose writer is unproved.

On October 4, 2026, the `13e6525c` macOS candidate completed public
setup/use/check/verify with Claude Code 2.1.288
for AIHubMix and UCloud Sonnet 5.5, DMXAPI Sonnet 5.5 CC, and UCloud Opus 5.5.
The Sonnet Routes completed Read and same-session content recall. UCloud Opus
started with 5.5, then emitted native `model_refusal_fallback`; all assistant
messages, including Read and same-session recall, reported 4.8. Content recall
therefore does not qualify selected Opus 5.5 or prove outgoing wire identity.
All 43 bounded calls completed without process diagnostics; that does not close
selected-model qualification. The isolated run preserved effort and beta policy,
host settings, credentials and installations.

The current candidate and shipped team bytes also match retained Codex 0.160
AIHubMix and UCloud ordinary Sol 6.1 file-tool/continuation evidence. Only the
missing current DMXAPI CDX cell was rerun: public setup/use/check/verify,
unpredictable file contents read through a native shell tool, and exact named-
session recall passed in 31.33s. The native read-only sandbox, explicit effort
and zero retry policy stayed intact; all source/index inputs and exact cleanup
were verified. A redundant enable invocation failed before inference and remains
retained; public `use` already enabled the exact discovered client and target.
These results qualify exact client/Route inputs, not source-commit provenance,
final signing, native-store succession, Desktop, other Routes or other platforms.

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
At that September 25 read, MAI Thinking 1 and Step 5 Preview lacked completed
general-agent qualification. The later October 5–6 results supersede that
Step 5 observation; historical listing and failed endpoint evidence remain
separate from admitted Routes. Preview labels alone do not establish
unavailability.

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
The October 2 continuation review subsequently withdrew the ordinary DMXAPI
Sonnet 5.5 Route; only its CC and SSVIP variants remain. The earlier one-turn
response does not qualify continuation or restore the retired Route.

Specialized, small, unidentified or unqualified public catalogue entries from
Jina AI, Liquid, Dots Studio, Sao10k, and Stealth are not general-model Routes.

MiniMax M3 has AIHubMix and UCloud Routes; its DMXAPI candidate timed out.
AIHubMix's plain `minimax-m3` and `coding-minimax-m3` wire IDs produced
reasoning tags in final assistant text and failed strict Codex 0.159.3
verification; their successful text inference does not settle that client gap.
The plain AIHubMix Route was removed from the shipped manifest on October 7;
protocol membership alone must not expose a failed tool-continuation channel
as a usable Codex choice. The qualified CC Route and UCloud Route remain.
This manifest retirement does not alter an operator's existing Route or binding.

Its authenticated catalogue also listed `cc-minimax-m3`. With native effort
`none`, that exact channel and UCloud's `MiniMax-M3` each completed a real
Codex shell tool and same-thread note/follow-up sequence on October 1, 2026.
The current `7a5c1da6` candidate separately passed public Route addition,
selection, check and verification for `aihubmix-minimax-m3-cc`, then the same
tool/replay journey using its actual projection without a CLI model override.
The manifest retains the existing logical `minimax-m3` Model and adds only
that CC Route. Existing Routes, Models, client selections and projection bytes
were conserved. This result does not qualify other efforts, every platform,
Desktop GUI or final distribution; no reasoning text was stripped or retried.

Muse Spark 1.3 has an AIHubMix Route only; DMXAPI's unrelated Spark IDs are
not Meta models, and no UCloud Muse Route was observed.

On October 7, the existing native Hermes installation independently qualified
AIHubMix's exact `muse-spark-1.3` Responses and `laguna-s-2.1` Chat Routes.
Each read an unpredictable private file through the real `read_file` tool and
recalled its content in the same resumed session without another tool call.
All four bounded turns completed without native diagnostics. The isolated
homes used only the selected Account's existing credential command; operator
configuration remained unchanged and owned temporary state was removed.
This closes the earlier text-only evidence gap for these Hermes Routes, not
Codex, Claude Desktop, other platforms, sustained availability or final release
bytes. The source-bound native receipt remains under the verification owner.

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
above; Fable 5.1 and Astra 6 channels were withdrawn on October 10. Opus 5
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

On October 1, exact AIHubMix Mercury 2.5 completed official Hermes `f97608f`'s
file tool and same-session recall in 18.939 and 10.406 seconds with medium effort.
This supersedes its earlier bounded timeout as an unresolved compatibility
limitation, not as a latency guarantee or current-client qualification.

NVIDIA identifies [Nemotron 3 Ultra](https://research.nvidia.com/labs/nemotron/Nemotron-3-Ultra/)
as the family's final and strongest model; [Super](https://research.nvidia.com/labs/nemotron/Nemotron-3-Super/)
is smaller. Earlier 60-second Ultra deadlines and 75-second Super deadlines
did not establish incompatibility. On October 1, the exact `444bbd41`
candidate and unmodified official Hermes `f97608f` completed Super's real file
tool and same-session recall in 120.083 and 96.248 seconds. The listed
`nemotron-3-ultra-550b-a55b-free` Route then completed the same journey in
21.404 and 15.766 seconds with the original medium effort. The team manifest
therefore selects qualified Ultra; importing it preserves explicit Super
selections. These dated observations establish neither a latency guarantee
nor final-release or cross-platform client acceptance.

The same endpoint previously produced text for Upstage's
[Solar Pro 4](https://www.upstage.ai/blog/en/solar-pro-4). A September 30,
2026 direct Chat Completions recheck of the exact `solar-pro4` wire ID at
`https://api.inferera.com/v1` returned HTTP 400 with `no_available_channel`.
The public catalogue still listed that ID, but listing is not service
availability. The AIHubMix Route and its now-unreferenced logical Model were
removed from the shipped manifest; this does not claim Solar Pro 4 is
unavailable from Upstage or that the aggregator can never restore its channel.

Cohere's [Command A guide](https://docs.cohere.com/docs/command-a) identifies
`command-a-03-2025` as a general agent model. AIHubMix completed text inference
for that ID, but official Hermes and OpenAI SDK `tool_choice=required`
requests returned no tool calls. AIHubMix's [Command A page](https://aihubmix.com/model/cohere-command-a)
identifies the alternate wire `cohere-command-a` as Cohere Command A. That
exact Route completed official Hermes's file tool and same-session recall in
19.366 and 13.172 seconds on October 1. It replaces the dated wire in the team
manifest without claiming the aliases are the same model generation or
overwriting explicit selections. The newer Command A+ wire returned HTTP 400
`no_available_channel`; it remains unqualified, not globally unavailable.

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
On 2026-09-27, an isolated Codex CLI 0.157.1 session ran Grok 4.7 but its chooser
listed only bundled GPT models. Separately, the AIGW-projected Grok Route completed
an authenticated tool loop against a controlled endpoint. Select such Routes with
[`aigw use --for codex`](../../README.md#use-it-every-day) and verify them through
Codex; AIGW does not alter an existing Desktop conversation's model.

Codex 0.160 still omits Grok 4.7 from its bundled metadata. Its controlled native
tool loop succeeds but emits a fallback-metadata warning. Public `aigw verify`
therefore reports incomplete native qualification and does not update a successful
checkpoint; AIGW invents no entry to silence Codex. Functional inference, native
metadata, the chooser, and current upstream availability are separate claims.
On 2026-10-14, official Codex 0.162.1 passed real-Provider `aigw verify` through
the signed 0.4.0 candidate on the UCloud GPT-6.1 Sol Route, with native file tools
and same-session recall, but emitted that warning on UCloud DeepSeek V4.1 Flash.

On October 4, 2026, the installed official Codex 0.160.0 bundled catalogue contained
11 models. The team manifest then had 37 Responses Routes: nine matched native wire
IDs, three DMXAPI Routes required canonical alias projection, and 25 Routes across
11 non-GPT Models lacked native metadata, 11 on AIHubMix and seven each on DMXAPI
and UCloud. Client acceptance remains Route-specific, including the recorded Sol 6.1
CDX acceptance. The selected GPT-6.1 Sol entry existed in both the native and
preserved user catalogues. This identity audit does not qualify live inference or
authorize replacing the user's catalogue. The earlier fourteen-Route observation
described a provider's complete test scope, not the current missing metadata count.

Every Claude Route in the [shipped team manifest](../../manifests/team.toml)
currently declares only an Anthropic interface, whereas Codex's
[custom-provider contract](https://learn.chatgpt.com/docs/config-file/config-reference#configtoml)
supports the Responses wire API. No shipped AIGW Claude Route is therefore
admitted for Codex. A provider-native Responses endpoint or an explicitly
selected external adapter would need real Codex text, tool, streaming, and
replay qualification before such a Route could be offered. AIGW does not
translate inference traffic or configure ordinary ChatGPT conversations.

## Windows native sandbox boundary

The October 4, 2026 original Codex 0.160 Windows journey passed four retained-
predecessor stages and fourteen Route outcomes, but its sandboxed PowerShell
child exited `0xC0000142`. A same-SID observer ran in Session 0 on a service
window station; it could not open `WinSta0`. This is an execution-context
observation, not proof of the actual Codex child's desktop.

The exact upstream [desktop implementation](https://github.com/openai/codex/blob/rust-v0.160.0/codex-rs/windows-sandbox-rs/src/desktop.rs)
creates a private desktop in the caller's current station but specifies
`Winsta0` in the child startup name. The [process implementation](https://github.com/openai/codex/blob/rust-v0.160.0/codex-rs/windows-sandbox-rs/src/process.rs)
explicitly connects an invalid startup desktop to PowerShell's
`STATUS_DLL_INIT_FAILED`. Open [issue #46412](https://github.com/openai/codex/issues/46412)
reports the same Session 0 versus interactive-session split on earlier Codex
versions. Together they support a station-mismatch hypothesis; they do not
establish that every Windows failure has that cause.

Keep the original failed result and [native private-desktop isolation](https://learn.chatgpt.com/docs/windows/windows-sandbox).
Qualification requires the original tool loop in a supported native Windows
context, or a released vendor repair. Disabling the private desktop, widening
ACLs, replacing PowerShell, or substituting WSL would prove a different boundary.
Ordinary source CI does not silently enable real-client acceptance; an explicitly
selected `--clients` release journey must still pass the complete native suite.
