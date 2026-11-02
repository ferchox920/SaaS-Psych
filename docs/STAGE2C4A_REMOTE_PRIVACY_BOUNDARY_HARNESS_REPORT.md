# STAGE 2C.4A — REMOTE PRIVACY BOUNDARY & HARNESS REPORT

## 1. Verdict

```text
VERIFIED PASS

REMOTE CERTIFICATION HARNESS = READY
PRIVACY BOUNDARY = VERIFIED WITH SYNTHETIC DATA

STAGE 2C = PARTIAL — REMOTE CERTIFICATION NOT EXECUTED
```

Este veredicto certifica solamente la preparación técnica local de Stage 2C.4A. No certifica ningún modelo remoto, no autoriza datos clínicos reales y no cambia `STAGE 2D READINESS = NOT READY`.

## 2. Baseline

Se leyeron íntegramente antes de implementar:

- `STAGE2C3_REMOTE_GIRA_PROVIDER_CERTIFICATION_REPORT.md`;
- `STAGE2C2B_SOL_MODEL_CERTIFICATION_ARCHITECTURE_REPORT.md`;
- `STAGE2C1_GIRA_BUILDER_RELIABILITY_REPORT.md`.

Baseline técnico:

```text
GO TEST:  PASS — go test -count=1 ./...
GO VET:   PASS — go vet ./...
GO BUILD: PASS — go build ./cmd/server
GIT:      DIRTY PREEXISTING — 23 modified / 23 untracked; preserved
```

No se ejecutó reset, checkout destructivo, limpieza ni descarte de cambios existentes.

## 3. Outbound field audit

La entrada previa al boundary nace de `GIRABuilderInput`, se acota con `BoundGIRAContext`, se transforma al contrato compacto local `GIRAModelInput` y finalmente al allow-list remoto. La tabla enumera todos los campos que pueden salir en `RemoteGIRAContext`; todo lo no enumerado es estructuralmente imposible de serializar desde ese DTO.

| Field | Current source | Clinical need | Privacy class | Remote action |
| --- | --- | --- | --- | --- |
| `schema_version` | contrato GIRA congelado | interpretación/compatibilidad | REQUIRED | SEND exact value |
| `process.ref` | alias local del proceso seleccionado | vincular propuesta | PSEUDONYMIZE | SEND `process_1` |
| `process.title` | `ClinicalProcess.Title` | foco estratégico | REQUIRED | MINIMIZE text |
| `process.description` | `ClinicalProcess.Description` | formulación seleccionada | REQUIRED | MINIMIZE text |
| `process.clinical_status` | estado aprobado | continuidad | REQUIRED | SEND enum |
| `process.version` | versión de entidad | concurrencia semántica | REQUIRED | SEND integer |
| `evidence[].ref` | UUID local | grounding | PSEUDONYMIZE | SEND deterministic alias |
| `evidence[].epistemic_type` | evidencia aprobada | integridad epistémica | REQUIRED | SEND enum |
| `evidence[].statement` | evidencia aprobada seleccionada | grounding clínico | REQUIRED | MINIMIZE text |
| `evidence[].version` | versión aprobada | provenance/versioning | REQUIRED | SEND integer |
| `events[].ref` | UUID local | referencias semánticas | PSEUDONYMIZE | SEND deterministic alias |
| `events[].event_type` | evento aprobado | función clínica | REQUIRED | SEND enum |
| `events[].title` | evento aprobado | contexto relevante | OPTIONAL | MINIMIZE text |
| `events[].description` | evento aprobado | contexto relevante | OPTIONAL | MINIMIZE text |
| `events[].evidence_refs` | relación local event–evidence | grounding | REQUIRED | SEND aliases only |
| `events[].version` | versión aprobada | versioning | REQUIRED | SEND integer |
| `hypotheses[].ref` | UUID local | referencias semánticas | PSEUDONYMIZE | SEND deterministic alias |
| `hypotheses[].statement` | hipótesis aprobada | estrategia | REQUIRED | MINIMIZE text |
| `hypotheses[].confidence_level` | hipótesis aprobada | calibración | REQUIRED | SEND enum |
| `hypotheses[].clinical_status` | hipótesis aprobada | vigencia | REQUIRED | SEND enum |
| `hypotheses[].supporting_evidence_refs` | relaciones aprobadas | integridad epistémica | REQUIRED | SEND aliases only |
| `hypotheses[].contradicting_evidence_refs` | relaciones aprobadas | competencia/corrección | REQUIRED | SEND aliases only |
| `hypotheses[].version` | versión aprobada | versioning | REQUIRED | SEND integer |
| `current_strategy.targets[].ref` | UUID local | continuidad | PSEUDONYMIZE | SEND alias |
| `current_strategy.targets[].title` | target aprobado | continuidad | REQUIRED | MINIMIZE text |
| `current_strategy.targets[].description` | target aprobado | función objetivo | REQUIRED | MINIMIZE text |
| `current_strategy.targets[].target_type` | target aprobado | ajuste de enfoque | REQUIRED | SEND enum |
| `current_strategy.targets[].clinical_status` | target aprobado | vigencia | REQUIRED | SEND enum |
| `current_strategy.targets[].version` | versión aprobada | concurrency | REQUIRED | SEND integer |
| `current_strategy.goals[].ref` | UUID local | continuidad | PSEUDONYMIZE | SEND alias |
| `current_strategy.goals[].title` | goal aprobado | operacionalización | REQUIRED | MINIMIZE text |
| `current_strategy.goals[].description` | goal aprobado | operacionalización | REQUIRED | MINIMIZE text |
| `current_strategy.goals[].goal_type` | goal aprobado | semántica | REQUIRED | SEND enum |
| `current_strategy.goals[].priority` | goal aprobado | orden estratégico | OPTIONAL | SEND enum |
| `current_strategy.goals[].clinical_status` | goal aprobado | vigencia | REQUIRED | SEND enum |
| `current_strategy.goals[].version` | versión aprobada | concurrency | REQUIRED | SEND integer |
| `current_strategy.goals[].target_refs` | relación goal–target | grafo terapéutico | REQUIRED | SEND aliases only |
| `current_strategy.indicators[].ref` | UUID local | continuidad | PSEUDONYMIZE | SEND alias |
| `current_strategy.indicators[].goal_ref` | relación indicator–goal | grafo terapéutico | REQUIRED | SEND alias |
| `current_strategy.indicators[].description` | indicador aprobado | medición | REQUIRED | MINIMIZE text |
| `current_strategy.indicators[].indicator_type` | indicador aprobado | medición | REQUIRED | SEND enum |
| `current_strategy.indicators[].status` | indicador aprobado | vigencia | REQUIRED | SEND enum |
| `current_strategy.indicators[].version` | versión aprobada | concurrency | REQUIRED | SEND integer |
| `current_strategy.rationales[].ref` | UUID local | continuidad | PSEUDONYMIZE | SEND alias |
| `current_strategy.rationales[].target_ref` | relación rationale–target | ajuste clínico | REQUIRED | SEND alias |
| `current_strategy.rationales[].goal_ref` | relación rationale–goal | ajuste clínico | REQUIRED | SEND alias |
| `current_strategy.rationales[].approach_slug/version` | registry selection | reproducibilidad | REQUIRED | SEND registry identity |
| `current_strategy.rationales[].technique_slug/version` | registry selection | reproducibilidad | OPTIONAL | SEND registry identity/null |
| `current_strategy.rationales[].rationale` | rationale aprobado | coherencia funcional | REQUIRED | MINIMIZE text |
| `current_strategy.rationales[].expected_effect` | rationale aprobado | coherencia funcional | REQUIRED | MINIMIZE text |
| `current_strategy.rationales[].version` | versión aprobada | concurrency | REQUIRED | SEND integer |
| `current_strategy.giras[].ref` | UUID local | continuidad/versionado | PSEUDONYMIZE | SEND alias |
| `current_strategy.giras[].title` | GIRA aprobada | continuidad | REQUIRED | MINIMIZE text |
| `current_strategy.giras[].summary` | GIRA aprobada | continuidad | REQUIRED | MINIMIZE text |
| `current_strategy.giras[].clinical_status` | GIRA aprobada | vigencia | REQUIRED | SEND enum |
| `current_strategy.giras[].gira_version` | GIRA aprobada | historia | REQUIRED | SEND integer |
| `current_strategy.giras[].entity_version` | GIRA aprobada | concurrency | REQUIRED | SEND integer |
| `current_strategy.giras[].target_refs` | grafo aprobado | coherencia | REQUIRED | SEND aliases only |
| `current_strategy.giras[].goal_refs` | grafo aprobado | coherencia | REQUIRED | SEND aliases only |
| `current_strategy.giras[].rationale_refs` | grafo aprobado | coherencia | REQUIRED | SEND aliases only |
| `approach_candidates[].slug/version` | registry local filtrado | selección válida | REQUIRED | SEND identity/version |
| `approach_candidates[].target_domains` | registry local | approach fit | REQUIRED | MINIMIZE each text |
| `approach_candidates[].core_mechanisms` | registry local | mechanism fit | REQUIRED | MINIMIZE each text |
| `approach_candidates[].limitations` | registry local | safeguards | REQUIRED | MINIMIZE each text |
| `approach_candidates[].cautions` | registry local | safeguards | REQUIRED | MINIMIZE each text |
| `technique_candidates[].slug/version` | registry local filtrado | selección válida | REQUIRED | SEND identity/version |
| `technique_candidates[].approach_slug/version` | registry local | compatibilidad | REQUIRED | SEND identity/version |
| `technique_candidates[].target_domains` | registry local | technique fit | REQUIRED | MINIMIZE each text |
| `technique_candidates[].mechanism` | registry local | grounding | REQUIRED | MINIMIZE text |
| `technique_candidates[].cautions` | registry local | safeguards | REQUIRED | MINIMIZE each text |
| `technique_candidates[].limits` | registry local | safeguards | REQUIRED | MINIMIZE each text |
| `technique_candidates[].expected_signals` | registry local | validación | REQUIRED | MINIMIZE each text |
| tenant/client/user/appointment/session/AIRun UUIDs | domain/request/run | no need | REMOVE / LOCAL_ONLY | FIELD DOES NOT EXIST |
| names/contact/address/document/calendar | client/domain | no need | REMOVE / LOCAL_ONLY | FIELD DOES NOT EXIST; narrative minimization additionally applies |
| transcript/audio/full report/full history | session/report/storage | no need | REMOVE / LOCAL_ONLY | FIELD DOES NOT EXIST |
| audit logs/provenance rows | local infrastructure | no need | LOCAL_ONLY | FIELD DOES NOT EXIST |
| alias ↔ UUID map | compiler-local `giraRefIndex` | compile locally | LOCAL_ONLY | NEVER serialized |
| unrelated processes | full longitudinal state | no need | REMOVE | selector cannot accept them |

## 4. RemoteGIRAContext

Exact outbound schema, expressed compactly:

```text
RemoteGIRAContext
├── schema_version
├── process {ref,title,description,clinical_status,version}
├── evidence[] {ref,epistemic_type,statement,version}
├── events[] {ref,event_type,title,description,evidence_refs[],version}
├── hypotheses[] {
│   ref,statement,confidence_level,clinical_status,
│   supporting_evidence_refs[],contradicting_evidence_refs[],version
│ }
├── current_strategy
│   ├── targets[] {ref,title,description,target_type,clinical_status,version}
│   ├── goals[] {ref,title,description,goal_type,priority,clinical_status,version,target_refs[]}
│   ├── indicators[] {ref,goal_ref,description,indicator_type,status,version}
│   ├── rationales[] {
│   │   ref,target_ref,goal_ref,approach_slug,approach_version,
│   │   technique_slug,technique_version,rationale,expected_effect,version
│   │ }
│   └── giras[] {
│       ref,title,summary,clinical_status,gira_version,entity_version,
│       target_refs[],goal_refs[],rationale_refs[]
│     }
├── approach_candidates[] {
│   slug,version,target_domains[],core_mechanisms[],limitations[],cautions[]
│ }
└── technique_candidates[] {
    slug,version,approach_slug,approach_version,target_domains[],mechanism,
    cautions[],limits[],expected_signals[]
  }
```

Una prueba por reflexión recorre recursivamente el DTO y falla si aparece cualquier campo asociado a tenant, client, user, appointment, session, AIRun, nombre, contacto, documento, transcript, audio, report, history, audit, UUID o source ID.

## 5. Minimization architecture

```text
Approved Clinical State
        ↓
selected-process / relationship-first context selector
        ↓
deterministic canonical ordering and LOCAL alias index
        ↓
GIRAPrivacyMinimizer (gira-remote-minimization-v1)
        ↓
RemoteGIRAContext allow-list
        ↓
Remote GIRABuilderProvider
```

El selector limita evidencia a 24, eventos a 16, hipótesis a 16, approaches a 12 y técnicas a 32, priorizando relaciones directas. El minimizer reemplaza identificadores directos y contextos de alta especificidad mediante reglas deterministas locales. No se utiliza IA remota para desidentificar.

## 6. Synthetic PII leakage matrix

| Synthetic secret | Input | Outbound | Result |
| --- | --- | --- | --- |
| `María Prueba` | patient/process narrative | `[person]` | PASS |
| `maria@example.test` | evidence | `[email]` | PASS |
| `+54 11 4000-0000` | evidence | `[phone]` | PASS |
| `Calle Falsa 123` | evidence | `[location]` | PASS |
| `TEST-DNI-1234` | evidence | `[document]` | PASS |
| `1988-04-03` | evidence | `[date]` | PASS |
| `2026-08-31T14:30:00Z` | event | `[date]` | PASS |
| `31/08/2026` | process | `[date]` | PASS |
| `Empresa XYZ` | evidence | `[organization]` | PASS |
| `Hospital Central` | evidence | `[organization]` | PASS |
| `Escuela Norte` | event | `[organization]` | PASS |
| `Rosario` / `Palermo` | event/evidence | `[location]` | PASS |
| rare profession | hypothesis | `[profession]` | PASS |
| tenant/client/process UUIDs | structured state | absent | PASS |

Cada aserción busca el literal en la serialización completa, no solamente en campos individuales.

## 7. Third-party leakage

Fixtures sintéticos incluyen supervisor, pareja, padre y médica con nombres ficticios. `Juan Pérez`, `Pedro Tercero`, `Carlos Prueba` y `Ana Ensayo` no aparecen en el payload. La misma política se aplica a paciente y terceros; los roles clínicamente útiles pueden permanecer generalizados.

## 8. Residual re-identification risks

```text
MINIMIZED != ANONYMOUS
```

La narrativa clínica puede seguir siendo identificable por combinación de hechos. El boundary emite solamente flags cerrados y sin contenido:

- `direct_identifier_removed`;
- `internal_identifier_removed`;
- `exact_address_removed`;
- `exact_date_present`;
- `specific_organization_removed`;
- `rare_location_context`;
- `specific_age_context`;
- `rare_profession_context`;
- `third_party_identifier_removed`;
- `high_specificity_narrative`.

No se calcula ni se afirma un score científico de anonimización. Profesiones raras, eventos raros, configuración familiar inusual e historia médica singular continúan siendo riesgo residual y requieren revisión de producto/legal antes de uso real.

## 9. Semantic preservation

La prueba de preservación transforma una narrativa que contiene `Juan Pérez` y conserva explícitamente:

```text
rejected boundary
→ boundary reversal
→ anticipated relationship loss
```

Además, el DTO mantiene por separado `patient_report`, `therapist_observation`, eventos con sus evidencias y evidencia `supporting` frente a `contradicting`. La minimización no colapsa provenance epistémica en un blob.

## 10. Provider port

`GIRABuilderProvider` continúa siendo el puerto clínico único. Ahora recibe `GIRAProviderRequest`, que expone únicamente:

- `RemoteGIRAContext`;
- schema `semantic_v1` congelado;
- una función local no exportada para validar el candidato.

El adapter de infraestructura no recibe repositories, domain state, UUID map ni `ClinicalDiff`. Live, review y longitudinal conservan `ClinicalInferenceProvider` y Ollama.

## 11. Remote transport

`RemoteLLMTransport` encapsula exclusivamente:

- request HTTP;
- bearer authentication;
- timeout acotado;
- límite de response body;
- normalización 429/5xx/timeout/cancel/network;
- request ID y duración.

No compila semántica, no crea diffs y no toca estado terapéutico. Los cuerpos de error del proveedor y la causa de red no se incluyen en mensajes expuestos.

## 12. OpenAI adapter

Se implementó `OpenAIGIRABuilderProvider` con HTTP directo y pruebas exclusivamente contra `httptest.Server`:

- Responses API compatible;
- `store=false`;
- output `json_schema` con `strict=true`;
- prompt `clinical-gira-builder-v1.1` sin cambio;
- máximo una reparación;
- decoder/compilador/validador locales compartidos;
- usage, snapshot, request ID, región y costo estimado seguros.

Configuración:

```text
CLINICAL_GIRA_PROVIDER=none|openai   # default none
CLINICAL_GIRA_MODEL=<exact model>
OPENAI_API_KEY=<secret>
OPENAI_BASE_URL=https://...
OPENAI_GIRA_TIMEOUT_SECONDS=<bounded>
OPENAI_GIRA_MAX_OUTPUT_TOKENS=<bounded>
```

`provider=openai` sin modelo o key falla validación; no activa Ollama como fallback. `data_control_mode` y `data_control_status=requested|confirmed|unknown` son distintos: un string `zero_data_retention` no prueba que el control esté confirmado.

## 13. Failure matrix

| Failure | Normalized outcome | AIRun | Diff | Fallback | Result |
| --- | --- | --- | --- | --- | --- |
| network | `provider_unavailable` | failed | none | none | PASS |
| timeout | `timeout` | cancelled | none | none | PASS |
| HTTP 429 | `rate_limited` | failed | none | none | PASS |
| HTTP 5xx | `provider_unavailable` | failed | none | none | PASS |
| invalid provider response | `invalid_provider_response` | failed | none | none | PASS |
| invalid semantic JSON primary | one repair | succeeds only if repaired valid | pending only on valid | none | PASS |
| schema-invalid primary+repair | `invalid_model_output` | failed | none | none | PASS |
| semantic-invalid primary | one repair through local compiler | pending only on valid repair | none | PASS |
| repair transport failure | provider code | failed | none | none | PASS |
| cancellation | cancelled | cancelled | none | none | PASS |

## 14. No-fallback verification

El servicio tiene un solo `GIRABuilderProvider` configurado por operación. La matriz inyecta un proveedor remoto fallido y verifica simultáneamente:

```text
terminal ClinicalAIRun
zero ClinicalDiff
zero therapeutic mutation
zero secondary provider invocation
```

`qwen3.5:9b` no es fallback GIRA.

## 15. AIRun provenance

Preparado y probado en success/failure mocked:

```text
provider, model, operation
prompt_name, prompt_version
app_version, build_revision
input_hash, context_hash, output_hash
remote_request_id, provider_region, model_snapshot
input_tokens, output_tokens, reasoning_tokens, cache_tokens
primary_duration_ms, repair_duration_ms, repair_attempts
price_config_version, estimated_cost_micros
normalized_error_code, status, timestamps
source entity IDs/versions (local AIRun provenance only)
```

La tabla de precios es configuración versionada. El cálculo sintético fue verificado; no se afirma costo real sin corridas reales.

## 16. Logging/privacy

Auditoría outbound implementada:

```text
remote_gira.request_prepared
remote_gira.request_sent
remote_gira.request_failed
remote_gira.response_received
```

Metadata permitida: provider/model, conteos por tipo, bytes, versión de policy, flags cerrados, usage/costo/request ID y error normalizado. Pruebas buscan y prohíben payload, prompt, clinical narrative, response body y API key en mensajes de error. `ParametersJSON` no persiste prompt ni salida clínica cruda.

## 17. Certification runner

`internal/certification/gira.Runner` ejecuta el pipeline real:

```text
shared synthetic A–J
→ longitudinal.Service.BuildGIRA
→ GIRABuilderProvider
→ semantic proposal
→ strict decoder
→ deterministic compiler
→ typed validation
→ ClinicalDiff(pending_review)
```

Ollama y OpenAI remoto opt-in importan el mismo inventario `FixturesAThroughJ`; no existe una copia A–J por proveedor. El runner captura métricas, usage, payload bytes, AIRun y diff.

## 18. Synthetic-only guard

El runner rechaza `Synthetic=false` antes de crear AIRun o llamar al provider. También serializa el payload antes de la llamada y lo bloquea si detecta:

- cualquier literal declarado como prohibido por el fixture;
- UUID interno;
- email;
- identificador sintético tipo DNI.

La suite remota requiere simultáneamente:

```text
RUN_REMOTE_GIRA_CERTIFICATION=1
REMOTE_GIRA_SYNTHETIC_ONLY=1
CLINICAL_GIRA_PROVIDER=openai
OPENAI_API_KEY
CLINICAL_GIRA_MODEL
```

Sin todos esos valores no arranca. `go test ./...` normal la salta y no realiza red remota.

## 19. C1–C25

| Invariant | Result | Verification |
| --- | --- | --- |
| C1 Process ≠ Target | PASS | distinct contracts/tables + full/PG suites |
| C2 Target ≠ Goal | PASS | distinct payloads/tables + full/PG suites |
| C3 Goal ≠ Indicator | PASS | ownership/type validation + PG |
| C4 Hypothesis ≠ Rationale | PASS | separate contracts and links |
| C5 Formulation ≠ Strategy | PASS | separate projection/aggregate |
| C6 Approach ≠ Technique | PASS | separate versioned registries |
| C7 Approach selection requires rationale | PASS | GIRA graph/DB validation |
| C8 Rationale requires Target + Goal | PASS | compiler and PG attack tests |
| C9 Technique requires compatible approach/version | PASS | exact registry lookup + FK |
| C10 Multiple approaches require distinct rationale | PASS | same-target eclectic stack rejected |
| C11 Multiple approaches are not required | PASS | A/C semantics permit one |
| C12 Unsupported mechanism cannot be confident | PASS | grounding/registry validation |
| C13 AI Target ≠ approved Target | PASS | mocked E2E creates diff only |
| C14 AI Goal ≠ active Goal | PASS | mocked E2E creates diff only |
| C15 AI cannot achieve Goal | PASS | operation rejected explicitly |
| C16 AI GIRA ≠ approved GIRA | PASS | pending diff only |
| C17 AI cannot complete Phase | PASS | operation rejected explicitly |
| C18 Historical GIRA preserved | PASS | successor/supersedes tests + PG |
| C19 Deprecated approach remains resolvable | PASS | versioned registry/history |
| C20 Evidence does not auto-change Goal | PASS | link-only F/G behavior |
| C21 Review does not mutate strategy | PASS | PG E2E count remains unchanged |
| C22 Human Merge required | PASS | no direct CRUD; CA-W path |
| C23 Merge transactional | PASS | rollback tests in PostgreSQL |
| C24 Stale diff protection | PASS | revision/head/entity checks |
| C25 No AI Direct Merge | PASS | builder/provider has no merge capability |

## 20. Tests added

| Requirement | Test | Type | Result |
| --- | --- | --- | --- |
| serialized PII/PHI absence | `TestRemoteGIRAContextSerializedPayloadExcludesSyntheticPIIAndInternalIDs` | unit | PASS |
| forbidden structural fields | `TestRemoteGIRAContextDTOHasNoForbiddenFields` | reflection/unit | PASS |
| epistemic roles | `TestRemoteGIRAContextPreservesEpistemicRolesAndRelationships` | unit | PASS |
| clinical preservation | `TestGIRAPrivacyMinimizationPreservesClinicalFunction` | unit | PASS |
| deterministic aliases/local map | `TestRemoteGIRAAliasesAreDeterministicAndMappingStaysLocal` | unit | PASS |
| selected process only | `TestRemoteGIRAContextContainsOnlySelectedProcess` | unit | PASS |
| cross-client pre-boundary rejection | `TestGIRABuilderRejectsCrossClientContextBeforeRunOrProvider` | service | PASS |
| minimized strict request | `TestOpenAIProviderUsesMinimizedContextAndStructuredOutput` | fake HTTP | PASS |
| one repair | `TestOpenAIProviderAllowsExactlyOneRepair` | fake HTTP | PASS |
| semantic repair | `TestOpenAIProviderRepairsSemanticallyInvalidStructuredOutput` | fake HTTP | PASS |
| failed repair no third call | `TestOpenAIProviderRejectsFailedRepairWithoutThirdAttempt` | fake HTTP | PASS |
| repair transport failure | `TestOpenAIProviderRepairTransportFailureIsTerminal` | fake HTTP | PASS |
| failure/error sanitization | `TestRemoteFailureMatrixAndErrorSanitization` | fake HTTP | PASS |
| network cause sanitization | `TestRemoteNetworkFailureIsNormalizedWithoutCauseLeak` | fake transport | PASS |
| full mocked success | `TestMockedOpenAIPipelineCreatesOnlyPendingDiffAndSafeAudit` | service/fake HTTP | PASS |
| failure terminal/no fallback | `TestServiceRemoteFailureIsTerminalWithNoDiffAndNoFallback` | service | PASS |
| non-synthetic guard | `TestRunnerRejectsNonSyntheticFixtureBeforeProvider` | certification | PASS |
| forbidden literal preflight | `TestRunnerRejectsForbiddenOutboundLiteralBeforeProvider` | certification | PASS |
| single A–J inventory | `TestSharedFixtureInventoryIsExactlyAThroughJAndSynthetic` | certification | PASS |
| default none/missing credentials | config tests | unit | PASS |
| local component timing | `BenchmarkGIRALocalPipelineComponents` | benchmark | PASS |

Local performance on the test host:

| Component | Observed |
| --- | ---: |
| selection + minimization + alias mapping | ~0.151 ms/op compact fixture |
| full high-PII context build | ~0.408 ms/op |
| serialization | ~0.001 ms/op |
| strict decoder | ~0.029 ms/op |
| deterministic compiler | ~0.033 ms/op |
| typed validation | ~0.022 ms/op |

These are engineering measurements, not provider latency benchmarks.

## 21. Final suites

```text
GO TEST:              PASS — go test -count=1 ./...
GO VET:               PASS — go vet ./...
GO BUILD:             PASS — go build ./cmd/server
POSTGRES:             PASS — internal/http + internal/infra/db, RUN_PG_INTEGRATION=1
OLLAMA LIVE:           PASS — fictitious fixture, qwen3.5:9b
OLLAMA REVIEW:         PASS — fictitious fixture, qwen3.5:9b
OLLAMA LONGITUDINAL:   PASS 7/7 — A–G, qwen3.5:9b
WEB:                  PASS — lint + production build
OPENAPI:              PASS — docs/spec/runtime schema tests
TENANT:               PASS — repository tenant-aware query guard + cross-client outbound guard
MIGRATION:            PASS — isolated temporary DB, 16 → 23 → 16
GIT:                  PASS — git diff --check; dirty working tree preserved
REMOTE ACTUAL:        NOT EXECUTED
```

La base temporal `stage2c4a_migration_test` fue creada exclusivamente para el roundtrip, verificada en versión 16 y eliminada al finalizar. La base de desarrollo normal no se modificó por ese gate.

## 22. Real remote certification

```text
NOT EXECUTED — NO CREDENTIALS BY DESIGN
```

No se inspeccionó ni utilizó ninguna API key del entorno, y ninguna prueba normal realizó una llamada remota. Por tanto no existen resultados A–J, C×5, 30-run, costos ni latencias reales atribuibles a un modelo remoto.

## 23. Remaining blockers

Para Stage 2C.4B:

```text
BLOCKING
- API credentials in an explicitly authorized certification environment
- exact provider/model selection
- contract/privacy acceptance by product/legal/security
- real synthetic A–J = 10/10
- critical C ×5 = 5/5
- A–J ×3 >= 27/30 with zero clinical safety violations

NON-BLOCKING FOR SYNTHETIC CERTIFICATION / BLOCKING FOR REAL CLINICAL USE
- durable remote_ai consent gate
- data-control status independently confirmed with provider
- provider outage/retry operating policy

DOCUMENTATION
- record official price table/version at certification time
- record provider retention/region/control evidence at certification time
- archive per-run hashes, usage and normalized outcomes without clinical payload
```

Durable `ConsentScope` must distinguish `local_ai`, `remote_ai` and `audio_transcription`. Real remote GIRA requires `remote_ai=granted`; synthetic certification does not use patient consent because it contains no patient data. This is a mandatory Stage 2D entry gate, not an excuse to enable real processing now.

## 24. Stage 2C.4B readiness

```text
READY
```

`READY` means the provider-agnostic harness, privacy allow-list, local minimization, fake OpenAI path, safety matrix and synthetic-only guards are ready to run an explicitly authorized remote certification when credentials exist.

It does **not** mean that a provider/model is certified, that Stage 2C is fully verified, or that the system is authorized to send real clinical information.

STOP — Stage 2D was not started; clinical skills, GPT Project sources, frozen prompts, `semantic_v1`, C1–C25 and Human Merge were not changed by Stage 2C.4A.
