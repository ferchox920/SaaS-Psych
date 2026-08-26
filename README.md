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
- Node `22+`
- Corepack (`pnpm` se usa via `corepack pnpm`)
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

Si `make db-up` o `make migrate-up` falla porque ya existia un volumen local de Postgres con credenciales/datos de otra corrida, resetea solo la DB local y vuelve a bootstrapear:

```bash
make db-reset-local
make migrate-up
```

Si `docker compose up -d postgres redis` falla por nombres de contenedores ya existentes (`sessionflow-postgres`, `sessionflow-redis`) o por puertos ocupados por contenedores legacy, limpialos primero:

```powershell
docker ps -a --filter "name=sessionflow-postgres" --filter "name=sessionflow-redis"
docker rm -f sessionflow-postgres sessionflow-redis
docker compose up -d postgres redis
```

Luego correr API:

```bash
cd apps/api
go run ./cmd/server
```

Luego correr frontend web:

```bash
cp apps/web/.env.example apps/web/.env.local
corepack pnpm install
corepack pnpm --filter web dev
```

Frontend web:

- App Next.js en `apps/web`
- URL local: `http://localhost:3000`
- API esperada: `http://localhost:8080/api/v1`
- Login demo owner: tenant `11111111-1111-1111-1111-111111111111`, usuario `owner@tenant-a.local`, password `ChangeMe123!`

Arquitectura frontend inicial:

- `src/app`: App Router, layouts y paginas.
- `src/features`: slices por feature (`auth`, `dashboard`, `clients`, `appointments`, `session-notes`, `audit`).
- `src/components`: shell compartido y base shadcn/ui.
- `src/lib`: config, cliente HTTP y utilidades.
- `src/providers`: auth y TanStack Query.

## Troubleshooting local

El bootstrap es reproducible sobre un host limpio, pero `docker compose` persiste el estado local en volúmenes. Si ya existía un volumen de Postgres de una corrida anterior, puede haber drift entre las credenciales/datos esperados por el repo y las que quedaron persistidas en tu máquina.

Síntomas típicos:

- `make db-up` levanta `postgres`, pero `make migrate-up` falla con autenticación.
- `make integration-preflight` detecta credenciales incompatibles para `sessionflow/sessionflow`.
- El contenedor usa un volumen legacy de una versión anterior del stack.
- `docker compose up -d postgres redis` falla porque otro contenedor legacy ya ocupa `5432` o `6379`.
- `make integration-preflight` falla aunque el volumen sea correcto, porque hay contenedores huérfanos o externos publicando los mismos puertos.

Recuperación recomendada para entorno local:

```bash
make db-reset-local
make migrate-up
make integration-preflight
```

`make db-reset-local` borra el volumen local de Postgres y reconstruye `postgres` + `redis`. Es un comando destructivo y solo aplica a desarrollo local.

Si el problema no es el volumen sino un conflicto por contenedores legacy, inspeccionar primero qué proceso ocupa los puertos:

```bash
docker ps --filter publish=5432 --filter publish=6379 --format "table {{.Names}}\t{{.Ports}}"
docker compose down --remove-orphans
```

Si aparece un contenedor ajeno al stack actual, detenerlo o removerlo antes de repetir `make integration-preflight`.

### Windows PowerShell (sin `make`)

En este host Windows `make` puede no estar instalado. El flujo equivalente es:

```powershell
# 1) Limpiar contenedores legacy si existen
docker rm -f sessionflow-postgres sessionflow-redis

# 2) Reset destructivo de la DB local
docker compose down --remove-orphans
docker volume rm -f sessionflow_postgres_data
docker compose up -d postgres redis

# 3) Preflight manual
docker compose exec -T postgres pg_isready -U sessionflow -d sessionflow
docker compose exec -T postgres sh -lc "PGPASSWORD=sessionflow psql -U sessionflow -d sessionflow -c 'select 1' >/dev/null"
docker compose exec -T redis redis-cli ping

# 4) Migraciones
migrate -path apps/api/migrations -database "postgres://sessionflow:sessionflow@127.0.0.1:5432/sessionflow?sslmode=disable" up
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

Hardening baseline:

- `JWT_ACCESS_SECRET=change-me` es solo para `APP_ENV=local`.
- El servidor falla en startup si `APP_ENV` no es `local` y `JWT_ACCESS_SECRET` sigue en `change-me`.
- Usar un secreto unico y largo por entorno, inyectado desde el runtime o secret manager, no commiteado en el repo.
- Mantener `APP_ENV` alineado con el entorno real (`local`, `dev`, `staging`, `production`) para que las validaciones de arranque apliquen correctamente.
- En entornos expuestos, tratar `DATABASE_URL` y `REDIS_URL` faltantes como configuracion invalida operativamente: auth/persistencia o rate limit quedan degradados.
- Mantener `OTEL_DB_STATEMENT_ENABLED=false` salvo debugging temporal controlado, para evitar exponer SQL sensible en trazas.

Guia de referencia: [docs/ENV_HARDENING_BASELINE.md](./docs/ENV_HARDENING_BASELINE.md)

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
make db-reset-local
make integration-preflight
make integration-recover-local
docker compose ps
docker compose logs -f postgres
docker compose logs -f redis
```

`make db-reset-local` elimina el volumen local `sessionflow_postgres_data` y vuelve a levantar `postgres` + `redis`. Es un reset destructivo de la DB local y se debe usar solo cuando el volumen persistente quedo incompatible o queres reconstruir el entorno desde cero.

`make integration-preflight` ahora tambien falla temprano si detecta contenedores legacy usando `5432` o `6379`, antes de ejecutar migraciones o tests.

`make integration-recover-local` automatiza la recuperacion local: resetea el stack de datos, ejecuta `integration-preflight` y reaplica migraciones.

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
make db-reset-local
make integration-preflight
make integration-recover-local
make migrate-up
make migrate-down
make migrate-down-1
make migrate-status
```

Caso operativo conocido:

- Si Postgres arranca pero rechaza autenticacion para `sessionflow/sessionflow`, el volumen persistente local suele venir de una corrida anterior con otras credenciales.
- El flujo recomendado en ese caso es `make db-reset-local` y luego `make migrate-up`.
- Si tenias un volumen legacy de Compose creado antes del nombre estable `sessionflow`, puede quedar huerfano en Docker; ya no lo usa el stack actual, pero podes removerlo manualmente si queres limpiar el host.
- Si `docker compose up` falla porque `sessionflow-postgres` o `sessionflow-redis` ya existen, remover esos contenedores legacy antes de volver a levantar el stack.
- Si `docker compose` no puede publicar `5432` o `6379`, revisar primero contenedores legacy con `docker ps --filter publish=5432 --filter publish=6379`.

Con DB explicita:

```bash
make migrate-up DATABASE_URL="postgres://sessionflow:sessionflow@127.0.0.1:5432/sessionflow?sslmode=disable"
```

Compatibilidad de `make tools`:

- Linux/macOS: instala/verifica `migrate` automaticamente (`scripts/install_migrate.sh`).
- Windows: usar instalacion manual (por ejemplo `choco install golang-migrate` o `scoop install migrate`) y luego ejecutar `migrate -path apps/api/migrations -database "postgres://sessionflow:sessionflow@127.0.0.1:5432/sessionflow?sslmode=disable" up` o los equivalentes manuales del bloque PowerShell anterior.

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
make integration-preflight
make test-integration-db DATABASE_URL="postgres://sessionflow:sessionflow@127.0.0.1:5432/sessionflow?sslmode=disable"
make test-integration-db-reset DATABASE_URL="postgres://sessionflow:sessionflow@127.0.0.1:5432/sessionflow?sslmode=disable"
```

`make test-integration-db` usa el mismo alcance que CI: `go test -count=1 ./internal/http ./internal/infra/db`.

`make integration-preflight` valida antes del test que el contenedor `postgres` responde con las credenciales esperadas (`sessionflow/sessionflow`), que `redis` responde `PONG` y que no hay contenedores legacy ocupando `5432` o `6379`.

Si queres reconstruir el entorno local y volver a correr la integracion real en un solo flujo, usa:

```bash
make test-integration-db-reset
```

Si preferis la recuperacion paso a paso, el flujo recomendado sigue siendo:

```bash
make db-reset-local
make migrate-up
make integration-preflight
make test-integration-db
```

Si falla por conflicto de contenedores/puertos y no por credenciales, revisar:

```bash
docker ps --filter publish=5432 --filter publish=6379 --format "table {{.Names}}\t{{.Ports}}"
docker compose down --remove-orphans
```

Si el conflicto es por nombres legacy ya reservados, removerlos explicitamente:

```powershell
docker rm -f sessionflow-postgres sessionflow-redis
```

Equivalente manual:

```bash
cd apps/api
RUN_PG_INTEGRATION=1 DATABASE_URL="postgres://sessionflow:sessionflow@127.0.0.1:5432/sessionflow?sslmode=disable" go test ./internal/http ./internal/infra/db
```

Equivalente manual en PowerShell, sin `make`:

```powershell
docker compose exec -T postgres pg_isready -U sessionflow -d sessionflow
docker compose exec -T postgres sh -lc "PGPASSWORD=sessionflow psql -U sessionflow -d sessionflow -c 'select 1' >/dev/null"
docker compose exec -T redis redis-cli ping
migrate -path apps/api/migrations -database "postgres://sessionflow:sessionflow@127.0.0.1:5432/sessionflow?sslmode=disable" up
cd apps/api
$env:RUN_PG_INTEGRATION="1"
$env:DATABASE_URL="postgres://sessionflow:sessionflow@127.0.0.1:5432/sessionflow?sslmode=disable"
go test -count=1 ./internal/http ./internal/infra/db
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

## Supervisor clinico local

SessionFlow incluye un bloc de microprocesos en `/clinical-workspace`, formulacion longitudinal aprobada en `/clinical-formulation` y revision posterior en `/clinical-review`. El texto clinico se procesa desde el backend contra Ollama en loopback; no existe fallback a APIs externas. La captura de audio esta apagada por defecto y el sidecar de faster-whisper debe iniciarse explicitamente.

Configuracion minima local:

```dotenv
OLLAMA_BASE_URL=http://127.0.0.1:11434
OLLAMA_MODEL=qwen3.5:9b
CLINICAL_AUDIO_ENABLED=false
CLINICAL_RISK_PROTOCOL=Define aqui el circuito operativo local de Fernando.
```

Consulta `docs/LOCAL_CLINICAL_AI.md`, `docs/LOCAL_TRANSCRIPTION.md` y `docs/CLINICAL_EVALUATION_REPORT.md` antes de habilitar material clinico.

## Portfolio

Este proyecto demuestra:

- Multi-tenant real: aislamiento por `tenant_id` en middleware, usecases y repositorios.
- Guardrail automatico: test en `internal/infra/db` que detecta repositorios con acceso a DB sin referencia a `tenant_id`.
- Auth robusta: access JWT + refresh token opaco hasheado con rotacion y revocacion.
- RBAC en endpoints de negocio (`owner/admin/member`).
- Rate limiting Redis en login (`/api/v1/auth/login`).
- Calidad: unit + integration tests, incluidos tests de aislamiento tenant.
- Observabilidad: request logging estructurado + metricas Prometheus (`/metrics`).
- CI: test, lint, build e integracion con servicios.

## Documentacion adicional

- Roadmap y estado por sprints: [SPRINTS.md](./SPRINTS.md)
- Historial de entregas: [PROGRESS/PROGRESS_INDEX.md](./PROGRESS/PROGRESS_INDEX.md)
- ADR arquitectura API: [docs/adr/0001-api-architecture.md](./docs/adr/0001-api-architecture.md)
- Checklist de revision PR: [docs/PR_REVIEW_CHECKLIST.md](./docs/PR_REVIEW_CHECKLIST.md)
- Diseno de refresh token opaco: [docs/AUTH_REFRESH_TOKEN_DESIGN.md](./docs/AUTH_REFRESH_TOKEN_DESIGN.md)
- Especificacion OpenAPI 3.0: [docs/openapi.yaml](./docs/openapi.yaml)
- Operacion del supervisor local: [docs/LOCAL_CLINICAL_AI.md](./docs/LOCAL_CLINICAL_AI.md)
- Transcripcion local efimera: [docs/LOCAL_TRANSCRIPTION.md](./docs/LOCAL_TRANSCRIPTION.md)
- Sincronizacion con Google Calendar: [docs/GOOGLE_CALENDAR_SYNC.md](./docs/GOOGLE_CALENDAR_SYNC.md)
- Evaluacion ficticia reproducible: [docs/CLINICAL_EVALUATION_REPORT.md](./docs/CLINICAL_EVALUATION_REPORT.md)
