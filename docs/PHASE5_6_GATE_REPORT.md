# Gate de Fases 5 y 6 - Rendimiento y revision posterior

Estado: **implementado y verificado con material ficticio; calibracion clinica continuada requerida**.

## Modo sesion optimizado

- Qwen precargable, `think:false`, temperatura 0,1, contexto 4096 y streaming de actividad.
- Cancelacion desde UI y prioridad sobre revisiones posteriores.
- Salida JSON limitada, una reparacion local maxima y descarte seguro si vuelve a fallar.
- Una o dos intervenciones como maximo; escuchar o no intervenir son resultados validos.
- Metricas Prometheus sin datos clinicos: primer token, total, tokens, contexto, errores, reparaciones mediante auditoria y decisiones humanas.

## Revision posterior

- Ruta UI `/clinical-review` y endpoint SSE `/api/v1/clinical-ai/review-session`.
- Texto completo transitorio de 20 a 40000 caracteres; no se persiste como transcripcion o nota.
- Contexto longitudinal exclusivamente aprobado y preparado por el servidor.
- Perfil independiente: 8192 tokens de contexto, temperatura 0,15, salida maxima 1024 y `think` configurable.
- Evalua formulacion, intervenciones, calibracion, respuesta, alianza, patrones del terapeuta y proximos focos.
- Salida validada persiste solo como sugerencia `review`, con aceptar/corregir/descartar/posponer.
- Una solicitud en vivo cancela la revision activa y adquiere el modelo en un maximo de 750 ms.
- Riesgo detectado por texto o modelo fuerza evaluacion humana y reemplaza los focos profundos.
- La UI muestra el protocolo operativo definido por Fernando en `CLINICAL_RISK_PROTOCOL`, sin automatizar acciones externas.

## Gate comprobado

- Suite Go completa.
- Lint y build web offline.
- OpenAPI parseable.
- Test unitario de preempcion revision -> vivo.
- Evaluacion real de cinco fixtures: 5/5 resultados contractualmente seguros, 5/5 JSON sin reparacion.
- Revision real ficticia: 58,56 s total, salida valida sin reparacion.

## Limites conocidos

- La GPU de 6 GB y el modelo de 6,6 GB producen offload mixto; el tiempo total en vivo sigue sobre el ideal de 7 s.
- Las pruebas ficticias no sustituyen supervision humana ni una evaluacion con habla y sesiones desidentificadas representativas.
- `OLLAMA_REVIEW_THINK=true` esta disponible, pero viene apagado porque aumenta latencia y debe compararse antes de adoptarlo.
