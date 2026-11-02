# Perfiles y registro de asociaciones con Calendar

## Uso

En Pacientes, seleccionar una ficha para abrir Perfil: correo, teléfono internacional, fecha de nacimiento, canal preferido, autorización del paciente y anticipación de 24 horas. Los valores desconocidos permanecen vacíos. Guardar el perfil no envía mensajes.

El perfil también permite guardar hasta cinco referencias de proyectos de ChatGPT en `chatgpt_projects`, con `project_id` y `project_name`. Son asociaciones de identidad declaradas por el profesional; no acreditan acceso o importación del contenido del proyecto. Los IDs admitidos tienen formato `g-p-` seguido de 32 caracteres hexadecimales. No se guardan enlaces arbitrarios ni instrucciones del proyecto.

Los cambios conservan los demás datos del perfil y la revisión optimista. Un proyecto no puede asociarse a dos pacientes del mismo tenant: el repositorio toma bloqueos de proyecto en orden estable y comprueba las referencias existentes dentro de la transacción. Los proyectos retirados o duplicados del origen quedan separados hasta que el profesional seleccione expresamente otro. La interfaz permite corregir o quitar referencias y guardar la nueva revisión.

Las preferencias son preparatorias. El programador, la cola de envíos y los proveedores de correo/WhatsApp todavía no están implementados ni conectados; no hay envíos automáticos activos. La interfaz comunica explícitamente ese estado. La siguiente etapa requiere definir remitente/proveedor y completar destinatarios y autorizaciones. Los avisos deberán utilizar contenido administrativo, sin notas clínicas, y evitar duplicados ante reintentos y reprogramaciones.

En Agenda, Google Calendar muestra las citas locales ya vinculadas del rango activo. El vínculo identifica una ocurrencia de Google y su cita local, que a su vez referencia al paciente. La importación usa los casos de uso existentes, crea la asignación tratante al dar de alta cada paciente y conserva auditoría. Los eventos ya vinculados dejan de aparecer como pendientes. Esta etapa no mantiene un motor de sincronización continua ni crea reglas de identidad para futuras series: los cambios posteriores de Google requieren revisión.

## Contrato HTTP

- `GET /api/v1/clients/:id/profile`: perfil autorizado; si aún no se guardó, devuelve valores vacíos, canal `none`, anticipación `24` y revisión `0`.
- `PUT /api/v1/clients/:id/profile`: reemplaza el perfil con los campos `email`, `phone`, `birth_date` (`YYYY-MM-DD` o vacío), `reminder_channel` (`none`, `email`, `whatsapp`), `reminder_lead_hours` (`24`), `reminder_consent` y `revision` actual. Un conflicto de revisión devuelve `409`; datos inválidos, `400`. Elegir email o WhatsApp requiere contacto válido y autorización marcada. La autorización se registra como dato declarado por el profesional; no se infiere al importar.
- `GET /api/v1/integrations/google-calendar/linked-events?from=&to=&limit=&offset=`: lista paginada del calendario del usuario, limitada también por acceso clínico. Fechas RFC3339, inicio inclusivo y fin exclusivo, máximo 90 días. Respuesta `items` y `next_offset`; cada elemento contiene `appointment_id`, `client_id`, `client_name`, `event_id`, `starts_at`, `ends_at` y `sync_status`. El estado `synced` indica el vínculo guardado; no acredita revisión continua de cambios remotos.

## Persistencia y permisos

Migración `000035_client_profiles`: clave primaria `(tenant_id, client_id)` y FK compuesta a pacientes. El repositorio aplica revisión optimista y confirma datos y auditoría en una transacción. La escritura bloquea el paciente para serializar con el archivo. Lectura para tratante/supervisor según reglas existentes; edición solo tratante. Los listados de vínculos comprueban conexión del usuario y asignación/excepción clínica vigente.

Checklist tenant-aware completada: nuevas lecturas y escrituras limitadas por tenant; uniones y FK compuestas; identidad tomada del contexto autenticado; pruebas de lectura y escritura cruzadas, usuario sin asignación, revisión obsoleta y auditoría transaccional. No se registran contactos, fechas de nacimiento ni payloads de perfiles en auditoría.

## Verificación

Pruebas de dominio de datos, preferencias e identificadores de proyectos; integración PostgreSQL de persistencia, conservación de referencias al editar contacto, rechazo de un proyecto repetido en otro paciente, conflictos, auditoría y aislamiento; prueba de aislamiento del listado de vínculos; pruebas existentes de handlers y HTTP; typecheck y lint web. El test de integración genera únicamente fixtures sintéticas, fuera del tenant de uso personal.

```powershell
$env:RUN_PG_INTEGRATION='1'
go test ./internal/infra/db -run 'TestClientProfileIsolationAndRevisionPostgres|TestRepositoryQueriesRemainTenantAware' -count=1
```
