# Gate de Fase 4 — Transcripción local efímera

Estado: **implementado técnicamente; validación clínica acústica más amplia pendiente**.

## Gate técnico comprobado

- Comparación reproducible de faster-whisper y whisper.cpp con el mismo audio ficticio español.
- Selección documentada de faster-whisper `medium` CUDA/int8_float16, con fallback CPU/int8 y convivencia real comprobada con Qwen.
- Runtime y modelo locales, sidecar ligado a `127.0.0.1`, sin descarga automática.
- Micrófono iniciado explícitamente con confirmación de consentimiento e indicador visible.
- Pausa, continuación, finalización, descarte y cancelación.
- Fragmentos limitados a 30 segundos y recomendación de 8–30 segundos para evitar latencia desproporcionada y falta de contexto.
- Transcripción editable antes de enviarse manualmente a Qwen.
- Audio en memoria del navegador y archivo temporal del sidecar eliminado por defecto.
- Test automatizado de limpieza y prueba real con directorio temporal vacío.
- Tamaño máximo de audio, timeout, exclusión concurrente y errores seguros.
- Autorización tratante, tenant y auditoría sin audio ni texto.

## Restricción de uso

El WER 0 % corresponde a una única voz sintética limpia y no demuestra precisión clínica general. Antes de usar audio real deben evaluarse voces argentinas, velocidad espontánea, silencios, ruido y solapamiento, siempre con consentimiento y material ficticio/desidentificado durante la calibración.
