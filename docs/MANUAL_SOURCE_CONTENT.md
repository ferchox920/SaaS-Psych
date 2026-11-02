# Materiales de un paciente e importación manual

La vista `/clients/{client_id}/materials` conserva conversaciones, instrucciones de proyecto y variantes de transcripción como materiales por revisar. Se abre desde Pacientes o desde Sesión clínica. Permite leer dos materiales lado a lado y seleccionar revisiones anteriores de una misma fuente.

La carga es manual. Este servicio recibe una copia preparada; no inicia un navegador, no descarga proyectos de ChatGPT por su cuenta y no ejecuta instrucciones contenidas en el material. Tampoco genera ni aprueba informes o memoria longitudinal.

## Persistencia y procedencia

- La migración 36 conserva identidad, fechas candidatas y revisiones. La 37 añade el contenido JSON y una referencia explícita a la revisión actual, con claves compuestas por tenant, paciente y fuente.
- Identidad: tenant + tipo + proyecto + identificador estable de origen. Los títulos y fechas no identifican una sesión clínica.
- Se comprueba la asociación vigente del proyecto con el perfil del paciente dentro de la transacción. La misma fuente no puede terminar asignada a otro paciente del tenant.
- La huella SHA-256 usa el contenido tipado y metadatos canonicalizados (`manual-content-json-v1`). Cambiar el orden de claves JSON o la fecha de consulta no crea revisiones nuevas. Los metadatos de contenido deben excluir fechas volátiles de extracción.
- Repetir una carga idéntica reutiliza su revisión. Una modificación conserva el contenido anterior y crea una revisión pendiente. Reintentar una versión antigua no sustituye la actual.
- El registro de la revisión, el contenido, la selección de versión actual y la auditoría se confirman juntos. La auditoría contiene identificadores, tipo y huella, sin transcripciones.
- Los dictados pueden mezclar terapeuta y paciente. Se mantienen como `user`. La supervisión generada se conserva como `assistant`. En transcripciones por canal, los roles son sugerencias de dispositivo y deben indicarse como no verificados.
- La fecha del chat se guarda como `source_timestamp`, separada de una fecha profesionalmente confirmada. Importar una conversación no crea por sí solo una sesión clínica ni aprueba conclusiones.

## HTTP

Todos los endpoints requieren tenant, autenticación y asignación clínica. El tratante puede importar; el tratante y el supervisor pueden leer. Las lecturas incluyen únicamente pacientes activos. Un rol administrativo sin asignación no habilita acceso clínico.

- `POST /api/v1/clients/{client_id}/materials`: recibe una fuente. Responde `201` si creó una revisión y `200` si ya existía, con `{material, created}`.
- `GET /api/v1/clients/{client_id}/materials?limit=50&offset=0`: resumen paginado de versiones actuales, `{items, next_offset}`. No incluye cuerpos de conversación.
- `GET /api/v1/clients/{client_id}/materials/{source_id}?revision_id={revision_uuid}`: contenido de una revisión; sin parámetro, devuelve la actual. `source_id` en la ruta es el UUID interno devuelto por la API.
- `GET /api/v1/clients/{client_id}/materials/{source_id}/revisions?limit=50&offset=0`: historial paginado de revisiones, sin sus cuerpos. La interfaz muestra hasta las últimas 100 revisiones por fuente.

Límites: páginas de 1–100; JSON de entrada hasta 1.000.000 bytes; contenido canonicalizado hasta 900.000 bytes; hasta 2.000 mensajes y 30 referencias de archivo por revisión. Un cuerpo truncado o una lectura incompleta del origen debe rechazarse antes de importar, nunca completarse inventando texto.

La ausencia de un cursor de paginación no basta para afirmar que un historial externo está completo. El importador debe contrastar su comienzo con el origen, conservar la procedencia de los mensajes recuperados y comprobar el enlace con el tramo ya leído. Una copia visual puede tener límites de caracteres: los mensajes largos requieren una copia completa verificable. Si la recuperación no entrega la hora individual de un mensaje, se conserva su orden sin inventar ese dato.

Formato mínimo (datos ficticios):

```json
{
  "source_kind": "chatgpt_project",
  "project_id": "g-p-11111111111111111111111111111111",
  "source_id": "stable-conversation-id",
  "source_created_at": "2026-01-15T12:00:00Z",
  "source_updated_at": "2026-01-15T13:00:00Z",
  "candidate_session_date": "2026-01-15",
  "session_date_basis": "source_timestamp",
  "source_timezone": "America/Buenos_Aires",
  "clinical_session_id": null,
  "content": {
    "title": "Conversación ficticia",
    "messages": [
      {"id":"message-1","role":"user","text":"Texto ficticio aportado"},
      {"id":"message-2","role":"assistant","text":"Supervisión ficticia por revisar"}
    ],
    "text": "",
    "files": [],
    "metadata": {"warnings":[],"user_text_can_mix_speakers":true}
  }
}
```

Tipos: `chatgpt_project`, `project_instructions`, `desktop_recording`, `phone_recording`, `local_transcript`, `comparison`. Roles de mensaje: `user`, `assistant`, `patient`, `therapist`, `system`. Los mensajes temporizados admiten `start_seconds` y `end_seconds`; ambos se conservan en la misma base temporal declarada en metadatos.

## Grabaciones y comparaciones

`content.files` conserva nombre, ruta local, media type, tamaño y SHA-256 de archivos originales. Una referencia **no sube el audio ni habilita acceso al filesystem desde la API**. La interfaz lo dice expresamente. El almacén durable de audio existente conserva sus controles de consentimiento y cifrado; no se habilita ni se evita a través de este servicio.

Las transcripciones del escritorio se pueden importar como materiales distintos del dictado, con la identidad de la grabación original y el intervalo analizado. Una grabación parcial no se trata como una sesión completa. Al agregar posteriormente una grabación del teléfono, debe conservar su propia identidad y procedencia, y delimitar el intervalo común antes de calcular diferencias. No hay un porcentaje automático de exactitud ni una transcripción de referencia humana implícita.

## Operación y verificación

Las migraciones se verifican primero en una base aislada. Antes de aplicarlas a datos locales existentes se conserva un respaldo. No se ejecuta un seed ni se restablecen credenciales. La migración DOWN rechaza una base con contenido importado, además de tipos de fuente nuevos; no elimina historia para forzar el rollback.

Para la ejecución local con datos privados, `HTTP_HOST=127.0.0.1` limita la API al loopback. Si se omite, se conserva el comportamiento anterior de escucha del servidor.

Pruebas: canonicalización y límites; concurrencia de cargas idénticas; conservación de revisiones; reintento antiguo sin retroceso; asociación de proyecto; aislamiento por tenant y paciente; permisos de tratante/supervisor; auditoría sin duplicados; navegación real Pacientes → materiales → comparación → API → PostgreSQL.
