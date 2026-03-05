# RELEASE CHECKLIST - SessionFlow API

Checklist minimo de release para asegurar calidad funcional, operativa y de observabilidad.

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
| Multi-tenant DB hardening | Constraints/FK compuestas + integration PG cross-tenant | `RUN_PG_INTEGRATION=1 go test ./internal/infra/db` |
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
- [ ] Migraciones aplicadas.
- [ ] Tests con `RUN_PG_INTEGRATION=1` pasan.

Comandos:

```bash
docker compose up -d postgres redis
make db-prepare
make test-integration-db
```

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

- [ ] `SPRINTS.md` y `PROGRESS/PROGRESS_INDEX.md` sin drift de estado para pasos cerrados.
- [ ] `README.md` (endpoints principales) consistente con `docs/openapi.yaml` y handlers/rutas actuales.
- [ ] Si hay diferencias, se corrigen antes de release y se registra evidencia en nuevo `PROGRESS/S<SPRINT>/S<SPRINT>_<STEP>.md`.

Comandos sugeridos:

```bash
rg "S10_10\\.[1-6]" SPRINTS.md PROGRESS/PROGRESS_INDEX.md
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

Go:
- Todos los checks A-G en verde.
- Sin alertas `firing` inesperadas en Prometheus durante smoke.
- Sin errores 5xx no explicados en logs.

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
