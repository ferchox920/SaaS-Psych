# Sincronización con Google Calendar

## Alcance inicial

La integración pertenece a cada usuario del tenant y trabaja sobre el calendario primario que ese usuario posee.

- Importa eventos temporizados de Google y permite asociarlos manualmente a un paciente existente.
- Al asociar, crea un turno local y guarda un vínculo único con el evento para impedir duplicados.
- Permite crear o actualizar en Google el turno seleccionado en SessionFlow.
- Si el turno local fue cancelado, la siguiente sincronización manual elimina su evento enlazado de Google.
- No importa eventos de día completo ni conserva copias locales de títulos de eventos sin asociar.

## Privacidad

La relación evento-paciente solo se almacena en SessionFlow. Los eventos creados o actualizados por SessionFlow usan:

- título `Sesión`;
- visibilidad privada;
- horario y ubicación administrativa;
- identificadores técnicos en `extendedProperties.private`.

No se envían a Google el nombre del paciente, notas, transcripciones, formulaciones ni resultados de IA. El refresh token se cifra con AES-256-GCM y una clave separada de los secretos JWT.

## Configuración de Google Cloud

1. Crear o seleccionar un proyecto en Google Cloud Console.
2. Habilitar **Google Calendar API**.
3. Configurar la pantalla de consentimiento OAuth.
4. Crear credenciales OAuth de tipo **Web application**.
5. Registrar exactamente este redirect URI para desarrollo local:

   `http://127.0.0.1:8080/api/v1/integrations/google-calendar/callback`

6. Agregar el usuario de Google como usuario de prueba mientras la aplicación permanezca en modo Testing.

SessionFlow solicita únicamente `https://www.googleapis.com/auth/calendar.events.owned`, acceso offline, `state` de un solo uso y PKCE S256.

## Variables de entorno

Generar una clave local una sola vez:

```powershell
$bytes = New-Object byte[] 32
[Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
[Convert]::ToBase64String($bytes)
```

Guardar el resultado y las credenciales OAuth solo en `.env`:

```dotenv
GOOGLE_CALENDAR_ENABLED=true
GOOGLE_CALENDAR_CLIENT_ID=...
GOOGLE_CALENDAR_CLIENT_SECRET=...
GOOGLE_CALENDAR_REDIRECT_URL=http://127.0.0.1:8080/api/v1/integrations/google-calendar/callback
GOOGLE_CALENDAR_TOKEN_KEY=...
GOOGLE_CALENDAR_TIMEOUT_SECONDS=20
```

No cambiar `GOOGLE_CALENDAR_TOKEN_KEY` mientras existan conexiones activas: hacerlo vuelve indescifrables los refresh tokens y obliga a reconectar cada cuenta.

## Estrategia de sincronización

La primera versión usa acciones explícitas para evitar cambios silenciosos en una agenda clínica:

1. **Actualizar eventos** lee el rango visible de Google.
2. **Asociar y crear turno** vincula un evento a un paciente y crea el turno local.
3. **Sincronizar turno seleccionado** crea o actualiza el evento genérico correspondiente.
4. Los conflictos de superposición siguen las reglas existentes de SessionFlow y no crean turnos duplicados.

Una sincronización incremental automática mediante `syncToken` y notificaciones push queda fuera de esta primera capa; requiere un endpoint HTTPS público y una política explícita de resolución de conflictos.

## Fuentes oficiales

- OAuth para aplicaciones web: https://developers.google.com/identity/protocols/oauth2/web-server
- Scopes de Calendar: https://developers.google.com/workspace/calendar/api/auth
- Propiedades privadas de eventos: https://developers.google.com/workspace/calendar/api/guides/extended-properties
- Sincronización incremental: https://developers.google.com/workspace/calendar/api/guides/sync

