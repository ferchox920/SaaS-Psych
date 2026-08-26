# Supervisor clínico local: operación y límites

## Alcance actual

SessionFlow ofrece un bloc de microprocesos en `/clinical-workspace`. El navegador envía el fragmento al backend autenticado de SessionFlow y el backend llama exclusivamente al Ollama configurado en loopback. No existe fallback a OpenAI ni a otro proveedor externo.

La fase actual permite seleccionar paciente y cita, escribir o pegar un fragmento, precargar `qwen3.5:9b`, cancelar una generación y recibir una salida clínica validada. Cada resultado se conserva en un ledger de sugerencias separado. Aceptar, corregir, descartar o posponer registra la decisión de Fernando, pero ninguna sugerencia se incorpora automáticamente a notas o formulaciones.

## Configuración

Variables disponibles:

```dotenv
OLLAMA_BASE_URL=http://127.0.0.1:11434
OLLAMA_MODEL=qwen3.5:9b
CLINICAL_RISK_PROTOCOL=Define aqui el circuito operativo local que Fernando quiere ver ante un posible indicador de riesgo.

# Perfil posterior, de menor prioridad que el modo en vivo
OLLAMA_REVIEW_CONTEXT_TOKENS=8192
OLLAMA_REVIEW_TEMPERATURE=0.15
OLLAMA_REVIEW_TIMEOUT_SECONDS=180
OLLAMA_REVIEW_MAX_OUTPUT_TOKENS=1024
OLLAMA_REVIEW_THINK=false
OLLAMA_CONTEXT_TOKENS=4096
OLLAMA_TEMPERATURE=0.1
OLLAMA_TIMEOUT_SECONDS=45
OLLAMA_KEEP_ALIVE=15m
OLLAMA_MAX_OUTPUT_TOKENS=384
```

El arranque rechaza endpoints de inferencia que no sean `localhost`, `127.0.0.0/8` o `::1`. El backend nunca descarga modelos: el modelo debe estar instalado previamente con autorización del usuario.

## Contrato y seguridad clínica

- Solo una asignación tratante activa permite analizar la cita.
- El prompt se versiona como `clinical-live-v1`.
- Ollama recibe `think:false`, temperatura baja, contexto acotado y un JSON Schema estricto.
- Solo se muestra el resultado después de validar JSON y aplicar guardas deterministas.
- Un semáforo rojo no puede confrontar.
- Un supuesto hecho sin evidencia directa se degrada a hipótesis roja.
- Se admiten como máximo cuatro evidencias y dos intervenciones.
- Una corrección explícita del paciente debilita una hipótesis verde y no se etiqueta como resistencia.
- Tras una confrontación se prioriza reflejar la respuesta antes de volver a confrontar.
- Un posible indicador de riesgo fuerza evaluación humana y suspende interpretación profunda o confrontativa.
- Cuando se activa riesgo, la UI muestra el texto de `CLINICAL_RISK_PROTOCOL`; no ejecuta llamadas, mensajes ni otras acciones externas.
- La reparación del formato está limitada a un único intento local.
- La concurrencia está limitada a una generación; las siguientes reciben estado ocupado.
- El contexto longitudinal procede solo de la versión de formulación aprobada en el servidor; el navegador no puede inyectar resúmenes consolidados.
- Una formulación exige entre dos y cuatro antecedentes etiquetados. Una sugerencia descartada o pendiente no puede citarse como fuente consolidada.

Los eventos de auditoría `clinical_ai.analysis.start`, `clinical_ai.analysis.complete` y `clinical_ai.analysis.failed` registran identificadores, versión, tiempos y clase de fallo, pero nunca el fragmento, prompt completo o transcripción.

## Streaming

`POST /api/v1/clinical-ai/analyze-live` usa Server-Sent Events:

- `status`: preparación y autorización;
- `progress`: caracteres generados y tiempo, sin contenido clínico;
- `result`: salida completa ya validada;
- `done`: finalización;
- `error`: fallo seguro sin revelar contenido.

No se transmite texto clínico parcial porque una interpretación incompleta todavía no ha superado las guardas.

## Límites conocidos

- No hay modo de revisión posterior todavía.
- No hay captura ni transcripción de audio; la política mantiene audio deshabilitado.
- El detector determinista de riesgo es una bandera conservadora, no una confirmación ni descarte clínico.
- El JSON estructurado consume tokens adicionales. En el equipo objetivo, `qwen3.5:9b` no cumple aún el objetivo ideal de completar en menos de siete segundos.
- La revision posterior usa un perfil separado y una solicitud en vivo cancela la revision activa para liberar el unico modelo local.
- La entrada completa de revision es transitoria; solo la salida validada se registra como sugerencia separada.

Solo deben utilizarse datos ficticios o completamente desidentificados hasta aprobar la calibración clínica y el gate correspondiente.
