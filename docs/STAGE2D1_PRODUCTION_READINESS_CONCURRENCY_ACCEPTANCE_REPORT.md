# STAGE 2D.1 — PRODUCTION READINESS & CONCURRENCY ACCEPTANCE REPORT

Fecha de cierre técnico: 2026-09-09. Entorno: Windows, Go, PostgreSQL 17 temporal, Ollama local `qwen3.5:9b`, faster-whisper local `large-v3`. Exclusivamente fixtures ficticios y TTS sintético. No se usaron pacientes, notas, reportes ni audio reales.

## 1. Verdict

```text
STAGE 2D = VERIFIED PASS
```

Aceptación del backend dentro del contrato soportado: ingestión local, audio acotado a 25 MiB, trabajos PostgreSQL, consentimiento explícito, revisión humana y bridge manual. No es una autorización de despliegue con datos reales ni una certificación de infraestructura de producción. Los bloqueos de deployment se separan en §27; UI en §28–29. Stage 3 NO se inició. GIRA remoto NO se activó.

El gate real HTTP/Whisper/Ollama se ejecutó nuevamente después del endurecimiento: PASS, 45,17 s. La suite completa con PostgreSQL y aceptación: 438 PASS / 0 FAIL / 10 SKIP. La suite ordinaria sin opt-ins: 313 PASS / 0 FAIL / 74 SKIP. Los conteos consideran eventos Go con nombre de test, incluyendo padres/subtests, y excluyen resultados de paquetes.

## 2. Stage 2D reported-gap review

Se leyó completamente `STAGE2D_CLINICAL_INGESTION_DURABLE_PROCESSING_ACCEPTANCE_REPORT.md`. No se adoptó su resumen como nueva evidencia.

| Pendiente original | Clasificación original | Clasificación actual y evidencia | ¿Bloquea backend? |
|---|---|---|---|
| HTTP E2E configurado | BLOCKING | Cerrado: socket HTTP real, Echo/middleware, PG, filesystem cifrado, Worker, Whisper y Ollama; §4 | No |
| Soak/multi-worker/crash | BLOCKING | Cerrado: cuatro instancias Worker, 600 trabajos, proceso hijo terminado, 50 trabajos concurrentes y reemplazo; §7–10 | No |
| Retención legal y borrado de transcript | BLOCKING producción | DEPLOYMENT/PRODUCT POLICY. Ya no existe default arbitrario de audio; habilitación exige tres decisiones explícitas; §19 | No, sí deployment |
| Streaming para audio mayor de 25 MiB | BLOCKING producción | Extensión de capacidad fuera del contrato actual. Se verifica buffering cifrado acotado y rechazo del exceso. No se promete grabación ilimitada | No dentro del límite |
| Scheduler durable y supervisión de procesos | BLOCKING producción | Cola y recuperación durables ya verificadas. El scheduler actual recorre tenants configurados; provisionar lista, réplicas y supervisor es DEPLOYMENT. No se necesita otra cola para conservar trabajo | No, sí deployment |
| Speaker/confidence enriquecidos | NON-BLOCKING | Mejora opcional de producto; timestamps y provenance actuales verificados | No |
| Dashboard de huérfanos | NON-BLOCKING | UI/operación futura. Backend ahora detecta inconsistencia con código seguro y conserva desconocidos para reconciliación; §15 | No |
| Paginación transcript/job | NON-BLOCKING | Deuda documentada. Lecturas actuales: artifacts hasta 200, transcripts hasta 100, job por ID; no se promete historial paginado completo | No |
| Consent UI y deletion UI | NON-BLOCKING y Stage 3 | STAGE 3; endpoints y contratos listos | No |
| Active Session, recording, transcript/report/diff review, procesos, Goals/GIRA, bridge, job UI | Stage 3; usadas antes para NOT READY | STAGE 3, evaluadas individualmente en §29 | No |

Correcciones explícitas al informe anterior:

- Sus «336 observed» mezclaban paquetes y tests. El baseline técnico nuevo fue 303 PASS / 0 FAIL / 50 SKIP, contado sin paquetes.
- La bajada anterior de migración 25 dejaba 24; no demostraba el recorrido completo hasta 16. Esta etapa ejecutó realmente 16 → 26 → 16 → 26 y comparó catálogos.
- Ollama live/review/A–G y web ya no se reportan como «baseline PASS»: se ejecutaron y se esperó su salida final.
- `artifact_cleanup` existe como tipo reservado, pero no es un job ejecutable del Worker. La limpieza efectiva es `Maintenance.RunBatch`, programada en el servidor; no hay enqueue público arbitrario.
- Las afirmaciones agrupadas D1–D30 se sustituyen por la matriz individual de §22.
- El antiguo default de audio 24 h y rango 1–168 h no se consideran una política aprobada: se eliminaron como defaults/restricción arbitraria.

## 3. Baseline

| Check inicial | Resultado |
|---|---|
| Git | Working tree preexistente sucio, 97 entradas en la vista inicial; preservado sin reset, checkout destructivo, limpieza ni commit |
| `go test -count=1 ./...` | 303 PASS / 0 FAIL / 50 SKIP, antes de este endurecimiento |
| `go vet ./...` | PASS |
| `go build ./cmd/server` | PASS |
| Última migración inicial | 25 |
| Última migración final | 26 |

Inventario por archivo: [STAGE2D1_GIT_INVENTORY.json](STAGE2D1_GIT_INVENTORY.json). `PREEXISTING` significa cambio anterior preservado; `MODIFIED` significa archivo preexistente editado por esta etapa, incluso si Git todavía lo muestra `??`; `NEW` significa creado en esta etapa. No se atribuye todo el diff contra HEAD a Stage 2D.1.

Cambios funcionales de esta etapa, todos derivados de aceptación/hardening:

1. Timing durable de transcript para reproducir el hash canónico; migración 26.
2. Compatibilidad del sampler de SessionReport con Ollama, con límites canónicos validados localmente y máximo un repair.
3. Lease revalidado con reloj PostgreSQL después de adquirir el row lock; FailAttempt usa estado bloqueado, no presupuesto recibido del caller.
4. Clasificación retryable acotada de indisponibilidad/busy del proveedor local.
5. Auditoría transaccional e idempotente de borrado completado; reconciliación de archivos y señal de huérfanos.
6. Configuración de retención explícita y contratos OpenAPI completos.
7. Corrección exclusivamente del fixture de latencia de una prueba remota preexistente; ningún provider remoto real fue llamado.

## 4. Full HTTP E2E

`TestStage2D1HTTPRealWhisperAndOllama`: PASS, ejecución final 45,17 s.

```text
seed tenant/user/client/assignment y JWT sintético
  → POST ClinicalSession (sin Appointment), completar por HTTP
  → POST grants separados por HTTP
  → POST audio TTS sintético
  → artifact cifrado available y hash
  → POST transcriptions → job queued
  → Worker real → sidecar Whisper real → TranscriptVersion v1
  → GET transcripts
  → POST analysis-jobs con transcript_version_id exacto
  → AIRun running persistido ANTES de inferencia
  → Ollama local real → validación estricta
  → transacción de AIRun succeeded + SessionReport draft + resultado de job
  → GET jobs / GET reports
```

El harness usa `httptest.NewServer` con puerto TCP real, router `NewServer`, `RequireTenant`, `RequireAuth`, roles y repositorios PG reales. No sustituye upload/enqueue/report con INSERTs. Sólo identidad/autenticación/asignación inicial se siembran directamente. Worker es llamado por el runner de aceptación porque reclamar trabajos no es un endpoint público.

Se verifican por respuestas HTTP y consultas de estado: tenant/client/session, Appointment opcional, autorización histórica, artifact/hash, versión y origen de transcript, identidad de artifact, hash canónico reproducible, configuración, estado de jobs, asociación attempt–AIRun–transcript–report, hashes input/context/output y estado final draft.

Al terminar: cero Evidence, Event, Process, Hypothesis y ClinicalDiff para ese cliente. No hay aprobación ni merge automático.

El proxy de diagnóstico del test sólo reenvía a Ollama loopback; no genera respuestas simuladas. Su diagnóstico se limita a errores de fixtures sintéticos y no se incorpora al logging de producción.

## 5. Negative HTTP E2E

`TestStage2D1NegativeHTTP`: PASS, incluyendo sus cinco casos.

| Caso | Resultado y estado |
|---|---|
| Audio sin AUDIO_RECORDING | HTTP 403, cero artifact y cero job |
| Audio legítimo, sin LOCAL_TRANSCRIPTION | HTTP 403, cero transcription job |
| Session de otro tenant | HTTP 404; sin filtración de fuente |
| Cliente sin asignación | HTTP 403 |
| `storage_path`/campo arbitrario | HTTP 400, comando estricto |
| Cancelación queued por endpoint | HTTP 200, job cancelled; Worker no lo reclama |

La matriz ampliada de §21 verifica otros 42 accesos y autenticación ausente. No se corrigió una denegación válida para favorecer un test: el contrato de consent responde 403 cuando no hay asignación, incluso para cliente extranjero; el test fue ajustado a esa semántica sin filtración.

## 6. Worker architecture

PostgreSQL es la cola durable. `Claim` utiliza transacción y `FOR UPDATE SKIP LOCKED`; cada intento obtiene token de lease propio y fila de attempts. El worker revisa consentimiento/acceso/fuente antes del proveedor; el commit vuelve a validar consentimiento, fuente, token y vigencia.

El servidor contiene un loop por proceso, tenants explícitos y maintenance por lotes. Múltiples instancias usan la misma cola sin coordinación en memoria. El soak utiliza cuatro instancias Worker concurrentes en un proceso; la prueba de muerte utiliza además un proceso OS separado y su reemplazo. No se afirma haber desplegado cuatro servidores completos en hosts distintos.

Timeout por intento configurado, limitado a 1–1500 s; lease de Worker = timeout + un minuto. La recuperación es acotada. No hay heartbeat que prolongue un intento indefinidamente. La ejecución tolera reintento después de una muerte; lo garantizado es exclusión durante lease vigente y una salida durable legítima, no «exactly once» físico tras cualquier falla.

Live/review siguen fuera de esta cola. GIRA sigue manual/humano. No se añadió Redis jobs, router inteligente, proveedor remoto ni merge automático.

## 7. Multi-worker concurrency

`TestStage2D1FourWorkerSoak`: PASS usando el Claim PG real.

| Métrica | Resultado final |
|---|---:|
| Workers simultáneos | 4 |
| Lote de transcription/analysis | 100 / 100 |
| Trabajos totales | 600 |
| Ejecuciones simultáneas del mismo job observadas | 0 |
| Salidas durables duplicadas | 0 |
| Stuck running al finalizar | 0 |
| TranscriptVersion / AIRun / SessionReport | 300 / 300 / 300 |

Se controla identidad activa por job en el transcriber ligero, estado persistido de todos los jobs, outcomes de attempts, filas de resultados y conteos exactos de salidas. Todos los analysis recorren el Worker real y crean AIRun antes del proveedor determinista.

## 8. Soak

Tres lotes, cada uno con 100 transcription jobs y 100 analysis jobs. Ejecución final con regresión paralela: 24,99 s. Es una prueba acotada de consistencia, no un benchmark de capacidad del hardware ni SLA.

```text
submitted                  600
succeeded                  600
failed                       0
retried                      0
duplicate executions         0
duplicate durable outputs    0
stuck running                0
lease recoveries             0
cancelled                    0
```

Estos números pertenecen al soak sano; retries/cancel/recovery se prueban en casos separados, no se omiten del denominador. Los proveedores del soak son deterministas ligeros. No se presenta esa carga como 300 invocaciones Whisper. Las invocaciones Whisper reales se informan en §4 y §18.

## 9. Worker death recovery

`TestStage2D1ProcessDeathAndRestart`: PASS.

Un ejecutable de test hijo abre su pool PG y reclama el trabajo. Se confirma el claim persistido, se mata ese proceso con `Process.Kill`, se deja intacta la fila running y otros cuatro workers procesan 50 trabajos. Se espera el vencimiento natural del lease de cinco segundos; un reemplazo recupera el trabajo, salvo que un sobreviviente ya lo haya recuperado legítimamente en un host lento.

Resultado del trabajo recuperado: attempt 2, una salida, una fila histórica `worker_lost`. No se reescriben timestamps/status por SQL para fabricar recuperación. Los 50 trabajos concurrentes no bloquean ni duplican la salida recuperada.

El mismo test comprueba luego reinicio de API/worker y produce 52 transcripts totales: 50 concurrentes + uno recuperado + uno queued antes del reinicio. Duplicados = 0.

## 10. Stale worker protection

`TestStage2D1NaturalLeaseExpiryRejectsStaleCompletion`: PASS. A reclama, vence naturalmente, B recupera y termina; `CompleteTranscription` de A devuelve conflicto. Resultado de B intacto, dos attempts, un transcript.

`TestStage2D1LeaseExpiresDuringRowLockWait`: primero FAIL reproducible, después PASS tres ejecuciones consecutivas y PASS en regresión final. Se sostiene un row lock competidor mientras vence el lease; el intento viejo no puede registrar un fallo/retry después de esperar.

Causa corregida: `NOW()` representa el inicio de la transacción; además un predicado puede evaluarse antes de esperar el row lock. La comprobación definitiva usa `clock_timestamp()` después de adquirir el lock. `FailAttempt` usa el attempt/max_attempts de la fila bloqueada y repite el fence al actualizar. Los resultados usan el mismo helper de bloqueo y guardas al commit.

## 11. Retry/idempotency races

| Test | Ejecución |
|---|---|
| `TestStage2D1ConcurrentHTTPIdempotency` | 24 POST concurrentes: un HTTP 202, 23 HTTP 200, mismo job ID, un transcript |
| `TestStage2D1RetryContention` | Fallo transient inicial + worker automático + 20 POST retry concurrentes: dos attempts, una salida, cero solapamiento |
| `TestStage2D1AnalysisFailureBoundaries/provider-retry-budget` | Proveedor local indisponible: tres attempts; después failed y retry HTTP 409 |

El retry manual adelanta un retry ya queued, no resetea presupuesto ni reabre terminales. Backoff: `5 × attempt²` segundos, con máximo tres attempts en enqueue público. La clave incorpora sesión, operación, fuente exacta, hash, configuración e idioma.

La indisponibilidad/busy local de Ollama ahora se clasifica como `provider_unavailable` retryable, con el mismo presupuesto. No se añade fallback. Códigos operacionales no contienen el error bruto del proveedor.

## 12. Cancellation races

Queued cancel: nunca se invoca el transcriber. Running cancel: el proveedor del fixture está bloqueado después de entrar, se cancela por HTTP y luego se libera su respuesta. La finalización tardía falla; cancelled no pasa a succeeded.

PASS para transcripción (`TestStage2D1RunningBoundaryRaces/cancel`) y análisis (`TestStage2D1AnalysisFailureBoundaries/cancel`). En análisis, el AIRun previamente running termina y no aparece SessionReport.

La cancelación es cooperativa a través del contexto donde aplica y, siempre, tiene fence durable. No se promete matar inmediatamente un cómputo Whisper/Ollama ya iniciado por cancelar un job: puede terminar localmente, pero su resultado no se publica.

## 13. Consent revocation races

PASS queued y running mediante endpoints. Grants distintos para audio, transcripción, IA local y bridge externo manual; autorización de un scope nunca sustituye a otro.

Semántica escogida y verificada: revocación prospectiva cancela jobs covered queued/running, invalida el lease y termina AIRuns asociados. Una operación local ya iniciada puede consumir cómputo hasta regresar; el commit se rechaza. No se inicia análisis posterior bajo consentimiento revocado. La revocación no borra retroactivamente registros legítimos: deletion/retention tienen lifecycle separado.

Evidencia: `TestConsentScopesHistoryRevocationAndIsolationPostgres`, queued/running revoke y analysis/revoke. La autorización histórica conserva grant y definición v1 después de revoke/regrant.

## 14. Deletion races

PASS para audio queued y running; el endpoint cambia a deleting, cancela trabajos de esa fuente, elimina el archivo y completa deleted. El worker tardío no crea transcript contra audio borrado.

PASS para borrar transcript durante análisis: se conserva tombstone, se termina AIRun y no se publica reporte. Borrar audio no destruye automáticamente transcript previamente válido ni registros clínicos derivados; se conserva provenance.

La transición física/DB no pretende ser una transacción distribuida: usa `deleting` durable y compensación/maintenance. `clinical_audio.deleted` se escribe transaccionalmente con el tombstone una sola vez, con actor de sistema; el actor humano solicitante permanece en `deletion_requested`.

## 15. File/DB consistency

Después de cada lote y de las carreras se escanea el storage sintético y se cruza con metadata.

| Categoría al terminar el soak normal | Conteo |
|---|---:|
| Artifact available sin archivo | 0 |
| Archivo sin artifact | 0 |
| Temporal residual | 0 |
| Artifact deleted con archivo permanente | 0 |

Pruebas explícitas adicionales PASS:

- Fallo de UPDATE de finalización inyectado mediante trigger exclusivo del tenant ficticio: el POST real reserva, cifra/escribe y compensa; queda tombstone sin archivo. El trigger se elimina al terminar.
- Corte entre borrado físico y completion DB: `Maintenance` completa el tombstone; repetir completion no duplica auditoría.
- Archivo disponible desaparecido: estado missing y cancelación de job asociado; borrado posterior consistente.
- Retención vencida naturalmente: archivo eliminado y metadata deleted.
- Temporal viejo: eliminación acotada; el fixture de mtime es una inyección de fallo, no evidencia de esperar 24 horas reales.
- Archivo que reaparece con tombstone deleted exacto: reconciliado.
- Archivo final deliberadamente creado fuera del protocolo metadata-first: devuelve `artifact_storage_inconsistent`, permanece intacto para investigación. El test verifica esa alarma y luego retira explícitamente SU archivo ficticio; no atribuye esa retirada a recuperación automática.

Maintenance procesa metadata 50 / filesystem 100 por lote. Los cursores avanzan incluso tras error aislado y vuelven al inicio, evitando que un huérfano impida atender otros artifacts. Desconocidos no se destruyen silenciosamente. La reconciliación operacional es: inventariar sólo IDs, comprobar backup/metadata/ownership y decidir recuperación o eliminación autorizada; nunca borrar toda la raíz.

## 16. Encryption validation

PASS: `TestEncryptedArtifactIntegrityIsolationAndDelete`, `TestArtifactBoundedWriteCancellationAndKeyRequirements`, `TestEncryptedArtifactWrongKeyTamperTruncation` y sus tres subtests.

AES-256-GCM, nonce aleatorio, AAD con clave opaque y key ID. Se verifica roundtrip autorizado y que bytes finales no sean plaintext; claves equivocadas con el MISMO key ID, ciphertext alterado y truncado fallan cerrados sin devolver audio. Paths arbitrarios y claves inválidas son rechazados.

Límites reales de la afirmación:

- El upload y la lectura descifrada se mantienen en memoria, acotados. El temporal publicado contiene ciphertext, no audio en claro.
- Whisper recibe `io.BytesIO(audio)`; el directorio temporal de esta ejecución sólo contuvo logs operacionales, no archivos de audio/decrypt.
- El WAV TTS de prueba permanece deliberadamente como fixture sintético fuera del storage de aplicación. No es un archivo clínico real.
- No se certifica ausencia de plaintext en RAM, swap, crash dumps o backup. ACL de Windows, cifrado de volumen/backup y gestión de claves son deployment.
- PostgreSQL contiene texto de transcript por diseño. El cifrado de artifacts NO significa que toda la base esté cifrada por la aplicación.
- Un único key ID activo no constituye rotación online multi-key. No debe cambiarse la clave mientras existan artifacts que dependan de ella sin un procedimiento de migración/retención verificado.

## 17. Process restart durability

PASS en `TestStage2D1ProcessDeathAndRestart`. Se encola mediante HTTP, se cierra y reconstruye router/listener Echo, se inicia un nuevo proceso worker con otro pool/conexión y se completa el trabajo persistido.

El worker sí es un proceso OS real; el reinicio de API en el harness es del servidor/listener dentro del runner, no un reinicio de la máquina ni una prueba de balanceador. Ninguna cola se reconstruye en memoria y no se pierde el trabajo queued. Supervisión del ejecutable desplegado es tarea de operación, §27.

## 18. Faster-whisper outage/recovery

`TestStage2D1RealWhisperOutageRecovery`: PASS final, 26,45 s. El Worker real intenta conectar a un puerto loopback no disponible: durable retry `sidecar_unavailable`. Se restaura el mismo endpoint mediante proxy hacia el sidecar Whisper real, se adelanta el retry por HTTP y se genera exactamente un TranscriptVersion en attempt 2. No se reemplaza la segunda respuesta por un fixture preconstruido.

`TestFasterWhisperDurableSyntheticIntegration`: PASS, tres segmentos, `faster-whisper-large-v3`, 19,23 s. También pasa la matriz de contrato local: JSON inválido/parcial, campos desconocidos, trailing JSON, request/config incorrectos, busy, crash, conexión rechazada, timeout/cancel y redirecciones.

Sólo audio TTS ficticio. No se descargó otro modelo ni se activó un servicio remoto.

## 19. Retention capability

### TECHNICAL CAPABILITY

| Dato | Capacidad verificada |
|---|---|
| Audio raw | `CLINICAL_AUDIO_RETENTION_HOURS` positivo explícito, duración técnica representable, fecha por artifact, borrado manual/maintenance |
| Transcript | Política explícita `CLINICAL_TRANSCRIPT_RETENTION_POLICY=explicit_deletion`; borrado de contenido con tombstone/versiones y bloqueo de análisis |
| Job metadata | `CLINICAL_JOB_METADATA_RETENTION_POLICY=retain_provenance` explícito; attempts/identidades/hashes conservados, sin narrativa |
| Report/registro aprobado derivado | Lifecycle clínico separado; borrar raw o revocar no lo elimina implícitamente |

No se incluye TTL automático para transcripts ni purga automática de provenance. Si producto requiere otra política, deberá especificarse antes de deployment. Los valores de prueba (incluido un segundo en el fixture de maintenance y 300 horas en test de rango) son exclusivamente sintéticos.

### LEGAL/PRODUCT POLICY

```text
PRODUCTION RETENTION POLICY = UNCONFIGURED / DEPLOYMENT BLOCKED
```

Default audio = 0/no configurado; políticas transcript/job vacías. Habilitar ingestión sin políticas explícitas falla. `.env.example` documenta nombres sin escoger un plazo legal ni introducir secretos. Las 24 h para temporales cifrados son una ventana técnica de limpieza, no una política legal de retención de audio.

## 20. Manual Project bridge regression

PASS reejecutado:

- `TestManualProjectTriangulationNoDirectMergeAndReverseProvenancePostgres`.
- `TestManualProjectUnsafeImportCreatesNoArtifactsPostgres`.
- Tests de export determinista/scoped, import estricto/stale/unsafe, PII sintético, acceso HTTP y referencias OpenAPI.
- `TestManualBridgeDoesNotRequireAPIKeyAndRemoteIsDormant`.

El bridge funciona sin API key ni audio/transcription job. La autorización externa manual es un scope separado cuando está configurada. Export/import siguen mediados por persona; no hay auto-approve, direct merge ni dependencia de OpenAI API para uso normal. Las pruebas `remotegira` ordinarias usan HTTP simulado local; la certificación remota opt-in permanece SKIP.

## 21. Tenant/client attack matrix

`TestStage2D1HTTPIsolationMatrix`: 14 comandos × tres contextos = 42 casos PASS, más falta de JWT → 401. Los comandos cubren listar artifacts/transcripts/consents, upload, enqueue transcription/analysis, leer/borrar/corregir transcript, borrar artifact y leer/retry/cancel job.

| Contexto | Lecturas | Escrituras | Mutación causada |
|---|---|---|---|
| Tenant extranjero | 404; consent 403 | 404; consent 403 | Ninguna |
| Mismo tenant, actor sin asignación al cliente | 403 | 403 | Ninguna |
| Supervisor asignado | 200 | 403 | Ninguna |
| Treating asignado | Pipeline legítimo 200 | 201/202 según operación | Sólo operaciones autorizadas |
| JWT ausente | 401 | Middleware obligatorio | Ninguna |

Complemento PG: FKs tenant/client/session, grants extranjeros, transcript extranjero, source foreign y asignaciones activas; todos PASS en la suite completa. `TestRepositoryQueriesRemainTenantAware`: PASS. El handler no expone storage_key, key ID secreto ni lease token.

## 22. D1–D30

Todas las filas siguientes corresponden a ejecución en esta etapa, no «previous PASS». Los tests compartidos prueban varios invariantes; no se infla ese hecho como 30 suites independientes.

| ID | Invariante | Evidencia reejecutada | Resultado |
|---|---|---|---|
| D1 | Appointment != ClinicalSession | HTTP harness crea sesión sin Appointment; lifecycle PG | PASS |
| D2 | ClinicalSession raíz durable | HTTP pipeline, IDs session en artifact/job/transcript/run | PASS |
| D3 | Scopes distintos | `TestConsentScopesHistoryRevocationAndIsolationPostgres` | PASS |
| D4 | Local AI no implica audio | Mismo test autoriza LocalAI y rechaza Audio | PASS |
| D5 | Audio no implica transcripción | `TestStage2D1NegativeHTTP/no-transcription-consent` | PASS |
| D6 | Local no implica external manual | Consent scopes PG + bridge config | PASS |
| D7 | Consent histórico reconstruible | Grant/revoke/regrant conserva autorización original v1 | PASS |
| D8 | Revocado bloquea procesamiento futuro | Queued revoke + consent PG | PASS |
| D9 | Worker revalida consentimiento | Running revoke; analysis/revoke; commit PG | PASS |
| D10 | Transcript no es hecho clínico | Pipeline comprueba cero entidades longitudinales | PASS |
| D11 | Machine distinto de corrección humana | `TestIngestionDurablePipelineAndRevocationPostgres`, v1/v2/origin | PASS |
| D12 | Revisión no destruye historia | Mismo test relee texto original y parent/version | PASS |
| D13 | Audio en tenant/client/session exactos | HTTP pipeline + aislamiento + FKs PG | PASS |
| D14 | Raw local | E2E loopback/storage; tests de rechazo de endpoints remotos | PASS |
| D15 | Job sin narrativa | Pipeline con comandos sólo IDs; Job/SQL tipados sin payload clínico, audit de failure boundaries | PASS |
| D16 | Claim impide ejecución concurrente duplicada | Cuatro workers/600 jobs, attempts exactos | PASS |
| D17 | Retry acotado | Analysis/provider-retry-budget: tres attempts y terminal | PASS |
| D18 | Retry conserva idempotencia | 24 requests y 20 retries concurrentes | PASS |
| D19 | Cancelled no puede terminar succeeded | Running cancel transcription y analysis | PASS |
| D20 | Crash recuperable | Proceso OS muerto, lease natural, reemplazo bajo concurrencia | PASS |
| D21 | Analysis refiere transcript exacto | Pipeline HTTP y corrección v2 en pipeline PG | PASS |
| D22 | AIRun previo a inferencia | Provider observa running persistido; analysis failure boundaries | PASS |
| D23 | Provenance de SessionReport exacta | Join run-attempt-transcript-report + hashes + timing durable | PASS |
| D24 | No auto-approve de reporte | GET reports retorna draft tras Whisper/Ollama real | PASS |
| D25 | No conocimiento longitudinal aprobado automático | Cero Evidence/Event/Process/Hypothesis/Diff del pipeline | PASS |
| D26 | Borrado sin raw no administrado | Carreras + compensación + maintenance + scanner | PASS |
| D27 | Audit/logs sin contenido clínico crudo | Analysis failure metadata, API/store/provider errores seguros; inspección de logging sin cuerpos | PASS |
| D28 | No resolución cross-tenant/client | Matriz HTTP 42 casos + FKs/repositorios PG | PASS |
| D29 | Bridge mediado por humano | Triangulation/no direct merge/unsafe import PG | PASS |
| D30 | No remoto AI/GIRA activado | Config dormant, tests sin key; suites remotas reales no habilitadas | PASS |

## 23. Migration roundtrip

PASS real: `16 → 26 → 16 → 26` en base temporal nueva `stage2d1_roundtrip_d872ebdef73b43ea93ee441ea7ffcedd`, puerto 55435. No se tocó la base de desarrollo/producto.

`tools/acceptance/stage2d1_migration_roundtrip.ps1` crea una base aislada con nombre validado y rechaza una existente; cada SQL se ejecuta con `ON_ERROR_STOP` y transacción. Baja TODAS las migraciones 26…17. Compara relaciones/índices, funciones, triggers, constraints, columnas/tipos y tipos de catálogo contra baseline16: cero objetos residuales. Luego reaplica 17…26. Las bases sintéticas se conservan, no se borran indiscriminadamente.

La migración nueva 26 persiste `duration_seconds` y `processing_seconds`, ya incluidos por el transcriber en el hash de contenido. El test de roundtrip del hash falló antes y pasó después. Valores históricos ausentes quedan NULL: NO se inventa timing ni se reescribe un hash histórico. Un transcript antiguo que perdió timing no puede reconstruir retrospectivamente ese dato; no se certifica reparación de esos registros. `DOWN` elimina sólo las dos columnas nuevas. Las migraciones 1–25 no fueron reescritas por esta etapa.

## 24. Test traceability

Raíces de código: `apps/api/internal/acceptance/ingestion`, `internal/usecase/ingestion`, `internal/infra/db`, `internal/infra/artifactstore`, `internal/infra/ollama`, `internal/infra/transcriber`.

| Requisito | Código | DB/estado observado | Test ejecutado | Resultado |
|---|---|---|---|---|
| HTTP durable completo | router/handlers + service/worker | consent→artifact→job→transcript→run→draft | HTTPRealWhisperAndOllama | PASS |
| Concurrencia | Claim / Worker | 600 succeeded, attempts/salidas exactas | FourWorkerSoak | PASS |
| Crash/restart | RecoverExpired + subprocess harness | worker_lost, attempt2, una salida | ProcessDeathAndRestart | PASS |
| Stale completion | lock/completeIngestionAttemptTx | viejo token no publica | NaturalLeaseExpiryRejectsStaleCompletion | PASS |
| Lease durante espera | post-lock clock_timestamp + FailAttempt | expirado no modifica estado | LeaseExpiresDuringRowLockWait | FAIL→fix→PASS ×3 y final |
| Idempotencia/retry | Enqueue / RetryJob | mismo job; dos attempts, una salida | ConcurrentHTTPIdempotency / RetryContention | PASS |
| Revocación/cancel/deletion | consent locks + source/token fences | cancelled, run terminal, sin salida | RunningBoundaryRaces / AnalysisFailureBoundaries | PASS |
| File/DB parcial | Upload compensation / Maintenance | tombstone, missing, sin residual normal | StoragePartialFailureAndRecovery | PASS |
| Retención/inventario | Config / Maintenance / Scan | expiración, temp cleanup, orphan alarm | StorageInventoryAndRetention + config | PASS |
| Cifrado real | AES-GCM store | wrong key/tamper/truncation rechazados | EncryptedArtifactWrongKeyTamperTruncation | PASS |
| Contrato SessionReport | sampler adapter + validación original | sin cambio de schema canónico; longitud validada | SessionReportSamplerCompatibilityPreservesAcceptedContract | PASS |
| Provenance reproducible | migration26 / scan+insertTranscript | contenido rehidratado reproduce SHA | HTTPIngestion / HTTPRealWhisperAndOllama | PASS |
| OpenAPI completo | docs/openapi.yaml | 16 operaciones consent/ingestion, cuerpos/respuestas/auth | IngestionOpenAPICompleteContracts + referencias | PASS |
| Bridge | project bridge + human merge | import pending, merge sólo humano, provenance | ManualProjectTriangulation… / UnsafeImport… | PASS |

Errores descubiertos y no ocultados: grammar HTTP400 de Ollama por expansión de `maxLength`, pérdida de timing en persistencia, lease expirado durante lock wait y prueba simulada de latencia cero. Todos fueron corregidos con reejecución. El sampler omite únicamente límites de longitud en su copia de schema para generación; el schema original y sus límites se aplican al resultado en Go, incluso en repair. No se modificaron `clinical-live-v1`, `clinical-review-v1`, prompt longitudinal, GIRA ni skills/GPT sources.

Reproducción (desde `apps/api`, con PostgreSQL temporal migrado y fixtures sintéticos; nunca apuntar estas pruebas a producto):

```powershell
go test -count=1 ./...
go vet ./...
go build ./cmd/server

$env:RUN_PG_INTEGRATION='1'
$env:RUN_STAGE2D1_ACCEPTANCE='1'
$env:DATABASE_URL='<URL de base PostgreSQL temporal con migraciones hasta 26>'
go test -count=1 -timeout 15m ./...

# Sólo después de preparar el sidecar/modelo local y WAV TTS ficticio:
$env:RUN_STAGE2D1_REAL='1'
$env:SYNTHETIC_TRANSCRIBER_URL='http://127.0.0.1:8092'
$env:SYNTHETIC_TRANSCRIPTION_AUDIO='<WAV sintético>'
$env:SYNTHETIC_TRANSCRIPTION_CONFIGURATION_HASH=(Invoke-RestMethod "$env:SYNTHETIC_TRANSCRIBER_URL/health").configuration_hash
go test -count=1 -v -timeout 10m ./internal/acceptance/ingestion -run '^TestStage2D1(HTTPRealWhisperAndOllama|RealWhisperOutageRecovery)$'
```

Ollama local se reejecutó con `RUN_OLLAMA_INTEGRATION=1`, `RUN_CLINICAL_REVIEW_INTEGRATION=1` y `RUN_OLLAMA_LONGITUDINAL_EVAL=1`, seleccionando únicamente esos tres tests. No se ejecutaron benchmarks GIRA locales/remotos. Web: `npm run lint` y `npm run build` desde `apps/web`. Migraciones: ejecutar el script aislado desde la raíz.

## 25. Full regression

| Gate final | Resultado |
|---|---|
| GO TEST ordinario | PASS: 313 / FAIL 0 / SKIP 74 |
| GO TEST con PG + aceptación | PASS: 438 / FAIL 0 / SKIP 10 |
| GO VET | PASS, salida 0 |
| GO BUILD API | PASS, salida 0 |
| POSTGRES | PASS: 58 tests/subtests en paquete db; también HTTP PG |
| HTTP E2E / aceptación | PASS: 71 tests/subtests en paquete acceptance con PG; tres opt-ins/subprocess omitidos en esa invocación |
| HTTP E2E real separado | PASS, 45,17 s |
| MULTI-WORKER / SOAK | PASS: cuatro workers, 600 jobs, cero duplicados/stuck, 24,99 s |
| WORKER RECOVERY / RESTART | PASS: proceso muerto + 50 jobs concurrentes; 52 outputs finales |
| CANCELLATION / REVOKE / DELETE | PASS queued/running/transcript/analysis |
| FASTER-WHISPER | PASS real directo 19,23 s; outage/recovery 26,45 s |
| ENCRYPTED STORAGE | PASS integridad, clave equivocada, tamper, truncación, compensaciones |
| OLLAMA LIVE | PASS real 35,61 s, sin repair |
| OLLAMA REVIEW | PASS real 63,24 s, sin repair |
| OLLAMA LONGITUDINAL | PASS real A–G 7/7, 293,89 s |
| MANUAL PROJECT BRIDGE | PASS reejecutado unit/HTTP/PG |
| WEB LINT | PASS, salida 0 |
| WEB BUILD | PASS, salida 0; Next.js 16.1.6, generación 13/13 |
| OPENAPI | PASS parser estricto, refs y 16 operaciones documentadas |
| TENANT | PASS query guard + PG + matriz HTTP |
| MIGRATION | PASS 16→26→16→26, cero objetos residuales |
| GIT | `git diff --check` PASS; sin reset, delete de código previo ni commit |

Los skips no se presentan como pruebas ejecutadas. En la suite con PG quedan diez opt-ins: evaluaciones de modelos/sidecar y helper subprocess. Los requeridos Whisper y Ollama se ejecutaron por separado; certificación remota GIRA y evaluación GIRA local deliberadamente no se habilitaron. No se ejecutó Go race detector: no había compilador C `gcc` disponible; la aceptación de concurrencia se apoya en las pruebas reales PG/worker y sus assertions, no en afirmar un `-race` inexistente.

La prueba preexistente `TestOpenAIProviderAllowsExactlyOneRepair` fue intermitente en la primera regresión ordinaria. Se reprodujo con 30 repeticiones; el fixture de latencia explícita se corrigió y pasó 100/100. No se cambiaron reglas de repair, modelo, contrato ni código productivo remoto para obtener PASS.

## 26. Remaining technical blockers

Ninguno abierto para los gates técnicos de Stage 2D dentro del contrato acotado descrito. No se confunde esta conclusión con pruebas de HA multi-host, caída eléctrica, corrupción de disco, despliegue productivo o certificación legal.

Deuda no bloqueante: paginación ampliada, dashboard de inventario, métricas segmentales si producto las requiere, streaming para archivos mayores y futuras herramientas de rotación multi-key. La pérdida histórica de timing previa a migración26 no se rellena con datos inventados; cualquier despliegue que ya contenga esos registros debe inventariarlos explícitamente.

## 27. Deployment blockers

Antes de habilitar datos reales:

1. Aprobar plazo de audio y políticas de transcript/provenance; configurar las tres variables explícitas. No existe autorización legal implícita en este informe.
2. Provisionar raíz local persistente y compartida entre réplicas del mismo host, ACL restrictivas, claves/backup/volumen cifrado. No rotar key ID/material dejando artifacts ilegibles; definir procedimiento y recuperación de backup.
3. Aplicar migraciones mediante el procedimiento de deployment, con respaldo; no ejecutar el script de aceptación contra una base existente.
4. Supervisar API/worker/sidecar, configurar todos los tenants atendidos en `CLINICAL_INGESTION_WORKER_TENANTS`, monitorizar health/errores seguros y efectuar reconciliación de inventario. Lista vacía equivale a ningún tenant atendido; no es una configuración productiva completa.
5. Fijar y verificar modelo/config hash de Whisper, Ollama local, límites/timeouts, capacidad RAM y acceso sólo loopback al sidecar. Los 25 MiB son límite, no promesa de sesiones sin límite.
6. Establecer política de logs/backups/swap y respuesta a archivos missing/desconocidos. Un archivo sin metadata se investiga; no se elimina automáticamente como supuesto «huérfano seguro».

No hay workflow nuevo de consentimiento remoto automatizado ni activación de GIRA remoto. Las políticas aquí exigidas son configuración/operación/producto, no una razón para mantener el backend Stage 2D en PARTIAL.

## 28. Stage 3 items

Recording UI, consentimiento UI, transcript review, job status/retry/cancel, report review, ClinicalDiff review, procesos/hipótesis, Goals/GIRA, bridge manual y controles de retención/borrado pertenecen a Stage 3. Ninguna pantalla nueva fue implementada en esta etapa. El build web verifica no regresión de la aplicación existente, no existencia de esas futuras pantallas.

## 29. Stage 3 readiness

```text
STAGE 3 BACKEND READINESS = READY
PRODUCTION DEPLOYMENT = BLOCKED UNTIL EXPLICIT CONFIGURATION/POLICY
```

| Área | Backend para iniciar Stage 3 | Trabajo de Stage 3 / condición |
|---|---|---|
| Active Session UI | READY | Integrar lifecycle ClinicalSession y accesos existentes |
| Consent UI | READY | Mostrar scopes separados, grants versionados, revoke e impacto |
| Recording controls | READY dentro de límite | Captura local/upload binario, límite 25 MiB, consentimiento; no grabación ilimitada |
| Transcript review | READY | Listar/leer, corregir creando versión, mostrar origen y provenance |
| Job status/retry | READY | Poll por ID, estados, retry acotado y cancel; no prometer reabrir terminal |
| Report review | READY | Draft/versión/aprobación humana; no auto-approve |
| ClinicalDiff review | READY | Decisiones, stale/version, merge humano transaccional |
| Processes/hypotheses | READY | Proyecciones clínicas existentes, no CRUD directo |
| Goals/GIRA | READY, manual | Estrategia/lecturas/merge existentes; no proveedor automatizado certificado activo |
| Manual GPT Project export/import | READY | Descarga/subida mediada por humano y consentimiento externo independiente |
| Retention/deletion controls | READY, política pendiente | UI de metadata/tombstone/borrado; no escoger duración legal desde UI por defecto |

Se detiene el trabajo tras este informe. No se inicia Stage 3, no se alteran skills ni fuentes GPT Project, no se envía información clínica real y no se habilita procesamiento remoto automatizado.

Cierre del entorno de aceptación: se detuvo únicamente el sidecar sintético iniciado para esta etapa (PID 23072, identidad/puerto/directorio comprobados). Ollama preexistente y PostgreSQL temporal se dejaron intactos. Las bases y el WAV TTS ficticios se conservaron; no hubo limpieza destructiva del workspace. Inventario final: 94 PREEXISTING, 16 MODIFIED y 20 NEW (130 archivos en la vista expandida, no comparable directamente con las 97 entradas agrupadas del baseline).
