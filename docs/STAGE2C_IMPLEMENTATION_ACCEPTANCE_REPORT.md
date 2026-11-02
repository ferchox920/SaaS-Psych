# STAGE 2C — IMPLEMENTATION & ACCEPTANCE REPORT

Fecha de cierre: 2026-08-28  
Repositorio: `SaaS-Psi`  
Alcance: Therapeutic Strategy Core. No se inició Stage 2D.

## 1. Verdict

```text
PARTIAL
```

El núcleo determinista, persistencia PostgreSQL, revisión humana, merge, historia, provenance, APIs y regresión pasan. El único gate que impide `VERIFIED PASS` es el comportamiento real de `qwen3.5:9b` en Ollama GIRA A–J: pasó 1/10 (J por rechazo seguro) y A–I no produjeron un diff válido dentro del contrato/timeout. Las salvaguardas funcionaron: ninguna salida inválida creó un diff ni mutó estrategia.

## 2. Baseline

Baseline recibido: `PASS 192 / FAIL 0 / SKIP 38`, PostgreSQL/Ollama legacy/A–G/web/OpenAPI/tenant guard/migration/Git en PASS.

Baseline final ejecutado:

```text
GO TEST:              PASS 215 / FAIL 0 / SKIP 43
GO VET:               PASS
GO BUILD:             PASS
POSTGRES OPT-IN:      PASS 70 / FAIL 0 / SKIP 0
OLLAMA LEGACY LIVE:   PASS
OLLAMA LEGACY REVIEW: PASS
OLLAMA A–G:           PASS 7/7
OLLAMA GIRA A–J:      PASS 1/10 — NOT VERIFIED
WEB LINT:             PASS
WEB BUILD:            PASS
OPENAPI:              PASS — 3.0.3, 70 paths
TENANT GUARD:         PASS
MIGRATION ROUNDTRIP:  PASS — 16 → 23 → 16 → 23
GIT DIFF CHECK:       PASS
```

Los 43 skips de la corrida Go estándar son suites opt-in. PostgreSQL y las familias Ollama exigidas se ejecutaron explícitamente por separado; no se convierten en PASS por estar skipped en la corrida estándar.

Inventario Git preservado:

- `PREEXISTING`: todo el working tree Stage 2B.1, incluidos `clinical_longitudinal_acceptance_postgres_integration_test.go`, `longitudinal_eval_integration_test.go` y `docs/STAGE2B1_GAP_REMEDIATION_REPORT.md`.
- `MODIFIED`: composición del servidor, handler/rutas longitudinales, repositorio/merge longitudinal, provider Ollama, métricas, ClinicalAIRun, contratos/validación longitudinal y OpenAPI.
- `NEW Stage 2C`: migración 23 UP/DOWN; `strategy_types.go`, `strategy_validation.go`, `gira_builder.go`, `gira_prompt.go`, `gira_validation.go`; repositorio/merge/tests PostgreSQL de estrategia; evaluación Ollama GIRA; tests HTTP de estrategia; este informe.
- No se descartó, reseteó ni sobrescribió trabajo preexistente.

## 3. Requirement totals

```text
Total audited:    50
Verified:         49
Partial:           0
Not implemented:  0
Not verified:      1
N/A:               0
```

`R-OLLAMA-GIRA` es el requisito no verificado. Por definición de aceptación, un solo gate crítico no verificado obliga a `PARTIAL`.

## 4. Master Compliance Matrix

| ID | Requirement | Status | Code | DB/State | Automated Test | Executed Result |
| -- | ----------- | ------ | ---- | -------- | -------------- | --------------- |
| G0 | Baseline y extensión sin regresión | VERIFIED | composición incremental | migraciones 20–22 intactas | full suites | PASS |
| G1 | ClinicalTarget + grounding | VERIFIED | structs/validator/merge | targets + 3 links + deferred constraint | unit + PG | PASS |
| G2 | ClinicalGoal + M:N Target | VERIFIED | payloads/merge | goals + goal_targets + deferred constraint | unit + PG | PASS |
| G3 | GoalIndicator cualitativo/progreso | VERIFIED | payloads/projection | indicators + evidence/event links | A–J fake + PG | PASS |
| G4 | Approach registry versionado | VERIFIED | repository/API | global DB registry, UUID + slug/version | PG/history | PASS |
| G5 | Technique registry/compatibilidad | VERIFIED | validation | composite FK technique→approach | PG attack | PASS |
| G6 | TherapeuticRationale | VERIFIED | strict chain validator | normalized grounding and registry FKs | unit + PG | PASS |
| G7 | GIRA versionado | VERIFIED | merge/projection | immutable rows, gira_version, supersedes | PG | PASS |
| G8 | GIRAPhase ordenada/normalizada | VERIFIED | merge/projection | position + goal/rationale/indicator links | PG | PASS |
| G9 | ClinicalDiff strategy operations | VERIFIED | strict Go payloads | operation CHECK extended | validation tests | PASS |
| G10 | Transactional strategy merge | VERIFIED | same transaction/advisory lock | head/entity versions/history | rollback/idempotency PG | PASS |
| G11 | GIRA Builder | VERIFIED | separate provider/service/prompt/schema | run then diff-only atomic completion | unit + PG E2E | PASS |
| G12 | Builder safeguards A–J | VERIFIED | semantic simulator + DB validation | merge revalidation | deterministic A–J | PASS |
| G13 | LongitudinalState strategy projection | VERIFIED | explicit therapeutic_strategy | approved entities only | PG projection | PASS |
| G14 | Progress tracking | VERIFIED | link operations | normalized progress/regression links | fixtures F/G + PG | PASS |
| G15 | Goal/GIRA history | VERIFIED | history repository | shared transition ledger | PG reverse history | PASS |
| G16 | Provenance E2E | VERIFIED | source/relationship reconstruction | structural FKs + run sources | PG E2E | PASS |
| C1 | Process != Target | VERIFIED | distinct types/ops | distinct tables/FKs | unit + PG | PASS |
| C2 | Target != Goal | VERIFIED | distinct payloads | distinct tables/M:N | unit + PG | PASS |
| C3 | Goal != Indicator | VERIFIED | distinct types | owner FK | unit + PG | PASS |
| C4 | Hypothesis != Rationale | VERIFIED | separate contracts | separate tables/links | unit + PG | PASS |
| C5 | Formulation != Strategy | VERIFIED | separate state property/prompt rule | separate aggregates | projection test | PASS |
| C6 | Approach != Technique | VERIFIED | separate definitions | separate tables | registry tests | PASS |
| C7 | Approach selection requires rationale | VERIFIED | GIRA requires rationale IDs | normalized gira_rationales | unit + PG | PASS |
| C8 | Rationale requires Target + Goal | VERIFIED | semantic chain validation | required FKs + goal-target check | unit + PG attack | PASS |
| C9 | Technique requires compatible approach/version | VERIFIED | exact lookup | composite FK | PG compatibility | PASS |
| C10 | Multiple approaches need distinct rationale | VERIFIED | selection-key guard | one rationale per selection row | fixture B | PASS |
| C11 | Multiple approaches not required | VERIFIED | no minimum >1 | no such DB constraint | fixtures A/C | PASS |
| C12 | Unsupported mechanism cannot be confident | VERIFIED | grounding guard | approved rationale must be grounded | fixture J + PG | PASS |
| C13 | AI Target != approved Target | VERIFIED | builder emits operation only | entity absent pre-merge | PG E2E | PASS |
| C14 | AI Goal != active Goal | VERIFIED | create proposal only | created planned during human merge | PG E2E | PASS |
| C15 | AI cannot achieve Goal | VERIFIED | builder rejects operation | human merge supports reviewed transition | explicit unit | PASS |
| C16 | AI GIRA != approved GIRA | VERIFIED | diff-only builder | entity absent pre-merge | PG E2E | PASS |
| C17 | AI cannot complete phase | VERIFIED | builder rejects operation | completion requires reviewed merge | explicit unit | PASS |
| C18 | GIRA update preserves prior version | VERIFIED | create vN+1/supersedes | immutable version rows | fixture H + PG | PASS |
| C19 | Deprecated approach remains resolvable | VERIFIED | registry returns versions | no-delete + historical FK | fixture I + PG | PASS |
| C20 | Evidence does not auto-change Goal | VERIFIED | link only | goal row unchanged | fixtures F/G | PASS |
| C21 | Review does not mutate strategy | VERIFIED | decision only touches diff | target count remains zero | PG | PASS |
| C22 | Human merge required | VERIFIED | no direct CRUD | entity writes only in merge tx | E2E | PASS |
| C23 | Merge transactional | VERIFIED | single tx | rollback all operations | PG stale Op2 | PASS |
| C24 | Stale diff cannot overwrite | VERIFIED | revision/version checks | head + row locks | PG | PASS |
| C25 | No AI direct merge | VERIFIED | actor CA-W required | AI run cannot be merger | unit + PG | PASS |
| R-AJ | A–J through real Service/fake provider | VERIFIED | real BuildGIRA pipeline | diff-only | 10 subtests | PASS 10/10 |
| R-OLLAMA-GIRA | Ollama GIRA A–J | NOT VERIFIED | opt-in evaluation exists | safe rejection | real qwen3.5:9b | PASS 1/10 |
| R-ISOLATION | Attack matrix | VERIFIED | service/merge checks | tenant-aware FKs | PG subtests | PASS |
| R-API | Required strategy API | VERIFIED | handler/server/service | repository queries | route/auth/status/docs | PASS |
| R-AUDIT | Metadata-only audit | VERIFIED | bounded event metadata | audit_log | PG inspection/tests | PASS |
| R-OBS | Low-cardinality metrics | VERIFIED | five metric families | no clinical labels | compile/unit/full suite | PASS |
| R-MIGRATION | UP/DOWN/no residual | VERIFIED | migration 23 | 0/8 objects at 16 | real migrate | PASS |
| R-REGRESSION | Final full gates | VERIFIED | whole repository | PostgreSQL latest | all suites | PASS except separately listed Ollama GIRA |

## 5. ClinicalTarget

`ClinicalTarget` has separate `approval_status` and `clinical_status`, compact target taxonomy, version, human/AI provenance, and normalized links to Evidence, approved Hypothesis and approved Event. An approved Target must retain at least one grounding link; a deferred constraint also protects bulk transaction ordering. Creation/update validates approved, tenant/client-local sources. Resolution and retirement retain the row/history; hard delete is rejected.

## 6. ClinicalGoal

`ClinicalGoal` is process-local and linked M:N to one or more approved Targets. Approval and lifecycle (`planned`, `active`, `progressing`, `achieved`, `paused`, `abandoned`) are independent. New Goals materialize as approved/planned only after human merge. Update and transitions increment entity version. The deferred target constraint prevents an approved Goal without Target.

## 7. GoalIndicator

Indicators support `qualitative`, `behavioral`, `self_report`, `frequency`, `scale`, `other`; baseline/target are nullable text, so qualitative observation is first-class. Evidence/Event links carry `supports_progress`, `supports_regression` or `neutral`. Linking increments indicator version and never mutates Goal status.

## 8. Approach Registry

Architecture: DB global versioned registry. This was selected because historical resolution, composite referential integrity, deployment through migration and auditable exact versioning outweigh file-only convenience. Each row has global UUID plus `(slug, version)` identity, arrays for domains/mechanisms/families/signals/limitations/cautions and active/deprecated status. No patient state or PHI is stored. Six initial definitions exist: CBT/TCC, ACT, Logotherapy, Systemic, Gestalt/Phenomenological and Motivational Interviewing. Referenced/deprecated versions cannot be deleted.

## 9. Technique Registry

Eight versioned intervention families are seeded. Each carries purpose/mechanism/indications/cautions/limits/signals—not deterministic scripts. Compatibility is enforced by composite FK `(technique_slug, technique_version, approach_slug, approach_version)`. Historical resolution uses the exact version, including deprecated approaches.

## 10. TherapeuticRationale

The structurally recoverable chain is:

```text
Process → Target ↔ Goal → Rationale → Approach/version → Technique/version?
```

The merge additionally proves the Target belongs to that Goal, not merely the same Process. Approved rationales require `grounding_status=grounded`, approved Evidence/Hypothesis support, exact registry versions and a compatible technique. Unsupported mechanisms remain uncertainty/rejection.

## 11. GIRA

GIRA stores a roadmap row per version with approval and clinical lifecycle separated. `gira_version` is process-unique; entity concurrency uses `entity_version`. A successor must reference an approved same-process predecessor and increment exactly one version. Targets, Goals and Rationales are normalized M:N links. Previous versions are never updated destructively.

## 12. GIRA Phases

Phases have explicit positive `position`, lifecycle, optional entry/exit criteria and version. Goals, Rationales and Indicators are normalized links. Merge verifies they belong to the selected GIRA, and each indicator belongs to a Goal included in the phase. Revisiting earlier phases uses lifecycle/history rather than deletion or a workflow engine.

## 13. ClinicalDiff extension

New operation types:

```text
create_target update_target resolve_target retire_target
create_goal update_goal activate_goal pause_goal achieve_goal abandon_goal
create_goal_indicator update_goal_indicator
link_indicator_evidence link_indicator_event
create_therapeutic_rationale update_therapeutic_rationale
create_gira supersede_gira activate_gira pause_gira complete_gira
create_gira_phase update_gira_phase activate_gira_phase complete_gira_phase pause_gira_phase
```

Every payload is decoded with strict Go structs; unknown fields/forms fail. Original AI JSON remains immutable and human modification is separate.

## 14. Human Review

The existing Stage 2B review boundary is reused. Each operation can be `approved`, `modified` or `rejected` once under optimistic diff revision. PostgreSQL verifies that completing all decisions changes only the diff; strategy tables remain unchanged until merge. Modified payloads are revalidated by operation type.

## 15. Human Merge

Merge requires non-nil human actor and CA-W `treating`. It acquires patient advisory lock, locks head/entities, checks global state version plus entity versions, resolves ordered dependencies, applies only approved/modified operations, writes audit/history, increments patient state once and commits once. A stale second operation rolls back an internally created first Target. Repeat merge returns the stored result without reapplication.

## 16. GIRA Builder

Actual input is `GIRABuilderInput`: approved selected Process, active Evidence, approved Events/Hypotheses, current approved strategy, complete versioned registries and state version. Output schema is a strict `oneOf` per supported operation plus typed uncertainties.

Before provider invocation the service starts:

```text
operation:      gira_build
prompt_name:    clinical-gira-builder
prompt_version: clinical-gira-builder-v1
status:         running
```

Sources record exact Process/Evidence/Event/Hypothesis/Target/Goal/Indicator/Rationale/GIRA/Phase and Approach/Technique definition UUID+version. Input/context/output hashes cover structured content. After validation, run completion and diff persistence share the repository transaction. Provider error, invalid output or cancellation leaves a terminal run and no diff. The PG fake-provider test observes the running row before inference and confirms zero Target rows after a successful build.

## 17. Fixtures A–J through real Service

| Fixture | Objective/input | Expected | Actual | Test | Result |
| ------- | --------------- | -------- | ------ | ---- | ------ |
| A | clear avoidance | one CBT function | target→goal→indicator→CBT rationale→GIRA/phase diff | `TestGIRABuilderFixturesAThroughJRealServicePipeline/A` | PASS |
| B | avoidance + values conflict | distinct CBT/ACT functions | two distinct chains/rationales | `/B` | PASS |
| C | simple behavioral case | no eclectic stack | one approach | `/C` | PASS |
| D | amorphous self-esteem | operationalize + indicator | observable Goal + qualitative indicator | `/D` + amorphous rejection test | PASS |
| E | qualitative change | no invented number | nullable baseline/target | `/E` | PASS |
| F | progress Evidence | progress link, no achievement | `supports_progress` diff only | `/F` | PASS |
| G | regression Evidence | retain history/review | `supports_regression` link | `/G` | PASS |
| H | material change | GIRA v2 supersedes v1 | new immutable row proposal | `/H` | PASS |
| I | deprecated version | exact historical resolution | old version retained/resolved | `/I` + PG | PASS |
| J | unsupported mechanism | uncertainty/rejection | semantic rejection; no diff | `/J` | PASS |

These fixtures use `Service.BuildGIRA`, run tracking, provider boundary, semantic validation and repository diff persistence. They are not direct validator-only fixtures.

## 18. Ollama A–J

Model: `qwen3.5:9b`; opt-in `RUN_OLLAMA_GIRA_EVAL=1`; semantic assertions, not wording.

| Fixture | Result | Actual observation |
| ------- | ------ | ------------------ |
| A | FAIL | after repair, absent/wrong-kind entity reference; backend rejection; 467 s |
| B | FAIL | provider context deadline at 180 s; no candidate/diff |
| C | FAIL | confident rationale without approved grounding; backend rejection; 284 s |
| D | FAIL | Target without approved grounding; backend rejection; 290 s |
| E | FAIL | Target without approved grounding; backend rejection; 322 s |
| F | FAIL | introduced confident rationale without grounding instead of progress-only link; rejection; 308 s |
| G | FAIL | Target without approved grounding instead of regression-only link; rejection; 234 s |
| H | FAIL | created Goal without Indicator instead of valid GIRA v2-only path; rejection; 325 s |
| I | FAIL | created Goal without Indicator while processing deprecated history; rejection; 314 s |
| J | PASS | unsupported mechanism ended in explicit `ErrInvalidModelOutput` backend rejection; 273 s |

Aggregate: `PASS 1 / FAIL 9`. Safety boundary: PASS; model usefulness/readiness: NOT VERIFIED. No invalid result was persisted.

## 19. Progress Tracking

Fixtures F/G and normalized link tables demonstrate Evidence→Indicator with progress/regression semantics. Indicator version/history advances only through reviewed merge. Goal remains active/planned until an explicit human-reviewed transition; new Evidence never auto-achieves it.

## 20. LongitudinalState extension

Each approved Process now exposes an explicit `therapeutic_strategy` containing approved Targets, Goals with Indicators/links, Rationales and versioned GIRAs with ordered Phases. Formulation fields remain separate. List APIs can expose historical lifecycle states, while current process strategy excludes non-approved entities. Ordering uses process/status/priority/version/position plus UUID tie-breaks.

## 21. History

The shared `clinical_longitudinal_transitions` ledger now supports Target, Goal, Indicator, Rationale, GIRA and Phase. It records entity/diff/operation/actor, before/after version and status, action and timestamp; clinical bodies/reasoning remain in diff/entity storage. Goal and GIRA repository history reconstruct creation and subsequent transitions without full event sourcing.

## 22. Provenance E2E

The PostgreSQL acceptance fixture structurally traverses:

```text
SessionReport → Evidence → Process → Target → Goal → Indicator
                         → Rationale → Approach v1 → Technique v1
                         → GIRA v1 → Phase → Diff → Human Merge
```

Stage 2B tests cover Event/Hypothesis links; Stage 2C Target/Rationale accept both. Reverse traversal is executed with joins from Phase/Indicator through GIRA/Rationale/Goal/Target and exact registry version. Run sources independently preserve Process, Evidence and both global definition versions. No textual matching is used.

## 23. Tenant/client attack matrix

| Entity/boundary | Attacks | Enforcement | Result |
| --------------- | ------- | ----------- | ------ |
| Target | cross-tenant/client grounding | composite FK + merge validation | PASS |
| Goal | cross-client/process Target | composite FK + same-process check | PASS |
| Indicator | cross-client Goal; phase indicator from other Goal | composite FK + goal membership | PASS |
| Rationale | cross-client/process/goal-target chain | local approval + chain check + FKs | PASS |
| GIRA | cross-client/process refs; invalid supersession/version | same-process + exact predecessor | PASS |
| GIRAPhase | Goal/Rationale from another GIRA; cross-goal Indicator | owner-link checks + FKs | PASS |
| Strategy Diff | tenant/client mismatch | diff/operation composite FKs | PASS |
| Strategy Merge | cross-tenant diff/actor; stale head/entity | tenant lookup + CA-W + locks | PASS |

The matrix uses real PostgreSQL attacks, not only the static tenant query guard. The static guard also passes.

## 24. API

| Method | Route | Auth | CA | Usecase | Tests |
| ------ | ----- | ---- | -- | ------- | ----- |
| GET | `/api/v1/clients/:id/targets` | tenant+JWT+role | CA-R | `ListTargets` | route + service + PG |
| GET | `/api/v1/clients/:id/goals` | tenant+JWT+role | CA-R | `ListGoals` | route + service + PG |
| GET | `/api/v1/clients/:id/giras` | tenant+JWT+role | CA-R | `ListGIRAs` | route + service + PG |
| GET | `/api/v1/giras/:id` | tenant+JWT+role | CA-R resolved from entity client | `GetGIRA` | route + UUID/status + PG |
| GET | `/api/v1/therapeutic-approaches` | tenant+JWT+role | global non-PHI read | `ListApproaches` | route + docs + PG |
| GET | `/api/v1/therapeutic-techniques` | tenant+JWT+role | global non-PHI read | `ListTechniques` | route + docs + PG |
| POST | `/api/v1/processes/:id/gira-analysis` | tenant+JWT+role | CA-W treating | `BuildGIRA` | route + 400 + CA-W + PG |

No direct Target/Goal/GIRA mutation endpoint exists. Review and merge reuse Stage 2B endpoints. OpenAPI is valid 3.0.3 with 70 paths and explicit strategy schemas/operation enum.

## 25. Database schema

Migration 23 adds:

- `therapeutic_approach_definitions`, `therapeutic_technique_definitions`;
- `clinical_targets` plus evidence/hypothesis/event links;
- `clinical_goals`, `clinical_goal_targets`;
- `clinical_goal_indicators` plus Evidence/Event progress links;
- `therapeutic_rationales` plus Evidence/Hypothesis links;
- `giras` plus Target/Goal/Rationale links;
- `gira_phases` plus Goal/Rationale/Indicator links.

It extends operation/source/entity checks, source validation trigger and transition types. Patient tables use tenant/client composite FKs and covering indexes. Deferred constraints protect approved Target grounding and Goal target membership. No-delete triggers protect all durable strategy/registry entities; relationship replacement is allowed only within controlled versioned merge updates.

## 26. Audit

Proposal audit uses reserved entity UUID without materializing it. Merge emits bounded actions including Target created/updated/resolved/retired, Goal created/updated/lifecycle, Indicator created/updated/progress-linked, Rationale created/updated, GIRA created/lifecycle and Phase created/updated/lifecycle. Metadata contains IDs, operation type, versions/statuses/counts/diff/run references; no statements, rationales, prompts or clinical bodies are copied into audit.

## 27. Observability

Implemented metrics:

```text
gira_build_total{result}
gira_build_duration_seconds
gira_diff_operations_total{operation}
gira_version_created_total
goal_transition_total{transition}
```

Labels are bounded enums/outcomes. No tenant/client/process IDs, patient data or clinical text appear as labels. Merge metrics are recorded only after successful first application, preventing idempotent retries from double-counting versions/transitions.

## 28. Migration roundtrip

Executed against a freshly recreated local test database:

```text
16 clean
→ 23 dirty=false
→ 16 dirty=false; Stage 2C tables present: 0/8
→ 23 dirty=false; approaches: 6; techniques: 8
```

The migration 23 DOWN first removes only Stage 2C source/transition/operation records that cannot be represented by Stage 2B checks, temporarily disabling and restoring only the Stage 2B no-delete trigger required for migration-owned compatibility cleanup. It then removes all Stage 2C objects. Migrations 20–22 were not rewritten.

## 29. Test Traceability

| Requirement | Test file | Test | Type | Executed | Result |
| ----------- | --------- | ---- | ---- | -------- | ------ |
| A–J deterministic/boundary | `gira_builder_test.go` | `TestGIRABuilderFixturesAThroughJRealServicePipeline` | unit/service | yes | PASS 10/10 |
| amorphous Goal | `gira_builder_test.go` | `TestGIRABuilderRejectsAmorphousGoalWithoutIndicator` | unit | yes | PASS |
| AI completion forbidden | `gira_builder_test.go` | `TestGIRABuilderCannotCompleteClinicalState` | unit | yes | PASS 3/3 |
| chain membership | `gira_builder_test.go` | `TestGIRABuilderRejectsRationaleOutsideGoalTargetChain` | unit | yes | PASS |
| CA-W/run cancellation | `gira_builder_test.go` | access/cancellation tests | unit/service | yes | PASS |
| review/merge/projection/history/provenance | `clinical_strategy_postgres_integration_test.go` | `TestClinicalStrategyReviewMergeProjectionHistoryAndProvenancePostgres` | PG | yes | PASS |
| run-before-inference/sources/diff-only | same | `TestGIRABuilderPersistsRunSourcesAndDiffOnlyPostgres` | PG E2E | yes | PASS |
| rollback/stale | same | `TestClinicalStrategyTransactionalRollbackAndStaleConflictPostgres` | PG | yes | PASS |
| isolation/compatibility/no-delete/history | same | `TestClinicalStrategyIsolationCompatibilityNoDeleteAndHistoricalRegistryPostgres` | PG attacks | yes | PASS |
| API routes/status | HTTP strategy tests | route registration + malformed IDs | HTTP | yes | PASS |
| tenant query guard | `tenant_aware_query_guard_test.go` | `TestRepositoryQueriesRemainTenantAware` | static | yes | PASS |
| Ollama GIRA | `gira_eval_integration_test.go` | `TestOllamaGIRAEvaluationAThroughJ` | real Ollama | yes, individually | FAIL 9 / PASS 1 |
| legacy prompts | provider/longitudinal eval tests | live/review/A–G | real Ollama | yes | PASS 1+1+7 |

## 30. Final invariant matrix C1–C25

| Invariant | Result | Proof summary |
| --------- | ------ | ------------- |
| C1 Process ≠ Target | VERIFIED | type/table/FK |
| C2 Target ≠ Goal | VERIFIED | type/table/M:N |
| C3 Goal ≠ Indicator | VERIFIED | owner entity/table |
| C4 Hypothesis ≠ Rationale | VERIFIED | separate epistemic/strategy links |
| C5 Formulation ≠ Strategy | VERIFIED | separate projection/property |
| C6 Approach ≠ Technique | VERIFIED | registries + compatibility FK |
| C7 Selection requires Rationale | VERIFIED | GIRA validation/link |
| C8 Rationale requires Target+Goal | VERIFIED | required FKs + membership |
| C9 Technique compatible with Approach version | VERIFIED | exact composite FK/validator |
| C10 Multiple approaches distinct rationale | VERIFIED | selection guard/fixture B |
| C11 Multiple approaches optional | VERIFIED | fixtures A/C |
| C12 Unsupported mechanism not confident | VERIFIED | semantic+DB rejection |
| C13 AI Target not approved Target | VERIFIED | diff-only |
| C14 AI Goal not active Goal | VERIFIED | diff-only/planned merge |
| C15 AI cannot achieve Goal | VERIFIED | explicit rejection test |
| C16 AI GIRA not approved GIRA | VERIFIED | diff-only |
| C17 AI cannot complete phase | VERIFIED | explicit rejection test |
| C18 GIRA previous version preserved | VERIFIED | successor row/supersedes |
| C19 Deprecated version resolves | VERIFIED | no-delete/exact FK |
| C20 Progress does not auto-change Goal | VERIFIED | link-only fixtures |
| C21 Review does not mutate state | VERIFIED | PG pre-merge count=0 |
| C22 Human merge required | VERIFIED | only mutation path |
| C23 Merge transactional | VERIFIED | partial rollback PG |
| C24 Stale diff blocked | VERIFIED | head/entity conflict PG |
| C25 No AI direct merge | VERIFIED | CA-W human actor + E2E |

## 31. Final suites

```text
GO TEST:              PASS 215 / FAIL 0 / SKIP 43
GO VET:               PASS
GO BUILD:             PASS
POSTGRES:             PASS 70 / FAIL 0 / SKIP 0
OLLAMA LIVE:          PASS
OLLAMA REVIEW:        PASS
OLLAMA LONGITUDINAL:  PASS 7/7
OLLAMA GIRA:          NOT VERIFIED — PASS 1/10
WEB LINT:             PASS
WEB BUILD:            PASS
OPENAPI:              PASS — 3.0.3, 70 paths
TENANT GUARD:         PASS
MIGRATION:            PASS — 16 → 23 → 16 (0 residual) → 23
GIT DIFF CHECK:       PASS
```

## 32. Gaps

### BLOCKING

- Ollama GIRA behavior with `qwen3.5:9b`: A–I do not reliably produce valid diffs and generation/repair takes 180–467 seconds per fixture. Stage 2C cannot be `VERIFIED PASS`, and Stage 2D should not begin, until A–J semantics and latency meet acceptance without weakening safeguards.

### NON-BLOCKING

- Safe rejection works in every invalid observed case; therefore the gap affects usefulness/readiness, not integrity of approved therapeutic state.
- The opt-in evaluator supports `OLLAMA_GIRA_FIXTURE=<name>` for reproducible per-case diagnosis.

### DOCUMENTATION

- Registry architecture, provenance IDs, migration downgrade cleanup and Ollama per-fixture evidence are documented here and in OpenAPI. No separate operational runbook for selecting/tuning a future local model was added because model routing is explicitly out of scope.

## 33. Stage 2D readiness

```text
ClinicalTarget:                     VERIFIED
ClinicalGoal:                       VERIFIED
GoalIndicator:                      VERIFIED
Approach Registry:                  VERIFIED
Approach Versioning:                VERIFIED
Technique Registry:                 VERIFIED
Technique Versioning:               VERIFIED
TherapeuticRationale:               VERIFIED
GIRA:                               VERIFIED
GIRA Versioning:                    VERIFIED
GIRA Phases:                        VERIFIED
Progress Tracking:                  VERIFIED
ClinicalDiff Strategy Operations:   VERIFIED
Granular Human Review:              VERIFIED
Transactional Human Merge:          VERIFIED
Strategy History:                   VERIFIED
GIRA Builder:                       VERIFIED
Ollama GIRA Behavior:               NOT VERIFIED
Provenance E2E:                     VERIFIED
Tenant Isolation:                   VERIFIED
Client Isolation:                   VERIFIED
No-AI-Direct-Merge:                 VERIFIED
```

Overall Stage 2D readiness: `NOT READY` solely because Ollama GIRA behavior is not verified. Work stops at Stage 2C as required.
