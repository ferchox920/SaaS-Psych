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

Preflight recomendado antes de tests de integracion locales:

```bash
make integration-preflight
```

Este comando detecta temprano drift del entorno local: valida conflictos por contenedores legacy usando `5433/6379`, health + credenciales de Postgres (`sessionflow/sessionflow`) y conectividad basica a Redis.

Si estas en Windows sin `make`, el equivalente manual es:

```powershell
docker compose up -d postgres redis
docker compose exec -T postgres pg_isready -U sessionflow -d sessionflow
docker compose exec -T postgres sh -lc "PGPASSWORD=sessionflow psql -U sessionflow -d sessionflow -c 'select 1' >/dev/null"
docker compose exec -T redis redis-cli ping
```

Si el entorno local ya quedo inconsistente y queres reconstruirlo antes de volver a testear, usar:

```bash
make integration-recover-local
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

# Citas creadas exitosamente
sum(rate(appointments_created_total{result="success"}[5m]))

# Citas canceladas con error por causa
sum by (reason) (rate(appointments_canceled_total{result="error"}[5m]))

# Errores auth por endpoint y causa
sum by (endpoint, reason) (rate(auth_errors_total[5m]))
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

Si el contenedor arranca pero falla autenticacion con `sessionflow/sessionflow`, asumir primero drift del volumen local antes que un bug de aplicacion:

```bash
make db-reset-local
make migrate-up
```

Impacto:
- `make db-reset-local` borra el volumen local `sessionflow_postgres_data`.
- Debe usarse solo en entorno local cuando se acepta perder el estado persistido de Postgres.

Si `docker compose up -d postgres redis` o `make integration-preflight` falla por puertos ocupados, revisar contenedores legacy antes de tocar volumenes:

```bash
docker ps --filter publish=5433 --filter publish=6379 --format "table {{.Names}}\t{{.Ports}}"
docker compose down --remove-orphans
```

Si el conflicto es por nombres reservados de contenedores legacy:

```powershell
docker rm -f sessionflow-postgres sessionflow-redis
docker compose up -d postgres redis
```

Reset equivalente sin `make` en Windows:

```powershell
docker compose down --remove-orphans
docker volume rm -f sessionflow_postgres_data
docker compose up -d postgres redis
migrate -path apps/api/migrations -database "postgres://sessionflow:sessionflow@127.0.0.1:5433/sessionflow?sslmode=disable" up
```

Solo despues de descartar conflicto de contenedores conviene usar `make db-reset-local`.

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
make integration-preflight
make test-integration-db

# Recuperacion local + test integrado real
make test-integration-db-reset
```

Estandar actual local/CI para integracion DB:
- alcance: `./internal/http ./internal/infra/db`
- flags: `go test -count=1`
- env: `RUN_PG_INTEGRATION=1`
# IP cliente y rate limiting

El API utiliza la IP de la conexión TCP para el rate limit de login. Si se despliega detrás de un proxy de confianza, configurar `TRUSTED_PROXY_CIDRS` con sus CIDR separados por comas y hacer que el proxy de borde elimine los encabezados `X-Forwarded-For` entrantes antes de establecer el suyo. Sin esa configuración, los encabezados enviados por clientes no influyen en `RealIP`; detrás de un proxy, el límite se aplica a la IP del proxy.

El servidor limita solicitudes JSON a 1 MiB (también con transferencia fragmentada), sin aplicar ese límite a audio binario, que tiene su propio límite. Los tiempos máximos son 10 s para cabeceras, 60 s para leer una solicitud y 120 s de conexión inactiva. No se configura un timeout global de escritura porque los análisis clínicos usan SSE; el proveedor y las operaciones largas conservan sus propios deadlines.
