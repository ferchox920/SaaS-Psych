# Gate de Fase 3 — Memoria longitudinal separada

Estado: **implementado técnicamente; calibración clínica con datos ficticios todavía requerida**.

## Separación de registros

- `session_notes`: nota clínica profesional y sus versiones.
- `clinical_ai_suggestions`: salida validada de la IA, sin almacenar el fragmento de entrada.
- decisión sobre la sugerencia: `accepted`, `corrected`, `discarded` o `postponed`, con corrección separada cuando corresponde.
- `clinical_formulation_snapshots`: formulación longitudinal versionada como borrador, aprobada o superseded.
- `clinical_formulation_anchors`: dos a cuatro hechos o hipótesis concretas, con semáforo obligatorio para hipótesis.

Aceptar o corregir una sugerencia no la promueve automáticamente. Para citarla como fuente de una formulación, Fernando debe crear un borrador explícito; el backend rechaza fuentes pendientes, pospuestas o descartadas. Solo la formulación aprobada alimenta a Qwen.

## Integridad y acceso

- Tratante: crea y decide sugerencias; crea y aprueba formulaciones.
- Supervisor: lectura de sugerencias y formulaciones, sin escritura.
- Owner/admin sin asignación clínica: sin acceso al contenido.
- Las lecturas se autorizan antes de recuperar contenido y se auditan sin contenido clínico.
- Creación, decisiones y aprobación se auditan dentro de la misma transacción.
- Las aprobaciones y la asignación de versiones se serializan por tenant/paciente mediante advisory lock.
- Nunca se hard-deletean sugerencias ni formulaciones.

## Contexto enviado al modelo

El backend ignora cualquier resumen o antecedente longitudinal enviado desde el navegador. Antes de analizar, carga como máximo la versión aprobada y sus dos a cuatro anclajes, preservando `fact` e `hypothesis`. Los borradores y toda sugerencia no consolidada quedan fuera.

## Evidencia técnica

- Migración limpia 1–15 en PostgreSQL 16.
- Integración PostgreSQL del ciclo sugerencia → decisión → borrador → aprobación → contexto.
- Auditoría transaccional verificada.
- Suite PostgreSQL de HTTP e infraestructura completa aprobada contra migraciones 1–15.
- Tests unitarios prueban autorización antes de leer contenido y exclusión de sugerencias descartadas.
- Frontend ofrece ledger de decisión dentro del bloc y editor/historial de formulaciones.

## Pendientes del gate clínico global

- Revisión humana de fixtures longitudinales por Fernando.
- Modo de revisión posterior, que consumirá esta memoria sin bloquear el modo vivo.
- Evaluación de calidad que compruebe que una sugerencia descartada nunca reaparece.

Hasta cerrar el gate global, utilizar solo casos ficticios o totalmente desidentificados.
