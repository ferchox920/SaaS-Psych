# Plan de implementación: supervisor clínico local

## Objetivo

Evolucionar SessionFlow hacia un supervisor clínico local de microprocesos que permita capturar texto o audio, transcribirlo localmente, analizarlo con Ollama y `qwen3.5:9b`, y someter cada sugerencia al juicio explícito de Fernando. Ningún contenido clínico debe enviarse a proveedores externos.

## Estado comprobado al inicio

- La API es Go/Echo con capas de dominio, casos de uso e infraestructura.
- El frontend es Next.js y ya permite seleccionar citas y crear notas de sesión.
- No existe integración de IA, audio ni transcripción.
- Ollama está instalado en Windows y expone `qwen3.5:9b` cuando está activo.
- El modelo está cuantizado como Q4_K_M, se reparte entre CPU/GPU y admite texto, visión, herramientas y razonamiento; no admite audio.
- El benchmark inicial de respuesta compacta fue 5,44 s y 13,82 tokens/s con 4096 tokens de contexto, pero la salida incumplió el formato clínico. La calibración es un requisito, no una optimización opcional.
- El worktree contiene cambios previos del usuario y un frontend nuevo todavía no versionado. Toda implementación debe preservar esos cambios.

## Hallazgos que condicionan el orden

1. El frontend crea notas compartidas por defecto aunque PostgreSQL declare privacidad por defecto.
2. `owner` y `admin` pueden leer y sobrescribir notas privadas ajenas solo por su rol administrativo.
3. Las notas se reescriben sin conservar versión o adenda.
4. El borrado físico de clientes puede borrar citas y notas por cascada.
5. Las lecturas clínicas no se auditan y los fallos de escritura de auditoría de dominio se ignoran.
6. El refresh token está disponible para JavaScript y persistido en `localStorage`.
7. No existe separación entre transcripción, nota profesional, formulación, sugerencia de IA y decisión humana.

Estos puntos hacen que el hardening clínico sea una dependencia de la incorporación de información real, aunque el adaptador de Ollama puede desarrollarse y probarse en paralelo únicamente con datos ficticios.

## Decisiones de arquitectura propuestas

### Frontera local

```text
Navegador -> API SessionFlow -> Ollama 127.0.0.1:11434
                         \-> transcriptor local
```

- El navegador nunca llama directamente a Ollama.
- La API no contiene fallback a OpenAI u otro proveedor.
- Ollama y el servicio de transcripción se enlazan a loopback por defecto.
- Los logs registran métricas y estados, nunca prompts, transcripciones ni respuestas clínicas completas.

### Separación de datos

Mantener entidades distintas para:

- sesión clínica;
- fragmento/transcripción;
- nota clínica versionada o adenda;
- elemento de formulación longitudinal;
- evidencia de formulación;
- ejecución de supervisión;
- hallazgo generado;
- decisión humana sobre el hallazgo;
- evento de riesgo;
- consentimiento/configuración de audio.

### Proveedor de inferencia

Definir un puerto de aplicación equivalente a:

```text
ClinicalInferenceProvider
  Health
  ListModels
  WarmModel
  AnalyzeLive
  ReviewSession
  UnloadModel
```

La primera implementación será `OllamaClinicalInferenceProvider`. El contrato recibirá contexto clínico ya reducido y devolverá eventos de streaming más un resultado estructurado validado.

### Defensa clínica en profundidad

No confiar solo en el prompt. Aplicar validaciones deterministas después de la generación:

- `red` no admite `confront`;
- un riesgo que requiere evaluación humana anula interpretaciones profundas e intervenciones confrontativas;
- una hipótesis no puede marcarse como hecho;
- en vivo se permiten como máximo dos intervenciones;
- evidencia inexistente no puede presentarse como sustentación longitudinal;
- una salida inválida produce un estado seguro y un único intento acotado de reparación.

## Fases y gates

### Fase 0 — Hardening clínico y privacidad

**Alcance**

- Hacer privadas las notas por defecto en UI y contrato HTTP.
- Separar capacidad administrativa de acceso/edición clínica.
- Diseñar notas inmutables con versiones o adendas.
- Sustituir el borrado clínico por archivo lógico y eliminar cascadas destructivas de historia.
- Auditar lecturas, cambios y accesos excepcionales.
- Hacer que fallos críticos de auditoría sean observables y definir cuándo deben abortar la operación.
- Migrar refresh token a cookie `HttpOnly`, `Secure` en producción y `SameSite`, manteniendo access token corto fuera de almacenamiento persistente.
- Documentar retención y tratamiento de audio; audio no persistente por defecto.

**Gate de aceptación**

- Pruebas de permisos demuestran que un rol administrativo sin relación clínica no accede ni edita contenido.
- Cada modificación clínica conserva autor, fecha, contenido anterior y motivo/adenda.
- Archivar un paciente no elimina su historia.
- Toda lectura y cambio relevante deja auditoría tenant-aware.
- Ningún refresh token es legible mediante JavaScript.

### Fase 1 — Bloc de texto y Ollama local

**Alcance**

- Agregar configuración `OLLAMA_BASE_URL`, `OLLAMA_MODEL`, contexto, temperatura, timeout, keep-alive y límites.
- Implementar health, listado de modelos, precarga, cancelación y errores tipados.
- Crear endpoints autenticados y tenant-aware para estado y análisis ficticio/manual.
- Agregar espacio de trabajo con selector de paciente/cita, bloc y botón `Analizar localmente`.
- Implementar streaming desde Ollama a través de la API.
- No persistir automáticamente entrada ni salida.

**Gate de aceptación**

- La UI distingue Ollama detenido, modelo ausente, carga, listo, ocupado, timeout y cancelación.
- El navegador no conoce el endpoint de Ollama.
- No existe tráfico a proveedores externos.
- Un análisis compacto con modelo caliente se completa idealmente en menos de 7 s en el hardware objetivo y se registra el benchmark sin contenido clínico.

### Fase 2 — Contrato estructurado y calibración

**Alcance**

- Versionar un prompt de sistema reducido a partir de `clinical_microprocess_supervisor_SKILL.md`.
- Definir DTO/esquema estructurado para modo vivo y revisión.
- Validar, reparar una sola vez y aplicar guardas clínicas deterministas.
- Renderizar Nodo, Hipótesis, nivel epistemológico, Semáforo, Ahora, Cuidado, intervenciones, evidencia, meta-terapeuta y riesgo.
- Construir fixtures ficticios para contradicciones, correcciones, confrontación, no intervención y riesgo.

**Gate de aceptación**

- El 100% de respuestas mostradas cumplen el esquema o se reemplazan por un estado seguro explícito.
- Ningún fixture rojo termina en confrontación directa.
- Ningún fixture de riesgo muestra interpretación profunda antes de evaluación humana.
- Las correcciones del paciente modifican o debilitan la hipótesis.

### Fase 3 — Formulación longitudinal

**Alcance**

- Crear sesiones, fragmentos, formulaciones, evidencias, ejecuciones, hallazgos y decisiones con aislamiento por tenant.
- Permitir `accept`, `revise`, `discard` y `postpone`.
- Construir contexto solo con material aprobado y 2–4 anclajes pertinentes.
- Impedir que material descartado reaparezca como antecedente consolidado.

**Gate de aceptación**

- Toda afirmación longitudinal mostrada enlaza evidencia concreta.
- La nota profesional permanece separada de las sugerencias.
- Se puede reconstruir la evolución y decisión humana de cada hipótesis.

### Fase 4 — Transcripción local

**Alcance**

- Evaluar `faster-whisper` y `whisper.cpp` con el mismo audio ficticio en español.
- Medir precisión cualitativa, tiempo, RAM/VRAM, convivencia con Ollama y complejidad de operación en Windows.
- Documentar la decisión y encapsular el transcriptor detrás de una interfaz.
- Implementar captura iniciada explícitamente, indicador visible, pausa/finalización y revisión antes de analizar.
- Eliminar audio temporal al completar o cancelar, salvo consentimiento/configuración explícitos.

**Gate de aceptación**

- La aplicación nunca activa el micrófono sin acción explícita.
- El audio no se conserva por defecto y la limpieza se prueba.
- Se puede editar la transcripción antes de enviarla a Qwen.
- No hay conexión externa durante captura, transcripción o análisis.

### Fase 5 — Modo sesión optimizado

**Alcance**

- Mantener modelo caliente durante una sesión activa.
- Usar `think: false`, salida de 80–150 tokens, temperatura baja y contexto compacto.
- Mostrar actividad inmediata, streaming y cancelación.
- Implementar cola o exclusión para no saturar el hardware.

**Gate de aceptación**

- La UI permanece operable mientras Ollama genera.
- Se registran tiempo al primer token, tiempo total, throughput, contexto, cancelación y errores sin contenido sensible.
- Las regresiones de latencia y calidad se comparan contra una línea base versionada.

### Fase 6 — Revisión posterior y evaluación

**Alcance**

- Agregar revisión profunda fuera del camino crítico del modo vivo.
- Evaluar formulación, intervenciones, alianza, calibración, respuesta del paciente, patrones del terapeuta y próximos focos.
- Crear harness de evaluación clínica con material ficticio/desidentificado.
- Medir cumplimiento, calibración, evidencia, no intervención, correcciones, riesgo y decisiones humanas.

**Gate de aceptación**

- La revisión no bloquea el modo vivo.
- Los resultados incluyen procedencia y versión de prompt/modelo.
- Un reporte reproducible compara calidad y rendimiento entre configuraciones.

## Dependencias principales

```text
Fase 0 ───────────────┐
                     ├─> Fase 3 ─> Fase 6
Fase 1 ─> Fase 2 ────┤
          └───────────┴─> Fase 5
Fase 1 ─> Fase 4 ───────> Fase 5
```

Fase 1 puede probarse con material ficticio mientras termina Fase 0. Ningún dato clínico real debe entrar en el nuevo flujo hasta superar el gate de Fase 0.

## Riesgos técnicos

- `qwen3.5:9b` comparte VRAM con el transcriptor y ya descarga parte del trabajo a CPU.
- Contextos longitudinales grandes pueden superar el objetivo de latencia aunque la salida sea corta.
- El modelo inicial incumplió el formato y cerró una interpretación roja; se necesitan esquema, validadores y evaluación.
- La transcripción y Ollama simultáneos pueden provocar swapping o descarga adicional a CPU.
- Las migraciones de historia clínica y autenticación afectan contratos existentes y requieren estrategia de compatibilidad.
- El worktree actual está sucio; los cambios deben aislarse y nunca revertir trabajo previo.

## Decisiones que requieren aprobación de Fernando

1. **Modelo de acceso clínico:** confirmar si solo el autor ve/edita, o si habrá profesionales tratantes y supervisores explícitamente asignados.
2. **Correcciones:** elegir entre versiones completas o nota cerrada más adendas. Se recomienda nota cerrada más adendas para la historia definitiva y borradores versionados antes del cierre.
3. **Archivo de pacientes:** confirmar que no habrá borrado físico desde UI y definir quién puede archivar/restaurar.
4. **Auditoría crítica:** confirmar que una nota no debe considerarse guardada si falla su evento de auditoría.
5. **Audio:** confirmar que la primera versión será efímera y sin opción de conservación, reduciendo superficie de riesgo.
6. **Orden de trabajo:** aprobar Fase 0 como primera implementación productiva y permitir Fase 1 en paralelo solo con datos ficticios.

## Verificación final

Antes de declarar el objetivo completo se realizará una auditoría requisito por requisito contra:

- migraciones y esquema real;
- pruebas unitarias e integración con Postgres;
- aislamiento multi-tenant;
- contratos OpenAPI;
- pruebas frontend, lint y build;
- tráfico de red observado durante el flujo local;
- ejecución real con Ollama y transcriptor;
- benchmarks reproducibles;
- fixtures de seguridad clínica;
- trazabilidad completa de lectura, cambio, generación y decisión humana.

