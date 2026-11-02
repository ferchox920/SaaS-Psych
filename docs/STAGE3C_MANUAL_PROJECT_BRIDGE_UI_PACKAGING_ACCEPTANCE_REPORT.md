# STAGE 3C — MANUAL GPT PROJECT BRIDGE UI & PROJECT PACKAGING ACCEPTANCE REPORT

Fecha: 2026-09-09. Implementación y aceptación con fixtures exclusivamente sintéticos.

## 1. Verdict

```text
PARTIAL
BRIDGE UI + BACKEND READ PROJECTION = VERIFIED PASS
PROJECT PACKAGING V1 DESIGN = DELIVERED
ACTUAL CLINICAL SKILL COMPATIBILITY = NOT VERIFIED — SOURCE NOT AVAILABLE
PRODUCTION DEPLOYMENT = BLOCKED UNTIL EXPLICIT CONFIGURATION/POLICY
```

La UI y los flujos sintéticos solicitados están implementados y verificados. No se declara PASS integral porque la auditoría requerida de `clinical-microprocess-supervisor` y otras instrucciones activas no puede completarse sin sus fuentes autorizadas. Se entregó la matriz de requisitos de compatibilidad, sin afirmar defectos o compatibilidad de un archivo no disponible. No se modificó ninguna skill ni fuente real para suplir esa ausencia.

## 2. Baseline

Stage 3B: Go estándar 315 PASS / 0 FAIL / 76 SKIP; PostgreSQL opt-in 376 PASS / 0 FAIL / 32 SKIP; web 29 PASS. Working tree anterior preservado, incluyendo modificaciones no commit de etapas previas. No reset, commit, cambio de proveedor ni migración nueva.

Documentación revisada: ADR del bridge, contrato V1, informe Stage 3B y `STAGE2C5_MANUAL_PROJECT_BRIDGE_ACCEPTANCE_REPORT.md`. Este último es el nombre real del informe equivalente al nombre largo citado en el pedido; no se creó un duplicado para simular su existencia.

| Inventario Stage 3C | Alcance |
| --- | --- |
| PREEXISTING | Resto del working tree Stage 2B–3B, prompts, skills, bridge core, migraciones, configuración y dependencias |
| NEW | `apps/web/src/features/project-bridge/model.ts`, `workspace.tsx`, `provenance.tsx` |
| NEW | `apps/web/tests/project-bridge.spec.ts` |
| NEW | `apps/api/internal/usecase/longitudinal/project_workspace.go` |
| NEW | `apps/api/internal/infra/db/clinical_project_workspace.go`, `clinical_project_workspace_test.go` |
| NEW | `apps/api/internal/http/handlers/clinical_project_workspace.go` |
| NEW | `docs/GPT_PROJECT_PACKAGE_V1.md`, este informe y captura sintética en `output/playwright/stage3c-bridge-mobile.png` |
| MODIFIED | Workspace Stage 3B: nueva sección; detalle de diff: lectura de provenance externa |
| MODIFIED | `server.go`: GET de historial; `docs/openapi.yaml`: proyección tipada |
| MODIFIED | Tests de rutas/OpenAPI del bridge y fixture PII del export (tercero sintético agregado) |

La clasificación se refiere al inicio de esta etapa, no a todo `git diff HEAD`. No se atribuyen a Stage 3C los cambios anteriores de archivos tracked o untracked.

## 3. Bridge architecture

```text
SaaS-Psi: export local aprobado/minimizado
  → terapeuta inspecciona preview
  → terapeuta copia/descarga y transfiere manualmente
  → terapeuta pega/sube JSON estructurado
  → backend: decoder estricto + aliases/versiones + compilador/validador
  → ClinicalDiff pending_review con external proposal
  → Stage 3B: revisión granular
  → Human Merge transaccional
```

No SDK, API de IA, upload a Projects, browser automation de ChatGPT, webhook, RAG, embedding ni GIRA remoto. La lectura de documentación pública durante el trabajo no fue una llamada de inferencia ni incluyó contenido clínico.

## 4. Routes/navigation

Sección **GPT Project** dentro de `/clients/[clientId]/clinical`. No aplicación paralela ni ruta clínica de escritura nueva. Handoff al mismo inbox/detalle Stage 3B mediante selección del diff creado. Consentimiento administrado en el workspace de sesión existente.

Auditoría de capacidades; prefijo `/api/v1` omitido:

| Capability | Endpoint | Existing | UI required / implementada |
| --- | --- | --- | --- |
| Selección de proceso | GET `/clients/:id/processes` | Sí | Procesos aprobados, título/estado/versión |
| Consentimiento | GET `/clients/:id/consents` | Sí | Scope externo distinto; gestión Stage 3A |
| Export | POST `/clients/:id/project-exports` | Sí | Generación explícita local |
| Artifact/preview | GET `/clients/:id/project-exports/:export_id` | Sí | Preview completo, copy/download y reautorización |
| Historial | GET `/clients/:id/project-exports?offset=0` | Nuevo GET mínimo | Recibos paginados y política derivada del servidor |
| Import | POST `/clients/:id/project-imports` | Sí | Texto/archivo JSON original, máximo 1 MiB |
| External provenance | GET `/clients/:id/project-proposals/:proposal_id` | Sí | Inspección colapsable desde diff |
| ClinicalDiff | GET `/clinical-diffs/:id` + decisiones/merge existentes | Sí | Reutilizados sin duplicar workflow |

## 5. External-manual consent UX

Scope exacto `EXTERNAL_MANUAL_AI_PROCESSING`, nunca inferido de `LOCAL_AI_PROCESSING`. La nueva respuesta de historial expone si el servicio tiene authorizer de consentimiento configurado. Composición productiva existente siempre lo configura. UI deniega generación/recuperación cuando falta el grant requerido o no puede verificar política/acceso.

Antes de copiar/descargar se invoca nuevamente el GET protegido del export: una revocación posterior al preview impide liberarlo. Respuesta 403/404 descarta preview cacheado. El backend sigue siendo autoridad; no se garantiza atomicidad entre una comprobación y una revocación simultánea posterior. No se cambió el contrato de import, que exige CA-W pero no consulta consentimiento externo: el import es ingreso local, no una transferencia saliente nueva.

## 6. Export configuration

Selección de un proceso aprobado, mostrando título, aprobación, estado clínico y versión. Modo **MINIMIZED fijo**, sin toggle ficticio de historia completa. Descripción del alcance real: runtime, evidencia activa vinculada, eventos/hipótesis aprobados y estrategia seleccionada. No promete SessionReports completos, audio, transcripción ni todos los procesos.

## 7. Preview

Después de generar o recuperar, se muestra el artifact backend completo con secciones/campos/aliases; el Markdown exacto también está inspeccionable en detalles. Afirmación explícita: nada se ha enviado externamente. Copy/download requieren confirmación de revisión, reiniciada para cada snapshot. No se interpreta Markdown como HTML ejecutable ni se extraen enlaces externos automáticos de la narrativa.

## 8. Privacy boundary

La UI no reconstruye el contexto desde otras proyecciones. El fixture de procesos contiene nombre, correo, teléfono, dirección, documento y tercero sintéticos; no se reinsertan en preview/copy/download. El test backend de minimización incluye ahora también `pareja Carlos Gomez` y verifica su eliminación. Procesos ajenos y UUIDs clínicos se excluyen por el selector/minimizer existentes.

Advertencia visible sobre riesgo residual: minimización no es anonimato; narrativa y terceros pueden ser identificables. No se afirmó cobertura perfecta de nombres libres. Copias y descargas quedan fuera del control de retención de la aplicación; el terapeuta debe aplicar la política correspondiente.

## 9. Copy

`Copiar para GPT Project` entrega exactamente `snapshot.markdown` a Clipboard API, tras volver a autorizar la lectura y comprobar identidad/hash/contenido del snapshot. Confirmación solo al resolver la operación: **Copiado al portapapeles**. No declara envío a ChatGPT.

En Windows el portapapeles nativo convierte LF a CRLF: test verifica la entrada exacta a la API y equivalencia de lectura normalizando exclusivamente esos saltos de línea. La aplicación no reescribe contenido ni recalcula hash a partir del portapapeles.

## 10. Download

Blob local Markdown canónico, nombre con versión de schema y prefijo de hash, URL temporal revocada después del click. Archivo verificado byte-for-byte contra el Markdown recibido. Mensaje: **Archivo generado localmente para transferencia manual**. Ninguna descarga desde Project ni reemplazo de fuente externa.

## 11. Portable references

Artifact conserva `process_1`, `evidence_1`, `hypothesis_1` y referencias existentes cuando están presentes. UI no resuelve ni aproxima aliases. El recibo `source_export_id`/hash se muestra por separado: es un identificador opaco del export, no UUID de paciente/proceso. No se copia el envelope API completo con tenant/client/user IDs. La correlación del recibo debe incorporarse manualmente al import según contrato.

## 12. Runtime profile

Renderiza `case_runtime_profile` tal como lo entrega el backend: scope selected_process, estado, referencias y preguntas/ausencias explícitas. No es historial completo ni formulación clínica sintetizada por frontend. Un snapshot nuevo puede sustituir conceptualmente al runtime anterior en el Project solo mediante acción humana; el export local anterior permanece inmutable.

## 13. Structured import

Pegar texto o cargar archivo UTF-8. Validación cliente mínima: no vacío, objeto JSON aparente y tamaño UTF-8 ≤ 1 MiB. Archivo demasiado grande/inválido descarta input anterior para evitar envío accidental de una propuesta previa. Lectura de archivo y envío tienen bloqueo de acción simultánea.

Se envía Blob JSON con el texto recibido, sin parse/stringify, reparación, eliminación de claves desconocidas o deduplicación. Claves duplicadas llegan al decoder estricto. Textarea puede normalizar saltos de línea como control de texto nativo; no se promete conservar bytes originales de una aplicación externa, pero sí enviar sin reinterpretación el texto del control/archivo UTF-8 cargado.

## 14. Validation/error UX

400/422: formato/campos/referencias exactas; 401: sesión; 403: autorización; 404: recurso no disponible para el paciente; 409: conflicto. No se muestra mensaje crudo de parser, DB ni proveedor.

El backend existente agrupa varios errores de validación en el mismo código; la UI informa categorías seguras, no identifica falsamente cuál campo falló ni afirma que un alias concreto exista. Sin fuzzy matching ni botón de aproximación. Para diagnósticos de campo más precisos haría falta un contrato de errores tipados futuro, no una extracción permisiva del error crudo.

## 15. Stale import

409 explica export posiblemente desactualizado, hash incompatible o reimportación duplicada según la semántica existente. No auto-retry ni auto-rebase. El terapeuta debe consultar el estado y generar otro export si cambió. Las pruebas backend de stale y reimportación sin artefactos se ejecutaron nuevamente; tests UI verifican error visible y ausencia de handoff exitoso.

## 16. External provenance

El JSON admite los campos opcionales existentes provider_name, model_name, surface y project_context_version (200 bytes por campo); la UI documenta cómo declararlos explícitamente dentro de provenance, sin reescribir automáticamente el sobre. No se creó un formulario de metadata que altere por detrás el texto original.

En Stage 3B, **Inspeccionar provenance externa** muestra la metadata como declarada por terapeuta/user-supplied, no verificada. Se conserva original externo completo, hash y export origen bajo detalles. No se crea un AIRun ficticio. Ejemplo sintético OpenAI / GPT-5.6 Sol / ChatGPT Project es una declaración de fixture, no una verificación de disponibilidad o una ejecución de ese modelo.

## 17. ClinicalDiff handoff

Import válido muestra `ClinicalDiff pending_review`, no cambios aplicados. CTA **Revisar propuesta clínica** abre el diff en Stage 3B. La UI nueva no ofrece decisión, aprobación ni merge; esas acciones conservan sus endpoints, permisos y versiones existentes. Test de red falla si Stage 3C intenta merge.

## 18. Export history

Nueva lista de recibos con generated_at, alias del proceso dentro del snapshot, state_version y hash. Página servidor de 25, `has_more`, offset validado y orden generated_at/UUID descendente. No se devuelve narrativa, artifact, snapshot interno, autor ni source mappings en el listado. Inspección individual exige consentimiento según configuración.

`process_1` es alias de cada snapshot, no identidad compartida entre exports; se aclara en pantalla. Para conocer el proceso detallado hay que inspeccionar su artifact autorizado. No estado `used`, edición ni borrado inventados. Offset no equivale a snapshot estable ante inserciones concurrentes.

## 19. Project package V1

Entregado `GPT_PROJECT_PACKAGE_V1.md`: cuatro capas (metodología general, runtime actual, fuentes seleccionadas, contrato de salida), actualización humana, recibo/hash, manifest y requisitos de salida. Diseño documental, no entidad DB ni paquete subido a un Project.

## 20. Source/version strategy

Skill general estable/versionada; runtime compacto actual; historia seleccionada intencionalmente; output schema versionado. Manifest incluye package version, export receipt/hash/state version, fuentes/versiones y output schema. No acumulación automática por sesión. Historia profunda adicional requiere futura selección/política; V1 no expande el alcance exportable para llenar la UI.

## 21. Skill compatibility audit

Se buscaron `clinical-microprocess-supervisor` y `clinical_microprocess_supervisor_SKILL.md` en el repositorio y catálogo local. Solo se encontraron referencias documentales históricas; el contenido requerido no estaba disponible. Tampoco se proporcionaron instrucciones activas generales de los Projects.

**UNKNOWN / NOT VERIFIED** para compatibilidad real. El documento de paquete clasifica REQUIRED (obtener fuente autorizada, comprobar runtime/refs/salida y ausencia de contradicciones), RECOMMENDED (ejemplo sintético, manifest y comprobación humana) y NOT NEEDED (API, sync, paciente dentro de metodología). No se presentan esas condiciones como defectos demostrados de una skill no leída. Esta falta bloquea el PASS integral de Stage 3C, no la implementación/verificación de la UI.

Se usaron OpenAI Docs únicamente para corroborar la organización general de Projects, y guías Next.js/React/Playwright para límites de componentes y aceptación. [Documentación oficial de Projects](https://learn.chatgpt.com/docs/projects). No se accedió a una cuenta ni fuente de paciente.

## 22. Browser acceptance

Playwright existente, Edge, Next build local, fixtures ficticios y API interceptada. No se afirma end-to-end browser→API real para estos tests; PostgreSQL/servicio/handler se prueban por separado.

| Flujo | Resultado | Evidencia |
| --- | --- | --- |
| A Export | PASS | Selección→generación→preview; sin red externa |
| B Privacy | PASS | Forbidden literals no reinsertados; minimizer backend prueba paciente y tercero |
| C Copy | PASS | String exacto a Clipboard API, estado copiado, sin red externa |
| D Download | PASS | Markdown byte-for-byte, schema/aliases, hash en nombre, sin UUID clínico |
| E Valid import | PASS | Texto original→pending_review→Stage 3B; cero merge |
| F Bad ref | PASS | Error seguro, sin fuzzy ni éxito falso; rechazo real en Go |
| G Stale export | PASS | 409, sin retry/rebase; persistencia real regresada |
| H Invalid schema | PASS | Error seguro; claves duplicadas pasan intactas al backend |
| I Read-only | PASS | Inspección autorizada; generar/importar deshabilitados |
| J External provenance | PASS | User-supplied, original externo y no AIRun |

Adicionalmente: consentimiento externo vs local, recuperación denegada después de preview, política servidor, tamaño UTF-8 y archivo excesivo que limpia input anterior. Captura móvil 390×844 sin overflow horizontal; inspección visual efectuada. No auditoría WCAG integral. Total Stage 3C: 10 tests, incluidos flujos combinados; regresión web completa: 39.

## 23. External network assertion

El spec intercepta toda solicitud: destinos distintos del entorno local permitido se registran/aborten y hacen fallar las aserciones. API inesperada también falla. Por ello openai.com, chatgpt.com, anthropic.com, googleapis.com u otro host externo no pueden pasar inadvertidos. Cada flujo verifica lista externa vacía. El test no demuestra ausencia universal de egress en toda configuración futura: demuestra la frontera de los flujos implementados y ejecutados.

## 24. Authorization

CA-W derivado de `can_write` servidor existente; no de admin. Export/import requieren treating según servicios existentes; lectura individual y provenance admiten treating/supervisor, y artifact exige consentimiento configurado. La lectura nueva de metadata requiere CA-R pero no concede acceso al contenido exportable.

Prueba HTTP handler+PostgreSQL del historial cubre treating, supervisor, sin asignación, tenant/paciente ajenos, offset inválido, páginas sin solapamiento y ausencia de metadata clínica oculta. Prueba recuperación sin consentimiento deniega aunque el recibo pueda listarse. La caché se mantiene aislada por tenant/usuario/paciente/ruta, con estado de bridge descartado al salir de sección/cambiar identidad.

## 25. W1–W30 matrix

PASS indica preservación en código y evidencia señalada, no necesariamente un test nuevo aislado para cada fila.

| Invariante | Resultado | Evidencia |
| --- | --- | --- |
| W1 Export no transmite | PASS | Flujo A + generación local existente |
| W2 Preview antes de transferencia | PASS | Botones deshabilitados hasta revisión explícita |
| W3 Consent externo distinto | PASS | Test scope/política; reautorización GET |
| W4 Default minimized | PASS | Contrato backend y UI fija |
| W5 Procesos ajenos excluidos | PASS | Selector/test determinista existente regresado |
| W6 UUIDs internos fuera del artifact | PASS | Test Go y B–D; recibo separado |
| W7 Refs estables en snapshot | PASS | Test export determinista |
| W8 Labels epistemológicos preservados | PASS | Artifact canónico/render Fields y Go |
| W9 Runtime no es historia completa | PASS | Scope y ausencias explícitas |
| W10 Export inmutable | PASS | Sin edición; repo/constraints existentes |
| W11 Copy no es send | PASS | C y red vacía |
| W12 Download no es sync | PASS | D, Blob local |
| W13 External input untrusted | PASS | Raw Blob y decoder estricto |
| W14 Unknown refs exact reject | PASS | Go unknown_ref + F UI |
| W15 No fuzzy matching | PASS | Sin normalización/resolución frontend |
| W16 Stale no importa silenciosamente | PASS | G + strict/stale backend |
| W17 Cross-tenant/patient blocked | PASS | PG bridge y nuevo historial; UI 404 seguro |
| W18 Import no muta estado | PASS | Triangulación PG, fingerprint antes de merge |
| W19 Solo pending ClinicalDiff | PASS | E + atomicidad backend existente |
| W20 External provenance no AIRun | PASS | J y conteo AIRun PG |
| W21 Metadata user-supplied | PASS | J, labels y contrato |
| W22 Original externo inmutable | PASS | Raw input no reescrito; original en provenance |
| W23 Review Stage 3B | PASS | CTA a inbox/detalle existente |
| W24 Human Merge requerido | PASS | Cero merge Stage 3C; triangulación PG |
| W25 No sync Project | PASS | Ninguna implementación/red externa |
| W26 No API IA externa | PASS | Assert de red y sin SDK/provider nuevo |
| W27 No GIRA automático | PASS | Configuración/core sin cambios |
| W28 Skills no modificadas | PASS | Ninguna escritura a skills; auditoría real pendiente |
| W29 Fuentes pacientes no modificadas | PASS | Sin acceso/cambio a Projects reales |
| W30 Runtime vs historia curada | PASS | Diseño GPT_PROJECT_PACKAGE_V1 |

## 26. Backend changes

Una única lectura nueva: `ProjectExports` en servicio/repositorio/handler y registro GET, con schema OpenAPI. Allowance demostrada: no había lista para recuperar exports existentes ni proyección de política para habilitar acciones sin adivinar configuración. No nuevo CRUD, provider, job, consentimiento paralelo, schema de import ni shortcut de merge. Tests de OpenAPI/rutas ampliados; fixture de PII incorpora tercero. Ningún cambio productivo al minimizer, compilador, decoder o transacción clínica.

## 27. Performance/size

Mediciones de fixture browser sintético, no latencia de red/DB productiva:

| Medición | Resultado |
| --- | ---: |
| Markdown canónico recibido | 1.500 bytes UTF-8 |
| Preview generate-click→visible, corrida final | 111 ms |
| Entrada exacta a Clipboard API | 1.500 bytes |
| Lectura portapapeles Windows CRLF | 1.566 bytes |
| Archivo descargado | 1.500 bytes |
| Máximo import | 1.048.576 bytes UTF-8 |
| Historial export | 25 recibos/página; PG probó 25 + 2 |

Selector backend existente limita evidencia a 24, eventos a 16, hipótesis a 16; candidatos de registro tienen límites existentes. No se expandió whole-history. Estos límites de entidades no equivalen a un techo universal de bytes para toda estrategia/texto: medir casos sintéticos grandes antes de producción si crece el contenido. No se añadió una truncación frontend que altere el artifact. Render de preview permite inspección completa; no es benchmark de carga ni P95.

## 28. Full regression

| Gate | Resultado |
| --- | --- |
| `npm run lint` | PASS |
| `npm run build` | PASS |
| `npm test` | 39 PASS / 0 FAIL |
| `go test -count=1 -json ./...` estándar | 315 PASS / 0 FAIL / 77 SKIP |
| Mismo comando `RUN_PG_INTEGRATION=1` | 377 PASS / 0 FAIL / 32 SKIP |
| `go vet ./...` | PASS |
| `go build ./cmd/server` | PASS |
| PostgreSQL manual triangulation / unsafe import | PASS |
| Stale, cross-tenant/client, No-AI-Direct-Merge, reverse provenance | PASS, suites bridge existentes reejecutadas |
| Stage 3B clinical workspace | PASS en web y PostgreSQL |
| OpenAPI/rutas/tenant query guard | PASS |
| `git diff --check` | PASS |
| Worker/Whisper soak | No reejecutado: ingestion/storage/jobs sin cambios Stage 3C |
| Ollama/remoto | No benchmarks ni inferencia ejecutados |

Conteos Go incluyen padres/subtests nombrados, excluyen eventos de paquete. Los perfiles se solapan: no sumar. SKIP son opt-ins no ejecutados, no PASS. PostgreSQL local sintético `stage2d_roundtrip` puerto 55435; sin modificar configuración de base productiva. Logs `%TEMP%/stage3c-go-default.jsonl`, `%TEMP%/stage3c-go-postgres.jsonl`.

La primera corrida browser falló por selectores ambiguos (announcer Next y texto en detalles) y normalización CRLF nativa. Se corrigieron los tests para medir sus objetivos precisos, conservando aserción exacta de entrada a Clipboard API; posteriormente se agregó cobertura de archivo excesivo y descarte de preview denegado. No se relajó validador ni contrato clínico.

## 29. Remaining gaps

### BLOCKING

Para PASS integral del pedido: falta copia autorizada/versionada de la skill clínica general e instrucciones generales pertinentes. Su auditoría de compatibilidad no puede suplirse con referencias históricas. No requiere compartir datos ni fuentes de pacientes reales.

### NON-BLOCKING

- Browser fixtures y pruebas DB son capas separadas; incorporar smoke completo API real + navegador en entorno sintético de CI antes de despliegue.
- Evaluar tamaño/render con estrategia grande; offset no es cursor estable ante inserciones.
- Historial muestra alias snapshot-scoped, no nombre clínico en metadata; inspeccionar individualmente para detalle.
- Motivos 400/409 se presentan en categorías seguras por el contrato actual, no diagnóstico fino inventado.
- Copia conserva contenido clínico pero Windows normaliza EOL. Hash canónico siempre es el provisto por backend.
- Accesibilidad básica móvil verificada; falta auditoría WCAG completa.

### SKILL/SOURCE INTEGRATION

Paquete, manifest y criterios entregados; instalación/edición de instrucciones y fuentes actuales no realizada ni autorizada automáticamente. Revisión específica pendiente al disponer del archivo general. Ninguna fuente histórica adicional debe agregarse indiscriminadamente.

### DEPLOYMENT

Persisten decisiones/configuración explícitas de producción de Stage 2D.1/3A/3B (consentimiento, retención, asignaciones, infraestructura/operación). Validar política de transferencia manual, ubicación/retención de copias y cuenta externa apropiada antes de uso real. Este reporte no constituye aprobación legal, clínica o de privacidad productiva.

## 30. Stage 3 final readiness

| Área | Estado |
| --- | --- |
| Therapist operational workflow | UI Stage 3A preservada y regresada |
| Clinical review workflow | Stage 3B preservado y regresado |
| Longitudinal workspace | Stage 3B preservado y regresado |
| Manual Project triangulation | Implementada y verificada con sintéticos; sin ejecutar Project real |
| Skill/source integration | NOT READY: auditoría de fuentes generales pendiente |
| Production configuration | NOT READY: políticas/configuración explícitas pendientes |

```text
STAGE 3 FINAL READINESS = NOT READY
```

Se detiene el trabajo después de este informe. No se inicia otra etapa, no se modifica ninguna skill ni fuente de paciente, no se sincroniza ChatGPT y no se activa GIRA remoto.
