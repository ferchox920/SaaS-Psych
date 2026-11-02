# STAGE 3B — CLINICAL REVIEW & LONGITUDINAL WORKSPACE ACCEPTANCE REPORT

Fecha: 2026-09-09. Aceptación exclusivamente con datos sintéticos.

## 1. Verdict

```text
STAGE 3B = VERIFIED PASS
STAGE 3C IMPLEMENTATION READINESS = READY
PRODUCTION DEPLOYMENT = BLOCKED UNTIL EXPLICIT CONFIGURATION/POLICY
```

Workspace post-sesión implementado sobre los contratos clínicos existentes. La aprobación de reportes, la revisión de operaciones y el merge humano permanecen separados. Las adiciones backend son cuatro lecturas; no se agregó una vía de escritura clínica directa.

El veredicto cubre implementación y aceptación sintética de Stage 3B, no certificación clínica, auditoría WCAG completa ni validación de despliegue productivo. La prueba de navegador usa API simulada; las nuevas lecturas también se verificaron por HTTP handler contra PostgreSQL real, separadamente.

## 2. Baseline

Se retomó el working tree existente, con Stage 3A aceptado y cambios acumulados de etapas anteriores sin commit. No se revirtió, limpió ni consolidó ese trabajo. Referencia preservada: `STAGE3A_ACTIVE_SESSION_TRANSCRIPT_WORKSPACE_ACCEPTANCE_REPORT.md`.

Baseline técnico de esta etapa: Go estándar 313 PASS / 0 FAIL / 75 SKIP y navegador 18 PASS. Resultado final: Go estándar 315 PASS / 0 FAIL / 76 SKIP; PostgreSQL opt-in 376 PASS / 0 FAIL / 32 SKIP; navegador 29 PASS. Los conteos Go incluyen subtests y excluyen eventos de paquete; los dos perfiles no deben sumarse.

Inventario atribuible a Stage 3B, independiente del estado tracked/untracked de etapas anteriores:

| Clasificación | Archivos / alcance |
| --- | --- |
| PREEXISTING | Resto del working tree Stage 2B–3A, dependencias, Playwright/configuración, bridge manual, ingestion, workers, prompts, migraciones y reportes anteriores |
| MODIFIED | `apps/api/internal/http/server.go`: registro de cuatro GET |
| MODIFIED | `docs/openapi.yaml`: contratos de esas lecturas |
| MODIFIED | `apps/web/src/features/session-workspace/workspace.tsx`: enlace al workspace clínico |
| NEW | `apps/web/src/app/(app)/clients/[clientId]/clinical/page.tsx` |
| NEW | `apps/web/src/features/clinical-review-workspace/{model.ts,shared.tsx,reports.tsx,diffs.tsx,knowledge.tsx,workspace.tsx}` |
| NEW | `apps/web/tests/clinical-review.spec.ts` |
| NEW | `apps/api/internal/usecase/longitudinal/workspace_reads.go` |
| NEW | `apps/api/internal/infra/db/clinical_workspace_reads.go` |
| NEW | `apps/api/internal/http/handlers/clinical_workspace_reads.go` |
| NEW | `apps/api/internal/infra/db/clinical_workspace_postgres_integration_test.go` |
| NEW | `apps/api/internal/http/clinical_workspace_routes_test.go` |
| NEW | `apps/api/internal/http/handlers/clinical_workspace_openapi_test.go` |
| NEW | Este informe |

No nuevas dependencias, migraciones, configuración de proveedores ni modificaciones del servicio de merge en esta etapa. Un archivo marcado untracked por Git puede ser PREEXISTING o MODIFIED respecto del inicio de Stage 3B; no se atribuyó todo `git diff HEAD` a esta etapa.

## 3. Routes/navigation

Nueva ruta `/clients/[clientId]/clinical`, enlazada desde el workspace de sesión. Navegación por Resumen, Reportes, Propuestas, Procesos, Evidencia y eventos, Hipótesis, Targets y Goals, GIRA, Contexto aprobado e Historia. Regreso explícito a sesión/transcripción. Las secciones son botones accesibles y no un wizard clínico irreversible.

## 4. SessionReport review

Selección de sesión y lista de versiones con estado, revisión, fechas y referencias. Secciones del `report_json` real renderizadas estructuralmente, incluyendo IDs de ítems, candidatos e incertidumbres. No se convierte el reporte en un bloque JSON opaco ni se presenta un candidato como conocimiento aprobado.

## 5. Report approval/versioning

Edición estructurada por el PUT existente con `expected_revision`. Un draft se revisa conforme al contrato; editar un aprobado produce draft de nueva versión, no sobrescribe la aprobada. Los IDs existentes se preservan y el servidor asigna los nuevos. Superseded no se edita.

Aprobación con confirmación: habilita el reporte como contexto aprobado, pero no aprueba hipótesis/estrategia ni ejecuta merge. La interpretación longitudinal requiere acción separada, sesión completada, reporte aprobado y consentimiento local válido, comprobado nuevamente al pulsar. La API selecciona el reporte aprobado vigente de la sesión; la UI lo explica y no promete analizar una versión histórica seleccionada.

## 6. ClinicalDiff inbox

Lista por paciente con fecha, origen, estado, versión base, operaciones y progreso. Diferencia ejecución de IA local de propuesta externa manual mediante referencias persistidas. Draft/revisión/aprobación/rechazo/merged no se confunden con estado longitudinal materializado.

## 7. Granular review

Cada operación permite aprobar, modificar o rechazar según su estado y permisos. Se envía `expected_diff_revision`. Una operación decidida deja de ofrecer decisiones nuevas. El progreso refleja operaciones revisadas; pendientes impiden merge. No se agregó aprobación paralela por frontend.

## 8. Human modification/rejection

Original y modificación humana se muestran por separado. Editor por campos y estructura; la validación tipada del servidor sigue siendo autoritativa. Una modificación inválida muestra error seguro sin reemplazar el original. El rechazo permanece visible. El contrato de decisión no admite un campo de motivo de rechazo: no se inventó ni se simuló persistencia de ese dato.

## 9. Merge workflow

Acción humana separada con resumen de aprobadas, modificadas, rechazadas y versión base. El conteo aplicable excluye rechazos. Confirmación y POST existente con revisión esperada. No hay mutación optimista del conocimiento clínico: se invalida/refresca tras respuesta del servidor. Durante el envío se bloquean duplicados; merged muestra resultado persistido y no ofrece otro botón de merge. La idempotencia y la transacción siguen siendo responsabilidad del backend existente.

## 10. Stale/conflict UX

`is_stale` bloquea revisión/merge. HTTP 409 informa que cambió el estado/revisión; no se reintenta automáticamente. Existe consulta explícita del estado vigente. Manejo seguro de forbidden, not found, modificación inválida, revisión incompleta y estado terminal, sin revelar errores crudos de DB/proveedor.

## 11. Patient longitudinal overview

Resumen basado en `longitudinal-state`: versión, procesos y su conocimiento aprobado, hipótesis, estrategia, eventos recientes y propuestas pendientes separadas. El frontend no sintetiza conclusiones ni probabilidades. Estados vacíos y de carga son explícitos; propuestas no se incorporan al conocimiento aprobado por estar visibles.

## 12. Processes

Título, descripción, aprobación, estado clínico, versión, cantidades de hipótesis/targets/goals. Detalle anidado con eventos/evidencia, hipótesis, estrategia e historia. Se muestran valores backend observing/active/stabilized/closed sin deducir progreso. No hay edición directa del proceso.

## 13. Evidence & Events

Evidencia identifica expresamente patient_report, therapist_observation, measurement, documented_fact e inference. Hipótesis es una entidad distinta. Provenance incluye reporte, versión e ítem fuente; statement conserva el contenido recibido.

Eventos muestran tipo, descripción, tiempo y evidencia vinculada; el contexto de proceso también es visible en el detalle anidado de proceso. No se reinterpreta una corrección como resistencia. No se añadió una proyección inversa de todos los procesos a cada tarjeta del listado global de eventos.

## 14. Hypotheses

Statement, aprobación, estado clínico, confidence y cantidades de soporte/contradicción. Varias hipótesis del mismo proceso pueden coexistir sin ranking de ganador. Confidence se presenta como estado registrado, no probabilidad calculada. Entidades históricas disponibles en la lista y sus transiciones bajo demanda.

## 15. Support/contradiction representation

Dos grupos visuales explícitos: evidencia supporting y contradicting. Cada grupo conserva tipos, statement y provenance. La evidencia contradictoria no se oculta ni reduce a una puntuación agregada. Fixture E verifica dos hipótesis competidoras con ambos roles.

## 16. Targets

Tarjeta independiente de Process y Goal: tipo, título, descripción, aprobación/estado y grounding. Las referencias se muestran como referencias, sin fabricar contenido cuando una entidad no viene expandida.

## 17. Goals

Título, tipo, prioridad, estado servidor, targets e indicadores asociados. Sin botón directo de achieve y sin calcular achieved a partir de evidencia o valor numérico. Historia propia inspeccionable.

## 18. Indicators/progress

Representación cualitativa y cuantitativa mediante baseline, target, método/valores y estado recibido. No se fuerzan barras o porcentajes. Enlaces supports_progress, supports_regression y neutral separados. No cambian automáticamente el estado del Goal.

## 19. Therapeutic rationale

Cadena explícita Process → Target → Goal → Rationale → Approach → Technique → señal/efecto esperado. Grounding y referencias permanecen visibles. Rationale no se presenta como hipótesis y técnica no aparece como recomendación independiente de esa cadena.

## 20. Approaches/techniques

Consultas a los listados del registro existente, resolviendo cada referencia por slug y versión exacta. Nombre, versión y estado active/deprecated; técnica y compatibilidad contextualizadas. Una versión histórica deprecated sigue siendo entendible. Si falta metadata del registro se informa, sin sustituir silenciosamente por otra versión.

## 21. GIRA

Versiones con aprobación, estado clínico, resumen, proceso, targets/goals, rationales y fases. GIRA aprobada activa marcada actual; superseded permanece visible. La acción de cambio conduce a propuestas ClinicalDiff existentes, no a CRUD ni a generación remota. No se introdujo creación manual de un diff fuera del bridge existente.

## 22. GIRA phases/history

Fases ordenadas por posición explícita, con descripción, estado, entrada/salida y referencias de goals/indicators/rationales. No wizard irreversible, auto-completion ni inferencia de avance. Versiones GIRA y sus transiciones pueden inspeccionarse sin sobrescribir historia.

## 23. Provenance/history

Historia paginada de Process, Hypothesis, Goal y GIRA: fecha, estado/versiones anterior/posterior, operación, diff, actor/merger. Desde la transición se abre su ClinicalDiff, con original/modificación y origen de IA o propuesta externa.

Provenance AIRun colapsable: ID, provider/model, operación, prompt nombre/versión, status, timestamps y transcript_version_id si una ejecución de ingestion lo vincula. No se expone `parameters_json`, prompt completo, storage keys ni error crudo. Un reporte humano sin AIRun no recibe provenance inventada. Las referencias a fuentes son inspeccionables; no todas son enlaces navegables todavía.

## 24. Authorization

Se reutiliza `can_write` derivado del servidor en la consulta de sesiones, nunca el rol admin como sustituto de CA-W. Cache por tenant/usuario/paciente/ruta y reinicio del workspace al cambiar identidad. Supervisor solo lectura; sin asignación no accede. Fallo de acceso oculta contenido y deshabilita escrituras.

Las cuatro lecturas nuevas aplican el servicio CA-R existente (treating/supervisor). Prueba HTTP handler + PostgreSQL verifica 200 para ambos, 403 sin asignación, rechazo cross-tenant y 404 de entidad ajena al paciente. Las mutaciones mantienen autorización backend preexistente.

## 25. Performance/query strategy

Consultas por sección activa; historia y provenance bajo demanda. TanStack Query existente deduplica, propaga cancelación y separa identidades; `gcTime=0`, `staleTime=10s`, sin retry automático del hook clínico. Listas legacy renderizadas en ventanas de 20 elementos. Evidencia e historia nuevas usan páginas servidor de 25, consulta LIMIT 26 y `has_more`, orden fecha/UUID determinista.

Medición sintética en PostgreSQL: resumen de un proceso y 28 evidencias, 16.242 bytes, aproximadamente 48 ms en una observación; historia de ese proceso, 713 bytes. Evidencia comprobada en páginas 25 + 3 sin solapamiento. No es benchmark productivo ni P95. Algunas proyecciones legacy siguen retornando listas completas por sección; la ventana de render no reduce ese payload. Evaluar cursor/volumen real antes de ampliar o rediseñar. Offset no ofrece snapshot estable frente a inserciones concurrentes.

## 26. Accessibility/responsive basics

Controles nativos, labels, navegación identificada, estado seleccionado con aria-pressed, mensajes status/alert y detalles expandibles. Textos e IDs largos permiten salto de línea. Verificación móvil 390 × 844: sin overflow horizontal en GIRA; captura inspeccionada visualmente. Sin errores de página en el flujo verificado. No equivale a auditoría completa de lector de pantalla, contraste o WCAG.

Revisión final con guías Next.js/React: params asíncronos resueltos en la página servidor, prop serializable clientId, componentes interactivos cliente, hooks estables, limpieza del timer y consultas condicionales. Se mantuvo la API Go y el cliente autenticado existente; no se creó una segunda capa de mutación mediante Server Actions.

## 27. Browser acceptance

Se reutilizó Playwright de Stage 3A con Edge y build Next de producción local. Todas las llamadas API del spec nuevo están interceptadas con fixtures ficticios; rutas no previstas fallan. Ningún paciente o proveedor real intervino.

| Flujo | Resultado | Comprobación |
| --- | --- | --- |
| A Report approval | PASS | Draft→approved explícito, sin merge; editar aprobado crea nueva versión draft |
| B Granular review | PASS | Aprobar op1, modificar op2, rechazar op3; originales preservados |
| C Merge | PASS | Confirmación separada, refresh, resultado único y sin segundo botón |
| D Stale conflict | PASS | 409 visible, sin auto-retry ni error crudo |
| E Competing hypotheses | PASS | H1/H2 y soporte/contradicción separados |
| F Qualitative indicator | PASS | Sin porcentaje ficticio ni achieve |
| G GIRA history | PASS | v1 superseded y v2 actual, versiones de registro y fases; móvil |
| H Supervisor | PASS | Inspección disponible; revisión/merge deshabilitados |

Además: predicados/modelo, flag stale, origen externo manual, modificación inválida e interpretación explícita con consentimiento. Total spec Stage 3B: 11 tests; total web con Stage 3A: 29 PASS, 0 FAIL. No se atribuye a esta suite mock una prueba end-to-end navegador→API real→DB; las pruebas backend separadas cubren persistencia y permisos.

## 28. Backend contract changes

Auditoría basada en rutas, handlers, usecases y OpenAPI actuales, no exclusivamente en reportes históricos. Prefijo API `/api/v1` omitido en la tabla.

| UI domain | Existing endpoint | Missing read projection? | Missing mutation? |
| --- | --- | --- | --- |
| Permiso/sesiones | GET `/clients/:id/clinical-sessions` | No; can_write existente | No |
| SessionReport | GET `/clinical-sessions/:id/reports`, GET/PUT `/session-reports/:report_id`, POST `.../approve` | No | No; edición/versionado existente |
| ApprovedClinicalContext | Servicio canónico existente | Sí: nuevo GET `/clients/:id/approved-clinical-context` | No |
| AIRun/transcript provenance | Repositorio/run attempts existentes | Sí: nuevo GET `/clients/:id/clinical-runs/:run_id/provenance` seguro | No |
| Interpretación | POST `/clinical-sessions/:id/longitudinal-analysis` | No | No |
| ClinicalDiff | GET `/clients/:id/clinical-diffs`, GET `/clinical-diffs/:id` | No | No |
| Review/Merge | PUT `/clinical-diffs/:id/operations/:operation_id/decision`, POST `/clinical-diffs/:id/merge` | No | No |
| Overview | GET `/clients/:id/longitudinal-state` | No | No |
| Evidence | GET `/clients/:id/evidence` | Sí: nuevo GET `/clients/:id/evidence-page?offset=0` para paginación | No |
| Events/Processes/Hypotheses | GET `/clients/:id/events`, `/processes`, `/hypotheses` | No | No; escritura solo por diff |
| Targets/Goals/Indicators | GET `/clients/:id/targets`, `/goals`; indicadores incluidos en Goal | No | No |
| Rationales/GIRA/Phases | GET `/clients/:id/giras`; estrategia anidada en procesos | No | No |
| Registry | GET `/therapeutic-approaches`, `/therapeutic-techniques`; resolución local por slug/versión exacta | No | No |
| History | Servicio/repositorio de historia existentes | Sí: nuevo GET `/clients/:id/clinical-history/:kind/:entity_id?offset=0` | No |

Las cuatro adiciones son GET exclusivamente, con validación de parámetros, acceso tenant/client-local y schemas OpenAPI tipados. History admite process/hypothesis/goal/gira mediante allowlist. Proyección safe de AIRun sin metadata clínica oculta. No nueva tabla ni migración.

Pruebas agregadas: `TestStage3BReadProjectionHTTPPostgres`, `TestStage3BReadRoutesAndNoDirectCRUD`, `TestStage3BOpenAPIReadContracts`. Regresión de rutas descarta escrituras directas de process/hypothesis/goal/GIRA y goal achieve; OpenAPI comprueba seguridad y máximo de página 25.

## 29. V1–V30 matrix

PASS significa preservado por la implementación y la evidencia indicada; no todos los invariantes tienen un test nuevo aislado.

| Invariante | Resultado | Evidencia |
| --- | --- | --- |
| V1 Draft report != approved report | PASS | A, estados y versionado |
| V2 Report approval != longitudinal merge | PASS | A, no llamada merge |
| V3 Evidence type explicit | PASS | Model labels y EvidenceCard |
| V4 Evidence != hypothesis | PASS | Componentes separados, E |
| V5 Supporting != contradicting | PASS | Matriz E |
| V6 Competing hypotheses coexist | PASS | H1/H2 en E |
| V7 Approval != clinical status | PASS | Status muestra ambos campos |
| V8 Process != Target | PASS | ProcessCard/TargetCard separados |
| V9 Target != Goal | PASS | Tipos/tarjetas independientes, F/G |
| V10 Goal != Indicator | PASS | Goal e indicadores anidados, F |
| V11 Hypothesis != Rationale | PASS | HypothesisCard y RationaleCard separados |
| V12 Approach != Technique | PASS | Referencias/versiones separadas, G |
| V13 Rationale Target+Goal grounding | PASS | Cadena explícita, G |
| V14 Technique has approach context | PASS | Rationale y registro exacto, G |
| V15 Qualitative without percentage | PASS | F |
| V16 Evidence does not auto-achieve | PASS | F, sin ruta directa achieve |
| V17 Proposal != approved operation | PASS | Estado por operación, B |
| V18 Modification preserves original | PASS | B y modificación inválida |
| V19 Rejected proposal auditable | PASS | B, rechazo visible e historia |
| V20 Partial review not merge-ready | PASS | Predicado y B/C |
| V21 Review does not mutate state | PASS | B, solo decisiones antes de merge; regresión backend |
| V22 Human Merge explicit | PASS | C, confirmación separada |
| V23 Stale cannot silently merge | PASS | D y flag stale |
| V24 Repeated merge no duplicate | PASS | C, envío único/terminal; idempotencia backend regresada |
| V25 GIRA version history visible | PASS | G |
| V26 Superseded not overwritten | PASS | G, lectura sin CRUD |
| V27 GIRA edits via ClinicalDiff | PASS | Acción a inbox; guard de rutas |
| V28 Read-only cannot review/merge | PASS | H; CA-R/CA-W backend preservado |
| V29 Manual bridge human-mediated | PASS | Bridge intacto; tests Go de bridge incluidos |
| V30 No remote GIRA/API activated | PASS | Sin cambio de proveedor ni ejecución remota; pruebas sintéticas |

## 30. Full regression

| Ejecución | Resultado |
| --- | --- |
| `npm run lint` | PASS |
| `npm run build` | PASS |
| `npm test` | 29 PASS / 0 FAIL, aproximadamente 28 s |
| `go test -count=1 -json ./...` estándar | 315 PASS / 0 FAIL / 76 SKIP |
| Mismo comando con `RUN_PG_INTEGRATION=1` | 376 PASS / 0 FAIL / 32 SKIP |
| `go vet ./...` | PASS |
| `go build ./cmd/server` | PASS |
| Nuevas lecturas HTTP/PostgreSQL | PASS: permisos, aislamiento, versión/provenance y paginación |
| OpenAPI y guard de rutas | PASS, incluidos en Go |
| `git diff --check` | PASS |
| Worker/Whisper soak pesado | NO REEJECUTADO: ingestion/storage/jobs no modificados en esta etapa |
| Ollama/remote benchmarks | NO EJECUTADOS; no son requisito de esta UI ni se activó proveedor remoto |
| Migration roundtrip | NO REEJECUTADO: sin cambios de migración Stage 3B |

PostgreSQL: base sintética local `stage2d_roundtrip`, puerto 55435. `RUN_STAGE2D1_ACCEPTANCE` no habilitado. Los SKIP son pruebas opt-in no ejecutadas, no PASS. Evidencia anterior Stage 2D.1 preservada; no se compara su conteo de soak con el perfil actual.

Logs locales de esta ejecución: `%TEMP%/stage3b-go-default.jsonl` y `%TEMP%/stage3b-go-postgres-final.jsonl`. Captura sintética: `%TEMP%/stage3b-gira-mobile.png`. Son artefactos temporales de máquina, no fixtures productivos ni evidencia almacenada con PHI.

## 31. Remaining gaps

### BLOCKING

Ningún bloqueo identificado para aceptar el alcance Stage 3B bajo los contratos actuales. No implica autorización de producción.

### NON-BLOCKING

- Aceptación browser mock y aceptación PostgreSQL separadas; agregar en CI un flujo completo con API real y entorno sintético antes de despliegue.
- Medición de payload pequeña; faltan perfiles de alto volumen/P95 y cursor estable si se requiere navegación consistente con inserciones concurrentes.
- Algunas listas legacy pagan payload completo por sección aunque rendericen páginas. No se rediseñaron sin mediciones adicionales.
- Referencias técnicas aún no son todas enlaces; pulir nombres de campos/identificadores sin alterar semántica.
- Accesibilidad básica verificada, no auditoría WCAG integral.
- Motivo de rechazo y fuente transcript directa para reportes humanos no están en el contrato; no se inventan.

### STAGE 3C

Export/preview/copy/download/import manual y packaging de Project pendientes por alcance, no implementados aquí. Mantener revisión/merge humano, provenance externo y validación del bridge existente.

### DEPLOYMENT

Continúa bloqueo de producción hasta configuración/política explícitas de Stage 2D.1/3A: autenticación/asignaciones, consentimiento, retención, storage, claves, workers y operación segura. No se modificaron políticas ni se habilitó envío remoto.

## 32. Stage 3C readiness

```text
STAGE 3C IMPLEMENTATION READINESS = READY
STAGE 3C IMPLEMENTATION = NOT STARTED
```

| Capacidad siguiente | Evaluación |
| --- | --- |
| Manual GPT Project export UI | Backend bridge existente; UI pendiente Stage 3C |
| Preview | Pendiente; debe mostrar payload y frontera de privacidad antes de acción humana |
| Copy/download | Pendiente; acción humana explícita, sin sincronización |
| Structured import | Backend existente; formulario UI pendiente |
| Import validation | Reutilizar contrato/validador backend, no validación paralela permissiva |
| External proposal provenance | Distinción ya visible en inbox/detail; conectar import sin ocultar origen |
| ClinicalDiff handoff | Workspace Stage 3B listo para recibir/revisar diff y merge humano |
| Project runtime/source packaging | Auditar/reutilizar contratos existentes en Stage 3C; no editar skills ni fuentes de pacientes aquí |

Se detiene el trabajo después de este informe. No se inició Stage 3C, no se sincronizaron Projects, no se cambiaron skills clínicas ni se activó GIRA remoto.
