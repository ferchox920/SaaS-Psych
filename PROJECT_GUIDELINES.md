# SessionFlow (Go) - Directrices del Proyecto

> SaaS multi-tenant para gestion de sesiones (agenda + clientes + notas + auditoria), disenado para ser *portfolio-grade*: seguro, observable, testeable y con arquitectura clara.

---

## 1) Objetivo y Alcance

### Objetivo principal
Construir un sistema multi-tenant "production vibes" que demuestre:
- arquitectura limpia (capas, dominio, casos de uso)
- seguridad (auth + RBAC + aislamiento por tenant)
- calidad (tests + CI)
- operaciones (logs estructurados + metricas + tracing)

### Alcance MVP
- Multi-tenant con `tenant_id` en todas las tablas.
- Auth: login + refresh token (rotacion) + logout.
- RBAC: owner/admin/member.
- CRUD: Clients.
- Appointments con lifecycle state (sin delete): `scheduled -> canceled`.
- Session Notes (privadas) con permisos.
- Audit log (acciones clave).
- Rate limiting (Redis).
- Observabilidad basica (logs + metricas; tracing si entra).

### Fuera de alcance (por ahora)
- Facturacion/pagos.
- Notificaciones (email/whatsapp).
- IA (sin APIs pagas).
- Multi-db por tenant (schema/database per tenant).

---

## 2) Principios de Diseno (no negociables)

1. **Aislamiento por tenant siempre**
   - Toda consulta a DB debe filtrar por `tenant_id`.
   - `tenant_id` se obtiene del request (header en MVP).
2. **Dominio primero**
   - Entidades y reglas en `domain`.
   - Infraestructura no "contamina" el dominio.
3. **Casos de uso**
   - Cada feature relevante es un `usecase` claro (`CreateAppointment`, `ListClients`, etc.).
4. **Errores explicitos**
   - Tipos de error del dominio (`NotFound`, `Forbidden`, `ValidationError`).
5. **Logs estructurados**
   - JSON logs con `request_id`, `tenant_id`, `user_id`.
6. **Tests como contrato**
   - Unit tests para reglas de dominio.
   - Integration tests para endpoints + DB (docker).
7. **Documentacion viva**
   - Este archivo guia decisiones; el README final debe ser claro y ejecutable.

---

## 3) Stack y Herramientas

### Backend
- Go 1.22+
- Router: Echo o Fiber (elegir 1; recomendado: **Echo**)
- DB: Postgres
- Migraciones: golang-migrate (o goose)
- Queries: `sqlc` (recomendado) o `pgx` + hand-written queries
- Redis: rate limit + (opcional) sesiones/cache

### Observabilidad
- Logs: slog o zap (JSON)
- Metricas: Prometheus client
- Tracing: OpenTelemetry (opcional en MVP, ideal en Sprint 3)
- Stack local: Prometheus + Grafana (docker compose)

---

## 4) Arquitectura del Repositorio

Estructura recomendada:

```text
/apps/api
/cmd/server
/internal
/domain
/usecase
/infra
/db
/redis
/observability
/http
/handlers
/middleware
/presenters
/migrations
/docs
/deploy
docker-compose.yml
grafana/
```

### Responsabilidades por capa
- `domain`: entidades, value objects, validaciones, reglas.
- `usecase`: orquestacion (transacciones, llamadas a repos, permisos).
- `infra`: Postgres/Redis, logging, metricas, implementaciones concretas.
- `http`: handlers, request parsing, response mapping, middleware.

---

## 5) Modelo Multi-tenant

### Mecanismo en MVP
- Header obligatorio: `X-Tenant-ID: <uuid>`.
- Middleware:
  - valida que existe tenant
  - setea `TenantContext` en request context

> Evolucion futura: subdominios (`{tenant}.app.com`) o tokens con `tenant_id` embebido.

### Regla
**No existe** operacion sin `tenant_id`.

---

## 6) Seguridad

### Auth (minimo serio)
- Login (email + password)
- Access JWT (corto, ej 15 min)
- Refresh token opaco (largo, ej 30 dias) almacenado como hash en DB
- Refresh con rotacion (cada refresh invalida el anterior)
- Logout revoca refresh

### RBAC
Roles por tenant:
- `owner`: todo
- `admin`: gestion operativa + usuarios (opcional en MVP)
- `member`: agenda + clientes + notas (segun permisos)

Aplicacion:
- Middleware/Guard: `RequireRole(owner|admin|member)`
- Validaciones adicionales en usecase (defensa en profundidad)

### Notas privadas
- Si `is_private = true`: visible solo para autor (y opcional: admin).
- Decision MVP: visible solo autor + owner/admin (mas usable).

---

## 7) Entidades y Tablas

### Entidades base
- Tenant
- User
- Role / UserRole
- Client
- Appointment
- SessionNote
- AuditLog
- RefreshToken

### Campos minimos (todos con `tenant_id`)
- `tenant_id UUID NOT NULL`
- `id UUID PK`
- timestamps: `created_at`, `updated_at` (segun tabla)

---

## 8) Contratos HTTP (vision)

Convencion:
- `/api/v1/...`
- JSON
- Errores normalizados

Ejemplos:
- `POST /auth/login`
- `POST /auth/refresh`
- `POST /auth/logout`
- `GET /clients`
- `POST /clients`
- `GET /appointments?from=&to=`
- `POST /appointments`
- `POST /appointments/:id/cancel`
- `POST /appointments/:id/notes`
- `GET /audit?from=&to=&actor=`

Nota de diseno:
- Appointments no exponen endpoint `DELETE`.
- La eliminacion se modela como cancelacion para preservar historial clinico.

Errores (formato):

```json
{
  "error": {
    "code": "validation_error",
    "message": "starts_at must be before ends_at",
    "details": {"field": "starts_at"}
  }
}
```

---

## 9) Auditoria

### Que se audita (MVP)
- login/logout/refresh
- create/update/delete client
- create/update/cancel appointment
- create note (y update si existe)
- cambios de rol (si se implementa)

### Estructura
- `actor_user_id`
- `action` (string estable)
- `entity` + `entity_id`
- `metadata` (JSON: diffs basicos)

---

## 10) Rate Limiting

### MVP
- Redis token bucket o sliding window.
- Limites:
  - `/auth/login` por IP
  - endpoints de export (si existen) por user
  - general por user (opcional)

Debe ser configurable por env vars.

---

## 11) Observabilidad

### Logs (MVP)
Cada request loguea:
- `request_id`
- `tenant_id`
- `user_id` (si autenticado)
- `method`, `path`, `status`, `latency_ms`

### Metricas (Sprint 3 ideal)
- `requests_total{path,method,status}`
- `request_duration_ms` histogram
- `db_query_duration_ms` (opcional)

### Tracing (Sprint 3)
- trace por request
- spans para DB y Redis

---

## 12) Calidad: Tests y CI

### Tests
- Domain unit tests:
  - validacion de fechas en appointments
  - reglas de permisos para notas privadas
- Integration tests:
  - auth flow (login/refresh/logout)
  - CRUD con tenant isolation (un tenant no ve datos del otro)

### CI (GitHub Actions)
- `go test ./...`
- lint (`golangci-lint`)
- build docker (opcional)
- correr integration tests con servicios (postgres/redis) en CI si es posible

---

## 13) Variables de Entorno

Ejemplo:
- `APP_ENV=local`
- `HTTP_PORT=8080`
- `DATABASE_URL=postgres://...`
- `REDIS_URL=redis://...`
- `JWT_ACCESS_SECRET=...`
- `ACCESS_TTL_MIN=15`
- `REFRESH_TTL_DAYS=30`
- `RATE_LIMIT_LOGIN_PER_MIN=10`

---

## 14) Plan por Sprints (guia para Codex)

### Sprint 0 - Base
- repo + estructura
- docker compose (postgres + redis)
- migraciones iniciales
- healthcheck

### Sprint 1 - Auth + Tenant context + RBAC
- tenants, users, roles, refresh_tokens
- endpoints auth
- middleware tenant + auth + rbac
- tests de auth

### Sprint 2 - Clientes + Agenda
- CRUD clients
- appointments con lifecycle state (`scheduled -> canceled`) y no-solapamiento
- filtros por rango de fechas

### Sprint 3 - Notes + Auditoria + Rate limit
- session_notes + permisos
- audit_log
- rate limit redis

### Sprint 4 - Observabilidad + Pulido
- metricas + dashboards de Grafana
- tracing (opcional)
- README final + diagramas + seed demo

---

## 15) Reglas para trabajar con Codex

Cuando se pidan cambios:
1. Entregar salida tipo PR: que archivos crea/modifica.
2. Incluir:
   - codigo nuevo
   - migraciones si aplica
   - tests minimos
3. Actualizar documentacion (si toca arquitectura/contratos).
4. Mantener cada cambio pequeno y compilable.

Convencion de commits (sugerida):
- `feat(auth): ...`
- `feat(tenancy): ...`
- `test(...): ...`
- `chore(ci): ...`

---

## 16) Definition of Done (DoD)

Una feature esta lista si:
- compila
- tiene tests (unit o integration, segun corresponda)
- respeta `tenant_id` en todo
- logs no exponen datos sensibles
- endpoints documentados (aunque sea breve)
- no hay queries sin filtro tenant

---

## 17) Entregable final de Portfolio

- Demo local con `docker compose up`
- Seed data (2 tenants, usuarios, clientes, sesiones)
- Capturas de Grafana (si aplica)
- Postman collection u OpenAPI
- README para reclutador: que demuestra y por que importa

---

## Licencia y Notas

Este proyecto es para portfolio. Si se publica, evitar incluir datos sensibles reales.
