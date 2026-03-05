# SessionFlow (Go)

SaaS multi-tenant para gestion de sesiones con arquitectura por capas (`domain/usecase/infra/http`), auth JWT + refresh token opaco con rotacion, RBAC, auditoria, rate limit Redis, observabilidad y CI.

## Architecture Overview

- `domain`: entidades y reglas de negocio.
- `usecase`: casos de uso y orquestacion (sin logica de negocio en handlers).
- `infra`: repositorios Postgres/Redis y componentes tecnicos.
- `http`: handlers, middleware y wiring de rutas Echo.

Flujo de request:
1. Middleware (`request_id`, tenant, auth, RBAC, rate limit).
2. Handler HTTP (parse/validate request).
3. Usecase (reglas + permisos + auditoria de dominio).
4. Repository (queries con `tenant_id`).

Progreso historico: [PROGRESS/PROGRESS_INDEX.md](./PROGRESS/PROGRESS_INDEX.md)

## Requisitos

- Go `1.24+`
- Docker + Docker Compose
- `make`

`migrate` CLI se instala/verifica con `make tools` (Linux/macOS).

## Setup rapido

Flujo recomendado (bootstrap reproducible):

```bash
make tools
make db-up
make migrate-up
make test
```

Luego correr API:

```bash
cd apps/api
go run ./cmd/server
```

Si queres setup paso a paso:

1. Configurar entorno (opcional con `.env`):

```bash
cp .env.example .env
```

2. Levantar infraestructura:

```bash
make db-up
```

3. Ejecutar migraciones + seeds:

```bash
make migrate-up DATABASE_URL="postgres://sessionflow:sessionflow@127.0.0.1:5432/sessionflow?sslmode=disable"
```

## Variables de entorno

Base (`.env.example`):

- `APP_ENV=local`
- `HTTP_PORT=8080`
- `DATABASE_URL=postgres://sessionflow:sessionflow@localhost:5432/sessionflow?sslmode=disable`
- `REDIS_URL=redis://localhost:6379`
- `JWT_ACCESS_SECRET=change-me`
- `ACCESS_TTL_MIN=15`
- `REFRESH_TTL_DAYS=30`
- `RATE_LIMIT_LOGIN_PER_MIN=10`
- `OTEL_SERVICE_NAME=sessionflow-api`
- `OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317`
- `OTEL_TRACES_EXPORTER=none` (`otlp` para exportar trazas)
- `OTEL_RESOURCE_ATTRIBUTES=deployment.environment=local`
- `OTEL_DB_STATEMENT_ENABLED=false`

Variables usadas en integration tests/CI:

- `RUN_PG_INTEGRATION=1` habilita tests Postgres opt-in.

## Docker Compose

Servicios locales:

- `postgres` en `localhost:5432`
- `redis` en `localhost:6379`
- `prometheus` en `http://localhost:9090`
- `alertmanager` en `http://localhost:9093`
- `grafana` en `http://localhost:3000` (admin/admin)
- `jaeger` en `http://localhost:16686`

Comandos utiles:

```bash
make db-up
make db-down
docker compose ps
docker compose logs -f postgres
docker compose logs -f redis
```

Stack observabilidad local (metrics + dashboards):

```bash
docker compose up -d prometheus alertmanager grafana
docker compose logs -f prometheus alertmanager grafana
```

## Migraciones y seeds

Comandos estandar (Makefile):

```bash
make tools
make db-up
make migrate-up
make migrate-down
make migrate-down-1
make migrate-status
```

Con DB explicita:

```bash
make migrate-up DATABASE_URL="postgres://sessionflow:sessionflow@127.0.0.1:5432/sessionflow?sslmode=disable"
```

Compatibilidad de `make tools`:

- Linux/macOS: instala/verifica `migrate` automaticamente (`scripts/install_migrate.sh`).
- Windows: usar instalacion manual (por ejemplo `choco install golang-migrate` o `scoop install migrate`) y luego ejecutar `make migrate-up`.

Seed demo:

- Se aplica via `000002_seed_demo.up.sql` al correr `migrate up`.
- Tenants:
  - `11111111-1111-1111-1111-111111111111` (`demo-tenant-a`)
  - `22222222-2222-2222-2222-222222222222` (`demo-tenant-b`)
- Usuarios demo:
  - `owner@tenant-a.local` / `ChangeMe123!` (role `owner`)
  - `member@tenant-b.local` / `ChangeMe123!` (role `member`)

Decision explicita: no hay auto-migrate en startup del server.

## Correr API

```bash
cd apps/api
go run ./cmd/server
```

Healthcheck:

```bash
curl http://localhost:8080/health
```

Metricas:

```bash
curl http://localhost:8080/metrics
```

## Metrics Stack (Prometheus + Grafana)

Archivos de configuracion:

- Prometheus scrape: `deploy/observability/prometheus.yml`
- Reglas de alertas Prometheus: `deploy/observability/prometheus-rules.yml`
- Alertmanager routing/receivers: `deploy/observability/alertmanager.yml`
- Grafana provisioning datasource/dashboard:
  - `deploy/observability/grafana/provisioning/datasources/datasource.yml`
  - `deploy/observability/grafana/provisioning/dashboards/dashboard.yml`
- Dashboard base API: `deploy/observability/grafana-dashboard.json`

Pasos:

1. Levantar API local en `:8080`.
2. Levantar stack:

```bash
docker compose up -d prometheus alertmanager grafana
```

3. Verificar scrape en Prometheus:
   - `http://localhost:9090/targets` (job `sessionflow-api` en `UP`)
4. Verificar reglas y alertas:
   - Reglas: `http://localhost:9090/rules`
   - Alertas: `http://localhost:9090/alerts`
   - Alertas configuradas:
     - `SessionFlowApiDown`: `up == 0` por `1m`
     - `SessionFlowHigh5xxErrorRate`: ratio 5xx > `5%` por `5m`
     - `SessionFlowHighP95Latency`: p95 > `750ms` por `10m`
5. Verificar Alertmanager:
   - UI: `http://localhost:9093`
   - Estado de alertas recibidas desde Prometheus: Alerts tab
6. Abrir Grafana:
   - `http://localhost:3000` (`admin`/`admin`)
   - Dashboard: `SessionFlow API Overview` (provisionado automaticamente)

Notas:

- Prometheus scrapea `host.docker.internal:8080/metrics`; por eso la API debe correr en host local en `8080`.
- El dashboard incluye paneles de latencia (`p50/p95`), tasa de requests por ruta/status y error rate `5xx`.
- Alertmanager esta configurado con un receiver webhook placeholder para pruebas locales.
- El receiver dummy puede fallar entrega (endpoint intencionalmente inexistente), pero permite validar el pipeline Prometheus -> Alertmanager de forma reproducible.

### Probar alertas localmente

Caso rapido para `SessionFlowApiDown`:

1. Detener la API local (proceso `go run ./cmd/server`).
2. Esperar ~1 minuto.
3. Abrir `http://localhost:9090/alerts` y confirmar alerta en estado `firing`.
4. Abrir `http://localhost:9093` y confirmar que Alertmanager recibe la alerta (`SessionFlowApiDown`).
5. Levantar la API nuevamente y verificar que la alerta vuelve a `inactive`/`resolved`.

## Correr tests

Suite general:

```bash
make test
```

Integration DB (Postgres real, opt-in):

```bash
make test-integration-db DATABASE_URL="postgres://sessionflow:sessionflow@127.0.0.1:5432/sessionflow?sslmode=disable"
```

Equivalente manual:

```bash
cd apps/api
RUN_PG_INTEGRATION=1 DATABASE_URL="postgres://sessionflow:sessionflow@127.0.0.1:5432/sessionflow?sslmode=disable" go test ./internal/http ./internal/infra/db
```

## Endpoints principales

Base URL: `http://localhost:8080`

Publicos:

- `GET /health`
- `GET /metrics`

Auth (`/api/v1/auth`, requiere `X-Tenant-ID`):

- `POST /login`
- `POST /refresh`
- `POST /logout`
- `GET /me` (JWT)
- `GET /admin-check` (JWT + owner/admin)

Clients (`/api/v1/clients`, JWT + tenant + role owner/admin/member):

- `POST /`
- `GET /`
- `GET /:id`
- `PUT /:id`
- `DELETE /:id`

Appointments (`/api/v1/appointments`, JWT + tenant + role owner/admin/member):

- `POST /`
- `GET /` (filtro por rango)
- `PUT /:id`
- `POST /:id/cancel`

Session notes:

- `POST /api/v1/appointments/:appointment_id/notes`
- `GET /api/v1/appointments/:appointment_id/notes`
- `GET /api/v1/notes/:id`
- `PUT /api/v1/notes/:id`

Audit (`/api/v1/audit`, JWT + tenant + role owner/admin):

- `GET /`

## API Docs (Swagger UI)

- UI local: `http://localhost:8080/docs`
- Spec OpenAPI servida por la API: `http://localhost:8080/docs/openapi.yaml`
- Fuente de la spec en repo: `docs/openapi.yaml`

## CI

Workflow: `.github/workflows/ci.yml`

Jobs:

- `test-build`: `go test ./...` + `go build ./cmd/server`
- `lint`: `golangci-lint` (`apps/api/.golangci.yml`)
- `integration-db`: Postgres + Redis + `migrate up` + tests con `RUN_PG_INTEGRATION=1`

## Tracing (OpenTelemetry)

Cobertura actual:

- HTTP (span por request en middleware Echo).
- Postgres (`pgx` Query/QueryRow/Exec).
- Redis rate limit login (`INCR+EXPIRENX`).

Variables OTEL:

```bash
OTEL_TRACES_EXPORTER=otlp
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
OTEL_SERVICE_NAME=sessionflow-api
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=local,service.version=dev
OTEL_DB_STATEMENT_ENABLED=false
```

Levantar stack local (collector + jaeger):

```bash
docker compose up -d otel-collector jaeger
```

Config del collector:

- `deploy/observability/otel-collector.yaml`
- receiver OTLP (`4317` gRPC / `4318` HTTP)
- exporter OTLP hacia `jaeger:4317`

Ver trazas en UI:

1. Levantar stack: `docker compose up -d otel-collector jaeger`
2. Levantar API con env OTEL activos.
3. Abrir `http://localhost:16686`
4. Seleccionar servicio `sessionflow-api`
5. Ejecutar "Find Traces"

Mini demo (genera spans de auth/clients/appointments):

```bash
# 1) Login
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: 11111111-1111-1111-1111-111111111111" \
  -d '{"email":"owner@tenant-a.local","password":"ChangeMe123!"}'

# 2) Copiar access_token del login en TOKEN y crear client
curl -s -X POST http://localhost:8080/api/v1/clients \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: 11111111-1111-1111-1111-111111111111" \
  -H "Authorization: Bearer TOKEN" \
  -d '{"fullname":"Trace Demo Client","contact":"demo@example.com","notes_public":"demo"}'

# 3) Listar clients
curl -s http://localhost:8080/api/v1/clients \
  -H "X-Tenant-ID: 11111111-1111-1111-1111-111111111111" \
  -H "Authorization: Bearer TOKEN"

# 4) Crear appointment (reemplazar CLIENT_ID)
curl -s -X POST http://localhost:8080/api/v1/appointments \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: 11111111-1111-1111-1111-111111111111" \
  -H "Authorization: Bearer TOKEN" \
  -d '{"client_id":"CLIENT_ID","starts_at":"2026-03-05T15:00:00Z","ends_at":"2026-03-05T16:00:00Z","location":"demo"}'
```

Troubleshooting:

- No aparecen trazas:
  - validar `OTEL_TRACES_EXPORTER=otlp`
  - validar `OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317`
  - revisar logs: `docker compose logs -f otel-collector jaeger`
- Error de conexion OTLP:
  - verificar puertos publicados (`4317`, `4318`, `16686`) con `docker compose ps`
  - si corres API en contenedor, usar endpoint interno del collector en vez de `localhost`
- Spans sin SQL:
  - esperado cuando `OTEL_DB_STATEMENT_ENABLED=false`
  - habilitar solo para debugging y en entornos controlados

Correlacion logs-traces:

- Cada log de request incluye:
  - `request_id`
  - `trace_id`
  - `span_id`
- Flujo recomendado de investigacion:
  1. buscar el request en logs por `request_id`
  2. copiar `trace_id`
  3. abrir Jaeger y filtrar por trace (servicio `sessionflow-api`)
  4. inspeccionar spans HTTP/DB/Redis de ese request

Ejemplo de log JSON (realista):

```json
{
  "time": "2026-03-05T02:10:13.921Z",
  "level": "INFO",
  "msg": "http_request",
  "request_id": "d47f3c18-0f7a-4f7e-a9e1-49e251702db1",
  "trace_id": "8c9b44c840a56a75f7e4b36f7fa2a2f1",
  "span_id": "9f6ab9f7845d1023",
  "method": "POST",
  "path": "/api/v1/auth/login",
  "status": 200,
  "latency_ms": 42,
  "tenant_id": "11111111-1111-1111-1111-111111111111"
}
```

Mini guion de demo para traza completa:

```bash
# 0) stack tracing + API con OTEL activo
docker compose up -d otel-collector jaeger

# 1) login (genera spans HTTP + DB + Redis rate limit)
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: 11111111-1111-1111-1111-111111111111" \
  -d '{"email":"owner@tenant-a.local","password":"ChangeMe123!"}'

# 2) buscar en logs el request_id/trace_id del login
# revisar stdout de la API (terminal donde corre `go run ./cmd/server`)

# 3) usar trace_id para localizar la traza en Jaeger UI
# http://localhost:16686
```

Trade-off `db.statement`:

- `OTEL_DB_STATEMENT_ENABLED=false` (recomendado): menor riesgo de exponer datos sensibles y menor costo de serializacion.
- `OTEL_DB_STATEMENT_ENABLED=true`: mejora debugging SQL pero puede aumentar cardinalidad/tamano de spans y riesgo de privacidad.

## Portfolio

Este proyecto demuestra:

- Multi-tenant real: aislamiento por `tenant_id` en middleware, usecases y repositorios.
- Auth robusta: access JWT + refresh token opaco hasheado con rotacion y revocacion.
- RBAC en endpoints de negocio (`owner/admin/member`).
- Rate limiting Redis en login (`/api/v1/auth/login`).
- Calidad: unit + integration tests, incluidos tests de aislamiento tenant.
- Observabilidad: request logging estructurado + metricas Prometheus (`/metrics`).
- CI: test, lint, build e integracion con servicios.

## Documentacion adicional

- Roadmap y estado por sprints: [SPRINTS.md](./SPRINTS.md)
- Historial de entregas: [PROGRESS/PROGRESS_INDEX.md](./PROGRESS/PROGRESS_INDEX.md)
- Diseno de refresh token opaco: [docs/AUTH_REFRESH_TOKEN_DESIGN.md](./docs/AUTH_REFRESH_TOKEN_DESIGN.md)
- Especificacion OpenAPI 3.0: [docs/openapi.yaml](./docs/openapi.yaml)
