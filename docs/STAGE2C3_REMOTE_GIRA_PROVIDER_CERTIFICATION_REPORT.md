# STAGE 2C.3 — REMOTE GIRA PROVIDER CERTIFICATION REPORT

Fecha de corte de la investigación: 2026-09-02.

## 1. Verdict

```text
STAGE 2C = PARTIAL — REMOTE CERTIFICATION NOT EXECUTED
```

La investigación de capacidad, contrato y política de datos pudo completarse con documentación oficial. La certificación empírica no pudo ejecutarse porque el entorno no contiene credenciales para OpenAI, Anthropic, Google/Gemini, Azure OpenAI ni Mistral. No se hicieron llamadas remotas, no se enviaron datos clínicos —reales ni ficticios— y no se generó evidencia A–J artificial.

Las tres decisiones quedan separadas:

| Pregunta | Respuesta |
| --- | --- |
| Q1 — Capability | **UNKNOWN / NOT EXECUTED**. Ningún modelo remoto está certificado. |
| Q2 — Privacy | **DESIGN COMPLETE / NOT ACCEPTED**. Es viable diseñar minimización y aliasing, pero faltan implementación, pruebas, consentimiento y revisión legal/producto. |
| Q3 — Architecture | **YES, structurally feasible**. El puerto GIRA existente admite un adaptador remoto exclusivo por configuración, sin cambiar live/review/longitudinal. |

No se implementó proveedor remoto porque el gate de certificación no pasó.

## 2. Baseline

Se leyeron completos antes de continuar:

- `STAGE2C2B_SOL_MODEL_CERTIFICATION_ARCHITECTURE_REPORT.md`;
- `STAGE2C1_GIRA_BUILDER_RELIABILITY_REPORT.md`.

No se repitieron benchmarks locales.

| Control | Resultado |
| --- | --- |
| GO TEST | `PASS` — `go test -count=1 ./...` |
| GO VET | `PASS` — `go vet ./...` |
| GO BUILD | `PASS` — `go build ./cmd/server` |
| GIT | Working tree preexistente preservado; 23 archivos modificados y 22 no versionados al iniciar Stage 2C.3 |
| Credentials | `OPENAI_API_KEY=false`, `ANTHROPIC_API_KEY=false`, `GOOGLE_API_KEY=false`, `GEMINI_API_KEY=false`, `AZURE_OPENAI_API_KEY=false`, `MISTRAL_API_KEY=false` |

Stage 2C.3 agregó únicamente este informe. No modificó código, migraciones, prompts, contratos, fixtures ni datos.

## 3. Remote provider research

La preselección se limitó a tres modelos y se verificó contra documentación oficial vigente.

### OpenAI — `gpt-5.6-sol`

- API: Responses y Chat Completions disponibles.
- Structured output: soportado.
- Capacidad: 1.050.000 tokens de contexto y 128.000 de salida.
- Razonamiento: `none`, `low`, `medium`, `high`, `xhigh`, `max`.
- Precio de texto: USD 4/M input y USD 20/M output; precio promocional documentado al menos hasta 2026-11-21.
- Datos: la API no usa datos para entrenamiento por defecto; los abuse-monitoring logs se conservan normalmente hasta 30 días. Zero Data Retention y Modified Abuse Monitoring requieren aprobación. La residencia/procesamiento regional depende de elegibilidad y configuración.
- Adecuación inicial: candidato principal por capacidad declarada y JSON estructurado. La corrección clínica sigue sin demostrarse.

Fuentes: [modelo GPT-5.6 Sol](https://developers.openai.com/api/docs/models/gpt-5.6-sol), [controles de datos de OpenAI](https://developers.openai.com/api/docs/guides/your-data).

### Anthropic — `claude-sonnet-5`

- API: Claude API, model ID exacto `claude-sonnet-5`.
- Structured output: disponible mediante `output_config.format`, con conformidad de esquema documentada.
- Capacidad: 1M de contexto y 128K de salida según el catálogo vigente.
- Razonamiento: adaptive thinking.
- Precio: USD 2/M input y USD 10/M output; la documentación confirma que el precio introductorio pasó a ser permanente.
- Datos: los productos comerciales/API no usan datos para entrenamiento salvo opt-in o acuerdo explícito. La retención API estándar es de hasta 30 días; ZDR requiere acuerdo/aprobación y los esquemas estructurados pueden quedar cacheados hasta 24 horas, por lo que el schema nunca debe contener PHI.
- Región: por defecto puede procesar en EE. UU., Europa, Asia y Australia; el almacenamiento permanece en EE. UU. salvo acuerdo diferente. El control US-only requiere acuerdo/preferencia específica. No se verificó una opción equivalente de procesamiento exclusivamente argentino o sudamericano.
- Adecuación inicial: alternativa fuerte; la aceptabilidad regional y contractual requiere negociación explícita.

Fuentes: [modelos Claude](https://platform.claude.com/docs/en/models/overview), [precios](https://platform.claude.com/docs/en/about-claude/pricing), [structured outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs), [retención](https://privacy.claude.com/en/articles/7996866-how-long-do-you-store-my-organization-s-data), [ubicación de procesamiento](https://privacy.anthropic.com/en/articles/7996890-where-are-your-servers-located-do-you-host-your-models-on-eu-servers).

### Google — `gemini-3.8-flash`

- API: Gemini Developer API, model ID exacto `gemini-3.8-flash`.
- Structured output: disponible; el proveedor documenta soporte de JSON Schema mediante `response_format`.
- Capacidad: 1M tokens de contexto y 64K de salida.
- Razonamiento: niveles low, medium y high.
- Precio publicado: la página de precios indica USD 0,75/M input y USD 4,50/M output incluyendo thinking tokens. La página de lanzamiento aún indica USD 3,75/M output, por lo que existe una inconsistencia documental y debe prevalecer la cifra de la página de precios/factura al certificar.
- Datos: Paid Services no usan prompts/respuestas para mejora de productos. ZDR requiere aprobación y evitar funciones que almacenan estado, grounding, File API y explicit caching.
- Región: la Gemini Developer API no documenta un control directo de residencia regional equivalente; por ello se registra `UNKNOWN`.
- Restricción decisiva: los términos oficiales vigentes dicen que el servicio no puede usarse en práctica clínica, para brindar consejo médico ni en usos sujetos a autorización de dispositivo médico. El uso objetivo de este backend es clínico. Por tanto, Gemini Developer API queda **no elegible** sin autorización contractual distinta y revisión legal; no debe probarse como futuro proveedor de producción bajo esos términos.

Fuentes: [Gemini 3.8 Flash](https://ai.google.dev/gemini-api/docs/latest-model), [precios](https://ai.google.dev/gemini-api/docs/pricing), [structured outputs](https://ai.google.dev/gemini-api/docs/structured-output), [ZDR](https://ai.google.dev/gemini-api/docs/zdr), [términos Gemini API](https://ai.google.dev/gemini-api/terms).

## 4. Candidate shortlist

| Provider | Model | Why tested |
| --- | --- | --- |
| OpenAI | `gpt-5.6-sol` | Principal: modelo insignia, razonamiento configurable y structured outputs. Investigación completada; benchmark no ejecutado. |
| Anthropic | `claude-sonnet-5` | Alternativa fuerte: equilibrio documentado entre inteligencia y velocidad, structured outputs y ZDR contractual. Investigación completada; benchmark no ejecutado. |
| Google | `gemini-3.8-flash` | Opción de costo/latencia para contraste. **No elegible** por términos de uso clínico vigentes; benchmark no ejecutado. |

Esta tabla no certifica capacidad clínica.

## 5. Data policy comparison

| Provider | Training | Retention | Zero retention | Region | Evidence |
| --- | --- | --- | --- | --- | --- |
| OpenAI API | No por defecto; opt-in separado | Abuse logs hasta 30 días por defecto; algunas APIs pueden mantener application state | ZDR/MAM sujeto a aprobación | Regiones/residencia sujetas a elegibilidad y configuración | [Your data](https://developers.openai.com/api/docs/guides/your-data) |
| Anthropic API | No para productos comerciales salvo opt-in/acuerdo | Hasta 30 días por defecto | Acuerdo/aprobación; structured outputs elegibles, schema cache hasta 24 h | Procesamiento multirregión por defecto; almacenamiento US; controles adicionales por acuerdo | [retención](https://privacy.claude.com/en/articles/7996866-how-long-do-you-store-my-organization-s-data), [regiones](https://privacy.anthropic.com/en/articles/7996890-where-are-your-servers-located-do-you-host-your-models-on-eu-servers) |
| Gemini Developer API Paid | No para mejora del producto | Logging limitado para seguridad; features concretas retienen más | Aprobación por proyecto y exclusión de features stateful/grounding/files/cache | `UNKNOWN` para residencia seleccionable en Developer API | [ZDR](https://ai.google.dev/gemini-api/docs/zdr), [términos](https://ai.google.dev/gemini-api/terms) |

`UNKNOWN` significa no verificado en documentación oficial, no ausencia de control.

## 6. A–J

No hubo acceso autenticado a ninguna API. `NE` significa **NOT EXECUTED**, no fallo ni aprobación.

| Model | A | B | C | D | E | F | G | H | I | J | Total |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | ---: |
| `gpt-5.6-sol` | NE | NE | NE | NE | NE | NE | NE | NE | NE | NE | 0/0 |
| `claude-sonnet-5` | NE | NE | NE | NE | NE | NE | NE | NE | NE | NE | 0/0 |
| `gemini-3.8-flash` | NE | NE | NE | NE | NE | NE | NE | NE | NE | NE | 0/0; no elegible por términos |

Fixture C (eclecticismo) y fixture J (mecanismo no sustentado) no se evaluaron remotamente. Ningún candidato alcanzó A–J 10/10 porque no existió corrida.

El harness A–J existente sí atraviesa `Service.BuildGIRA`, el puerto `GIRABuilderProvider`, decoder estricto, compiler determinista y creación de `ClinicalDiff(pending_review)`. Sus fixtures son ficticios. La entrada del harness está actualmente ubicada en el paquete Ollama y envuelve el provider concreto; para certificar un remoto debe extraerse el runner compartido sin alterar fixtures ni assertions, y conectarse a un adaptador real. No se usaron respuestas preconstruidas como sustituto de inferencia.

## 7. Repair dependency

| Model | Primary pass | Repair required | Repair pass | Final pass |
| --- | ---: | ---: | ---: | ---: |
| `gpt-5.6-sol` | NE | NE | NE | NE |
| `claude-sonnet-5` | NE | NE | NE | NE |
| `gemini-3.8-flash` | NE | NE | NE | NE |

El contrato permanece en `1 primary + maximum 1 repair`. No se aumentaron retries.

## 8. C ×5

```text
NOT EXECUTED — prerequisite A–J 10/10 not established
```

## 9. 30-run certification

```text
NOT EXECUTED — prerequisite A–J 10/10 + C×5 5/5 not established
```

No existe tasa 27/30, 30/30 ni resultado remoto de seguridad que pueda informarse.

## 10. Semantic clinical comparison

| Dimension | OpenAI | Anthropic | Google |
| --- | --- | --- | --- |
| Grounding discipline | UNKNOWN | UNKNOWN | UNKNOWN |
| Goal operationalization | UNKNOWN | UNKNOWN | UNKNOWN |
| Strategy minimality | UNKNOWN | UNKNOWN | UNKNOWN |
| Rationale quality | UNKNOWN | UNKNOWN | UNKNOWN |
| Uncertainty handling | UNKNOWN | UNKNOWN | UNKNOWN |
| Approach fit | UNKNOWN | UNKNOWN | UNKNOWN |
| Technique fit | UNKNOWN | UNKNOWN | UNKNOWN |
| Avoidance of invented mechanisms | UNKNOWN | UNKNOWN | UNKNOWN |
| Competing hypotheses | UNKNOWN | UNKNOWN | UNKNOWN |

Las descripciones comerciales del proveedor no constituyen evidencia clínica ni sustituyen A–J.

## 11. Cost

No existen tokens medidos, repair rate ni costo promedio observado. La tabla de aceptación queda correctamente en `UNKNOWN`.

| Model | Avg input | Avg output | Repair rate | Avg GIRA cost |
| --- | ---: | ---: | ---: | ---: |
| `gpt-5.6-sol` | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN |
| `claude-sonnet-5` | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN |
| `gemini-3.8-flash` | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN |

### Sensibilidad no empírica

Para presupuestar una futura certificación, y **no como evidencia**, se usa un ejemplo fijo de 3.000 tokens de entrada + 1.500 tokens facturables de salida, sin cache y sin repair. Los thinking/reasoning tokens facturables y un repair elevarán el costo real.

| Model | 1 primary hipotético | 1 primary + repair idéntico |
| --- | ---: | ---: |
| `gpt-5.6-sol` | USD 0,0420 | USD 0,0840 |
| `claude-sonnet-5` | USD 0,0210 | USD 0,0420 |
| `gemini-3.8-flash` | USD 0,0090 | USD 0,0180 |

La estimación Gemini usa USD 4,50/M output de la página oficial de precios, no USD 3,75/M de la página de lanzamiento.

| GIRAs/month | OpenAI | Anthropic | Google |
| ---: | ---: | ---: | ---: |
| 10 | USD 0,42 | USD 0,21 | USD 0,09 |
| 50 | USD 2,10 | USD 1,05 | USD 0,45 |
| 100 | USD 4,20 | USD 2,10 | USD 0,90 |
| 500 | USD 21,00 | USD 10,50 | USD 4,50 |

Los escenarios suponen exactamente una primary, cero repair y el tamaño hipotético indicado; no asumen volumen real ni elegibilidad del proveedor.

## 12. Latency

| Stage | Median | Mean | P95 | Max |
| --- | ---: | ---: | ---: | ---: |
| Provider primary | NE | NE | NE | NE |
| Repair | NE | NE | NE | NE |
| Local validation | NE | NE | NE | NE |
| Local compiler | NE | NE | NE | NE |
| Total | NE | NE | NE | NE |

La clasificación permanece:

```text
GIRA EXECUTION = ASYNC REQUIRED
```

Es una decisión de arquitectura y experiencia de usuario: la estrategia post-session no debe mantener una request/UI bloqueada ni depender de latencia o disponibilidad externa. Una API rápida no cambia ese criterio. Para preservar ZDR, el futuro async debe ser un job durable **local** que efectúe una llamada remota acotada; no debe depender automáticamente del modo background/stateful del proveedor.

## 13. Data minimization

El contrato `GIRAModelInput` ya elimina tenant/client UUID, actores, auditoría y timestamps, y asigna refs opacas (`process_1`, `evidence_1`, etc.) a las entidades relacionadas. `BoundGIRAContext` limita el contexto al proceso seleccionado y prioriza relaciones directas. Sin embargo, `Process.Title/Description`, `Evidence.Statement`, `Event.Title/Description`, `Hypothesis.Statement` y textos de estrategia son narrativa libre y pueden contener PHI/PII.

Clasificación propuesta:

| Class | Fields/data |
| --- | --- |
| REQUIRED | schema version; estado clínico mínimo del proceso seleccionado; extractos aprobados estrictamente relevantes; eventos relevantes; hipótesis aprobadas; targets/goals/indicators/rationales/GIRA actual solo si afectan la decisión; candidatos de registry filtrados |
| OPTIONAL | título del proceso; título del evento; orden temporal cualitativo; resumen de estrategia histórica estrictamente necesario |
| REMOVE | nombre, email, teléfono, dirección, documento, calendario; tenant/client/user/run/source UUID; timestamps exactos; historia completa; todos los reportes; transcript/audio; notas; auditoría; procesos no relacionados |
| PSEUDONYMIZE | refs de proceso/evidencia/evento/hipótesis/estrategia; edades exactas, fechas, lugares, organizaciones, profesiones raras y relaciones identificables cuando no sean clínicamente indispensables |
| LOCAL-ONLY | mapa alias→UUID; provenance canónica; consentimiento; datos de contacto; prompt/output bruto; estado y auditoría de merge |

### BEFORE vs REMOTE PAYLOAD — synthetic only

Antes, ejemplo ficticio que **no debe salir**:

```json
{
  "client": {
    "name": "Paciente Ficticia Uno",
    "email": "ficticia@example.test",
    "phone": "+54 11 0000 0000",
    "address": "Calle Inventada 123",
    "document": "TEST-000001"
  },
  "process_id": "00000000-0000-0000-0000-000000000001",
  "all_processes": ["proceso relevante", "proceso no relacionado"],
  "raw_transcript": "..."
}
```

Payload remoto deseado:

```json
{
  "schema_version": "clinical-gira-builder-v1.1",
  "process": {
    "ref": "process_1",
    "clinical_status": "active",
    "formulation": "Patrón clínico mínimo generalizado y revisado localmente"
  },
  "evidence": [
    {"ref": "evidence_1", "epistemic_type": "patient_report", "statement": "Extracto ficticio mínimo sin identificadores"}
  ],
  "events": [],
  "hypotheses": [],
  "current_strategy": {},
  "approach_candidates": [],
  "technique_candidates": []
}
```

El backend resuelve los aliases localmente después de recibir la propuesta.

## 14. PII/PHI boundary

Flujo obligatorio futuro:

```text
Approved Clinical State
        ↓
GIRA Context Selector
        ↓
PII/PHI Minimizer
        ↓
Opaque Alias Mapper
        ↓
Remote Provider Payload
```

El selector actual cubre límites y relaciones, y el alias mapper cubre UUID. Falta una etapa explícita de minimización/redacción de narrativa antes del proveedor.

No se afirma anonimización perfecta: una combinación de historia clínica, edad, fechas, geografía, ocupación y hechos poco frecuentes puede reidentificar aun sin nombre ni contacto. Se requiere minimización semántica, pruebas de leakage con PII sintética, revisión humana de categorías de alto riesgo y aceptación formal del riesgo residual.

## 15. Consent requirements

No se implementó consentimiento en esta etapa. Stage 2D debe representar por separado, como mínimo:

```text
local AI processing
remote AI processing
audio/transcription
```

El consentimiento local no habilita implícitamente remote GIRA. La condición propuesta es consentimiento explícito, durable, revocable, versionado y auditable para procesamiento remoto, con proveedor/propósito/categorías de datos/retención/región comunicados al paciente. La redacción y base jurídica requieren revisión legal y de producto; este informe no constituye asesoramiento legal.

Sin consentimiento remote vigente, el job no debe construirse ni invocar al proveedor.

## 16. AIRun provenance

`ClinicalAIRun` ya conserva:

```text
provider
model
operation
prompt_name
prompt_version
input_hash
context_hash
output_hash
app_version
build_revision
status
timestamps
```

También registra fuentes canónicas/versionadas. El servicio inicia el run antes de inferir y solo crea el diff después de decode estricto + compiler determinista.

Extensión segura futura, sin contenido clínico bruto:

- `remote_request_id` cuando exista;
- `provider_region` configurada/confirmada;
- `model_snapshot` real devuelto por el proveedor;
- número de intento (`primary` o `repair_1`);
- input/output/reasoning/cache tokens;
- estimación monetaria y tabla de precio/version usada;
- duración del proveedor y repair;
- código de error normalizado.

`ParametersJSON`/metadata no debe almacenar prompts, statements ni respuesta clínica. Los schemas enviados a proveedores con cache de schema no deben contener ejemplos o descripciones con PHI.

## 17. Failure safety

### Garantías existentes auditadas

- `ClinicalAIRun(running)` se crea antes de llamar al proveedor.
- Error o cancelación finaliza el run como `failed`/`cancelled` mediante contexto sin cancelación.
- `ClinicalDiff` se crea únicamente tras salida del provider, decoder estricto y compiler tipado.
- El provider no recibe repositorio ni capacidad de merge.
- Review no muta estrategia; solo CA-W humano puede fusionar.
- No existe fallback silencioso a qwen u otro modelo local no certificado.

### Matriz remota requerida

| Failure | Required outcome | Executed now |
| --- | --- | --- |
| Network failure | run terminal; no diff; no mutation | NE |
| Timeout/cancel | run cancelled/failed; no diff; no mutation | NE |
| 429 | normalized unavailable/rate-limited; no fallback; no diff | NE |
| 5xx | normalized unavailable; no fallback; no diff | NE |
| Invalid JSON | strict rejection; optional single repair; no diff if repair fails | NE |
| Provider schema refusal | safe rejection; no diff | NE |
| Semantic validation failure | optional single repair; no diff if repair fails | NE |
| Repair failure | terminal run; no diff; no state mutation | NE |

Estas pruebas solo pueden añadirse contra un adaptador real después de que exista acceso de certificación y el boundary de privacidad sea aceptado.

## 18. Selected provider/model

```text
NONE
```

No se configuró `GIRA_PROVIDER` ni `GIRA_MODEL` remoto.

## 19. Architecture

La auditoría encontró dos abstracciones:

- `ClinicalInferenceProvider` cubre health/models/live/review y hoy es implementado por Ollama;
- `GIRABuilderProvider` es un puerto estrecho con `BuildGIRA(...)`, usa tipos internos (`GIRAGenerationRequest`, `ProviderOutput`) y no expone tipos de SDK.

Por tanto no es necesario ampliar ni contaminar el contrato general para GIRA. Un adaptador remoto futuro puede implementar solamente `GIRABuilderProvider` y ser inyectado en `.WithGIRABuilder(remote)`, mientras el proveedor Ollama permanece en live/review/longitudinal. La selección debe ser configuración explícita, no `ModelRouter` inteligente.

Flujo propuesto:

```text
LOCAL SESSION DATA
      ↓
approved longitudinal state
      ↓
local GIRA context selection
      ↓
minimization / aliasing
      ↓
REMOTE GIRA MODEL
      ↓
semantic proposal
      ↓
LOCAL deterministic compiler
      ↓
LOCAL validator
      ↓
ClinicalDiff pending
      ↓
HUMAN REVIEW
      ↓
LOCAL Human Merge
```

Configuración conceptual futura, adaptada a las convenciones existentes:

```text
CLINICAL_LIVE_PROVIDER=ollama
CLINICAL_REVIEW_PROVIDER=ollama
CLINICAL_LONGITUDINAL_PROVIDER=ollama
CLINICAL_GIRA_PROVIDER=<certified-provider>
CLINICAL_GIRA_MODEL=<certified-model>
```

Las claves viven en entorno/secret manager, nunca en Git. Una falla remote deja GIRA unavailable y mantiene el control humano; no activa qwen como fallback.

## 20. C1–C25

Stage 2C.3 no cambió código ni contratos. El baseline completo continúa pasando y conserva los guards deterministas. La columna remote indica que ningún adaptador/call fue ejecutado.

| Invariant | Baseline | Remote path |
| --- | --- | --- |
| C1 Process != Target | PASS | NE |
| C2 Target != Goal | PASS | NE |
| C3 Goal != Indicator | PASS | NE |
| C4 Hypothesis != Rationale | PASS | NE |
| C5 Formulation != Strategy | PASS | NE |
| C6 Approach != Technique | PASS | NE |
| C7 Approach requires Rationale | PASS | NE |
| C8 Rationale requires Target+Goal | PASS | NE |
| C9 Technique/Approach exact compatibility | PASS | NE |
| C10 no unjustified eclecticism | PASS | NE |
| C11 multimodal not required | PASS | NE |
| C12 unsupported mechanisms blocked | PASS | NE |
| C13 AI Target remains proposal | PASS | NE |
| C14 AI Goal remains planned proposal | PASS | NE |
| C15 AI cannot achieve Goal | PASS | NE |
| C16 AI GIRA is not approved GIRA | PASS | NE |
| C17 AI cannot complete Phase | PASS | NE |
| C18 historical GIRA preserved | PASS | NE |
| C19 deprecated registry version resolvable | PASS | NE |
| C20 Evidence does not auto-change Goal | PASS | NE |
| C21 review does not mutate strategy | PASS | NE |
| C22 Human Merge | PASS | NE |
| C23 transactional merge | PASS | NE |
| C24 stale diff protection | PASS | NE |
| C25 No AI Direct Merge | PASS | NE |

Todo output remoto futuro debe atravesar exactamente el mismo decoder, compiler, validadores, `ClinicalDiff(pending_review)`, review humana y merge CA-W. No hay excepción por proveedor.

## 21. Regression

| Suite | Result |
| --- | --- |
| `go test -count=1 ./...` | PASS |
| `go vet ./...` | PASS |
| `go build ./cmd/server` | PASS |
| Remote GIRA A–J | NOT EXECUTED — no credentials/API access |
| Remote GIRA C×5 | NOT EXECUTED — prerequisites unmet |
| Remote GIRA 30-run | NOT EXECUTED — prerequisites unmet |
| Remote failure matrix | NOT EXECUTED — no adapter/access |
| Remote privacy guard tests | NOT EXECUTED — minimizer not implemented by design |
| PostgreSQL opt-in | NOT RUN — no provider integration occurred |
| Ollama live/review/longitudinal opt-in | NOT RUN — frozen; no integration occurred |
| Web lint/build | NOT RUN — no web changes |
| OpenAPI | UNCHANGED |
| Tenant guard | Covered by `go test ./...`; PASS |
| Migration roundtrip | NOT RUN — no migration changes |
| `git diff --check` | PASS |

Gate 23 exige la regresión extendida **si ocurre integración**. No ocurrió integración, por lo que no se presentó un opt-in incompleto como certificación.

## 22. Remaining gaps

### BLOCKING

1. Credenciales/proyecto de billing aislado para al menos OpenAI y Anthropic.
2. Confirmación contractual de uso clínico y DPA aplicable; Gemini Developer API no es elegible bajo términos actuales.
3. Aprobación de ZDR/MAM o control equivalente y región aceptada.
4. Implementación y aceptación del selector/minimizer/alias mapper con PII sintética.
5. Consentimiento remoto explícito, durable y auditable.
6. Adapter/harness provider-agnostic que ejecute el servicio real sin bypass.
7. A–J 10/10, C×5 5/5, >=27/30 y cero violaciones clínicas.
8. Matriz de fallos remotos y regresión completa.

### NON-BLOCKING

1. Persistir request ID, model snapshot, región, usage y estimación de costo sin contenido clínico.
2. Dashboard de latencia/costo de baja cardinalidad.
3. Alertas de outage/rate limiting y estado visible de `GIRA unavailable`.
4. Pinning y proceso de recertificación ante cambio de snapshot/precio/política.

### DOCUMENTATION

1. Matriz de campos REQUIRED/OPTIONAL/REMOVE/PSEUDONYMIZE/LOCAL-ONLY aprobada por producto/privacidad.
2. Registro de riesgo residual de reidentificación narrativa.
3. Política de retención y región por proveedor/contrato.
4. Runbook de rotación de secretos, outage, revocación de consentimiento y eliminación.
5. Evidencia reproducible de cada corrida con hashes, usage, timestamps y versión del harness, sin payload clínico bruto.

## 23. Stage 2D readiness

```text
NOT READY
```

Stage 2D no debe comenzar hasta resolver los blockers anteriores. En particular, no existe proveedor/modelo GIRA certificado, el boundary remoto no está implementado ni aceptado, y no existe consentimiento remoto durable.

La arquitectura manual actual se preserva. Live, review y longitudinal permanecen locales. No se modificaron clinical skills, GPT Project sources, `clinical-live-v1`, `clinical-review-v1`, `clinical-longitudinal-interpreter-v1`, `clinical-gira-builder-v1.1`, `semantic_v1`, compiler, C1–C25 ni Human Merge.

STOP — Stage 2D no iniciado.
