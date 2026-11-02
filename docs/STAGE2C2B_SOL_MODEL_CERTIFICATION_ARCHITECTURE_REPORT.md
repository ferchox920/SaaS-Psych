# STAGE 2C.2B — SOL MODEL CERTIFICATION & ARCHITECTURE REPORT

## 1. Executive verdict

```text
STAGE 2C = PARTIAL — NO SUITABLE LOCAL MODEL
```

No local candidate met the mandatory `A–J = 10/10` entry gate. The deterministic clinical safety architecture remains verified and rejected every invalid observed output without a human merge or therapeutic-state mutation. No model was selected and Stage 2D must not begin.

## 2. Luna handoff assessment

Accepted from the complete Stage 2C.2A handoff:

- hardware inventory and Ollama 0.33.1 runtime;
- the then-installed inventory containing only `qwen3.5:9b`;
- qwen baseline A–J evidence and optimized A/B/C reproduction;
- qwen's repeatable fixture C failure (2/2);
- latency and resource observations;
- green repository regression and the 44-entry pre-existing dirty working tree.

Independently verified in this stage:

- the handoff and Stage 2C.1/2C implementation reports were read in full;
- `go test -count=1 ./...`, `go vet ./...`, and API build pass;
- current hardware, free disk, Ollama version and model inventory;
- three additional model downloads and real-service fail-fast evaluations;
- PostgreSQL, Ollama live/review/longitudinal, web, OpenAPI, tenant guard and migration roundtrip;
- Git status and whitespace integrity.

Not repeated:

- qwen's full historical A–J run, because the existing real evidence already establishes failure and C was reproduced 2/2;
- qwen C again, because no new evidence could restore the 10/10 gate;
- any C×5 or 30-run certification, because no candidate met its prerequisite.

## 3. Baseline

```text
GO TEST:   PASS
GO VET:    PASS
GO BUILD:  PASS
GIT:       44 pre-existing modified/untracked entries; diff check PASS
```

The current baseline matched the handoff materially. The ordinary Go run retains opt-in skips; all relevant opt-in suites were invoked separately during final regression.

## 4. Hardware feasibility

| Resource | Observed |
|---|---|
| OS | Windows 11 Home 10.0.26200 |
| CPU | Intel Core i5-13420H, 8 physical / 12 logical cores |
| RAM | 15.8 GB total |
| GPU | NVIDIA RTX 4050 Laptop GPU |
| VRAM | 6141 MiB |
| Driver | 610.74 |
| Ollama | 0.33.1 |
| Free disk before downloads | C: 114.8 GB; D: 191.4 GB |

Practical model classes:

- approximately 6 GB Q4 model: fits with partial CPU offload; heavy but usable for asynchronous inference;
- 8–9 GB Q4 model: technically loadable, but leaves less than 1 GB system RAM and is impractical under the verified 180-second attempt timeout;
- 14–15 GB model: not viable on this 16 GB Windows host once runtime, KV cache and OS memory are included;
- larger classes: not viable.

## 5. Current model research

Research used current official Ollama pages rather than stale model recollection:

- [Ministral 3](https://ollama.com/library/ministral-3) offers 8B/14B Q4 variants, multilingual support, system-prompt adherence and native JSON output claims. Published sizes are 6.0 GB and 9.1 GB.
- [Gemma 3](https://ollama.com/library/gemma3) offers a 12B Q4_K_M at 8.1 GB, multilingual reasoning and structured-output compatibility.
- [GPT-OSS](https://ollama.com/library/gpt-oss/tags) has a 20B MXFP4 artifact of 14 GB. It was researched but rejected before download because total host RAM is only 15.8 GB.
- [Qwen 3](https://ollama.com/library/qwen3/tags) has a 14B Q4 artifact of 9.3 GB, but measured 8–9 GB candidates already exhausted practical memory/latency and the installed newer qwen3.5:9b already failed the critical clinical discriminator.
- Ollama's [structured output documentation](https://docs.ollama.com/capabilities/structured-outputs) confirms JSON Schema support through the `format` field used by the existing provider.

Parameter count was not used as the selection rule. Clinical correctness, instruction following, structured-contract reliability and actual hardware behavior controlled the shortlist.

## 6. Shortlist

| Model | Size/class | Hardware fit | Why tested |
|---|---:|---|---|
| `ministral-3:14b` | 14B Q4_K_M, 9.1 GB artifact | Technically loadable, partial CPU offload | Primary: instruction/system prompt and JSON-output claims |
| `gemma3:12b` | 12B Q4_K_M, 8.1 GB artifact | Technically loadable, partial CPU offload | Strong alternative: multilingual reasoning and structured output |
| `ministral-3:8b` | 8B Q4_K_M, 6.0 GB artifact | Fits with partial CPU offload | Hardware-adjusted final option after 12–14B pressure was measured |

`gpt-oss:20b` was not shortlisted for execution because 14 GB weights plus runtime/KV/OS memory are unrealistic on this host. No catalog was bulk-downloaded.

## 7. Downloads performed

| Model | Artifact size | Reason |
|---|---:|---|
| `ministral-3:14b` | 9.1 GB | Primary candidate |
| `gemma3:12b` | 8.1 GB | Strong alternative |
| `ministral-3:8b` | 6.0 GB | Final hardware-viable candidate |

Downloads were sequential. Existing models were not removed. Two uncensored qwen aliases appeared concurrently during final regression; they were not downloaded, selected or modified by this task.

## 8. First-pass A–J

Fail-fast is shown explicitly; `NR` means not run after the candidate had already lost the mandatory 10/10 gate.

| Model | A | B | C | D | E | F | G | H | I | J | Total observed |
|---|---|---|---|---|---|---|---|---|---|---|---:|
| `qwen3.5:9b` optimized | PASS-R | PASS-R | FAIL 2/2 | NR | NR | NR | NR | NR | NR | historical safe rejection | 2/3 optimized |
| `ministral-3:14b` | FAIL 2/2 | NR | NR | NR | NR | NR | NR | NR | NR | NR | 0/2 attempts |
| `gemma3:12b` | FAIL 2/2 | NR | NR | NR | NR | NR | NR | NR | NR | NR | 0/2 attempts |
| `ministral-3:8b` | PASS-R | FAIL-R | NR | NR | NR | NR | NR | NR | NR | NR | 1/2 |

Historical qwen baseline was reported as 1/10 because fixture J's backend rejection was counted as safe. Under this stage's stricter capability rule, safe backend rejection proves the safety system, not successful model capability; therefore J does not rescue qwen as a candidate.

## 9. Per-fixture details

### qwen3.5:9b

| Fixture | Primary | Repair | Final | Latency | Failure class/result |
|---|---|---|---|---:|---|
| A | FAIL | used | PASS | 319.8 s | dependency/reference corrected |
| B | FAIL | used | PASS | 243.2 s | valid integration only after repair |
| C | clinically invalid | insufficient | FAIL 2/2 | 167.1 / 177.6 s | unjustified ACT+CBT eclecticism on same function |
| D–I | historical FAIL | historical bounded repair | FAIL | 234–325 s | grounding or missing indicator |
| J | unsupported confident output | bounded backend rejection | model FAIL / safety PASS | 273 s | unsupported mechanism was not persisted |

### ministral-3:14b

| Fixture | Primary | Repair | Final | Latency | Failure class/result |
|---|---|---|---|---:|---|
| A cold | timeout | not entered | FAIL | 180.2 s | no output, no diff |
| A warm | timeout | not entered | FAIL | 180.0 s | no output, no diff |
| B–J | NR | NR | NR | — | eliminated: 10/10 impossible |

### gemma3:12b

| Fixture | Primary | Repair | Final | Latency | Failure class/result |
|---|---|---|---|---:|---|
| A cold | timeout | not entered | FAIL | 180.0 s | no output, no diff |
| A warm | invalid chain | used, still invalid | FAIL | 229.3 s | `GIRA requires target, goal and rationale` |
| B–J | NR | NR | NR | — | eliminated: 10/10 impossible |

### ministral-3:8b

| Fixture | Primary | Repair | Final | Latency | Failure class/result |
|---|---|---|---|---:|---|
| A | invalid dependency | used | PASS | 233.1 s | six operations; pending-review diff only |
| B | truncated semantic JSON | repair timed out | FAIL | 338.1 s | `unexpected EOF` then timeout |
| C–J | NR | NR | NR | — | eliminated: 10/10 impossible |

No fixture input, prompt, schema, compiler rule or C1–C25 invariant was changed between candidates.

## 10. Repair dependency

| Model | Primary success | Final success | Repair rate |
|---|---:|---:|---:|
| `qwen3.5:9b` optimized A–C | 0/2 known successes | 2/3 | at least 2/3; exact C repair flag was not retained |
| `ministral-3:14b` A attempts | 0/2 | 0/2 | 0/2; provider timed out first |
| `gemma3:12b` A attempts | 0/2 | 0/2 | 1/2 |
| `ministral-3:8b` A–B | 0/2 | 1/2 | 2/2 |

Every observed success from a GIRA candidate required repair. This is operationally undesirable even before clinical certification is considered.

## 11. Fixture C stress

```text
NOT RUN — no candidate reached A–J 10/10.
```

Existing qwen evidence remains C = 0/2 with repeatable unjustified eclecticism. It is not presented as the required C×5 finalist stress test.

## 12. Multi-run certification

```text
NOT RUN — no candidate reached A–J 10/10 plus C 5/5.
```

No 27/30 or 30/30 claim is made.

## 13. Semantic clinical comparison

| Property | qwen3.5:9b | ministral-3:14b | gemma3:12b | ministral-3:8b |
|---|---|---|---|---|
| Grounding discipline | Historical failures D–G | Not measurable beyond timeout | A omitted required chain | A valid after repair; B incomplete |
| Minimality | Failed C repeatedly | Not measurable | Not measurable | A final was minimal enough; B failed |
| Goal operationalization | Historical H/I indicator failures | Not measurable | Required Goal chain missing | A final valid; B unavailable |
| Rationale specificity | A/B recover after repair | Not measurable | Missing required rationale chain | A recovered after repair |
| Uncertainty handling | J confident output safely rejected | Not measurable | Not measurable | Not measured due early failure |
| Eclecticism | Critical C failure 2/2 | Not reached | Not reached | Not reached |
| Mechanism fabrication | J backend safety caught it | Not reached | Not reached | Not reached |

No model earned a qualitative finalist review because schema/contract success and the 10/10 gate were not achieved. Prose style was not scored.

## 14. Safety

Across all candidate runs and regression suites:

```text
direct merges:                              0
unauthorized therapeutic-state mutations:  0
unsupported confident strategies persisted:0
incompatible Technique/Approach persisted: 0
historical GIRA overwrites:                 0
```

The real GIRA service harness creates at most an in-memory `ClinicalDiff(pending_review)` for successful output. Invalid provider/decode/compiler/validator results create no diff. PostgreSQL acceptance tests independently confirm diff-only generation, human review, CA-W merge, rollback, stale rejection and immutable GIRA history.

## 15. Latency

| Model/sample | Median | Mean | P95 | Max |
|---|---:|---:|---:|---:|
| qwen baseline A–J | 299.0 s | 319.7 s | 467.0 s | 467.0 s |
| qwen optimized A–C | 243.2 s | 243.3 s | 319.8 s | 319.8 s |
| ministral-3:14b A×2 | 180.1 s | 180.1 s | 180.2 s | 180.2 s |
| gemma3:12b A×2 | 204.6 s | 204.6 s | 229.3 s | 229.3 s |
| ministral-3:8b A–B | 285.6 s | 285.6 s | 338.1 s | 338.1 s |

For small samples, P95 is reported as the observed maximum and is not statistically representative. Available diagnostics show provider/repair generation dominates; context build, validation and deterministic compilation are comparatively negligible.

## 16. Resource usage

| Model | Loaded footprint | CPU/GPU split | Free RAM observed | Classification |
|---|---:|---:|---:|---|
| ministral-3:14b | 9.8 GB | 65% / 35% | 0.41 GB | technically possible but impractical |
| gemma3:12b | 8.9 GB | 63% / 37% | 0.72 GB | technically possible but impractical |
| ministral-3:8b | 6.6 GB | 48% / 52% | 1.86 GB | heavy but practical async |

GPU use reached approximately 4.7–4.9 GB during sampled inference. No permanent monitor was added, and provider token/s was not consistently available for GIRA. The 8B model had viable resource occupancy but failed reliability; the larger models failed operational viability.

## 17. Selected model

```text
MODEL: NONE
```

No GIRA-specific configuration value was introduced because no candidate was certified.

## 18. Why qwen3.5:9b was not retained

qwen was not rejected merely for a serialization defect. It recovered A and B only through repair, took roughly four to five minutes, and repeated the critical clinical error in C: two approaches for the same Target/function without distinct justification. It also historically required backend rejection for an unsupported confident mechanism in J. Under the mandated priority order, these clinical failures outweigh its better hardware fit and its proven usefulness in live/review/longitudinal operations.

The alternatives did not beat qwen sufficiently to become finalists: 12–14B candidates were operationally impractical and failed A, while Ministral 8B passed A only after repair and failed B after a truncated primary plus repair timeout. Therefore qwen remains valid for its separately verified operations but is not configured for an accepted automated GIRA builder.

## 19. Execution architecture recommendation

```text
NO AUTOMATED GIRA MODEL AVAILABLE
```

For any future provider that passes the clinical gate, execution should be designed as `ASYNC REQUIRED`: every viable local observation took 180–338 seconds and successful qwen trials took 243–320 seconds. This latency recommendation is separate from the present model-capability verdict.

## 20. Configuration changes

```text
CODE:          none
CONFIGURATION: none
PROMPTS:       none
VALIDATORS:    none
SCHEMA:        none
RUNTIME:       three Ollama model downloads
DOCUMENTATION: this report only
```

The shared `OLLAMA_MODEL=qwen3.5:9b` behavior was not changed. A future certified model should use an operation-specific `OLLAMA_GIRA_MODEL`-style setting consistent with existing configuration, not a model router.

## 21. C1–C25

| Invariant | Result | Evidence |
|---|---|---|
| C1 Process != Target | PASS | typed model/schema tests |
| C2 Target != Goal | PASS | typed model and relationship tests |
| C3 Goal != Indicator | PASS | owner/entity tests |
| C4 Hypothesis != Rationale | PASS | separate epistemic/strategy types |
| C5 Formulation != Strategy | PASS | state projection tests |
| C6 Approach != Technique | PASS | registry and compatibility tests |
| C7 Approach requires Rationale | PASS | GIRA validator/compiler tests |
| C8 Rationale requires Target+Goal | PASS | chain validation; Gemma output rejected |
| C9 Technique/Approach compatibility | PASS | exact versioned compatibility tests |
| C10 no unjustified eclecticism | PASS | deterministic guard; qwen C rejected |
| C11 multimodal not mandatory | PASS | minimal single-approach fixture |
| C12 unsupported mechanisms blocked | PASS | qwen J safe rejection |
| C13 AI Target remains proposal | PASS | diff-only service/PG tests |
| C14 AI Goal remains planned proposal | PASS | lifecycle validation |
| C15 AI cannot achieve Goal | PASS | explicit unit tests |
| C16 AI GIRA is not approved GIRA | PASS | pending-review only |
| C17 AI cannot complete Phase | PASS | explicit unit tests |
| C18 historical GIRA preserved | PASS | version/supersession PG tests |
| C19 deprecated registry version resolves | PASS | exact-version PG tests |
| C20 progress does not auto-change Goal | PASS | link/lifecycle tests |
| C21 review does not mutate strategy | PASS | PG pre-merge assertions |
| C22 human merge required | PASS | CA-W merge path |
| C23 transactional merge | PASS | rollback PG test |
| C24 stale diff blocked | PASS | head/entity conflict PG test |
| C25 No AI Direct Merge | PASS | service boundary and actor tests |

No safeguard was weakened and no fixture semantics changed.

## 22. Regression suites

```text
GO TEST:              PASS
GO VET:               PASS
GO BUILD:             PASS

POSTGRES:             PASS — full opt-in HTTP + DB packages

OLLAMA LIVE:          PASS — 73.89 s, no repair
OLLAMA REVIEW:        PASS — 68.43 s, no repair
OLLAMA LONGITUDINAL:  PASS A–G — 7/7, 293.71 s

GIRA A–J:             NOT QUALIFIED — all candidates eliminated by fail-fast
GIRA C×5:             NOT RUN — 10/10 prerequisite absent
GIRA MULTIRUN:        NOT RUN — A–J 10/10 + C 5/5 prerequisites absent

WEB LINT:             PASS
WEB BUILD:            PASS

OPENAPI:              PASS
TENANT GUARD:         PASS
MIGRATION:            PASS — 23 → 16 → 23
GIT:                  PASS diff check; pre-existing dirty tree preserved
```

An initial combined Ollama regression was contaminated by a concurrently loaded, separately installed uncensored model and produced timeouts. After that model finished unloading, isolated reruns passed live, review and longitudinal A–G. Both the failed observation and the controlled rerun are retained here rather than hiding the environmental interference.

## 23. Remaining gaps

### BLOCKING

- no local model satisfies GIRA A–J 10/10;
- no candidate is eligible for C×5 or 30-run certification;
- automated GIRA generation is not ready for Stage 2D.

### NON-BLOCKING

- deterministic compiler, validators, review, human merge, history and tenant/client isolation remain green;
- qwen remains suitable for the separately tested live, review and longitudinal operations;
- downloaded candidates may remain available for future controlled experiments, but are not selected.

### DOCUMENTATION

- a future provider decision needs a privacy/consent runbook and operation-specific configuration;
- no GPT/OpenAI integration, model router or Stage 2D artifact was created.

## 24. Provider architecture recommendation

Recommended future direction: **hybrid local + remote, with the remote provider restricted to GIRA only**, subject to a separate authorization and acceptance stage. Keep live/review/longitudinal local on qwen because those operations pass. Until a remote provider is clinically certified, expose no automated GIRA builder rather than weakening safeguards.

Privacy requirements for any future remote evaluation:

- only the bounded approved Process context, selected approved Evidence/Events/Hypotheses, current strategy and registry candidates required for GIRA may leave the device;
- direct identifiers, raw transcript/audio, unrelated notes, contact data and free-form historic records must be excluded;
- explicit patient/tenant consent and a documented lawful processing basis are required before transmission;
- every request must retain provider/model/version, source entity versions, input/context hashes, run ID and terminal status;
- provider retention/training policies, region, encryption, deletion, sub-processors and audit/export capabilities must be reviewed;
- remote output must traverse the same strict schema, deterministic compiler, C1–C25 validation, `ClinicalDiff(pending_review)`, human review and CA-W transactional merge.

This is a recommendation only. No external provider was implemented.

## 25. Stage 2D readiness

```text
STAGE 2D READINESS = NOT READY
```

Blocking reason: no GIRA model is certified. If a future model qualifies, `ASYNC REQUIRED` should become an explicit Stage 2D architectural requirement, while human review and No-AI-Direct-Merge remain unchanged.

Work stops at Stage 2C.2B.
