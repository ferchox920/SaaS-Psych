# GPT_PROJECT_PACKAGE_V1 — packaging humano, no sincronización

Estado: diseño Stage 3C, 2026-09-09. No es entidad DB, integración API ni instrucción instalada en un Project real. Contratos de autoridad: `CLINICAL_PROJECT_BRIDGE_CONTRACT_V1.md`, OpenAPI y los structs/validadores Go de `project_bridge.go`. No se introduce un formato clínico paralelo.

## Capas y responsabilidad

| Capa | Responsabilidad | Versionado / actualización humana |
| --- | --- | --- |
| 1 — GENERAL CLINICAL SKILL | Metodología general de supervisión; conservar distinciones epistemológicas y límites de inferencia | `clinical-microprocess-supervisor` 1.0, fuente autorizada auditada en Stage 3C.1; identidad abajo. Original intacto, nunca rellenar con datos del paciente |
| 2 — CASE_RUNTIME_PROFILE | Snapshot compacto del proceso aprobado seleccionado; refs, estados, preguntas exploratorias y ausencias explícitas | Un runtime actual por proceso seleccionado/export. Nueva generación sustituye conceptualmente el snapshot anterior, no modifica su registro local |
| 3 — CLINICAL SOURCES | Evidencia/eventos/hipótesis y estrategia aprobados seleccionados por V1 | Solo selección intencional. Fuentes adicionales históricas requieren política y contrato explícitos; no acumulación por sesión |
| 4 — OUTPUT CONTRACT | `ClinicalProjectImportV1` y su estrategia `semantic_v1` | `clinical-project-import-v1`; tipos, versiones, referencias y límites del backend autoritativos |

Los Projects pueden compartir instrucciones y archivos entre sus conversaciones. Esta capacidad no constituye una conexión con SaaS-Psi; el diseño exige transferencia humana. Fuente oficial consultada: [Projects and chats](https://learn.chatgpt.com/docs/projects), 2026-09-09. No se presupone acceso a una cuenta, modelo particular, capacidad contratada ni sincronización.

## Actualización manual del runtime

1. Seleccionar un proceso aprobado y generar un export local MINIMIZED.
2. Inspeccionar artifact completo, tipos epistemológicos y narrativa, incluidos terceros. La minimización no garantiza anonimato.
3. Verificar consentimiento externo manual según política del producto; local AI no lo sustituye.
4. Copiar/descargar exclusivamente el Markdown canónico. El recibo export ID/hash está separado de ese contenido clínico: el UUID del recibo no identifica al paciente. Nunca copiar toda la respuesta API con tenant/client/user IDs.
5. Si se decide transferir, hacerlo manualmente al Project correcto. Declarar cuál snapshot pasa a ser CURRENT RUNTIME. No mezclar aliases de dos exports aunque ambos usen `process_1`.
6. Retirar o marcar explícitamente como histórico el runtime anterior en el Project mediante acción humana y conforme a la política de retención. Esto es una recomendación de organización, no una operación ejecutada por SaaS-Psi ni autorización para borrar fuentes reales.
7. Importar la respuesta estructurada con el recibo original. Si el estado cambió, el backend rechaza el import; generar un nuevo snapshot y producir una nueva propuesta, sin auto-rebase.

No editar snapshots locales, concatenar runtimes indefinidamente ni interpretar una generación como fuente ya usada o transferida.

## CURRENT RUNTIME frente a DEEP HISTORY

V1 exporta exclusivamente el proceso aprobado seleccionado, sus fuentes admitidas y estrategia aprobada. Formulación, reportes completos, transcripciones, audio, alianza y patrones del terapeuta no se agregan automáticamente. Las secciones no seleccionadas/no disponibles se declaran.

Deep history futuro podría contener resúmenes de reportes aprobados, evidencia seleccionada e historia de proceso/GIRA cuando se autorice expresamente su selección y minimización. No se implementa ese export aquí. Cada fuente histórica debe justificar relevancia, origen, versión, fecha y estado histórico; no puede reemplazar el runtime vigente ni establecer como hecho una hipótesis antigua. Revisar periódicamente pertinencia y retención. Nunca diseñar `cada sesión → otra fuente gigante para siempre`.

## Manifest humano (diseño, no artifact clínico nuevo)

```yaml
project_package_version: GPT_PROJECT_PACKAGE_V1
runtime_export_id: <recibo opaco del export local>
runtime_hash: <content_hash del backend, no recalculado desde Markdown>
runtime_state_version: <state_version>
runtime_scope: selected_process
runtime_role: CURRENT_RUNTIME
general_skill:
  name: clinical-microprocess-supervisor
  version: "1.0"
  sha256: "00646adb6f0503e41cb0636e6b0807cb5c18f1acab54b62871ba78a2e38583d6"
bridge_companion:
  name: clinical-project-bridge-output
  version: "0.1.0"
  sha256: "63f21bc9fd1bb0de0c4c4cca36290b176e4a018d782b3753cb61446741fd4970"
  status: APPROVED_NOT_DEPLOYED
  artifact: docs/clinical-project-bridge-output.md
sources:
  - role: CURRENT_RUNTIME_AND_SELECTED_SOURCES
    filename: <archivo Markdown canónico descargado>
    schema_version: clinical-project-export-v1
    hash: <content_hash del backend>
  # Fuentes históricas adicionales: ninguna en el paquete V1 automático.
output_schema_version: clinical-project-import-v1
strategy_contract: semantic_v1
manual_review: <fecha/revisor local según política, no copiar identidad sin necesidad>
```

`runtime_hash` es hash del artifact canónico serializado por Go, no del archivo Markdown, recibo o manifest. Fuentes clínicas usan aliases export-scoped, nunca UUIDs de entidades internas. El manifest no debe contener datos del paciente ni claves; si se transfiere, debe revisarse igual que cualquier otro archivo.

## Instrucciones de salida aprobadas como componente, no desplegadas

No se modifica ninguna skill actual. Autoridad del complemento aprobado: `clinical-project-bridge-output.md` 0.1.0. El siguiente resumen no activa modo import: se requiere petición explícita del terapeuta para el turno actual. Schema cargado o petición previa no lo activan. Sin CURRENT_RUNTIME explícito y no ambiguo, no elegir ni combinar snapshots ni generar objeto importable; aclarar humanamente y esperar selección explícita. No se han instalado instrucciones en Projects.

- Tratar el contexto como fuentes aprobadas con distintos tipos epistemológicos; aprobación no implica certeza ni convierte inferencia/hipótesis en hecho.
- Usar solamente las referencias exactas del snapshot actual. No inventar evidencia, corregir aliases por parecido ni consultar otros pacientes.
- Producir un objeto JSON `ClinicalProjectImportV1` identificable, separado de comentario humano. El terapeuta pega exclusivamente ese objeto en el importador, sin cercas Markdown.
- Incluir `schema_version`, `source_export_id`, `source_export_hash`, `provenance`, `operations`, `open_questions` y `supervision_observations`; `strategy` es opcional bajo el contrato congelado. Tomar ID/hash del recibo explícito, nunca inferirlos del texto.
- Provenance requiere `source_type=manual_external_ai`, `provenance_assertion=user_supplied`. Provider/model/surface/project_context_version son declaraciones opcionales del terapeuta, no observaciones de una API ni certificación de modelo.
- No escribir UUIDs internos en payloads clínicos. Operaciones de creación reservan refs nuevas; operaciones sobre entidades existentes requieren la versión exportada. No reutilizar refs entre snapshots.
- Reutilizar las operaciones portables permitidas y el contrato `semantic_v1`; no inventar goal achieve, phase complete, factual evidence, merge, borrado o sobrescritura histórica.
- Expresar incertidumbres sin fabricar mecanismos para completar una GIRA. Las notas de supervisión no son evidencia del paciente. Al menos una operación clínica válida es necesaria para crear diff; un comentario solo no debe forzarse a operación sin fundamento.
- Una propuesta nunca es estado clínico aprobado. Validación local, revisión por operación y Human Merge siguen siendo obligatorios.

El ejemplo de sobre y operaciones admitidas está en el contrato V1 existente. El schema completo de estrategia está en `docs/openapi.yaml` como `ClinicalProjectStrategy`; `go run ./cmd/clinical-project-schema` lo reproduce localmente desde `apps/api`. Ese comando imprime estrategia, no debe presentarse como schema de todo el sobre. No duplicar su lista de enums en otra fuente mantenida manualmente.

## Auditoría de compatibilidad — resultado y límites

Stage 3C quedó sin fuente disponible. En Stage 3C.1 el usuario aportó `C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md` y sus instrucciones generales en conversación. Se leyó el archivo completo: versión declarada 1.0, 16.789 bytes, 617 líneas, SHA-256 consignado en el manifest. Se auditó esa copia autorizada; no se inspeccionaron instalaciones ni Projects reales.

Resultado final: metodología clínica compatible; complemento separado aprobado tras los dos hardenings autorizados. `clinical-project-bridge-output.md` 0.1.0: 7.193 bytes, 58 líneas, hash del manifest, APPROVED_NOT_DEPLOYED. La propuesta 0.1.0-proposed y el informe previo conservan su valor histórico; no son el componente vigente. La skill original no cambió. Cierre: `STAGE3C1_FINALIZATION_STAGE3_ACCEPTANCE_REPORT.md`, Stage 3 VERIFIED PASS como aceptación de aplicación/paquete, no despliegue productivo.

| Clasificación futura | Requisito / razón |
| --- | --- |
| COMPLETED | Fuente general e instrucciones aportadas, identidad registrada y auditoría read-only completa |
| COMPLETED | Complemento 0.1.0 aprobado, con modo import por turno y bloqueo de runtime ambiguo; no desplegado |
| REQUIRED | Proveer el contrato backend versionado como fuente general de serialización cuando se habilite modo import, sin reconstruir enums de memoria |
| RECOMMENDED | Añadir guía de incertidumbre/contradicción y ejemplo sintético conforme al schema actual, solo si no existen |
| RECOMMENDED | Manifest, registro manual de runtime actual y comprobación de paciente/Project antes de transferir |
| NOT NEEDED | Datos del paciente dentro de la skill metodológica, sincronización, API remota, RAG o cambios al compilador para adaptar salida libre |

No se encontró una regla que obligue a fabricar hipótesis o a interpretar automáticamente correcciones como resistencia. Las adiciones técnicas propuestas y la ambigüedad contextual del ejemplo final están documentadas con líneas exactas de la fuente. Ninguna skill ni fuente de paciente existente fue modificada. Los fixtures deterministas verifican datos/contratos, no certifican conducta o confiabilidad de un modelo.
