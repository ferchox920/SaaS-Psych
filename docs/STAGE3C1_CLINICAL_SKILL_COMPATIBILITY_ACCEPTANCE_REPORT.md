# STAGE 3C.1 — CLINICAL SKILL COMPATIBILITY & FINAL STAGE 3 ACCEPTANCE REPORT

## 1. Verdict

```text
STAGE 3C = PARTIAL — REQUIRED SKILL PATCH PENDING
```

Source Gate superado. Auditoría read-only de la fuente real completada. La metodología de microprocesos es compatible; las reglas específicas de runtime/export/serialización requieren una adición separada, propuesta pero no aprobada ni instalada. “Skill patch” en este veredicto significa complemento de instrucciones generales del bridge, no modificación de la skill clínica original. No se declara Stage 3 VERIFIED PASS por la mera existencia del candidato.

## 2. Skill source identity

| Campo | Fuente verificada |
| --- | --- |
| Filename | `clinical_microprocess_supervisor_SKILL.md` |
| Nombre declarado | `clinical-microprocess-supervisor`, línea 2 |
| Versión declarada | `1.0`, línea 9; no inferida |
| Bytes reales | 16.789 |
| Líneas | 617 |
| SHA-256 | `00646adb6f0503e41cb0636e6b0807cb5c18f1acab54b62871ba78a2e38583d6` |
| Origen | Copia autorizada indicada por el usuario: `C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md` |
| Preservación | Lectura únicamente; hash comprobado antes y después; sin editar/instalar |

**S** en este informe refiere a esa fuente, con enlaces a sus líneas. Se leyó completa, no se reconstruyó de informes. Se auditó la copia que el usuario designó como marco operativo; no se afirma haber comprobado su instalación en una cuenta externa.

**U** refiere a las instrucciones generales aportadas en el mensaje del usuario que acompaña el archivo, puntos 1–10 y párrafos iniciales/finales. Versión de U: UNDECLARED. Origen: conversación, no archivo; no se atribuye hash/bytes de archivo a un mensaje cuyo transporte original no está disponible. U solicita conducción de Fernando, TCC/logoterapia, calibración de confrontación, corrección del paciente y formatos compacto/retrospectivo. No se recibieron ni buscaron instrucciones específicas de pacientes.

## 3. Skill purpose

Segunda capa de supervisión en tiempo real, no terapeuta principal ni generador automático de frases. Detecta nodos, conecta antecedentes, calibra interpretaciones e intervención y observa al terapeuta: [S:17–33](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:17). Juicio clínico retenido por el terapeuta: [S:39–44](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:39). U inicial y puntos 6–7 coinciden.

No se transformará esa función en exportador ni serializador obligatorio de cada fragmento.

## 4. Current clinical algorithm

La secuencia histórica coincide literalmente con la fuente, [S:46–52](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:46):

```text
DISPARADOR → CLASIFICACIÓN → ANCLAJE → NIVEL DE CERTEZA
→ MOMENTO CLÍNICO → INTERVENCIÓN POSIBLE
→ RESPUESTA DEL PACIENTE → ACTUALIZACIÓN DE HIPÓTESIS
```

Disparadores seleccionados por relevancia ([S:56–150](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:56)); anclaje ideal de 2–4 antecedentes sin inventarlos ([S:154–172](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:154)); semáforo ([S:176–213](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:176)); timing y posibilidad de no intervenir ([S:240–285](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:240)); respuesta posterior a confrontación ([S:339–362](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:339)). “Actualización” en este algoritmo es revisión conceptual de una hipótesis, no autorización de persistencia SaaS.

## 5. Epistemic compatibility

Taxonomía de tres niveles explícita en [S:217–236](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:217). No se exige reemplazarla por seis nombres durante supervisión. Sí hace falta aclarar la correspondencia al leer exports:

| Taxonomía de S | Correspondencia / límite para el bridge |
| --- | --- |
| HECHO: lo que el paciente dijo/hizo/describió | Una declaración conserva patient_report; que alguien la haya dicho no prueba el acontecimiento referido. Una observación del terapeuta conserva therapist_observation; documented_fact requiere su provenance específica |
| INFERENCIA | Lectura derivada, compatible con AI_INFERENCE cuando ese sea su origen |
| HIPÓTESIS | Explicación provisional; CLINICAL_HYPOTHESIS no se promueve a APPROVED_FORMULATION |
| Formulación longitudinal/consolidada | No define aprobación persistida del SaaS; conservar etiqueta y origen del export |

La fuente ya impide tomar plausibilidad por verdad ([S:41–42](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:41)); no define `approval_status`. Añadir **approval != truth** es adaptación de frontera, no corrección de una regla que equipare explícitamente ambos.

El semáforo interpretativo no se copia automáticamente a confidence longitudinal ni modifica su estado. [S:178–213](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:178) prescribe calibración interpretativa, no operaciones de DB.

## 6. Runtime compatibility

Puede razonar con antecedentes parciales: intenta anclar y, cuando falta sustento, explora ([S:156–172](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:156)). No obliga a adquirir historia completa. Compatible con adición que defina CASE_RUNTIME_PROFILE como snapshot de proceso seleccionado, sus etiquetas y secciones no incluidas.

La búsqueda en las 617 líneas no encontró `CASE_RUNTIME_PROFILE`, `CURRENT_RUNTIME`, `ClinicalProjectImportV1`, `semantic_v1`, `source_export` ni `Human Merge`. Se trata de ausencias de integración; la intención de anclaje sí está presente y se conserva.

## 7. Missing-data semantics

Regla positiva: “Nunca inventar antecedentes para sostener una hipótesis”, [S:172](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:172). Provisionalidad ante falta de anclajes en S:169–170; incertidumbre y esperar en [S:538–550](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:538).

Adición requerida: `NOT INCLUDED/unavailable` describe el input, no ausencia real de formulación, reportes o problemas de alianza. No se encontró una regla contraria; esa distinción técnica aún no está expresada. U:2 exige antecedentes concretos, pero no obliga a fabricarlos ni a exportar toda la historia.

## 8. Portable-reference compatibility

La fuente ancla en antecedentes clínicos ([S:154–172](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:154)), pero no define identidad portable ni resolución de refs. El contrato de salida humano ([S:438–488](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:438)) no incluye aliases.

Adición requerida: resolución exacta, sin fuzzy matching; refs pertenecen al export; nuevas entidades usan refs legales y UUIDs reservados por backend; recibo explícito. El UUID de source_export_id es metadata de correlación permitido, distinto de IDs clínicos internos. Ausencia de reglas sobre aliases no es evidencia de que la skill los invente.

## 9. CURRENT_RUNTIME / DEEP_HISTORY compatibility

Preservar formulación longitudinal es objetivo ([S:31](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:31)); datos nuevos y contradicciones pueden cambiar el análisis ([S:63–74](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:63)). No existe política de precedencia entre snapshots versionados.

El complemento exige un CURRENT_RUNTIME explícito. Con dos snapshots sin designación, pedir aclaración humana; no usar orden del prompt, fecha inferida o alias repetido como autoridad. Historia contextualiza, no sobrescribe automáticamente estado actual. U no agrega una precedencia distinta.

## 10. ClinicalProjectImportV1 compatibility

Formato compacto vivo y revisión retrospectiva en [S:438–488](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:438); plantilla ultrabreve en [S:575–586](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:575). Son formatos preferidos/contextuales, no una prohibición de JSON.

Adición necesaria en instrucciones del Project, solo al pedir import: comentario humano separado de un objeto JSON sin fences; recibo no inventado; contrato real disponible. Autoridades revisadas: structs `ClinicalProjectImportV1`/`ManualProvenance`, `DecodeProjectImport`, `ValidateProjectImport`, OpenAPI y contrato V1. No se duplicaron enums en el complemento.

Provenance manual_external_ai/user_supplied no existe como concepto en S; corresponde al bridge. Metadata opcional solo si la declara el terapeuta, sin inventar provider/model y sin AIRun ficticio. V1 requiere al menos una operación clínica: si no hay fundamento para proponerla, producir comentario humano, no forzar una operación para conseguir un diff.

## 11. semantic_v1/GIRA compatibility

TCC y logoterapia son marcos prioritarios, no una orden de seleccionar múltiples enfoques en cada intervención: [S:32–33](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:32), [S:366–434](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:366). No se encontró regla que obligue al eclecticismo o a fabricar mecanismos.

No define Process/Target/Goal/Indicator, TherapeuticRationale, registros ni GIRA. Adición: separación de entidades, rationale ligado a Target+Goal, versiones compatibles de approach/technique, funciones distintas para múltiples approaches y propuesta != aprobación. Semáforo ROJO exige explorar ([S:204–213](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:204)), compatible con rechazar estrategia sin sustento.

No goal achieve, phase complete, rewrite evidence o overwrite GIRA. No se confunde esta falta de permisos técnicos explícitos con que la fuente autorice esas operaciones: están fuera de su alcance original.

## 12. Human Merge compatibility

El terapeuta conduce y conserva juicio ([S:39–44](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:39)); la respuesta del paciente revisa hipótesis ([S:357–362](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:357)). Compatible conceptualmente con control humano.

Se debe añadir el límite técnico: export/import/review no materializan cambios longitudinales; solo Human Merge existente. “Actualizar hipótesis” en comentario no significa ejecutar actualización de la entidad. No se añade herramienta de merge a la skill.

## 13. Contradictions found

**No se encontró una contradicción normativa directa que obligue a diagnosticar, inventar evidencia, intervenir siempre, ignorar contradicción o rechazar toda confrontación intensa.** Evidencia positiva: [S:74](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:74), [S:169–172](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:169), [S:284–285](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:284), [S:359–362](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:359), [S:494–511](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:494), [S:540](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:540).

Ambigüedad localizada, no defecto demostrado: el ejemplo presenta un fragmento en [S:592–593](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:592), pero el sustento agrega “ha intentado resolver el asunto en momentos de amenaza vital” en [S:603–604](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:603), dato no contenido en ese fragmento. Puede asumir antecedentes ilustrativos no desplegados; no se afirma que sea un caso real o evidencia inventada por la skill. Adición protectora: los ejemplos metodológicos nunca son fuentes del caso actual y solo se usan anclajes efectivamente suministrados. No se reescribió el ejemplo.

U:9 cambia preferentemente los nombres de encabezados, pero coincide con plantilla S:579–584; U:7 coincide con S:494–511. No conflicto de formato ni mandato de suavizar confrontaciones.

## 14. Compatibility matrix

“WITH ADDITION” identifica integración faltante, no falla clínica demostrada. Las ausencias se verificaron en la fuente completa; los enlaces indican las secciones pertinentes existentes.

| Área | Resultado | Fuente / fundamento |
| --- | --- | --- |
| Epistemic separation | COMPATIBLE WITH ADDITION | [S:219–236](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:219); preservar taxonomía exportada, HECHO no equivale siempre a documented_fact |
| Uncertainty | COMPATIBLE | [S:204–213](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:204), S:517–534 |
| Longitudinal anchoring | COMPATIBLE | [S:154–172](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:154) |
| Competing hypotheses | COMPATIBLE WITH ADDITION | [S:66–74](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:66), S:486; permite alternativas, falta política explícita de coexistencia en estado |
| Patient correction | COMPATIBLE | [S:357–362](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:357) y U:5 |
| Runtime profile | COMPATIBLE WITH ADDITION | [S:156–170](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:156); anclaje parcial sí, schema runtime no definido |
| Missing-data handling | COMPATIBLE WITH ADDITION | [S:169–172](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:169); precisar not included != nonexistent |
| Portable refs | COMPATIBLE WITH ADDITION | [S:450–451](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:450); anclajes humanos sin contrato de identidad |
| Export-scoped refs | COMPATIBLE WITH ADDITION | [S:154–172](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:154); no scope/export/versionado en S completa |
| Human commentary | COMPATIBLE | [S:438–488](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:438), S:575–586 y U:9–10 |
| ClinicalProjectImportV1 | COMPATIBLE WITH ADDITION | [S:438–488](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:438); formato contextual no impide modo JSON separado |
| Provenance | COMPATIBLE WITH ADDITION | [S:450–451](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:450); fuentes conceptuales, sin receipt/user_supplied |
| Strategy semantic_v1 | COMPATIBLE WITH ADDITION | [S:366–434](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:366); metodología sin tipos/registro/serialización |
| GIRA | COMPATIBLE WITH ADDITION | [S:477–488](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:477); revisión/estrategia comunicacional no es contrato GIRA |
| Human Merge | COMPATIBLE WITH ADDITION | [S:39–44](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:39), S:357–359; autoridad humana sí, frontera técnica ausente |
| Safety | COMPATIBLE | [S:145–150](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:145), [S:554–571](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:554) |

No área clasificada INCOMPATIBLE por mera ausencia de schema. Compatibilidad conductual de un modelo concreto: UNKNOWN, fuera de esta auditoría de instrucciones.

## 15. Required changes

Propuesta localizada, en complemento independiente: (1) lectura epistemológica del export/approval; (2) CURRENT_RUNTIME y ausencia de datos; (3) aliases exactos/export-scoped; (4) recibo real sin inventarlo; (5) modo import explícito y contrato autoritativo suministrado; (6) provenance declarada; (7) límites semantic_v1 y Human Merge; (8) ejemplos nunca convertidos en antecedentes del caso. Motivación y ubicaciones originales en secciones 5–13.

Requeridos para cerrar compatibilidad integral del bridge, no para seguir usando la skill original como supervisión clínica en vivo. No instalar sin aprobación. No se requiere editar las 617 líneas originales.

## 16. Recommended changes

Tras aprobar responsabilidades, adjuntar un ejemplo sintético válido y contrato versionado del backend como fuentes generales. Manifest con versión/hash; comprobación humana del runtime/recibo antes de generar import. No duplicar los enums ni el schema de estrategia dentro del texto metodológico. Son ayudas de operación, no justificación para modificar criterios clínicos.

## 17. DO NOT CHANGE

Conservar algoritmo ([S:48](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:48)), anclaje y semáforo ([S:154–213](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:154)), timing/no intervención ([S:240–285](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:240)), metasupervisión no automática ([S:289–335](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:289)), respuesta del paciente ([S:339–362](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:339)), TCC/logoterapia, calibración de confrontación ([S:494–511](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:494)) y prioridad de seguridad ([S:554–571](C:/Users/ferna/Downloads/clinical_microprocess_supervisor_SKILL.md:554)). U solicita expresamente esa preservación.

## 18. Recommended responsibility split

**OPTION B**:

```text
clinical-microprocess-supervisor 1.0 → razonamiento clínico / timing / supervisión
Project bridge output companion   → runtime / aliases / receipt / modo import
Backend contract + validators     → schema / operaciones legales / persistencia
Fernando + Stage 3B               → revisión humana y Human Merge
```

Justificación: S declara supervisión en vivo y formatos humanos (S:17–33, S:438–488), no serialización. Preferencia del usuario por formato ultrabreve permanece vigente en vivo. El complemento solo se activa para la integración técnica correspondiente; no se confía en orden accidental del prompt para elegir snapshot. Conflictos no resueltos se explicitan y se solicita aclaración, sin alterar restricciones superiores o juicio humano.

## 19. Synthetic fixtures A–J

Se agregó `TestStage3C1InstructionBoundaryFixtures` con diez casos **deterministas**, usando el export/decoder/compilador/validador reales, no un proveedor. Los ejemplos siguientes describen expectativa de instrucción, no una salida observada de GPT:

| Fixture | Entrada/expectativa de revisión | Resultado ejecutado y límite |
| --- | --- | --- |
| A Fact vs hypothesis | Relato explícito, inferencia y H aprobada mantienen categoría | PASS: export conserva PATIENT_REPORT / AI_INFERENCE / CLINICAL_HYPOTHESIS |
| B Contradiction | Fuente apoya H; otra la contradice; no forzar certeza | PASS: ambos grupos y confidence amarillo preservados |
| C Competing H | H1/H2 plausibles, sin ganador obligatorio | PASS: export conserva ambas hipótesis |
| D Patient correction | “La explicación no describe mi experiencia”; atender la corrección | PASS: texto corrective preservado como patient_report, sin etiqueta resistance añadida |
| E Missing context | Formulación no incluida: no inventarla | PASS: runtime marca unselected_formulation; no prueba inferencia de un modelo |
| F Aliases | Usar fuentes exportadas, no inventar evidence_3 | PASS: validador rechaza ref ausente; S no especificaba esta regla, complemento sí |
| G Two exports | No mezclar process_1 entre snapshots; aclarar CURRENT_RUNTIME ambiguo | PASS: recibo de otro export rechazado. La petición humana de aclaración es requisito del complemento, no conducta medida |
| H Missing receipt | Explicar recibo faltante en comentario, sin inventarlo | PASS: decoder rechaza ID/hash ausentes; no modelo ejecutado |
| I Valid import | Recibo sintético y propuesta conforme a contrato | PASS: DecodeProjectImport + ValidateProjectImport + semantic compiler producen operaciones válidas |
| J Unsupported GIRA | Insufficient evidence: explorar, no rellenar mecanismo | PASS: approach inventado rechazado; propuesta solo de notas también rechazada como diff vacío. En ese caso corresponde comentario humano |

A–E prueban preservación/representación, F–J límites de contrato. No se afirma que esos tests ejecuten texto de la skill ni certifiquen que un modelo siempre obedecerá. Evaluación semántica real de un modelo no ejecutada ni requerida como certificación en esta etapa.

## 20. Backend schema validation

Reejecutado `go test -count=1 -v ./internal/usecase/longitudinal -run 'TestStage3C1|TestProject'`: **28 PASS / 0 FAIL** contando padres/subtests; incluye los diez A–J, import válido, unknown ref, UUID ajeno, mecanismo no soportado, achieve/complete/overwrite/rewrite prohibidos, stale y parsing estricto.

`go test -count=1 ./internal/http/handlers -run 'TestProjectOpenAPI|TestProjectHTTP'`: PASS. Se usan los schemas y validadores existentes; no se implementó validador relajado. Sin DB/proveedor/worker, sin pacientes ni llamadas externas. No se repitieron Whisper/Ollama ni regresión web completa porque solo cambiaron documentación y un archivo de tests.

## 21. Proposed patch

| Elemento | Estado |
| --- | --- |
| Original | Skill 1.0, 617 líneas, hash de sección 2; **cero cambios** |
| Propuesto | Nuevo `docs/clinical-project-bridge-output.proposed.md`, 0.1.0-proposed, 56 líneas |
| SHA-256 candidato | `d12cdddce31040791d3be451e04205fe56b0d2a2f24b109d79deec597e49b9e4` |
| Diff mínimo | Adición de ese archivo solamente como instrucciones candidatas; sin reemplazo de la skill original |
| Aprobación | PENDING |
| Instalación | NOT INSTALLED |

La propuesta separa responsabilidades y remite al schema autoritativo en lugar de reescribir metodología. El archivo completo es la adición revisable; su versión es del complemento, no una versión inventada de la skill. Reversible descartando el candidato; no se eliminó ningún archivo. El snapshot de fuentes reales sigue intacto.

## 22. GPT_PROJECT_PACKAGE_V1 update

Se reemplazaron únicamente los placeholders de identidad general por versión 1.0/hash reales, se agregó estado de complemento PROPOSED_NOT_APPROVED_NOT_INSTALLED y se actualizó la conclusión de auditoría. Recibos de runtime y fuentes de pacientes siguen siendo placeholders de diseño; no se completaron con datos reales. Los informes anteriores conservan su veredicto histórico.

Inventario de esta etapa: NEW este informe, complemento propuesto y `skill_compatibility_test.go`; MODIFIED `docs/GPT_PROJECT_PACKAGE_V1.md`; PREEXISTING todo el resto del árbol. Ningún archivo productivo Go/TS, schema, prompt o fuente clínica original cambió. `git status` revisado y `git diff --check` PASS. No commit ni limpieza del working tree.

## 23. Stage 3 final status

Source Gate: PASS. Auditoría de fuente autorizada: COMPLETE. Metodología: preservada y compatible. Compatibilidad técnica del bridge: adición necesaria, propuesta pendiente. UI Stage 3A/3B/3C mantiene su aceptación previa; no se reetiqueta la regresión antigua como ejecutada en esta etapa.

```text
STAGE 3C = PARTIAL — REQUIRED SKILL PATCH PENDING
STAGE 3 FINAL READINESS = NOT READY
PRODUCTION DEPLOYMENT = BLOCKED UNTIL EXPLICIT CONFIGURATION/POLICY
```

El bloqueo anterior por falta de fuente está resuelto; el pendiente actual es aprobación del complemento. No se certificó ningún modelo clínico ni se verificó una instalación externa.

## 24. Next step

Revisión y aprobación explícita del complemento propuesto, conservando la skill 1.0. No se instala, sobrescribe ni inicia otra etapa automáticamente. Las reglas clínicas que Fernando proporcionó se usan en las conversaciones pertinentes de este hilo; esto no afirma haber cambiado instrucciones persistentes de otros chats o Projects.
