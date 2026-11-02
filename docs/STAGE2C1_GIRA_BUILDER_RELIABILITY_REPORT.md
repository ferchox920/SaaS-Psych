# STAGE 2C.1 — GIRA BUILDER RELIABILITY REPORT

## 1. Verdict

```text
STAGE 2C = PARTIAL — MODEL UNSUITABLE
```

El backend determinista y sus límites clínicos quedan verificados. `qwen3.5:9b` mejoró su conformidad con el contrato semántico, pero no alcanzó el gate: A y B requirieron reparación y tardaron 319.8 s y 243.2 s; C produjo dos veces el mismo eclecticismo injustificado. Por ello no existe un primer A–J 10/10 y no corresponde ejecutar 30 corridas. No se debilitó ninguna salvaguarda y se reforzó C10.

## 2. Baseline

Baseline recibido y reproducido antes del hardening:

```text
GO TEST:              PASS 215 / FAIL 0 / SKIP 43
POSTGRES OPT-IN:      PASS 70 / FAIL 0 / SKIP 0
OLLAMA LEGACY:        PASS
OLLAMA LONGITUDINAL:  PASS A–G
OLLAMA GIRA:          PASS 1 / FAIL 9
```

Latencias GIRA baseline (segundos): A 467, B 180, C 284, D 290, E 322, F 308, G 234, H 325, I 314, J 273.

```text
median: 299.0 s
mean:   319.7 s
P95:    467.0 s
max:    467.0 s
```

## 3. Failure taxonomy

| Fixture | Provider/final result baseline | Duration | First/final class | Repair | State effect |
| ------- | ------------------------------ | -------: | ----------------- | ------ | ------------ |
| A | rejected after repair | 467 s | `INVALID_DEPENDENCY` / wrong-kind or absent ref | yes, failed | no diff/state |
| B | context deadline | 180 s | `TIMEOUT` | no usable repair | no diff/state |
| C | rejected confident ungrounded rationale | 284 s | `CLINICAL_GROUNDING_FAILURE` | yes, failed | no diff/state |
| D | rejected ungrounded Target | 290 s | `CLINICAL_GROUNDING_FAILURE` | yes, failed | no diff/state |
| E | rejected ungrounded Target | 322 s | `CLINICAL_GROUNDING_FAILURE` | yes, failed | no diff/state |
| F | rejected new ungrounded rationale instead of progress link | 308 s | `CLINICAL_GROUNDING_FAILURE` | yes, failed | no diff/state |
| G | rejected new ungrounded Target instead of regression link | 234 s | `CLINICAL_GROUNDING_FAILURE` | yes, failed | no diff/state |
| H | rejected Goal without Indicator | 325 s | `MISSING_TARGET_GOAL_LINK` | yes, failed | no diff/state |
| I | rejected Goal without Indicator | 314 s | `MISSING_TARGET_GOAL_LINK` | yes, failed | no diff/state |
| J | safe backend rejection | 273 s | `UNSUPPORTED_MECHANISM` | bounded | no diff/state |

Los tamaños/token counts históricos no estaban expuestos por el proveedor. Stage 2C.1 agrega input/output bytes, eval count, primary/repair timing y clasificación normalizada sin guardar prompts ni cuerpos clínicos.

## 4. Root causes

Taxonomía cuantificada de los nueve fallos baseline:

```text
clinical grounding / wrong clinical action: 5
missing Goal→Indicator contract:             2
invalid dependency/reference bookkeeping:   1
timeout/infrastructure:                      1
```

Determinación de causa:

- `SCHEMA COMPLEXITY / CONTRACT`: el schema anterior tenía un `oneOf` de 22+ operaciones, repetía objetos completos y exigía UUID/version/order bookkeeping al modelo.
- `CONTEXT CONSTRUCTION`: se enviaban objetos de dominio completos y registros enteros; no había presupuesto medido ni aliases opacos.
- `PROMPT AMBIGUITY`: razonamiento clínico, persistencia, dependencia y serialización estaban mezclados.
- `MODEL FAILURE`: aun con contrato compacto y seed fija, C seleccionó ACT+CBT sobre la misma función en dos corridas.
- `VALIDATOR BUG`: C10 comprobaba rationales individuales, pero no impedía dos enfoques sobre el mismo Target. Se reforzó para exigir Targets/funciones distintos.
- `INFRASTRUCTURE/LATENCY`: casi todo el tiempo pertenece a generación/repair local; el compilador Go tarda una fracción despreciable.

## 5. Changes made

### Prompt

- Nuevo contrato versionado `clinical-gira-builder-v1.1`.
- Jerarquía explícita: ROLE → INPUT → ALLOWED PROPOSALS → CLINICAL RULES → OUTPUT → INVALID CONDITIONS.
- Reglas concisas sobre grounding, Goal operacional, Indicator cualitativo, compatibilidad registry, progreso/regresión, supersesión y mínima cantidad de objetos.

### AI-facing schema

- Schema semántico separado del modelo de persistencia.
- Eliminado el gran `oneOf` de operaciones para la inferencia GIRA.
- Refs con namespaces restringidos por schema (`target_*`, `goal_*`, `evidence_*`, `existing_*`).
- `DisallowUnknownFields` y arrays/campos obligatorios se mantienen estrictos.

### Context and registry

- Aliases opacos deterministas; ningún UUID nuevo es generado por el modelo.
- Límite relationship-first: Evidence 24, Events 16, Hypotheses 16, Approaches 12 y Techniques 32.
- Prioridad a evidencia/eventos ligados a Process/Hypothesis/Target/Rationale/Indicator.
- Prefiltro registry por Target domain y enfoques históricos; sin embeddings/RAG.

### Compiler

- UUIDs reservados exclusivamente por backend.
- Resolución exacta de refs existentes; sin fuzzy matching.
- Orden topológico Target → Goal → Indicator → Rationale → GIRA → Phase → links.
- Fases ordenadas por position/ref; versiones y GIRA supersession derivadas del snapshot.
- Validación final continúa usando `ValidateGIRABuilderResult`.

### Repair and diagnostics

- Exactamente 1 primary + 1 targeted repair.
- El repair recibe el error, input permitido y propuesta; no hay retry sin límite.
- Error tipado no-PHI con clasificación, tamaños, first/final failure y métricas.
- AIRun exitoso persiste metadata bounded de repair/duración/eval/output bytes atómicamente con el diff.

### Generation and latency

- Perfil GIRA independiente de live/review.
- Métricas por `context_build`, `provider`, `validation`, `repair`, `diff_compile` y total.
- Histogramas de bytes por componente, sin tenant/client/patient labels.

### Migration reliability

- Corregido `000022_clinical_diffs.down.sql`: elimina únicamente source-links longitudinales no representables antes de restaurar el constraint de v16. El roundtrip con provenance existente ahora pasa.

## 6. Changes explicitly NOT made

- No se hizo el validator más permisivo.
- No se aceptó grounding vacío, mecanismo no soportado, técnica incompatible ni transición AI de Goal/Phase.
- No se creó CRUD longitudinal/strategy directo ni merge automático.
- No se modificaron `clinical-live-v1`, `clinical-review-v1`, skills clínicas ni microadvisor.
- No se añadió GPT, routing, RAG, embeddings, queues, workers, consent, audio ni Stage 2D.
- No se alteró ni limpió el working tree preexistente.

## 7. AI contract architecture

```text
approved Process + bounded approved context + registry candidates
                              ↓
           qwen3.5:9b semantic strategy proposal
                              ↓
 strict decoder + deterministic reference/dependency compiler
                              ↓
       existing Stage 2C typed proposal validator
                              ↓
          ClinicalDiff(pending_review), never merge
                              ↓
              complete human review + CA-W merge
```

## 8. Context budget

Medición sobre el fixture unitario de referencia:

| Component | Before | After |
| --------- | -----: | ----: |
| domain input | 1,729 B | 714 B |
| operation/schema | 19,151 B | 6,287 B |
| compact prompt | not included in historical subtotal | 2,049 B |
| complete optimized context | >20,880 B plus old prompt | 8,889 B |

Breakdown after: Process 68 B; Evidence 105 B; Events 2 B; Hypotheses 2 B; Strategy 68 B; Approaches 118 B; Techniques 190 B; schema 6,287 B; prompt 2,049 B. La reducción medida es al menos 57% respecto del subtotal antiguo, aun antes de sumar el prompt antiguo.

## 9. Generation configuration

| Setting | Baseline evaluation | Selected GIRA profile |
| ------- | ------------------- | --------------------- |
| temperature | 0.1 | 0.0 |
| top_p | provider default | 0.8 |
| top_k | provider default | 20 |
| num_predict | 2,048 | 1,536 |
| context | 12,288 | 8,192 |
| seed | unset | 42 |
| thinking | off | off |
| timeout | 180 s/attempt | 180 s/attempt |
| repair | up to prior multi-repair behavior | exactly 1 |

Thinking ON no se promovió a corrida A–J: thinking OFF ya excedió 4–5 minutos en los casos exitosos y C falló clínicamente dos veces. Añadir otra matriz costosa no podía recuperar el gate 10/10.

## 10. Latency

| Metric | Baseline A–J | Optimized service trials A–C | Delta |
| ------ | -----------: | ---------------------------: | ----: |
| median | 299.0 s | 243.2 s | -55.8 s |
| mean | 319.7 s | 243.3 s | -76.4 s |
| P95 | 467.0 s | 319.8 s | -147.2 s |
| max | 467.0 s | 319.8 s | -147.2 s |

Trials usados: A 319.8 s, B 243.2 s, C 167.1 s. Una segunda C tardó 177.6 s y reprodujo el mismo fallo. A requirió ~160 s de repair; B también requirió repair. El resultado sigue siendo impráctico para una operación síncrona.

## 11. A–J results

| Fixture | Optimized result | Repair | Latency | Notes |
| ------- | ---------------- | ------ | ------: | ----- |
| A | PASS | yes | 319.8 s | real Service; pending diff only; one approach |
| B | PASS | yes | 243.2 s | real Service; distinct Target functions |
| C | FAIL | not sufficient | 167.1 s / 177.6 s | ACT+CBT on same Target twice; C10 guard strengthened |
| D | not rerun | — | — | fail-fast after C made 10/10 impossible |
| E | not rerun | — | — | fail-fast after C made 10/10 impossible |
| F | not rerun | — | — | fail-fast after C made 10/10 impossible |
| G | not rerun | — | — | fail-fast after C made 10/10 impossible |
| H | not rerun | — | — | fail-fast after C made 10/10 impossible |
| I | not rerun | — | — | fail-fast after C made 10/10 impossible |
| J | baseline safe PASS | bounded | 273 s baseline | unsupported mechanism rejected safely |

El intento conjunto también demostró que el timeout estándar de `go test` (10 min) expira tras A+B; el evaluador requiere un `-timeout` explícito y permite selección por fixture.

## 12. Multi-run reliability

```text
A: 2/2 real-service trials (ambas con repair)
B: 1/1 real-service trial (con repair)
C: 0/2 (mismo fallo clínico)
D–J: not run after fail-fast

TOTAL OBSERVED: 3/5
REQUIRED 30-RUN GATE: NOT STARTED
```

La regla exigía iniciar 3×A–J solamente después de un primer 10/10. Esa precondición no ocurrió. El evaluador implementa `OLLAMA_GIRA_RUNS=1..3` para una futura reevaluación.

## 13. Clinical safeguard verification

| Invariant | Result |
| --------- | ------ |
| C1 Process != Target | PASS |
| C2 Target != Goal | PASS |
| C3 Goal != Indicator | PASS |
| C4 Hypothesis != Rationale | PASS |
| C5 Formulation != Strategy | PASS |
| C6 Approach != Technique | PASS |
| C7 Approach requires Rationale | PASS |
| C8 Rationale requires Target+Goal | PASS |
| C9 Technique/Approach exact compatibility | PASS |
| C10 multiple approaches require distinct Target/function | PASS, strengthened |
| C11 multimodal not required | PASS |
| C12 unsupported mechanism blocked | PASS |
| C13 AI Target remains proposal | PASS |
| C14 AI Goal remains planned proposal | PASS |
| C15 AI cannot achieve Goal | PASS |
| C16 AI GIRA is not approved GIRA | PASS |
| C17 AI cannot complete Phase | PASS |
| C18 prior GIRA immutable on supersession | PASS |
| C19 deprecated registry version remains resolvable | PASS |
| C20 Evidence does not auto-change Goal | PASS |
| C21 review does not mutate strategy | PASS |
| C22 human merge required | PASS |
| C23 merge transactional | PASS |
| C24 stale diff blocked | PASS |
| C25 no AI direct merge | PASS |

## 14. Failure safety

Las nueve salidas inválidas baseline terminaron sin diff ni mutación. En Stage 2C.1:

- provider/decode/compiler error → AIRun terminal failed/cancelled;
- invalid semantic output → no `CreateDiff`;
- successful A/B → solo `ClinicalDiff(status=pending_review)`;
- ningún Target/Goal/Rationale/GIRA se materializa antes del merge humano;
- C expuso un gap del guard: antes del refuerzo llegó a un diff pendiente, nunca a estado clínico. El mismo patrón estructural queda ahora rechazado por C10 y cubierto por unit test;
- PostgreSQL verifica rollback, stale conflict y cero estrategia materializada por BuildGIRA.

## 15. AIRun provenance

```text
operation:       gira_build
prompt_name:     clinical-gira-builder
prompt_version:  clinical-gira-builder-v1.1
ai_contract:     semantic_v1
repair_attempts: 1
```

Run-before-inference permanece probado. Input/context hashes cubren el contrato compacto y sources conservan las entidades/registry versions reales. En éxito, `generation_repaired`, repair class/duration, provider duration, eval count y output bytes se agregan a `parameters_json` en la misma transacción que marca run succeeded y crea el diff. No se persisten prompts ni outputs clínicos para diagnóstico.

## 16. Deterministic compiler tests

Cobertura añadida:

- backend-generated UUIDs;
- refs temporales nuevas y refs existing exactas;
- Target→Goal→Indicator y Target+Goal→Rationale;
- GIRA/Phase ordering;
- technique/approach compatibility;
- duplicate/unknown/self-cycle-like refs;
- expected version de Indicator;
- GIRA supersession/version derivada;
- schema unknown-field rejection;
- context relationship-first limits;
- eclectic stack sobre el mismo Target.

Resultado: PASS.

## 17. Regression suites

```text
GO TEST:              PASS 229 / FAIL 0 / SKIP 43
GO VET:               PASS
GO BUILD API:         PASS
POSTGRES:              PASS 70 / FAIL 0 / SKIP 0
OLLAMA LIVE:           PASS
OLLAMA REVIEW:         PASS
OLLAMA LONGITUDINAL:   PASS A–G (7/7, 300.22 s total)
OLLAMA GIRA:           FAIL acceptance — A/B pass, C repeatable failure
OLLAMA GIRA MULTIRUN:  NOT RUN — 10/10 precondition not met
WEB LINT:              PASS
WEB BUILD:             PASS
OPENAPI:               PASS
TENANT GUARD:          PASS
MIGRATION:             PASS 23 → 16 → 23 after DOWN remediation
GIT DIFF CHECK:        PASS
```

El inventario Git fue preservado. Los archivos Stage 2B.1/2C ya modificados o untracked siguen intactos; Stage 2C.1 añadió `gira_semantic.go`, `gira_semantic_schema.go`, `gira_semantic_test.go` y modificó de forma acumulativa builder/provider/config/metrics/run metadata/tests y migration 22 DOWN. No se ejecutó reset/checkout ni limpieza.

## 18. Remaining gaps

### BLOCKING

- `qwen3.5:9b` no cumple C sin intervención del guard y depende de repair en A/B.
- Latencia síncrona de 167–320 s por fixture optimizado.
- No existe A–J 10/10; por definición no existe confiabilidad 27/30.

### NON-BLOCKING BACKEND

- La metadata AIRun distingue primary/repair sin PHI.
- El evaluador necesita `-timeout` superior a 10 min para suites completas debido al proveedor, no al backend.

### DOCUMENTATION

- Una futura arquitectura review debe decidir proveedor/modelo; este stage no añade otro.

## 19. Model suitability

```text
Is qwen3.5:9b suitable for GIRA Builder? NO
```

Evidencia: 1/10 baseline; después de separar razonamiento de serialización, A/B solo pasan con repair y tardan 4–5 minutos; C falla dos veces con eclecticismo injustificado. El backend puede rechazarlo de forma segura, pero eso no lo vuelve útil ni confiable como builder síncrono.

## 20. Stage 2D readiness

```text
NOT READY
```

Se detiene el trabajo aquí. No se inicia Stage 2D ni se incorpora otro proveedor.
