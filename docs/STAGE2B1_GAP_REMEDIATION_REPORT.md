# STAGE 2B.1 — GAP REMEDIATION & FINAL ACCEPTANCE REPORT

## 1. Veredicto

```text
VERIFIED PASS
STAGE 2B = VERIFIED PASS
```

Todos los blockers definidos por Stage 2B.1 quedaron cubiertos por pruebas ejecutables. No se inició Stage 2C ni se agregaron funcionalidades terapéuticas fuera de alcance.

## 2. Baseline

- Commit/branch inicial registrado: `ab27248`, `main`.
- Working tree inicial: limpio; se preservó sin operaciones Git destructivas.
- Baseline certificado de Stage 2B: `PASS 184 / FAIL 0 / SKIP 20`.
- Revalidación final Stage 2B.1: `PASS 192 / FAIL 0 / SKIP 38`. Los skips adicionales corresponden a suites opt-in PostgreSQL/Ollama, ejecutadas por separado y aprobadas.

Inventario Git final:

- `PREEXISTING`: ninguno en el baseline limpio.
- `MODIFIED`: handlers docs, repositorio/merge longitudinal, provider Ollama, paquete usecase longitudinal y `docs/openapi.yaml`.
- `NEW`: suite PostgreSQL de aceptación, suite Ollama A–G y este informe.
- Migraciones existentes 20–22: no se reescribieron; no fue necesaria migration 23.

## 3. Gaps from prior audit

| Gap original | Acción | Test/demostración | Resultado | Cerrado |
|---|---|---|---|---|
| D7 post-review/pre-merge | Fingerprint completo antes/después de review | `TestReviewedApprovedDiffDoesNotMutateLongitudinalStateBeforeMerge` | PASS | Sí |
| Flujo real, no repository shortcut | `Service.Analyze → AIRun → Diff → review → merge` | `TestLongitudinalAnalyzeCreatesDiffWithoutMutatingState` | PASS | Sí |
| AIRun longitudinal | Success/failure/cancel/invalid/sources | `TestLongitudinalAIRunFailureCancellationAndInvalidOutput` y E2E Analyze | PASS | Sí |
| Fixtures A–G por Service | Estado y referencias reales antes de Analyze | `TestLongitudinalFixturesAThroughGUseRealService` | 7/7 PASS | Sí |
| Fixtures A–G Ollama | Suite opt-in semántica | `TestOllamaLongitudinalEvaluationAThroughG` | 7/7 PASS | Sí |
| Patient correction | Guarda epistemológica y referencias estrictas | tests de semantics + fixture F | PASS | Sí |
| Tenant/client matrix | Matriz A/A1/A2 y B/B1 | `TestClinicalLongitudinalTenantAndClientIsolationMatrix` | PASS | Sí |
| CA-R/CA-W | Denegaciones State/Analyze/Decide/Merge | `TestLongitudinalCARAndCAWDenials` | PASS | Sí |
| Rollback parcial | Op1 válida, Op2 conflictiva | `TestClinicalLongitudinalMergeRollsBackAfterPartialApplication` | PASS | Sí |
| Supporting + contradicting | Persistencia simultánea | `TestHypothesisCanPreserveSupportingAndContradictingEvidence` | PASS | Sí |
| Hipótesis competidoras | Dos hipótesis activas mismo proceso | test anterior + fixture G | PASS | Sí |
| Lifecycle Hypothesis | strengthen/weaken/retire/no auto-confidence | `TestHypothesisLifecycleAndEvidenceDoesNotAutoChangeConfidence` | PASS | Sí |
| Lifecycle Process | close/reopen/timestamps/version/history | `TestProcessCloseReopenLifecyclePreservesHistory` | PASS | Sí |
| Approval isolation | Conteos exactos Stage 2B | `TestSessionReportApprovalDoesNotMutateStage2BArtifacts` | PASS | Sí |
| Evidence V1/V2 | Provenance inmutable por versión | `TestEvidenceVersionProvenanceRemainsBoundToOriginalReportVersion` | PASS | Sí |
| Evidence duplicate/lifecycle | Unique active/invalidate/supersede | `TestEvidenceDuplicateInvalidationAndSupersessionLifecycle` | PASS | Sí |
| State exclusion | Matriz approved/rejected/proposed/invalidated/diffs | `TestLongitudinalStateExclusionMatrixAndDeterminism` | PASS | Sí |
| State determinism | Deep equality x3 y orden físico alterado | mismo test | PASS | Sí |
| Process/Hypothesis history | Queries internas con diff/operación/humano | lifecycle/history tests | PASS | Sí |
| Provenance completa | H1 hasta AIRun fuente del SessionReport | `TestLongitudinalProvenanceEndToEndReachesSessionReportSourceAIRun` | PASS | Sí |
| Review granular | approved/modified/rejected | `TestGranularHumanReviewAppliesApprovedAndModifiedButNotRejected` | PASS | Sí |
| Event correction | Original + modificación + audit corrected | mismo test | PASS | Sí |
| Constraints negativas | Event/Hypothesis sin evidence | `TestApprovedEventAndHypothesisRequireEvidenceAtCommit` | PASS | Sí |
| OpenAPI/metrics | Status/schemas/nombre Prometheus | docs tests + parse YAML | PASS | Sí |
| No hard delete | Cuatro entidades core | `TestLongitudinalCoreEntitiesRejectHardDelete` | PASS | Sí |

## 4. D7 verification

```text
before review: Evidence/Event/Process/Hypothesis/links/head = fingerprint X
after review:  Evidence/Event/Process/Hypothesis/links/head = fingerprint X
after merge:   fingerprint Y; head incrementado exactamente una vez
```

Solo `ClinicalDiff` y sus decisiones cambian durante review.

## 5. No-AI-Direct-Merge E2E

```text
Service.Analyze
→ ClinicalAIRun running antes del provider
→ provider controlado
→ ClinicalAIRun succeeded + ClinicalDiff pending_review
→ estado longitudinal sin cambios
→ review humano completo
→ estado longitudinal aún sin cambios
→ Service.Merge con actor CA-W
→ entidades aprobadas + transición + head incrementado
```

## 6. Longitudinal AIRun

| Caso | Estado terminal | Diff | Mutación longitudinal |
|---|---|---|---|
| Success | `succeeded` | Sí | No, hasta merge |
| Provider failure | `failed/provider_error` | No | No |
| Cancellation | `cancelled` | No | No |
| Invalid output/semantics/references | `failed/invalid_model_output` | No | No |

Las sources reales incluyen SessionReport, ApprovedClinicalContext y Evidence/Event/Process/Hypothesis aprobados con versión.

## 7. Fixtures A–G through Service

| Fixture | Resultado determinista por Service |
|---|---|
| A | `link_event_process`, sin Process duplicado — PASS |
| B | `create_process` pendiente — PASS |
| C | weaken/link contradicting — PASS |
| D | cero operaciones + `insufficient_evidence` — PASS |
| E | cero `create_hypothesis` — PASS |
| F | corrección vinculada como contradicción — PASS |
| G | dos `create_hypothesis` competidoras — PASS |

## 8. Ollama A–G

Modelo ejecutado: `qwen3.5:9b`. Corrida conjunta: `335.46s`.

```text
A: PASS — link_event_process; versión/UUID reales; sin duplicado
B: PASS — create_process con dos Evidence activas
C: PASS — link_contradicting_evidence + incertidumbre
D: PASS — operations=[] + insufficient_evidence
E: PASS — hypothesis_candidate permanece candidato
F: PASS — correction como dato correctivo; no resistencia factual
G: PASS — dos hipótesis red competidoras y coexistentes
```

El schema V1 se restringe por la operación estructurada de `longitudinal_candidates`, sin hardcodear lenguaje clínico. Toda salida además pasa validación de fuentes, tipos de entidad, UUID, dependencias y versiones. Hay hasta dos reparaciones longitudinales acotadas; si no convergen, falla cerrado.

## 9. Tenant/client isolation matrix

Se verificaron Tenant A/Client A1/A2 y Tenant B/Client B1 para lecturas, fuente de reporte, Event–Evidence, Process–Event, Hypothesis–Process/Evidence, Diff–Session/Report/Run y targets de merge. Todos los cruces tenant/client fueron rechazados.

## 10. Transaction rollback

```text
Op1 create_evidence: ejecutada dentro de la transacción
Op2 update_process: expected version incorrecta
→ merge conflict
→ rollback
→ Evidence Op1 ausente
→ Process/head/diff sin aplicación parcial
→ cero transitions parciales
```

## 11. Evidence lifecycle

- Identidad de fuente V1/V2 inmutable.
- Duplicado activo rechazado.
- `active → invalidated` permanece histórico.
- `active → superseded` permanece histórico.
- Hard delete rechazado.

## 12. Process lifecycle

`observing → closed → active` se verificó con incrementos de versión, `closed_at`, `opened_at`, ausencia de delete y reconstrucción histórica. El SQL close/reopen fue corregido con casts explícitos para parámetros `CASE` de PostgreSQL.

## 13. Hypothesis lifecycle

```text
red → yellow → green
green → yellow → red
active/weakened → retired
```

Cada transición incrementa versión; agregar Evidence por sí sola no cambia confidence.

## 14. Supporting / contradicting evidence

H1 conserva simultáneamente supporting `[E1,E2]` y contradicting `[E3]`. Los roles no se eliminan entre sí y una misma Evidence no puede ocupar ambos.

## 15. Competing hypotheses

P1 conserva H1 y H2 aprobadas/activas con explicaciones y evidencias distintas. No hay selección automática de winner ni colapso de alternativas.

## 16. LongitudinalState exclusion matrix

Incluye conocimiento aprobado/current, entidades cerradas/retiradas con estado y propuestas abiertas en sección separada. Excluye del estado activo entidades proposed/rejected, Evidence invalidated, diffs rejected/merged como propuestas y operaciones rejected/pending del conocimiento materializado.

## 17. LongitudinalState determinism

Tres lecturas consecutivas fueron `reflect.DeepEqual`. La inserción física inversa no alteró el orden lógico; fechas/estado y UUID actúan como desempate estable.

## 18. SessionReport approval isolation

La aprobación de SessionReport dejó idénticos los conteos de Evidence, Event, Process, Hypothesis, Head, Diff y DiffOperation. No hay materialización longitudinal implícita.

## 19. Provenance E2E

```text
ClinicalAIRun fuente
→ SessionReport SR1 v1 + source_ai_run_id
→ fact-001
→ Evidence E1
→ Event EVT1
→ Process P1
→ Hypothesis H1
→ ClinicalDiff D1 / Operation
→ reviewer / Human Merge
→ transition
```

La navegación inversa recuperó provider, model, prompt_name, prompt_version, app_version y build_revision del AIRun fuente.

## 20. Human review matrix

| Decisión | Pre-merge | Merge |
|---|---|---|
| approved | original preservado | original aplicado |
| modified | original + modificación + reviewer/time | modificación humana aplicada |
| rejected | decisión preservada | no aplicada |

La modificación de Event persistió título/descripción humanos y emitió `clinical_event.corrected`.

## 21. History reconstruction

`GetProcessHistory` y `GetHypothesisHistory` devuelven entidad actual más transiciones ordenadas, diff, operación, propuestas original/humana, actor, merger y timestamps. Cubren creación/aprobación, links, confidence, close/reopen y retire sin introducir event sourcing.

## 22. API authorization

- CA-R: `treating` o `supervisor` para lecturas/history internas.
- CA-W: `treating` humano para Analyze/Decide/Merge.
- Denegaciones comprobadas en State, Analyze, Decide y Merge.
- Actor nil/system/AI no puede fusionar.

## 23. OpenAPI consistency

OpenAPI `3.0.3`, 63 paths, parse YAML PASS. Analyze documenta 400 para sesión incompleta/output inválido, 404 para sesión/reporte ausente y 409 para conflicto de diff. Las respuestas longitudinales usan schemas Event/Process/Hypothesis/State tipados. Tests runtime/spec PASS.

La métrica documentada permanece en convención Prometheus: `longitudinal_analysis_duration_seconds`.

## 24. Migration

No se agregó migración. En base temporal aislada:

```text
16 → 22 → 16 → 22: PASS
```

La base temporal `stage2b1_roundtrip` fue eliminada al terminar.

## 25. Tests added

| Requirement | File | Test | Type | Result |
|---|---|---|---|---|
| D7 | `clinical_longitudinal_acceptance_postgres_integration_test.go` | `TestReviewedApprovedDiffDoesNotMutateLongitudinalStateBeforeMerge` | PostgreSQL | PASS |
| E2E Analyze/Merge | mismo | `TestLongitudinalAnalyzeCreatesDiffWithoutMutatingState` | PostgreSQL/usecase | PASS |
| AIRun lifecycle | mismo | `TestLongitudinalAIRunFailureCancellationAndInvalidOutput` | PostgreSQL | PASS |
| Atomic rollback | mismo | `TestClinicalLongitudinalMergeRollsBackAfterPartialApplication` | PostgreSQL | PASS |
| Evidence roles/competition | mismo | `TestHypothesisCanPreserveSupportingAndContradictingEvidence` | PostgreSQL | PASS |
| Hypothesis lifecycle | mismo | `TestHypothesisLifecycleAndEvidenceDoesNotAutoChangeConfidence` | PostgreSQL | PASS |
| Process lifecycle/history | mismo | `TestProcessCloseReopenLifecyclePreservesHistory` | PostgreSQL | PASS |
| Fixtures A–G | mismo | `TestLongitudinalFixturesAThroughGUseRealService` | PostgreSQL/Service | 7/7 PASS |
| Isolation | mismo | `TestClinicalLongitudinalTenantAndClientIsolationMatrix` | PostgreSQL | PASS |
| Approval isolation | mismo | `TestSessionReportApprovalDoesNotMutateStage2BArtifacts` | PostgreSQL | PASS |
| Version provenance | mismo | `TestEvidenceVersionProvenanceRemainsBoundToOriginalReportVersion` | PostgreSQL | PASS |
| Evidence lifecycle | mismo | `TestEvidenceDuplicateInvalidationAndSupersessionLifecycle` | PostgreSQL | PASS |
| Review/Event correction | mismo | `TestGranularHumanReviewAppliesApprovedAndModifiedButNotRejected` | PostgreSQL | PASS |
| Deferred constraints | mismo | `TestApprovedEventAndHypothesisRequireEvidenceAtCommit` | PostgreSQL | PASS |
| State exclusion/determinism | mismo | `TestLongitudinalStateExclusionMatrixAndDeterminism` | PostgreSQL | PASS |
| Provenance E2E | mismo | `TestLongitudinalProvenanceEndToEndReachesSessionReportSourceAIRun` | PostgreSQL | PASS |
| No hard delete | mismo | `TestLongitudinalCoreEntitiesRejectHardDelete` | PostgreSQL | PASS |
| Correction/source/reference guards | `validation_test.go` | semantic, source and reference tests | Unit | PASS |
| CA-R/CA-W | `service_test.go` | `TestLongitudinalCARAndCAWDenials` | Unit | PASS |
| Ollama A–G | `longitudinal_eval_integration_test.go` | `TestOllamaLongitudinalEvaluationAThroughG` | Ollama opt-in | 7/7 PASS |
| OpenAPI | `docs_test.go` | longitudinal schema/status test | Unit/spec | PASS |

## 26. Final suites

```text
GO TEST:              PASS 192 / FAIL 0 / SKIP 38
GO VET:               PASS
GO BUILD:             PASS
POSTGRES:             PASS — internal/infra/db 5.011s; internal/http 0.349s
OLLAMA LEGACY LIVE:   PASS — 37.98s
OLLAMA LEGACY REVIEW: PASS — 66.18s
OLLAMA LONGITUDINAL:  PASS — A–G 7/7, 335.46s
WEB LINT:             PASS
WEB BUILD:            PASS — 13 routes
OPENAPI:              PASS — 3.0.3, 63 paths
TENANT GUARD:         PASS
MIGRATION ROUNDTRIP:  PASS — 16 → 22 → 16 → 22
GIT DIFF CHECK:       PASS
```

## 27. Final invariant matrix I1–I15

| Invariant | Resultado |
|---|---|
| I1. Solo fuentes elegibles crean Evidence factual | VERIFIED |
| I2. IA solo persiste AIRun/Diff; no entidades longitudinales | VERIFIED |
| I3. Event approved exige Evidence local | VERIFIED |
| I4. Hypothesis approved exige supporting Evidence real | VERIFIED |
| I5. Aislamiento tenant | VERIFIED |
| I6. Aislamiento client dentro de tenant | VERIFIED |
| I7. Review no muta estado antes de merge | VERIFIED |
| I8. Merge es atómico y rollback elimina efectos parciales | VERIFIED |
| I9. Corrección del paciente conserva epistemicidad y no es resistencia factual | VERIFIED |
| I10. Concurrencia global + expected entity version | VERIFIED |
| I11. Solo actor humano CA-W fusiona | VERIFIED |
| I12. Lifecycle sin hard delete | VERIFIED |
| I13. Current state excluye propuestas/rechazos/invalidados | VERIFIED |
| I14. Orden y reconstrucción deterministas | VERIFIED |
| I15. Provenance llega hasta AIRun fuente y build metadata | VERIFIED |

## 28. Remaining gaps

```text
BLOCKING:      ninguno
NON-BLOCKING:  la UI de revisión longitudinal sigue fuera de Stage 2B backend;
               historiales grandes aún no tienen paginación específica.
DOCUMENTATION: ninguna divergencia conocida; métricas y OpenAPI alineados.
```

No se implementaron Goals, GIRA, registries, GPT, routing, jobs, RAG, embeddings, audio/transcripción/consentimiento durable ni Stage 2C.

## 29. Stage 2C readiness

```text
ClinicalEvidence: VERIFIED
ClinicalEvent: VERIFIED
ClinicalProcess: VERIFIED
ClinicalHypothesis: VERIFIED
Supporting/Contradicting Evidence: VERIFIED
Competing Hypotheses: VERIFIED
ClinicalDiff: VERIFIED
Granular Human Review: VERIFIED
Human Modification Provenance: VERIFIED
Human Merge: VERIFIED
Transactional Merge: VERIFIED
Rollback atomicity: VERIFIED
Idempotent Merge: VERIFIED
Optimistic Concurrency: VERIFIED
LongitudinalState: VERIFIED
State determinism: VERIFIED
Interpreter: VERIFIED
Ollama longitudinal behavior: VERIFIED
Provenance E2E: VERIFIED
Tenant Isolation: VERIFIED
Client Isolation: VERIFIED
No-AI-Direct-Merge: VERIFIED
```

```text
STAGE 2B = VERIFIED PASS
```

Detenerse aquí. Stage 2C no fue iniciado.
