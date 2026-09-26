# SessionFlow

[![CI](https://github.com/ferchox920/SaaS-Psych/actions/workflows/ci.yml/badge.svg)](https://github.com/ferchox920/SaaS-Psych/actions/workflows/ci.yml)

SessionFlow es una aplicación multi-tenant para organizar pacientes, citas y sesiones clínicas. Sus informes y propuestas asistidas por IA requieren revisión humana antes de incorporarse a la memoria longitudinal. Es un proyecto de portafolio técnico: la demostración usa exclusivamente datos ficticios y el software **no sustituye el juicio clínico**.

Descripción breve para GitHub: *Flujo clínico multi-tenant con revisión humana, auditoría y demo ficticia reproducible.* Topics propuestos: `go`, `nextjs`, `postgresql`, `multi-tenant`, `human-in-the-loop`, `clinical-workflow`, `portfolio-project`. La metadata remota se publica por separado del código.

## Alcance evaluable

El recorrido conecta paciente → cita → sesión → informe → edición y aprobación de una revisión concreta → propuesta longitudinal → decisión por operación → merge → estado y procedencia. Incluye autenticación JWT con rotación transaccional de refresh tokens, asignaciones clínicas independientes del rol administrativo, auditoría, observabilidad y pruebas de concurrencia sobre PostgreSQL real.

Audio y transcripción local son opcionales y están desactivados por defecto. El análisis libre en vivo necesita Ollama local. El modo demo usa un proveedor fijo, claramente simulado: **no analiza el texto ni produce inferencias clínicas**. La integración remota experimental no forma parte del recorrido demo.

## Capturas reales

Tomadas del frontend contra API, PostgreSQL y seed ficticio locales; no son maquetas.

| Acceso | Panel | Revisión humana |
| --- | --- | --- |
| ![Acceso de demo](docs/screenshots/01-login.png) | ![Panel de demo](docs/screenshots/02-dashboard.png) | ![Revisión clínica de demo](docs/screenshots/03-clinical-review.png) |

| Pacientes | Citas | Sesión clínica | Auditoría |
| --- | --- | --- | --- |
| ![Pacientes ficticios](docs/screenshots/02a-patients.png) | ![Cita ficticia vinculada](docs/screenshots/02b-appointments.png) | ![Sesión clínica ficticia completada](docs/screenshots/02c-session.png) | ![Auditoría del merge humano](docs/screenshots/04-audit.png) |

## Demo reproducible

Siga [la guía de demostración](docs/DEMO.md) para instalar dependencias, migrar, cargar el seed opt-in y arrancar API y web. La cuenta tratante local es tenant `11111111-1111-1111-1111-111111111111`, usuario `therapist@tenant-a.local`, contraseña `ChangeMe123!`. También hay un usuario no asignado y otro tenant para comprobar denegaciones. **No use estas credenciales ni el seed en un entorno expuesto.**

La migración `000029` invalida contraseñas demo conocidas creadas por una migración histórica; solo el seed local opt-in las restablece para la demo.

En Linux/macOS, con Go 1.24+, Node 22+, Corepack, Docker Compose y make:

```bash
cp .env.example .env
cp apps/web/.env.example apps/web/.env.local
make tools
make demo-prepare
cd apps/api && APP_ENV=local DEMO_MODE=true go run ./cmd/server
```

En otra terminal:

```bash
NEXT_PUBLIC_DEMO_MODE=true corepack pnpm install --frozen-lockfile
NEXT_PUBLIC_DEMO_MODE=true corepack pnpm --filter web dev
```

Abra http://localhost:3000. En PowerShell use los comandos de [docs/DEMO.md](docs/DEMO.md). PostgreSQL local usa **5433** y Redis **6379**. Ollama y el transcriptor no son necesarios para el informe y la propuesta simulada; sin ellos no hay análisis libre en vivo ni transcripción. El modo demo solo inicia con `APP_ENV=local` y se identifica en la interfaz y los logs.

## Arquitectura y decisiones

```text
Next.js (React Query) ──HTTP + tenant + JWT──> Echo (Go)
                                             │
                    dominio/casos de uso ────┼──> PostgreSQL (datos y auditoría)
                                             ├──> Redis (rate limit de login)
                                             └──> proveedor IA local / fixture demo
```

La API separa `domain`, `usecase`, `infra` y `http`. Toda consulta clínica se limita por tenant; el rol no sustituye la asignación tratante/supervisor. Los cambios críticos y su auditoría se confirman juntos. Las citas se serializan por tenant y usan revisión optimista; los informes exigen `expected_revision` para editar y aprobar. Cada generación conserva proveedor, versión, parámetros, fuentes y hashes, pero ninguna salida se aprueba automáticamente. La [especificación OpenAPI](docs/openapi.yaml) se sirve embebida en el binario.

Detalles: [concurrencia de agenda](docs/ADR_APPOINTMENT_CONCURRENCY.md), [listados autorizados](docs/ADR_VISIBLE_LISTS.md), [operación y retención](docs/RUNBOOK.md).

## Desarrollo y verificación

Para entorno local sin demo: copie los `.env.example`, ejecute `make tools && make db-up && make migrate-up`, arranque la API con `cd apps/api && go run ./cmd/server` y la web con `corepack pnpm --filter web dev`. `GET /health` es liveness; `GET /ready` comprueba PostgreSQL y Redis. El [runbook](docs/RUNBOOK.md) cubre preflight, recuperación, seguridad y observabilidad.

```bash
cd apps/api
gofmt -l .
golangci-lint run ./...
go test ./...
RUN_PG_INTEGRATION=1 DATABASE_URL="postgres://sessionflow:sessionflow@127.0.0.1:5433/sessionflow?sslmode=disable" go test ./internal/http ./internal/infra/db -count=1
go build ./cmd/server
cd ../..
corepack pnpm install --frozen-lockfile
corepack pnpm --filter web lint
corepack pnpm --filter web typecheck
corepack pnpm --filter web build
corepack pnpm --filter web test
python -m unittest discover -s tools/transcription -p 'test_*.py'
```

La CI ejecuta formato, lint, tests, integración y builds; el badge refleja el estado remoto, sin cifras de cobertura escritas a mano. Para el navegador contra servicios reales, arranque la demo, configure `WEB_ORIGIN` para la URL Playwright (`http://127.0.0.1:3103`), compile la web con `NEXT_PUBLIC_DEMO_MODE=true` y ejecute `RUN_LIVE_DEMO=1 corepack pnpm --filter web test`.

## Límites y hoja de ruta

No hay validación clínica externa ni datos reales. El modo simulado responde con una fixture fija; el análisis libre requiere Ollama. Audio, retención y proveedor remoto exigen configuración y consentimiento explícitos. Los listados de clientes, citas y sesiones filtran autorización, tienen orden estable y páginas de 50 elementos (máximo 100); los selectores web aún descargan todas las páginas, mientras que las tarjetas del panel consultan solo la primera y señalan con `+` un mínimo, no un total exacto. Eventos, hipótesis, targets, goals, versiones GIRA y procesos longitudinales también se paginan; el estado longitudinal completo y las asociaciones de un proceso individual aún pueden crecer sin límite de respuesta. Hay mediciones ilustrativas sobre la pequeña base de demo, no pruebas de latencia bajo carga. Las mejoras prioritarias son búsqueda remota para selectores grandes, vistas resumidas del estado clínico, cursores estables ante escrituras concurrentes y simplificar paneles de revisión; no se prevén funciones ornamentales.

Documentación: [directrices](PROJECT_GUIDELINES.md), [progreso histórico](PROGRESS/PROGRESS_INDEX.md), [procedimientos anteriores archivados](docs/ARCHIVED_README_OPERATIONS.md). No se añade licencia sin una decisión explícita del autor.
