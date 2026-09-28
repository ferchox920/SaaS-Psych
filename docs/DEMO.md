# Demostración local (datos exclusivamente ficticios)

El modo demo es **simulado**, no hace inferencia clínica. `DEMO_MODE=true` solo es válido con `APP_ENV=local`. Los informes y propuestas longitudinales provienen de una fixture fija; el texto de sesión no se analiza. El profesional conserva la decisión de editar, aprobar, descartar y fusionar. Nunca usar este modo con datos reales.

El frontend solo precarga las credenciales ficticias cuando se compila con `NEXT_PUBLIC_DEMO_MODE=true`. En una compilación normal, los campos están vacíos y el JavaScript enviado al navegador no incluye la contraseña de demostración.

## Preparación

Requisitos: Docker Compose, Go 1.24+, Node 22+ y pnpm mediante Corepack. La base local se publica en `127.0.0.1:5433` y Redis en `127.0.0.1:6379`. No es necesario Ollama ni el transcriptor para el recorrido de informe y memoria longitudinal; el análisis clínico en vivo, la revisión de texto libre y la transcripción no funcionan sin sus servicios respectivos. Captura/ingesta de audio permanece desactivada.

Desde la raíz del repositorio, en Linux/macOS:

```bash
cp .env.example .env
cp apps/web/.env.example apps/web/.env.local
make tools
make demo-prepare
```

En PowerShell (con `migrate` instalado o `$env:USERPROFILE\go\bin\migrate.exe`):

```powershell
Copy-Item .env.example .env
Copy-Item apps/web/.env.example apps/web/.env.local
docker compose up -d postgres redis
migrate -path apps/api/migrations -database "postgres://sessionflow:sessionflow@127.0.0.1:5433/sessionflow?sslmode=disable" up
Get-Content -Raw tools/demo/seed.sql | docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U sessionflow -d sessionflow
```

El seed clínico es idempotente y opt-in; **no** forma parte de las migraciones de producción. Una migración histórica creó dos cuentas demo con contraseña conocida; la migración `000029` invalida esas contraseñas y sus refresh tokens. Este seed local las reactiva deliberadamente solo para la demostración. Si `migrate ... up` indica `no change`, la base ya está preparada. No ejecute el seed fuera de una base local descartable. Levante API y web en terminales separadas:

```bash
cd apps/api && APP_ENV=local DEMO_MODE=true go run ./cmd/server
```

```bash
NEXT_PUBLIC_DEMO_MODE=true corepack pnpm install --frozen-lockfile
NEXT_PUBLIC_DEMO_MODE=true corepack pnpm --filter web dev
```

En PowerShell, establezca `$env:DEMO_MODE='true'` antes de `go run ./cmd/server` y `$env:NEXT_PUBLIC_DEMO_MODE='true'` antes de `corepack pnpm --filter web dev` en sus respectivas terminales. Mantenga `DATABASE_URL` y `REDIS_URL` de `.env.example`. Abra `http://localhost:3000`. La banda roja identifica el modo simulado. Para un despliegue público, use configuración separada y no active el modo demo.

## Cuentas locales

Todas usan la contraseña exclusivamente local `ChangeMe123!`; nunca reutilizarla fuera de esta base. El login requiere el UUID del tenant.

| Tenant | Usuario | Rol y propósito |
| --- | --- | --- |
| `11111111-1111-1111-1111-111111111111` | `owner@tenant-a.local` | Owner; asignado a Aurora |
| mismo tenant | `therapist@tenant-a.local` | Member; tratante de Aurora |
| mismo tenant | `unassigned@tenant-a.local` | Member; denegación de acceso clínico |
| `22222222-2222-2222-2222-222222222222` | `member@tenant-b.local` | Member; solo ve al paciente Río |

Para probar la administración de asignaciones desde Pacientes con el owner del tenant A, seleccione `therapist@tenant-a.local` o `unassigned@tenant-a.local` por correo. El selector solo muestra usuarios del tenant actual; la API vuelve a comprobar esa pertenencia y registra la asignación o finalización con actor y motivo. Los UUID siguen disponibles en el detalle técnico.

El seed crea los pacientes ficticios Aurora y Río, citas programadas a dos y tres días de una instalación nueva y una sesión completada para Aurora. Use la cuenta `therapist@tenant-a.local` para abrir Aurora y recorrer: Pacientes → Citas → cree una cita ficticia libre para Aurora (o seleccione la del seed si aún no tiene sesión) → «Abrir sesión clínica de esta cita» → iniciar y completar sesión → generar informe con cualquier texto **ficticio** de al menos 20 caracteres → revisar/editar borrador → aprobar la revisión visible → análisis longitudinal → aprobar o descartar cada operación → merge → estado y procedencia. La nueva sesión conserva el ID de la cita. Solo puede haber una sesión no anulada por cita. La fixture propone exactamente una evidencia que aún no pertenece a la memoria hasta que el profesional la aprueba y fusiona. Reaplicar el seed no mueve citas existentes; use una base local descartable nueva si necesita renovar las fechas.

Una ejecución de generación altera el estado de la demo; para repetir el recorrido desde cero, use otra sesión completada o reinicialice **solo una base local descartable** tras revisar los datos que contiene. El seed por sí solo no borra informes ni decisiones previas.

La prueba opt-in `RUN_LIVE_DEMO=1 corepack pnpm --filter web exec playwright test tests/live-demo.spec.ts` crea una cita ficticia libre y completa una sesión vinculada en cada ejecución. Puede repetirse sobre la misma base seed mientras queden horarios disponibles sin borrar informes ni decisiones anteriores; comprueba que Pacientes y Citas muestran los datos seed, generación simulada, aprobación, revisión y merge, auditoría con owner, y denegación a usuarios sin asignación y de otro tenant. No cambia capturas versionadas. Para actualizarlas deliberadamente en la demo ficticia local, añada `UPDATE_DEMO_SCREENSHOTS=1`.

## Verificación breve

`GET /health` comprueba liveness y `GET /ready` comprueba PostgreSQL y Redis. Con el servidor en marcha, el terapeuta asignado debe poder generar el informe y el usuario `unassigned@tenant-a.local` debe recibir `403` al solicitar la memoria de Aurora. Una cuenta del otro tenant también debe recibir `403` para esa memoria. El log del servidor declara explícitamente que la fixture no es inferencia clínica.
