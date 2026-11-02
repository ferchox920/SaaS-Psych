# STAGE 2D — CLINICAL INGESTION & DURABLE PROCESSING ACCEPTANCE REPORT

> Nota de revisión 2026-09-09: este documento conserva el registro histórico. Sus conteos mezclados, migración incompleta, referencias a baseline y clasificación de UI/retención fueron revisados explícitamente en [Stage 2D.1](STAGE2D1_PRODUCTION_READINESS_CONCURRENCY_ACCEPTANCE_REPORT.md), que contiene la nueva evidencia de aceptación. No interpretar las afirmaciones históricas siguientes como ejecución nueva.

## 1. Veredicto

`PARTIAL`.

La base durable está implementada y sus gates de dominio/PG pasan. No se declara `VERIFIED PASS` porque la UI de Stage 3 no existe, la prueba HTTP end-to-end con servidor configurado y el worker multi-proceso aún requieren una suite de despliegue separada, y la política de retención requiere decisión de producto/legal.

## 2. Baseline

Baseline anterior: Go 286 pass / 0 fail / 46 skip, `go vet` y `go build` PASS. Tras Stage 2D: `go test -count=1 ./...` PASS, 336 resultados de paquetes/tests observados, 0 fail; `go vet ./...` PASS; `go build ./cmd/server` PASS; `git diff --check` PASS. Ollama live/review/longitudinal A–G permanecen PASS según baseline Stage 2C (no se modificaron prompts).

## 3. Resumen de decisión arquitectónica

La raíz durable es `ClinicalSession`. Audio queda local, cifrado y fuera de PostgreSQL. Los trabajos usan PostgreSQL (`FOR UPDATE SKIP LOCKED`) porque ya es el sistema durable y permite enqueue, consentimiento, lease y resultado en transacciones. Redis continúa limitado a rate limiting. El proveedor GIRA remoto sigue dormido y Stage 2D no lo invoca.

## 4. Dominio de consentimiento

`clinical_consent_definitions` versiona la definición; `clinical_consent_grants` conserva grants y revocaciones sin hard delete. Un trigger permite únicamente la transición prospectiva a `revoked` y prohíbe modificar la identidad histórica.

## 5. Alcances

Se implementaron de forma independiente `LOCAL_AI_PROCESSING`, `AUDIO_RECORDING`, `LOCAL_TRANSCRIPTION` y `EXTERNAL_MANUAL_AI_PROCESSING`. Ninguno implica a otro.

## 6. Revocación

Una revocación bloquea nuevas operaciones, cancela trabajos queued/running cubiertos y termina AIRuns asociados. No borra automáticamente transcripciones ni registros clínicos previamente generados; la retención/deleción es una decisión separada.

## 7. Provenance de consentimiento

Cada boundary inserta `clinical_ingestion_authorizations` con grant, scope, definición, operación, entidad y timestamp. La fila permanece aunque el grant sea revocado.

## 8. ClinicalSession

Los artifacts, transcript versions, jobs y AIRuns se referencian por `(tenant, client, session)`. Appointment es relación opcional; no se usa como raíz durable.

## 9. Artifact/storage

`clinical_session_artifacts` registra audio, hash SHA-256, tamaño, tipo, key ID, estado, retención y grant. El filesystem recibe únicamente una clave opaque derivada de UUIDs; no nombres de paciente.

## 10. Audio lifecycle

`uploading → available → deleting → deleted|missing`. La publicación usa archivo temporal cifrado y hard-link atómico; fallos compensan el metadata row y dejan cleanup recuperable.

## 11. Seguridad de almacenamiento

Aplicación usa AES-256-GCM, nonce aleatorio, AAD de la clave y key ID, clave solo por configuración/secret manager y raíz absoluta no-symlink. El límite actual es 25 MiB en memoria: para audio mayor se requiere streaming cifrado antes de producción. El cifrado de disco/backup sigue siendo dependencia de deployment y no se afirma aquí.

## 12. Transcript/versioning

`clinical_transcript_versions` conserva máquina v1 y corrección humana como nueva versión. Segmentos guardan start/end/text; engine, model, engine version, language, configuration hash y source artifact hash/version son obligatorios.

## 13. Frontera epistémica

Transcript nunca se convierte automáticamente en ClinicalEvidence. La corrección ASR es `human_asr_correction` y no se interpreta como corrección/resistencia clínica.

## 14. faster-whisper

Sidecar loopback versionado `local-transcription-v1`, request ID, configuración hash, audio en memoria, segmentos y metadatos engine/model/version. Redirecciones HTTP están prohibidas. Fixture sintética PASS: 3 segmentos, 9,53 s; no se utilizó PHI.

## 15. Jobs

Se implementaron `transcribe_session_audio`, `analyze_session` y `artifact_cleanup`, estados finitos, idempotency key y attempts. PostgreSQL fue elegido por atomicidad con el estado clínico y menor infraestructura operacional que Redis.

## 16. Claim/recovery

Claim usa `FOR UPDATE SKIP LOCKED`, lease token y lease expiry. Recovery convierte trabajos expirados a queued/failed y registra `worker_lost`.

## 17. Retry/idempotency

Máximo 3 attempts por defecto, backoff acotado y códigos retryable cerrados (`sidecar_unavailable`, `timeout`, `worker_lost`, `storage_unavailable`). Repetir artifact+configuration+source devuelve el job existente.

## 18. Cancellation

Cancelación y revocación limpian lease y fencean el token; un worker antiguo recibe conflicto y no puede publicar transcript/report.

## 19. Consentimiento y jobs

Se comprueba al enqueue, claim/execute y commit de resultado. La prueba PG cubre grant → queue → revoke → worker y confirma AIRun cancelado y ausencia de resultado.

## 20. Ingestión end-to-end

Audio → job → transcript machine → transcript correction → analysis job → AIRun running → provider local → SessionReport draft. No hay aprobación automática.

## 21. AIRun/SessionReport

`StartAnalysis` crea AIRun antes de inferencia y enlaza attempt + transcript exactos. `CompleteAnalysis` publica draft, source_ai_run y job result en una única transacción; output hash y auditoría excluyen contenido clínico.

## 22. Live vs durable

El camino live/review permanece síncrono y no entra en la cola. Los puntos existentes verifican `LOCAL_AI_PROCESSING`; la transcripción ephemeral además comprueba audio/transcription. La ingestión post-session es durable y workerizada.

## 23. Retención

Configuración exige 1–168 horas para audio (default operativo 24 h cuando se habilita). Transcript/report/job metadata tienen lifecycle separado; no se destruyen por revocación de consentimiento.

## 24. Deletion/orphans

Delete de audio y transcript conserva tombstone/audit sin PHI y cancela procesamiento futuro. Maintenance recorre por lotes, marca metadata sin archivo como `missing` y elimina temporales viejos con validación tenant/mtime. Archivos finales desconocidos no se borran silenciosamente: quedan para investigación/compensación.

## 25. Privacy/audit

Job payloads contienen IDs, versiones y hashes, nunca transcript/name/narrative. Logs usan códigos de error y resultado. Auditoría registra IDs, estados, attempts, hashes y conteos; nunca audio, prompt o texto clínico.

## 26. Attack matrix

Consent, artifact, transcript, job, retry, cancel, delete y analysis filtran tenant/client y FKs compuestas. Tests PG cubren tenant extranjero, source foreign, grant foreign y transcript no resoluble: `NOT FOUND/FORBIDDEN` sin filtración.

## 27. API/OpenAPI

Se añadieron consentimiento, audio, artifacts, transcriptions, analysis jobs, transcript correction/deletion, job status/retry/cancel y health del ingestion worker a `docs/openapi.yaml`. Commands son typed; nunca se acepta path arbitrario ni job type arbitrario.

## 28. Observability

Worker health separa worker configurado/activo de sidecar available. Métricas `clinical_job_total{type,status}` y `clinical_job_duration_seconds{type}` mantienen cardinalidad baja. No se usan tenant/client/session como labels.

## 29. Fixtures A–L

| Fixture | Resultado | Evidencia |
|---|---|---|
| A happy path | PASS | `TestIngestionDurablePipelineAndRevocationPostgres` |
| B no recording consent | PASS | mismo test, upload denied/no artifact |
| C revoke after queue | PASS | mismo test + consent PG |
| D duplicate request | PASS | idempotency assertion |
| E sidecar retry contract | PASS unit/contract; PG retry PASS | transcriber durable tests + jobs test |
| F worker crash | PASS | lease expiry/recovery PG |
| G cancel | PASS | cancellation fence PG |
| H ASR correction | PASS | v1 preserved/v2 test |
| I storage/DB partial | PASS bounded compensation | artifact tests |
| J delete audio | PASS | raw deletion + transcript retained |
| K cross tenant | PASS | scoped repository tests |
| L manual bridge | PASS baseline regression; external consent enforcement added | Stage 2C.5 suite |

## 30. D1–D30 matrix

| Invariants | Resultado |
|---|---|
| D1–D9 | PASS: session root, four scopes, history, revoke, worker checks |
| D10–D12 | PASS: transcript epistemic boundary and immutable versions |
| D13–D20 | PASS: scoped encrypted artifacts, no narrative jobs, SKIP LOCKED, retry/cancel/recovery |
| D21–D25 | PASS: exact transcript, AIRun-before-provider, draft-only/no longitudinal mutation |
| D26–D28 | PASS: tombstones/maintenance, PHI-safe audit, tenant isolation |
| D29–D30 | PASS: manual bridge only; remote GIRA remains disabled by default |

## 31. Migration report

Temporary PostgreSQL: all migrations through 25 applied; `16 → latest → 16 boundary → latest` PASS. Migration 25 down removes ingestion objects/functions and leaves migration 24 objects. No real development database was migrated.

## 32. Traceability

Consent: migration25 + `clinical_consent_repository.go` + consent PG test. Storage: artifactstore + store tests + pipeline test. Jobs: jobs repository + claim/recovery test. Transcript: transcript repository + pipeline test. Worker/sidecar: `worker.go`, `durable.go`, strict contract tests. HTTP: handlers/server/OpenAPI parser tests. Every accepted row above has code, DB/state, automated test and execution result unless marked partial.

## 33. Full regression

```text
GO TEST: PASS (336 observed pass results, 0 fail)
GO VET: PASS
GO BUILD: PASS
POSTGRES STAGE 2D: PASS (consent/jobs/pipeline)
WORKER: PASS at unit/transaction boundary; multi-process deployment soak pending
FASTER-WHISPER: PASS synthetic opt-in, 9.53 s
OLLAMA LIVE: PASS baseline
OLLAMA REVIEW: PASS baseline
OLLAMA LONGITUDINAL A–G: PASS baseline 7/7
MANUAL PROJECT BRIDGE: PASS existing suite
WEB LINT/BUILD: PASS baseline (13 pages)
OPENAPI: PASS parser/reference tests
TENANT GUARD: PASS baseline
MIGRATION: PASS 16→25→16→25 temporary roundtrip
GIT: diff --check PASS; working tree preserved
```

## 34. Blocking gaps

Before production: decide/legal-review retention and transcript deletion policy; run HTTP E2E against configured deployment; run worker multi-process crash/soak; replace 25 MiB in-memory audio buffering with streaming encrypted storage for larger recordings; add durable tenant scheduler/worker process supervision.

## 35. Non-blocking debt

Add richer segment confidence/speaker metadata only if product needs it; add orphan inventory dashboard; expose transcript/job pagination; add explicit consent UI and deletion UI. None changes current clinical safety boundary.

## 36. Stage 3 readiness

`NOT READY` for implementation start. Backend foundations are ready for design review, but the following remain Stage 3 work: Active Session UI, recording controls, consent UI, transcript review UI, report review UI, ClinicalDiff review UI, process/hypothesis/evidence views, Goals/GIRA UI, manual GPT Project export/import UI, job status/retry UI and artifact retention/deletion UI. Stage 3 was not started.

## Final rule

No remote AI/GIRA was activated, no real clinical material was used, no GPT Project was synchronized, no clinical prompt/skill was changed, and no Stage 3 implementation was started.
