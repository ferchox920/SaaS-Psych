# RUNBOOK - SessionFlow API

Runbook operativo minimo para respuesta a incidentes en entorno local/dev.

## 0) Precondiciones

- API corriendo en host: `http://localhost:8080`
- Stack local: Postgres, Redis, Prometheus, Alertmanager, Grafana, Jaeger, OTel Collector

Comandos base:

```bash
# Infra de datos
docker compose up -d postgres redis

# Observabilidad
docker compose up -d prometheus alertmanager grafana jaeger otel-collector

# Ver estado
docker compose ps
```

## 1) Healthchecks

Objetivo: confirmar disponibilidad basica y superficie de observabilidad.

```bash
# API
curl -i http://localhost:8080/health

# Metrics endpoint
curl -i http://localhost:8080/metrics
```

Esperado:
- `/health` => HTTP `200` y body `{"status":"ok"}`.
- `/metrics` => HTTP `200` y metricas Prometheus.

Checks adicionales:

```bash
# Targets Prometheus
# http://localhost:9090/targets

# Alertas Prometheus
# http://localhost:9090/alerts

# Alertmanager UI
# http://localhost:9093
```

## 2) Logs correlacionados (request_id / trace_id)

Objetivo: ubicar request fallido y correlacionarlo con trazas.

Campos clave en logs de request:
- `request_id`
- `trace_id`
- `span_id`
- `tenant_id`
- `user_id`
- `status`, `latency_ms`, `path`, `method`

Pasos:
1. Identificar error en cliente (status 4xx/5xx y hora aproximada).
2. Buscar en logs del proceso API por `request_id` o `path`.
3. Copiar `trace_id` para inspeccion de trazas (seccion 4).

Si API corre en terminal local, revisar stdout.
Si corre en contenedor, usar:

```bash
docker compose logs -f <api-service>
```

## 3) Metricas (requests / latency / errors)

Objetivo: confirmar si hay degradacion o incidente en curso.

Fuentes:
- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3000` (admin/admin)
- Dashboard: `SessionFlow API Overview`

Consultas utiles en Prometheus:

```promql
# Rate total de requests
sum(rate(requests_total[5m]))

# Rate de 5xx
sum(rate(requests_total{status=~"5.."}[5m]))

# Ratio de 5xx
sum(rate(requests_total{status=~"5.."}[5m])) / clamp_min(sum(rate(requests_total[5m])), 0.001)

# p95 global
histogram_quantile(0.95, sum(rate(request_duration_seconds_bucket[5m])) by (le))
```

Alertas versionadas (ver `deploy/observability/prometheus-rules.yml`):
- `SessionFlowApiDown`
- `SessionFlowHigh5xxErrorRate`
- `SessionFlowHighP95Latency`

Pipeline local de alertas:
- Prometheus evalua reglas y envia a Alertmanager (`alertmanager:9093`).
- Alertmanager enruta a receiver `dummy-webhook` definido en `deploy/observability/alertmanager.yml`.

Prueba minima (end-to-end):
1. Detener API local (`go run ./cmd/server`).
2. Esperar ~1 minuto (regla `SessionFlowApiDown`).
3. Verificar `firing` en `http://localhost:9090/alerts`.
4. Verificar recepcion en `http://localhost:9093`.
5. Levantar API y confirmar `resolved/inactive`.

## 4) Trazas en Jaeger

Objetivo: diagnosticar cuellos de botella/fallos por request.

1. Abrir `http://localhost:16686`.
2. Seleccionar servicio `sessionflow-api`.
3. Buscar por tiempo del incidente o por `trace_id` obtenido en logs.
4. Revisar spans HTTP + DB + Redis.

Interpretacion rapida:
- Latencia alta concentrada en span DB => revisar Postgres y queries.
- Latencia alta en span Redis => revisar disponibilidad/red Redis.
- Ausencia total de spans => revisar OTel exporter/collector.

## 5) Fallas comunes y acciones

### A) DB down (Postgres)

Sintomas:
- errores 5xx en endpoints de negocio.
- fallos de conexion DB en logs.
- aumento de `SessionFlowHigh5xxErrorRate`.

Acciones:

```bash
docker compose ps postgres
docker compose logs -f postgres
```

Si estaba caido:

```bash
docker compose up -d postgres
```

Verificar schema/migraciones:

```bash
make migrate-status
# si corresponde
make db-prepare
```

### B) Redis down

Sintomas:
- problemas en rate limit/login.
- errores Redis en logs.

Acciones:

```bash
docker compose ps redis
docker compose logs -f redis
```

Recuperacion:

```bash
docker compose up -d redis
```

Validar endpoint de login luego de recuperar.

### C) OTel Collector down

Sintomas:
- API funcional pero sin trazas nuevas en Jaeger.
- logs de export OTLP con errores.

Acciones:

```bash
docker compose ps otel-collector jaeger
docker compose logs -f otel-collector jaeger
```

Recuperacion:

```bash
docker compose up -d otel-collector jaeger
```

Verificar env de API:
- `OTEL_TRACES_EXPORTER=otlp`
- `OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317`

### D) Alertmanager down

Sintomas:
- Prometheus muestra alertas `firing`, pero no hay recepcion en `http://localhost:9093`.
- Logs de Prometheus con errores de envio hacia Alertmanager.

Acciones:

```bash
docker compose ps alertmanager prometheus
docker compose logs -f alertmanager prometheus
```

Recuperacion:

```bash
docker compose up -d alertmanager
```

Validar:
- `http://localhost:9093` disponible.
- En Prometheus (`/alerts`) las alertas firing vuelven a figurar con receptor activo.

## 6) Cierre de incidente (checklist)

- [ ] `/health` y `/metrics` en `200`.
- [ ] `docker compose ps` sin servicios criticos caidos.
- [ ] alertas en Prometheus vuelven a `inactive`.
- [ ] Alertmanager accesible y recibiendo alertas activas durante el incidente (si aplica).
- [ ] se identifica `request_id`/`trace_id` representativo del incidente.
- [ ] causa raiz y accion correctiva registradas en nota interna.

## 7) Comandos rapidos

```bash
# Estado general
docker compose ps

# Logs servicios clave
docker compose logs -f postgres redis prometheus alertmanager grafana otel-collector jaeger

# Test suite rapida
make test

# Test con DB integrada (opcional)
make test-integration-db
```
