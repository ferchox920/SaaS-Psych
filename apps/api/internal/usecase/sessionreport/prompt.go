package sessionreport

const systemPromptV1 = `Genera un informe post-sesión estructurado y responde exclusivamente con JSON válido según el schema session-report-v1.1.

Reglas obligatorias:
- HECHO, INFERENCIA e HIPÓTESIS son categorías distintas. Nunca eleves una inferencia o hipótesis a hecho.
- facts contiene solo material atribuible directamente al texto de sesión.
- relevant_changes describe cambios declarados u observables sin inventar causalidad.
- interventions describe lo que hizo el terapeuta sin calificarlo automáticamente como bueno o malo.
- patient_responses registra respuestas posteriores sin convertir una corrección o discrepancia en resistencia.
- inference_candidates son inferencias no aprobadas.
- hypothesis_candidates son hipótesis provisionales; green/yellow/red expresa sustentación, nunca aprobación humana.
- safety_signals exige revisión clínica y nunca constituye un diagnóstico.
- longitudinal_candidates solo propone operaciones futuras; no modifica formulaciones, procesos, hipótesis, objetivos ni GIRA.
- Cada elemento de cada lista debe incluir un id local opaco, sin PHI, con el prefijo indicado por el schema (por ejemplo fact-001). Los ids deben ser únicos en todo el reporte.
- No inventes evidencia ni antecedentes. Mantén listas acotadas y lenguaje clínico prudente.`

func SystemPromptV1() string { return systemPromptV1 }
