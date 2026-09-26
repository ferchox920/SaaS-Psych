# RELEASE CHECKLIST - SessionFlow API

Checklist operativo para evaluar una entrega, no una certificación de producción. La CI de la PR de portafolio verifica automáticamente formato, tests, lint y build de Go; tests de integración con PostgreSQL y Redis; lint, tipos, build y tests de navegador del frontend; y tests del transcriptor Python. Consultar la ejecución de CI del commit evaluado, no asumir que el resultado de una ejecución anterior sigue vigente. Los controles manuales de este documento (smoke, métricas, alertas y trazas) no quedan aprobados por CI.

Este documento conserva casillas sin marcar para cada nueva evaluación. Ningún comando de recuperación que borre datos es un requisito para revisar la PR.

## 1) Variables de entorno requeridas

Base:
- `DATABASE_URL`
- `REDIS_URL`
- `JWT_ACCESS_SECRET`
- `ACCESS_TTL_MIN`
- `REFRESH_TTL_DAYS`
- `RATE_LIMIT_LOGIN_PER_MIN`

Integracion:
- `RUN_PG_INTEGRATION=1` (habilita tests de integracion con Postgres real)

Tracing/observabilidad:
- `OTEL_TRACES_EXPORTER` (`none` u `otlp`)
- `OTEL_EXPORTER_OTLP_ENDPOINT` (ej: `localhost:4317`)
- `OTEL_SERVICE_NAME`
- `OTEL_RESOURCE_ATTRIBUTES`
- `OTEL_DB_STATEMENT_ENABLED`

## 2) Matriz de cobertura minima por modulo

| Modulo | Cobertura minima requerida | Evidencia/Comando |
| --- | --- | --- |
| Auth + RBAC | Unit + integration (login/refresh/logout, guards) | `go test ./...` |
| Multi-tenant DB hardening | Constraints/FK compuestas + integration PG cross-tenant a nivel repository | `RUN_PG_INTEGRATION=1 go test -count=1 ./internal/infra/db` |
| Clients | Usecases + endpoints + integration tenant isolation | `go test ./internal/http ./internal/usecase/client` |
| Appointments | Regla no-solapamiento + lifecycle cancel + integration rango/aislamiento | `go test ./internal/http ./internal/usecase/appointment` |
| Session Notes | Privacidad + update permissions + integration | `go test ./internal/http ./internal/usecase/sessionnote` |
| Audit | Registro de eventos auth/dominio + endpoint listado | `go test ./internal/usecase/audit ./internal/http` |
| Rate limit Redis | Middleware login + tests de redis | `go test ./internal/infra/redis ./internal/http/middleware` |
| Observabilidad metrics | `/metrics` disponible + dashboard local | `curl http://localhost:8080/metrics` |
| Tracing OTel | spans HTTP/DB/Redis visibles en Jaeger | `docker compose up -d otel-collector jaeger` + ver UI |
| OpenAPI/Swagger | spec valida y UI accesible | `curl http://localhost:8080/docs/openapi.yaml` + `/docs` |

## 3) Checklist de release (Go/CI/calidad)

### A. Calidad estatica

- [ ] `go test ./...` pasa en limpio.
- [ ] `golangci-lint` sin errores (config en `apps/api/.golangci.yml`).
- [ ] `go build ./cmd/server` exitoso.

Comandos:

```bash
cd apps/api
go test ./...
go build ./cmd/server
```

Lint local (si tenes binario instalado):

```bash
cd apps/api
golangci-lint run --config .golangci.yml --timeout=3m
```

### B. Integracion Postgres/Redis

- [ ] Servicios arriba: Postgres + Redis.
- [ ] No hay contenedores legacy ocupando los puertos publicados `5433`/`6379` (PostgreSQL escucha en `5432` dentro del contenedor).
- [ ] Preflight local valida credenciales/estado de Postgres y respuesta de Redis.
- [ ] Migraciones aplicadas.
- [ ] Tests con `RUN_PG_INTEGRATION=1` pasan en local. CI ejecuta `go test -count=1 ./...` después de aplicar migraciones.

Comandos:

```bash
docker compose up -d postgres redis
make integration-preflight
make db-prepare
make test-integration-db
```

Si falla por puertos ocupados, identificar el proceso/contenedor responsable antes de cambiar nada:

```bash
docker ps --filter publish=5433 --filter publish=6379 --format "table {{.Names}}\t{{.Ports}}"
```

`make db-reset-local`, `make integration-recover-local` y `make test-integration-db-reset` eliminan el volumen `sessionflow_postgres_data` y todos sus datos. Son opciones excepcionales para un entorno local descartable, **solo** tras comprobar el volumen exacto y aceptar expresamente la pérdida de datos; no se ejecutan como preflight ni para resolver un puerto ocupado. `docker compose down --remove-orphans` puede detener otros servicios del proyecto: revisar primero `docker compose ps`.

### C. Contrato API (OpenAPI + Swagger)

- [ ] `/docs` responde 200 y muestra UI.
- [ ] `/docs/openapi.yaml` responde 200.
- [ ] Spec refleja endpoints actuales (sin drift conocido).

Comandos:

```bash
curl -i http://localhost:8080/docs
curl -i http://localhost:8080/docs/openapi.yaml
```

### D. Verificacion de consistencia documental

- [ ] `README.md`, `docs/DEMO.md` y `docs/openapi.yaml` describen el comportamiento actual, sin confundir el proveedor fijo de demo con análisis clínico real.
- [ ] Endpoints documentados coinciden con handlers/rutas actuales.
- [ ] Las diferencias detectadas se corrigen y se documenta la evidencia del commit evaluado.

Comandos sugeridos:

```bash
rg "PUT /api/v1/notes/:id|GET /api/v1/notes/:id|/appointments/:appointment_id/notes" README.md
rg "/api/v1/notes/\\{id\\}|/api/v1/appointments/\\{appointment_id\\}/notes" docs/openapi.yaml
rg "PUT|notes" apps/api/internal/http/handlers/session_note.go apps/api/internal/http/server.go
```

### E. Smoke test de endpoints principales

Usar tenant demo A:
- `X-Tenant-ID: 11111111-1111-1111-1111-111111111111`

- [ ] `GET /health`
- [ ] `POST /api/v1/auth/login`
- [ ] `GET /api/v1/auth/me` con JWT
- [ ] `POST/GET /api/v1/clients`
- [ ] `POST/GET /api/v1/appointments`
- [ ] `POST /api/v1/appointments/:id/cancel`
- [ ] `POST/GET/PUT session notes`
- [ ] `GET /api/v1/audit` (owner/admin)

Comandos base:

```bash
curl http://localhost:8080/health
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: 11111111-1111-1111-1111-111111111111" \
  -d '{"email":"owner@tenant-a.local","password":"ChangeMe123!"}'
```

### F. Verificacion de metricas

- [ ] `/metrics` expone metricas.
- [ ] Prometheus target `sessionflow-api` en `UP`.
- [ ] Reglas de alertas cargadas (`/rules`).

Comandos:

```bash
docker compose up -d prometheus grafana
curl -i http://localhost:8080/metrics
# UI: http://localhost:9090/targets
# UI: http://localhost:9090/rules
# UI: http://localhost:9090/alerts
```

### G. Verificacion de trazas

- [ ] OTel collector y Jaeger arriba.
- [ ] API con exporter OTLP activo (`OTEL_TRACES_EXPORTER=otlp`).
- [ ] Se ven trazas recientes en Jaeger (`sessionflow-api`).

Comandos:

```bash
docker compose up -d otel-collector jaeger
# Jaeger UI: http://localhost:16686
```

## 4) Criterios de Go/No-Go

Go para una entrega operativa:
- CI verde para el commit exacto que se entrega y controles manuales A-G verificados en el entorno de destino.
- Sin alertas `firing` inesperadas en Prometheus durante smoke.
- Sin errores 5xx no explicados en logs.

Una PR de portafolio con CI verde demuestra los controles automatizados, pero **no** equivale a un Go operativo ni certifica observabilidad o seguridad de producción.

No-Go:
- Fallo en tests/lint/build.
- Drift de OpenAPI/Swagger no resuelto.
- Fallas de integracion DB/Redis sin workaround documentado.
- Observabilidad rota (sin metricas o sin trazas, fuera de excepciones justificadas).

## 5) Referencias

- CI: `.github/workflows/ci.yml`
- Runbook operativo: `docs/RUNBOOK.md`
- OpenAPI: `docs/openapi.yaml`
- Roadmap/estado: `SPRINTS.md`
- Historial de ejecucion: `PROGRESS/PROGRESS_INDEX.md`
