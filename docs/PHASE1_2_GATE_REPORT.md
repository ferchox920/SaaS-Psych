# Gate de Fases 1–2 — Bloc local y contrato estructurado

Estado: **implementado técnicamente; no habilitado todavía para datos clínicos reales**.

## Criterios comprobados

- Abstracción `ClinicalInferenceProvider` separada del dominio de Ollama.
- Proveedor local restringido a loopback, sin descargas ni fallback externo.
- Estado, listado, precarga, descarga de memoria y análisis SSE expuestos solo por backend autenticado.
- Autorización por tenant y asignación tratante activa sobre la cita.
- Prompt clínico versionado a partir de `clinical_microprocess_supervisor_SKILL.md`.
- Salida JSON estricta, reparación local única y descarte seguro.
- Guardas para semáforo rojo, evidencia, corrección del paciente, post-confrontación, máximo de intervenciones y riesgo.
- Auditoría sin texto clínico.
- Interfaz con selección paciente–sesión, bloc, estado de Ollama, progreso, cancelación y decisiones no persistentes.
- OpenAPI y documentación operativa actualizados.

## Benchmark controlado

Fecha: 2026-08-10. Fixture completamente ficticio. Ollama en `127.0.0.1:11434`, `qwen3.5:9b` Q4_K_M, contexto 4096, `think:false`, temperatura 0.1, modelo precargado.

| Límite interno | Resultado | Primer token | Total | Velocidad | Reparación |
|---|---:|---:|---:|---:|---:|
| 180 tokens | salida truncada y descartada | — | 51,14 s con reparación fallida | — | fallida, un intento |
| 384 tokens | contrato válido | 1,34 s | 23,62 s | 11,67 tok/s | no |

Decisión: usar 384 tokens como techo interno para permitir completar el envoltorio JSON, manteniendo el contenido visible breve mediante prompt y límites de longitud. El objetivo de actividad en pocos segundos se cumple; el objetivo ideal de finalización menor a siete segundos no se cumple con el modelo y hardware actuales.

No se descargará un modelo menor automáticamente. Comparar otro modelo requerirá autorización explícita y una evaluación clínica equivalente.

## Pruebas

- Unitarias de configuración loopback.
- Unitarias de proveedor: esquema, `think:false`, streaming y reparación única.
- Integración local opt-in con Ollama y fixture ficticio.
- Unitarias de guardas clínicas y autorización tratante.
- Suite Go completa.
- ESLint y build de producción del frontend.
- Parseo de OpenAPI.

## Gate pendiente antes de datos reales

- Ejecutar un set más amplio de fixtures clínicos ficticios y revisión humana por Fernando.
- Definir y mostrar el protocolo de riesgo configurado por Fernando, no solo el aviso genérico actual.
- Implementar persistencia separada de sugerencias y decisiones en la Fase 3.
- Corroborar la experiencia end-to-end con una base local y usuarios de prueba.

Hasta cerrar esos puntos, usar únicamente casos ficticios o completamente desidentificados.
