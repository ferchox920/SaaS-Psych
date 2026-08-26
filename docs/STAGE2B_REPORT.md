# BACKEND EVOLUTION REPORT — STAGE 2B

## 1. Resultado ejecutivo

```text
PASS
```

Se implementó el núcleo clínico longitudinal con separación epistemológica, propuestas IA aisladas en `ClinicalDiff`, revisión humana granular, merge transaccional y estado reconstruido desde entidades normalizadas.

## 2. Baseline inicial

```text
GO TEST: PASS 170 / FAIL 0 / SKIP 19
```

El árbol ya contenía 47 archivos tracked modificados y numerosos archivos clínicos untracked de Stage 2A/2A.1. No se restauró, eliminó ni sobrescribió trabajo preexistente.

## 3. ClinicalEvidence

- Modelo tenant/client-aware con fuente, versión e ítem inmutables; versionado y lifecycle `active/superseded/invalidated`.
- Taxonomía: `patient_report`, `therapist_observation`, `measurement`, `documented_fact`, `inference`; `hypothesis` está excluido.
- Solo `facts`, `relevant_changes`, `patient_responses` y `affective_nodes` de reportes aprobados/superseded son fuentes válidas.
- Trigger valida existencia, cliente, versión, ítem y contenido literal normalizado; índice parcial evita evidencia activa duplicada.
- Trigger impide reescribir provenance y otro impide hard delete.

## 4. ClinicalEvent

- Tipos clínicos no diagnósticos, tiempos ocurrido/observado, aprobación separada y versión.
- Relación explícita N:M con Evidence y constraint diferido: un evento aprobado no puede quedar sin evidencia.
- Intervenciones solo contextualizan `therapeutic_response` cuando una Evidence citada proviene de `patient_responses`.
- Una modificación humana de la propuesta conserva el original y audita `clinical_event.corrected`.

## 5. ClinicalProcess

- Aprobación `proposed/approved/rejected` separada de estado `observing/active/stabilized/closed`.
- Relación N:M con eventos tenant/client-local; link, update, close y reopen incrementan versión.
- `opened_at` y `closed_at` permiten reconstruir el lifecycle; no existe hard delete.

## 6. ClinicalHypothesis

- Proceso opcional, confidence longitudinal `red/yellow/green` y lifecycle `active/weakened/superseded/retired`.
- Supporting y contradicting Evidence comparten una relación con rol único; una misma evidencia no puede ocupar ambos roles.
- Constraint diferido exige supporting Evidence para toda hipótesis aprobada.
- Varias hipótesis pueden coexistir por proceso y también quedar aprobadas sin proceso en `unassigned_hypotheses`.
- Confidence solo cambia por operación humana fusionada; no cambia al aparecer nueva Evidence.

## 7. ClinicalDiff

- Guarda sesión, reporte, run, revisión base, revisión optimista, incertidumbres y estado de revisión/merge.
- Operaciones tipadas y ordenadas conservan `original_proposal`, `human_modification`, reviewer y timestamps.
- El decoder rechaza campos desconocidos, operaciones no permitidas, targets sin UUID y políticas de versión incorrectas.
- La IA reserva UUIDs de targets, pero no crea entidades longitudinales.
- `ClinicalAIRun succeeded` y creación del diff ocurren en la misma transacción.

## 8. Human Merge

```text
AI
↓
ClinicalDiff pending_review
↓
Human Review: approved | modified | rejected
↓
Merge CA-W
↓
Longitudinal State
```

El merge usa advisory lock por tenant/paciente, bloquea head/diff, comprueba revisión global y versiones de entidad, valida todas las dependencias, aplica operaciones aceptadas, escribe historia/auditoría e incrementa el head una sola vez. Cualquier error hace rollback. Repetir un merge completado devuelve el resultado sin duplicar cambios.

## 9. LongitudinalState

- Se compone desde Evidence, Event, Process, Hypothesis y relaciones normalizadas.
- Incluye procesos aprobados cerrados/retirados, hipótesis aprobadas sin proceso, eventos recientes, Evidence activa y propuestas abiertas separadas.
- Excluye propuestas y rechazos del conocimiento actual.
- El orden usa estado/fecha/UUID como desempate determinista.
- `clinical_longitudinal_transitions` conserva cambios de entidad, versión, estado, diff, operación y actor.

## 10. Longitudinal Interpreter V1

- Prompt nuevo e independiente: `clinical-longitudinal-interpreter-v1`.
- Provider Ollama dedicado con schema estricto, una reparación máxima y recursos tipo review.
- Entrada: SessionReport aprobado, ApprovedClinicalContext y estado longitudinal actual.
- Run `longitudinal_interpretation` se crea antes del provider con fuentes canónicas, hashes y build metadata.
- Salvaguardas: evidence obligatoria, fuente factual restringida, correction-as-data y salidas `explore/insufficient_evidence/open_question`.
- `clinical-live-v1`, `clinical-review-v1` y `session-report-v1.1` no se modificaron.

## 11. Clinical Fixtures

| Fixture | Input | Expected | Actual |
|---|---|---|---|
| A | Evento y proceso existentes | Proponer link | Link tipado válido — PASS |
| B | Patrón con evidencia | Proceso posible | `create_process` pending — PASS |
| C | Evidencia contradictoria | Debilitar hypothesis | `weaken_hypothesis` — PASS |
| D | Evidencia insuficiente | No conclusión | `insufficient_evidence`, cero ops — PASS |
| E | hypothesis candidate de reporte | Seguir candidate | No `create_hypothesis` — PASS |
| F | Corrección del paciente | Contradicción, no resistencia | Link contradicting — PASS |
| G | Evidencia compatible con alternativas | Hipótesis competidoras | Dos propuestas coexistentes — PASS |

## 12. Seguridad y aislamiento

- Todas las tablas, consultas y FKs nuevas incluyen `tenant_id`; relaciones también verifican `client_id`.
- CA-R reutiliza `treating/supervisor`; CA-W reutiliza `treating`.
- Actor UUID humano no nulo es obligatorio para merge; no existe endpoint de CRUD directo ni diff manual.
- No se agregan textos clínicos a logs, métricas o audit metadata.

## 13. Auditoría

Se emiten acciones para propuestas, creación/aprobación/corrección/rechazo, lifecycle de procesos/hipótesis, decisiones y merge. Metadata limitada a IDs, tipos, estados, versiones, conteos y hashes.

## 14. Observabilidad

Métricas nuevas:

- `longitudinal_analysis_total`
- `longitudinal_analysis_duration_seconds`
- `clinical_diff_created_total`
- `clinical_diff_merge_total`
- `clinical_diff_operation_decision_total`

Solo usan labels acotadas de resultado, decisión u operación.

## 15. Migraciones

```text
20: Evidence + Event
21: Process + Hypothesis
22: Diff + Head + Transitions + AI source extensions
```

Resultado real:

```text
16 → 22 → 16: PASS
objetos Stage 2B residuales en 16: 0
16 → 22 nuevamente: PASS
```

## 16. Endpoints

| Method | Route | Permission | Purpose |
|---|---|---|---|
| POST | `/api/v1/clinical-sessions/:id/longitudinal-analysis` | CA-W | Generar/reusar diff |
| GET | `/api/v1/clients/:id/longitudinal-state` | CA-R | Estado aprobado |
| GET | `/api/v1/clients/:id/evidence` | CA-R | Evidence e historia |
| GET | `/api/v1/clients/:id/events` | CA-R | Eventos |
| GET | `/api/v1/clients/:id/processes` | CA-R | Procesos |
| GET | `/api/v1/clients/:id/hypotheses` | CA-R | Hipótesis |
| GET | `/api/v1/clients/:id/clinical-diffs` | CA-R | Diffs del paciente |
| GET | `/api/v1/clinical-diffs/:id` | CA-R | Diff y operaciones |
| PUT | `/api/v1/clinical-diffs/:id/operations/:operation_id/decision` | CA-W | Decisión granular |
| POST | `/api/v1/clinical-diffs/:id/merge` | CA-W | Merge humano transaccional |

## 17. Tests nuevos

- `longitudinal/validation_test.go`: frontera epistemológica, Evidence requerida y JSON estricto.
- `longitudinal/fixtures_test.go`: fixtures clínicos A–G.
- `longitudinal/service_test.go`: reutilización antes de IA y rechazo de actor nulo.
- `ollama/provider_test.go`: schema/provider longitudinal independiente.
- `clinical_longitudinal_repository_postgres_integration_test.go`: no-AI-direct-merge, provenance, lifecycle, revisión, modificación, rechazo, merge, idempotencia, stale/entity conflicts y rollback.
- `clinical_longitudinal_routes_test.go`: inventario de diez rutas.
- `clinical_metrics_test.go`: labels operativas acotadas.

## 18. Resultado final

```text
GO TEST:               PASS 184 / FAIL 0 / SKIP 20
GO VET:                PASS
GO BUILD:              PASS
POSTGRES:              PASS — suites HTTP/repositorio opt-in
OLLAMA:                PASS — qwen3.5:9b, 38.630s
WEB LINT:              PASS
WEB BUILD:             PASS — 13 páginas
OPENAPI:               PASS — OpenAPI 3.0.3, 63 paths
TENANT GUARD:          PASS
MIGRATION ROUNDTRIP:   PASS — 16 → 22 → 16 → 22
```

## 19. No-AI-Direct-Merge Verification

El test PostgreSQL demuestra:

```text
ClinicalDiff creado
→ Evidence/Event/Process/Hypothesis count = 0
→ revisión humana completa
→ merge
→ state_version = 1 y entidades aprobadas visibles
```

## 20. Provenance E2E

Cadena verificada:

```text
SessionReport + version + fact-001
→ ClinicalEvidence(source identity inmutable)
→ ClinicalEvent(event_evidence)
→ ClinicalProcess(process_events)
→ ClinicalHypothesis(hypothesis_evidence)
→ ClinicalDiff + operation
→ ClinicalAIRun
→ reviewer + merge + transition
```

Desde Hypothesis se recupera Evidence y desde esta `report_id`, `report_version`, `source_item_id`; Diff aporta `source_ai_run_id`, propuesta y decisión humana.

## 21. Concurrency verification

- Diff con `base_state_version` anterior: HTTP/domain conflict, cero cambios.
- Operación con `expected_entity_version` incorrecta: conflict y rollback completo.
- Decisión repetida o revisión de diff desactualizada: conflict.
- Merge repetido: respuesta idempotente, sin duplicados.

## 22. Regresiones

Los 170 tests baseline continúan pasando. Los prompts live/review, microadvisor, skills clínicas, reportes, formulaciones, transcripción, calendario, auth, RBAC y tenant isolation no cambiaron conceptualmente.

## 23. Git inventory

### PREEXISTING

El working tree completo de Stage 2A/2A.1 ya estaba sucio y fue preservado.

### STAGE 2B MODIFIED

- `apps/api/cmd/server/main.go`
- `apps/api/internal/http/server.go`
- provider Ollama, ClinicalAIRun types/metrics y OpenAPI dentro de paquetes preexistentes untracked
- `docs/openapi.yaml`

### STAGE 2B NEW

- paquete `internal/usecase/longitudinal`
- handler y tests de rutas longitudinales
- repositorio/merge y test PostgreSQL longitudinal
- migraciones 20–22 up/down
- este informe

### UNDETERMINABLE

Los paquetes clínicos Stage 2A/2A.1 permanecen untracked; Git no permite separar con certeza líneas previas y líneas Stage 2B dentro de esos mismos archivos. No se realizaron commits.

## 24. Deuda restante

- No existe UI Stage 2B para revisión; la API está lista.
- La reconstrucción normalizada usa varias consultas y deberá medirse con historiales grandes antes de paginar/optimizar.
- Un diff de solo incertidumbres queda abierto como resultado exploratorio; no cambia estado y se reutiliza mientras la revisión base no cambie.
- No hay jobs/retries asíncronos, por exclusión explícita de Stage 2B.
- Goals, GIRA, GPT, routing, RAG, embeddings y transcripción durable permanecen fuera de alcance.

## 25. Readiness for Stage 2C

```text
ClinicalEvidence: READY
ClinicalEvent: READY
ClinicalProcess: READY
ClinicalHypothesis: READY
Competing hypotheses: READY
ClinicalDiff: READY
Human Review: READY
Human Merge: READY
Optimistic concurrency: READY
LongitudinalState: READY
Longitudinal Interpreter: READY
Provenance E2E: READY
Tenant isolation: READY
No-AI-Direct-Merge: READY
```

**STAGE 2B: PASS.**
