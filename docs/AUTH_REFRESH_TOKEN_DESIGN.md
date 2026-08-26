# Refresh Token Design (Opaque + Rotation)

Este proyecto usa refresh tokens **opacos**, no JWT.

## Implementacion actual

- Generacion: `GenerateRefreshToken()` crea 32 bytes aleatorios y devuelve:
  - token plano (se entrega exclusivamente como cookie `HttpOnly`)
  - hash SHA-256 (se persiste en DB)
- Persistencia: tabla `refresh_tokens` guarda `token_hash`, `tenant_id`, `user_id`, `expires_at`, `revoked_at`.
- Refresh:
  1. Se hashea el token recibido (`HashRefreshToken`).
  2. Se busca por `tenant_id + token_hash`.
  3. Si existe y no esta revocado/expirado, se revoca el actual.
  4. Se emite nuevo access JWT y nuevo refresh opaco (rotacion).
- Logout: revoca el refresh token por `tenant_id + token_hash`.
- Transporte: login y refresh rotan la cookie `sessionflow_refresh`; el JSON contiene solo el access token.

## Propiedades de seguridad

- El refresh token nunca se firma ni verifica como JWT.
- Si hay filtracion de DB, se exponen hashes, no tokens planos.
- Rotacion reduce ventana de reutilizacion de tokens robados.
- El lookup esta aislado por tenant (`tenant_id`) para mantener multi-tenancy.
- JavaScript no puede leer el refresh token y el access token vive solo en memoria.

## Variables de entorno relacionadas

- `JWT_ACCESS_SECRET`: firma del access JWT.
- `REFRESH_TTL_DAYS`: expiracion de refresh token opaco.
- `WEB_ORIGIN`: unico origen web admitido por CORS con credenciales.
- `AUTH_COOKIE_SECURE`, `AUTH_COOKIE_SAME_SITE`, `AUTH_COOKIE_DOMAIN`: atributos de cookie; fuera de local `Secure` es obligatorio.

`JWT_REFRESH_SECRET` fue removida del config y de `.env.example` porque no se usa en esta arquitectura.
