# STAGE 3A — ACTIVE SESSION & TRANSCRIPT WORKSPACE ACCEPTANCE REPORT

Fecha: 2026-09-09. Entorno: Windows, Next.js 16.1.6/React 19, Edge headless, Go y PostgreSQL 17 temporal. Solo fixtures ficticios. No pacientes, sesiones, notas ni grabaciones reales.

## 1. Verdict

```text
VERIFIED PASS
```

Aceptación del alcance Stage 3A: workspace frontend operativo integrado con los contratos de ingestión existentes. No equivale a autorización de producción ni a aceptación de Stage 3B.

Resultados finales: frontend **18 PASS / 0 FAIL**; Go ordinario **313 PASS / 0 FAIL / 75 SKIP**; suite completa con PostgreSQL y aceptación habilitados **442 PASS / 0 FAIL / 10 SKIP**. Lint, build web, vet, build API, OpenAPI y tenant guard PASS. Los conteos Go incluyen padres/subtests con nombre, no resultados de paquetes.

La evidencia tiene dos niveles diferenciados: navegador real con API simulada y backend HTTP/worker/almacenamiento con PostgreSQL real y proveedores sintéticos. No se afirma haber ejecutado en esta etapa navegador → micrófono físico → Whisper/Ollama reales. La aceptación real de esos proveedores permanece documentada en Stage 2D.1; no se reutiliza como una nueva ejecución.

## 2. Baseline

Se auditó la aplicación existente: App Router, layout autenticado, AuthProvider con refresh, contexto tenant, API client común, React Query, formularios y componentes actuales. No existía una suite frontend. Se añadió únicamente `@playwright/test`, reutilizando Edge instalado y `next start`; no se añadió otra capa de estado, SDK clínico ni servidor de mocks.

Baseline histórico: `STAGE2D1_PRODUCTION_READINESS_CONCURRENCY_ACCEPTANCE_REPORT.md`, backend VERIFIED PASS, frontend pendiente y producción condicionada a políticas/configuración explícitas. Se preservó el árbol de trabajo sucio acumulado de etapas anteriores. No hubo commit, reset, checkout destructivo, migraciones nuevas ni borrado de archivos clínicos.

Inventario atribuible a Stage 3A — no confundir con todo el diff contra HEAD:

- NEW: ruta `apps/web/src/app/(app)/clients/[clientId]/session/page.tsx`; seis módulos en `apps/web/src/features/session-workspace/`; dos specs en `apps/web/tests/`; `apps/web/playwright.config.ts`; `apps/api/internal/acceptance/ingestion/stage3a_contract_test.go`; este informe.
- MODIFIED: enlace en `clients-view.tsx`; opción rawBody en API client; package/lock y exclusión de resultados de tests. Backend: handlers de sesión/ingestión, composición de rutas, servicios de sesión/ingestión, proyecciones de lectura de artifacts/jobs, matriz HTTP y test OpenAPI; `docs/openapi.yaml`.
- PREEXISTING: los demás cambios Stage 2B–2D.1, proveedores/modelos, prompts, compilador, bridge y migraciones. El estado Git contra HEAD sigue conteniéndolos; no se atribuyen a esta etapa.

Las skills Next.js y React best practices guiaron el límite server/client, carga independiente, limpieza de recursos y revisión de hooks. Se respetó la arquitectura REST/React Query del proyecto. Playwright se utilizó para la aceptación solicitada, sin capturar hardware real.

## 3. Routes/navigation

Ruta `/clients/:clientId/session`, con `params` asíncronos del App Router y un componente cliente autenticado. La lista existente de pacientes ofrece “Abrir sesión clínica” para el paciente seleccionado; el workspace incluye regreso a pacientes. No se sustituyeron rutas anteriores ni se creó una navegación clínica paralela.

## 4. Session workspace

Muestra sesiones del paciente, selección de sesión, estado real `in_progress/completed/voided`, inicio, finalización y cita opcional. Crear e iniciar usa POST `/clinical-sessions`; completar usa POST `/clinical-sessions/:id/complete`, previa confirmación que aclara que no aprueba reportes.

No se cambia el estado clínico antes de la respuesta del servidor. Completar se habilita solo para `in_progress`; no se expone void. Las sesiones voided se muestran, pero no habilitan escritura de ingestión. Una sesión en progreso visible bloquea crear otra desde esta UI.

## 5. Consent UI

Cuatro alcances independientes, sin checkbox genérico:

| Alcance | Habilitación presentada |
| --- | --- |
| LOCAL_AI_PROCESSING | Análisis local |
| AUDIO_RECORDING | Captura/almacenamiento de audio |
| LOCAL_TRANSCRIPTION | Transcripción local |
| EXTERNAL_MANUAL_AI_PROCESSING | Procesamiento externo manual; no activa una API remota |

Cada tarjeta muestra vigencia, definición/versionado y un historial desplegable de grants/revocaciones con fechas. Un grant futuro no se trata como vigente. La confirmación identifica exactamente el alcance. Registrar no presupone otros consentimientos. Revocar explica bloqueo futuro, posible cancelación de trabajos y ausencia de borrado automático de registros clínicos existentes. Se reutilizan los endpoints actuales; no se implementó una política jurídica nueva.

## 6. Recording/upload

MediaRecorder con detección de soporte, solicitud de micrófono, iniciar, pausar/reanudar, detener y envío explícito. Límite visible 25 MiB; cada captura tiene límite técnico visible de 10 minutos. No existe streaming continuo ni upload automático.

WAV, WebM, OGG, MP4 y M4A son las alternativas de archivo admitidas por el contrato. Se valida MIME y tamaño, incluyendo vacío; el servidor conserva la validación definitiva. El cuerpo del upload es binario, no JSON/base64, y usa el cliente HTTP autenticado existente.

Se reconsulta permiso y AUDIO_RECORDING antes de capturar y antes de subir. Sin consentimiento no se intenta grabar/subir como UX normal. Revocar o abandonar el panel detiene tracks/recorder y libera el buffer local. También se liberan tracks si falla la construcción/inicio del recorder. No hay persistencia de audio en localStorage, IndexedDB ni logs. Una petición ya enviada puede terminar del lado servidor; no se promete revertirla al navegar.

Estados: sin captura, permiso pendiente, grabando, pausado, subiendo, subido y fallo. El éxito dice expresamente que todavía no implica una transcripción.

## 7. Artifact state

Fecha de creación, tamaño y estado recibidos del servidor; referencia abreviada para distinguir artifacts. No se renderizan storage path, storage key ni encryption key ID. `created_at` se añadió a la proyección API porque la columna ya existía, pero el contrato no exponía la fecha requerida por la UI. Sin migración o cambio de almacenamiento.

Un artifact no disponible o con retención vencida no habilita transcripción. El listado existente está acotado a 200 elementos; la paginación de historiales extensos queda fuera de esta etapa.

## 8. Transcription jobs

Requiere artifact disponible, permiso de escritura y LOCAL_TRANSCRIPTION vigente. Enqueue usa el artifact_id exacto. Job queued no crea una transcripción en el estado frontend. Se muestran cola, ejecución, reintento programado, completado, fallido y cancelado.

Nueva lectura GET `/clinical-sessions/:id/jobs` permite recuperar trabajos al recargar sin inventar IDs ni depender de memoria de la pestaña. Orden: fecha descendente e ID; máximo 200. Usa la autorización clínica existente treating/supervisor.

## 9. Retry/cancel

Retry refleja el contrato actual: adelantar un reintento queued con motivo transitorio permitido y `attempt < max_attempts`. No reabre estados terminales, no envía attempt ni restablece presupuestos. Requiere el consentimiento del procesamiento correspondiente. No se añadió una clasificación alternativa de errores.

Cancelar se ofrece para queued/running. Antes del POST se cancela la consulta en vuelo; el estado terminal se incorpora únicamente desde la respuesta. Una lectura tardía consulta el cache después del await y no sobrescribe un terminal confirmado. Se probaron tanto una lectura pendiente abortada como una respuesta posterior sintética que pretendía devolver succeeded tras cancelled. Un 409 se explica y obliga a consultar estado, sin éxito optimista.

## 10. Transcript viewer

Selecciona por defecto la versión más reciente; muestra versión, origen, fecha, fuente de audio, engine/model y segmentos con tiempos. No inventa hablantes. Advertencia breve: la transcripción automática puede tener errores de reconocimiento o hablante y no constituye un hecho clínico validado.

Si el contenido fue eliminado, se conserva su entrada histórica y no se ofrecen acciones sobre contenido no disponible.

## 11. Transcript revision/history

“Corregir reconocimiento ASR” edita segmentos y crea una nueva versión mediante POST `/clinical-transcripts/:id/revisions`. Conserva intervalos y envía texto consistente con los segmentos. No sobrescribe v1 ni edita Evidence. La UI aclara que esto no representa una corrección clínica del paciente.

Se verificó v1 original → v2 humana, acceso posterior a v1 y rechazo 422 sin aparición ficticia de v2. El selector conserva una versión histórica elegida explícitamente; solo la selección por defecto sigue la más reciente.

## 12. Analysis job

Se habilita con sesión completed, contenido disponible, permiso de escritura y LOCAL_AI_PROCESSING. “Analizar esta transcripción · vN” envía el ID de esa versión, no resuelve latest al completar la solicitud.

El caso de aceptación analiza v1 mientras v2 ya existe y comprueba el ID enviado. La respuesta succeeded muestra “SessionReport borrador disponible. No está aprobado.” No se añadió aprobación, revisión de ClinicalDiff ni link ficticio a Stage 3B.

## 13. Error handling

Mensajes seguros para 401, 403, 404, 409, 413, 400/415/422, 503 y códigos provider_unavailable/sidecar_unavailable. Errores desconocidos usan un mensaje genérico. Nunca se renderizan mensajes SQL, prompts ni diagnósticos crudos del proveedor.

401 reutiliza refresh/logout y limpieza del cache. Un error en la lectura protegida de sesión oculta el contenido anterior; errores de lecturas de ingestión desmontan ese panel y ofrecen actualización. Los errores de comandos no inventan éxito. Se distinguen indisponibilidad temporal y conflicto/estado terminal.

## 14. Authorization

La UI no deduce CA-W del rol administrativo member/admin/owner. `can_write` proviene de la asignación treating vigente consultada por el backend. Supervisor recibe lectura sin controles activos de escritura. Si no se puede confirmar la capacidad, las escrituras quedan deshabilitadas.

Justificación del campo: el listado administrativo de asignaciones no era utilizable como fuente de permisos por el terapeuta member. La solución mínima es una proyección de la autorización existente, no un nuevo sistema de permisos. Cada mutación sigue revalidando acceso/consentimiento en el servidor. La lectura UI no reemplaza esa protección frente a carreras.

## 15. State management/query behavior

React Query existente. Claves de workspace incluyen tenant, usuario, cliente y sesión; componentes se desmontan al cambiar el contexto. Consultas utilizan AbortSignal y datos de ingestión tienen gcTime=0. No se persiste contenido clínico en almacenamiento del navegador.

Polling por trabajo: 4 segundos solo mientras no terminal, sin reintentos automáticos ocultos de consulta. Listado: 15 segundos mientras haya trabajos activos. Sesiones/consentimientos: 15 segundos y actualización explícita/foco. Los cambios confirmados invalidan lecturas relacionadas. Terminales detienen polling; la prueba observa un intervalo completo sin consultas adicionales.

No hay optimismo para sesiones, consentimiento, transcripts ni resultados clínicos. La invalidación tras un terminal actualiza resultados sin un bucle de autorrefetch.

## 16. Responsive/accessibility basics

Se conserva el shell y componentes existentes. Labels asociados a inputs/selects, botones semánticos, títulos, histories desplegables y regiones status/alert. Confirmaciones identifican su alcance. Estados no dependen exclusivamente de color.

El recorrido completo se comprobó en viewport 390×844 sin desborde horizontal, y se inspeccionó captura completa sintética. Es un workspace vertical largo dentro del shell actual, no un rediseño mobile. No se afirma una auditoría WCAG completa ni compatibilidad certificada con todos los navegadores/lectores de pantalla.

## 17. Tests

`apps/web/tests/workspace-model.spec.ts`: cinco pruebas de alcances/vigencia, límite/MIME, estados/presupuesto/terminales, claves aisladas y errores seguros.

`apps/web/tests/session-workspace.spec.ts`: trece pruebas de navegador contra API simulada:

1. Poll pendiente abortado al cancelar, sin resurrección visual.
2. Micrófono denegado, error seguro y alternativa de archivo.
3. Rechazo 413 del servidor sin declarar upload/transcript exitoso.
4. Corrección rechazada 422 conservando versión original.
5. 404 protegido sin mostrar fuente.
6. 401, refresh fallido y limpieza del workspace.
7. Flujo completo: sesión → scopes → upload binario → queued/running/succeeded → v1/v2 → análisis explícito → borrador; además mobile y errores de página.
8. Supervisor, cuatro alcances, versión visible y job fallido sin polling/retry/cancel.
9. Retry con presupuesto conservado, 409, cancel y lectura obsoleta posterior.
10. Consentimiento rechazado 403.
11. Servicio temporalmente indisponible 503.
12. Archivo >25 MiB bloqueado antes del POST y ocultación de cache tras revocación de acceso.
13. MediaRecorder sintético: pausa, reanudación, stop y limpieza al revocar consentimiento.

Los mocks interceptan la API y no llaman a proveedores externos. Las pruebas de grabación sustituyen MediaRecorder/getUserMedia; no acceden al micrófono real. Los fixtures no se presentan como evidencia de calidad del ASR.

Backend: `TestStage3AWorkspaceReadContracts` con PostgreSQL real verifica timestamp de artifact, ausencia de metadata interna, recuperación de jobs/cancelación y can_write treating/supervisor. La matriz HTTP incorpora GET jobs para foreign-tenant, unassigned-client y supervisor. Test OpenAPI verifica los 17 contratos de ingestión y las nuevas proyecciones.

## 18. U1–U18 invariant matrix

| Invariante | Resultado | Evidencia |
| --- | --- | --- |
| U1 No colapsar alcances | PASS | Cuatro tarjetas y prueba scopes/rendering |
| U2 No implicar consentimiento ausente | PASS | Vigencia temporal, scopes independientes, rechazos 403/503 |
| U3 Audio requiere consentimiento | PASS | Gating, verificación fresca, revocación detiene captura |
| U4 Transcripción requiere consentimiento | PASS | Botón bloqueado hasta LOCAL_TRANSCRIPTION; HTTP 403 real |
| U5 Upload no implica transcript | PASS | Copy explícito y flujo antes del enqueue |
| U6 Jobs reflejan servidor | PASS | queued/running/succeeded/failed/cancelled, sin éxito optimista |
| U7 Retry no resetea presupuesto | PASS | Predicate cerrado, POST sin attempt, prueba 1/3 y backend |
| U8 Cancel no resucita por stale response | PASS | Abort de poll pendiente y guard terminal post-await |
| U9 Transcript no es hecho clínico | PASS | Advertencia y ausencia de aprobación/evidencia |
| U10 Corrección crea versión | PASS | Endpoint revisions, v1→v2 y rechazo 422 |
| U11 Historia visible | PASS | Selector v1 original/v2 ASR; original conservada |
| U12 Análisis exacta versión | PASS | Análisis v1 con v2 existente; ID comprobado |
| U13 Éxito análisis no aprueba | PASS | Indicador borrador; backend real conserva draft |
| U14 Read-only sin controles activos | PASS | can_write servidor; test supervisor UI y HTTP |
| U15 Sin secretos de almacenamiento | PASS | Proyección/JSON backend y metadata UI mínima |
| U16 Sin errores crudos | PASS | Mappings y pruebas con mensajes SECRET sintéticos |
| U17 Sin activación remota | PASS | No cambios de configuración/model routing ni llamadas remotas |
| U18 Bridge manual intacto | PASS | Sin cambios Stage 3A al bridge; regresión completa |

## 19. Web regression

| Comando desde apps/web | Resultado |
| --- | --- |
| `npm run lint` | PASS |
| `npm run build` | PASS, incluida ruta dinámica de sesión |
| `npm test` | 18 PASS / 0 FAIL; última corrida 18,3 s |

Playwright inicia y detiene un servidor de producción Next aislado en 3103. No se arrancó un devserver ni se dejó un proceso web de aceptación escuchando. Para reproducir: instalar dependencias con pnpm, build y test. Default browser channel `msedge`; en otro entorno usar `PLAYWRIGHT_CHANNEL` con un canal instalado compatible. No se instalaron browsers masivos para esta etapa.

Durante desarrollo hubo errores de selectores de prueba —route announcer también tiene role alert— y de mock de logout; fueron corregidos sin debilitar los assertions clínicos. La corrida final no tiene fallos. Warnings de entorno FORCE_COLOR/NO_COLOR son no bloqueantes.

## 20. Backend regression

Cambios mínimos por brechas contractuales demostradas: can_write en listado de sesiones, GET jobs de sesión y created_at de artifact. Mismo dominio, middleware y autorización. Sin cambios de leases, presupuestos, cifrado, retención, transacciones, proveedores, prompts ni migraciones durante Stage 3A.

| Verificación | Resultado |
| --- | --- |
| `go test -count=1 ./...` sin opt-ins | 313 PASS / 0 FAIL / 75 SKIP |
| `go vet ./...` | PASS |
| `go build ./cmd/server` | PASS |
| Suite completa con RUN_PG_INTEGRATION=1 y RUN_STAGE2D1_ACCEPTANCE=1 | 442 PASS / 0 FAIL / 10 SKIP |
| HTTP/handlers/clinicalsession tras assertions OpenAPI finales | PASS |
| OpenAPI + tenant query guard | PASS dentro de las suites |
| `git diff --check` | PASS |

Aunque solo cambió la proyección de artifacts, se repitió la aceptación de almacenamiento/concurrencia por tocar ese archivo. Resultado: **4 workers, 3 lotes, 600/600 trabajos succeeded, 0 fallidos, 0 ejecuciones duplicadas, 0 outputs duplicados, 0 stuck; 8,573 s**. La prueba de caída/reinicio produjo 52 resultados sin duplicados. Idempotencia HTTP: 24 peticiones, una creación y 23 reutilizaciones; retry contendido: dos intentos, un resultado durable. Pasaron fallos parciales de storage, retención, cancelación/revocación, expiración de lease e aislamiento.

PostgreSQL: base temporal previamente creada `stage2d_roundtrip`, loopback 55435. Primera ejecución opt-in falló por servidor detenido; no fue un fallo del código. Se recuperó el cluster sintético. Un bloqueo de sharing de Windows durante recuperación se resolvió moviendo el log de arranque fuera del directorio de datos; no se eliminó ningún dato ni se modificó la base de producto. La suite se volvió a ejecutar íntegramente y pasó.

Los diez skips finales corresponden a providers locales/remotos opt-in y al entrypoint de proceso hijo —este último sí se invoca desde su prueba padre—. No se ejecutaron Whisper real, benchmarks Ollama, GIRA local ni remoto en Stage 3A. No se repitió roundtrip: no hubo migraciones nuevas/cambiadas en esta etapa. Las migraciones previas y su aceptación Stage 2D.1 se preservan.

Evidencia temporal generada: `stage3a-go-default.jsonl`, `stage3a-go-postgres.jsonl` y `stage3a-workspace-mobile.png` en el directorio Temp del usuario. Son artefactos locales de diagnóstico con fixtures sintéticos; no se requiere publicarlos ni incluirlos en control de versiones.

## 21. Remaining gaps

### BLOCKING

Ninguno para el alcance Stage 3A verificado en este entorno. Esto no elimina los bloqueos de deployment ni certifica dispositivos/navegadores no probados.

### NON-BLOCKING

- Verificación adicional de micrófonos físicos y codecs en navegadores objetivo; existe alternativa de archivo.
- Auditoría completa de accesibilidad y refinamiento del shell móvil preexistente.
- Paginación para más de 200 artifacts/jobs por sesión y optimización de historiales muy extensos.
- Prueba de navegador contra backend desplegado y proveedores reales como smoke de despliegue; no confundirla con las dos capas de aceptación actuales.
- Estados externos se refrescan por polling/manual; no hay push ni garantía de actualización instantánea de permisos. El backend sigue siendo autoridad en cada operación.

### STAGE 3B

Report review/approval, ClinicalDiff review, Process/Hypothesis/Evidence workspace, Goals/GIRA editor y UI del bridge manual siguen fuera de alcance. No se implementaron placeholders que aparenten esas funciones.

### DEPLOYMENT

Permanecen las condiciones explícitas de Stage 2D.1: políticas de consentimiento/retención, claves/storage, backups/acceso, worker y sidecar locales configurados, límites/timeout adecuados, HTTPS para micrófono fuera de loopback, autenticación/cookies/CORS y operación segura. No se activa GIRA remoto ni se hereda consentimiento local para procesamiento externo. No introducir datos reales hasta cumplir las condiciones de despliegue y revisión correspondiente.

## 22. Stage 3B readiness

```text
STAGE 3B IMPLEMENTATION READINESS = READY
PRODUCTION DEPLOYMENT = BLOCKED UNTIL EXPLICIT CONFIGURATION/POLICY
```

| Área futura | Evaluación |
| --- | --- |
| Report review | El workspace produce/indica draft con versión fuente explícita; revisión/aprobación humana pendiente |
| ClinicalDiff review | Backend existente preservado; UI de decisiones/conflictos/merge pendiente |
| Processes/hypotheses | Núcleo longitudinal existente; exploración y revisión visual pendiente |
| Goals/GIRA | Contratos/bridge existentes preservados; editor y flujo manual pendientes; ningún modelo remoto activado |

READY significa base disponible para abordar Stage 3B bajo una solicitud nueva, no que esas funcionalidades ya estén aceptadas. **STOP: no se inició Stage 3B/3C, no se activó GIRA remoto y no se modificaron skills clínicas ni fuentes de proyectos GPT.**
