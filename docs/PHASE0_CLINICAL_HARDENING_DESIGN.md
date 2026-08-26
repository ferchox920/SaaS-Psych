# Diseño previo: Fase 0 — hardening clínico y privacidad

Estado: **aprobado por Fernando el 2026-08-10; implementación en curso**.

Este documento convierte la Fase 0 del plan general en cambios implementables y pruebas verificables. No autoriza por sí mismo el uso de datos clínicos reales.

## 1. Privacidad segura por defecto

### Cambio propuesto

- Representar `is_private` como campo opcional en requests de creación para distinguir ausencia de `false` explícito.
- Aplicar `true` cuando el cliente omita el campo.
- Inicializar el formulario web en `true`.
- Mantener la posibilidad de compartir una nota únicamente como acción explícita y explicada.

### Compatibilidad

- Requests antiguos que envían `false` conservan ese significado.
- Requests que no envían el campo pasan a crear notas privadas.

### Pruebas

- Creación sin `is_private` produce una nota privada.
- Creación con `is_private: false` produce una nota compartida.
- El formulario nuevo muestra marcada la privacidad inicialmente.

## 2. Asignación clínica separada del RBAC administrativo

### Modelo recomendado

Agregar una relación tenant-aware:

```text
client_clinical_assignments
  id
  tenant_id
  client_id
  user_id
  relationship: treating | supervisor
  granted_by_user_id
  starts_at
  ends_at nullable
  created_at
```

Reglas propuestas:

- `treating`: puede consultar y redactar material del paciente asignado.
- `supervisor`: puede consultar material autorizado, pero no alterar notas de otro autor.
- `owner/admin`: administra usuarios y asignaciones, pero no obtiene contenido clínico por ese solo rol.
- El autor puede continuar accediendo a sus propias notas mientras tenga una relación clínica activa o una excepción documentada.
- Un acceso excepcional se modela aparte y exige motivo, caducidad y auditoría.

### Pruebas

- Owner no asignado no puede leer contenido clínico.
- Admin no asignado no puede editar una nota.
- Terapeuta asignado puede crear su propia nota.
- Supervisor asignado puede leer lo autorizado y no sobrescribir la nota original.
- Ninguna asignación cruza tenants, incluso mediante inserción directa en PostgreSQL.

## 3. Borrador, cierre y adenda

### Modelo recomendado

Mantener `session_notes` como identidad lógica e introducir contenido versionado:

```text
session_notes
  id
  tenant_id
  appointment_id
  author_user_id
  status: draft | signed
  current_version
  signed_at nullable
  created_at

session_note_versions
  id
  tenant_id
  note_id
  version
  body
  is_private
  change_kind: draft | correction | addendum
  change_reason
  actor_user_id
  created_at
```

Reglas propuestas:

- Un borrador crea una nueva versión en cada guardado; nunca actualiza el contenido histórico.
- Una nota firmada no vuelve a borrador.
- Una corrección posterior crea una adenda enlazada y exige motivo.
- La respuesta normal muestra la versión vigente y permite consultar el historial autorizado.
- `current_version` se actualiza en la misma transacción que inserta la nueva versión y la auditoría.

### Migración

- Crear una versión 1 para cada nota existente con su cuerpo, privacidad, autor y timestamps actuales.
- Verificar conteos y hashes antes de retirar el cuerpo mutable de la tabla lógica.
- Hacer el cambio en dos migraciones compatibles si se requiere despliegue gradual.

### Pruebas

- Guardar dos veces produce dos versiones recuperables.
- Firmar impide sobrescritura.
- Una adenda conserva autoría original y registra nuevo autor/motivo.
- Dos actualizaciones concurrentes no pierden contenido; una debe fallar con conflicto de versión.

## 4. Archivo lógico y preservación de historia

### Cambio propuesto

- Agregar `archived_at`, `archived_by_user_id` y `archive_reason` a clientes.
- Reemplazar `DELETE /clients/:id` por una transición de archivo o mantener temporalmente la ruta con semántica de archivo documentada.
- Agregar restauración autorizada.
- Cambiar foreign keys clínicas para impedir borrado destructivo accidental.
- Reservar purga física para un procedimiento separado, fuera de la UI cotidiana y sujeto a política de retención.

### Pruebas

- Archivar oculta de listados activos sin borrar citas o notas.
- Se puede consultar historia archivada con autorización.
- Un `DELETE` físico de cliente con historia falla en la base de datos.
- Restaurar conserva exactamente los identificadores y relaciones previas.

## 5. Auditoría transaccional y de lecturas

### Cambio propuesto

- Cambiar la interfaz de auditoría para propagar errores en escrituras clínicas.
- Ejecutar escritura clínica y auditoría dentro de la misma transacción.
- Registrar lectura de nota, listado de notas, historial, exportación, asignación, acceso excepcional y decisiones sobre sugerencias de IA.
- No registrar cuerpo, prompt, transcripción, hipótesis ni contenido de audio en metadata.
- Incluir actor, entidad, propósito/motivo cuando aplique, request ID y timestamp.

### Política propuesta

- Si falla la auditoría de una escritura clínica, falla y revierte la escritura.
- Una lectura no se entrega si no puede registrarse el evento de acceso, salvo un modo de emergencia explícito diseñado posteriormente.

### Pruebas

- Un auditor que falla revierte creación, nueva versión y adenda.
- Cada lectura autorizada produce un evento sin contenido sensible.
- Un acceso denegado puede medirse sin revelar que la nota existe en otro tenant.

## 6. Refresh token en cookie HttpOnly

### Contrato recomendado

- Login devuelve access token y metadata no sensible en JSON.
- Login y refresh escriben el refresh token rotado en cookie `HttpOnly`.
- Refresh toma el token exclusivamente de la cookie.
- Logout revoca el token y expira la cookie.
- El access token permanece en memoria del frontend y no se persiste en `localStorage`.
- Tras recargar la página, el frontend intenta refresh y reconstruye la sesión con `/me`.

### Configuración requerida

```text
WEB_ORIGIN=http://localhost:3000
AUTH_COOKIE_SECURE=false        # solo local
AUTH_COOKIE_SAME_SITE=lax
AUTH_COOKIE_DOMAIN=
```

La API y web usan orígenes distintos en desarrollo (`3000` y `8080`). Por tanto:

- `fetch` debe usar `credentials: "include"`;
- CORS debe permitir únicamente `WEB_ORIGIN` y credenciales;
- nunca usar `Access-Control-Allow-Origin: *` con cookies;
- producción debe exigir HTTPS y cookie `Secure`.

### Compatibilidad

No mantener indefinidamente refresh token en body y cookie al mismo tiempo. Si se necesita transición, protegerla con una configuración local temporal, documentada y probada para evitar dos fuentes de autoridad.

### Pruebas

- JavaScript no recibe ni almacena refresh token.
- Login establece atributos correctos de cookie.
- Refresh rota cookie y revoca token anterior.
- Logout expira cookie.
- Origen no permitido no recibe credenciales.
- Recarga de frontend recupera sesión sin `localStorage`.

## 7. Secuencia de implementación propuesta

1. Privacidad por defecto y pruebas de regresión.
2. Modelo de asignación clínica y autorización centralizada.
3. Versiones, firma/adendas y transacciones de auditoría.
4. Archivo lógico y endurecimiento de foreign keys.
5. Auditoría de lecturas y acceso excepcional mínimo.
6. Cookie HttpOnly, CORS explícito y sesión frontend en memoria.
7. OpenAPI, runbook, release checklist y pruebas PostgreSQL completas.

Cada paso debe compilar, mantener aislamiento tenant-aware y conservar los cambios preexistentes del worktree.

## 8. Decisiones aprobadas

Fernando aprobó el plan y sus decisiones recomendadas el 2026-08-10:

- asignación clínica explícita como autoridad para leer contenido;
- borradores versionados y notas firmadas corregibles por adenda;
- archivo lógico sin borrado físico desde UI;
- auditoría obligatoria y transaccional;
- cookie HttpOnly como único transporte del refresh token;
- audio efímero y deshabilitado durante toda la Fase 0.

## 9. Evidencia de implementación

- Migraciones `000012`–`000014`: archivo lógico, versiones, asignaciones y accesos excepcionales tenant-aware.
- Autorización: `treating` puede escribir; `supervisor` puede leer; una excepción temporal solo habilita lectura.
- Ciclo documental: borrador versionado, firma irreversible, adenda con motivo e historial consultable.
- Auditoría: escrituras clínicas en la misma transacción; lecturas fallan cerradas si no se registra el acceso.
- Sesión web: refresh token únicamente en cookie `HttpOnly`; access token solo en memoria; `localStorage` contiene como máximo el tenant no sensible.
- Audio: deshabilitado por configuración y sin endpoints durante la Fase 0; política en `CLINICAL_AUDIO_RETENTION_POLICY.md`.
- Verificación: suite Go, integraciones PostgreSQL sobre migraciones limpias, lint y build de Next.js.

El gate solo debe declararse cerrado después de ejecutar nuevamente todas estas verificaciones sobre el estado final del corte.
