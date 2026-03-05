# Refresh Token Design (Opaque + Rotation)

Este proyecto usa refresh tokens **opacos**, no JWT.

## Implementacion actual

- Generacion: `GenerateRefreshToken()` crea 32 bytes aleatorios y devuelve:
  - token plano (se entrega al cliente)
  - hash SHA-256 (se persiste en DB)
- Persistencia: tabla `refresh_tokens` guarda `token_hash`, `tenant_id`, `user_id`, `expires_at`, `revoked_at`.
- Refresh:
  1. Se hashea el token recibido (`HashRefreshToken`).
  2. Se busca por `tenant_id + token_hash`.
  3. Si existe y no esta revocado/expirado, se revoca el actual.
  4. Se emite nuevo access JWT y nuevo refresh opaco (rotacion).
- Logout: revoca el refresh token por `tenant_id + token_hash`.

## Propiedades de seguridad

- El refresh token nunca se firma ni verifica como JWT.
- Si hay filtracion de DB, se exponen hashes, no tokens planos.
- Rotacion reduce ventana de reutilizacion de tokens robados.
- El lookup esta aislado por tenant (`tenant_id`) para mantener multi-tenancy.

## Variables de entorno relacionadas

- `JWT_ACCESS_SECRET`: firma del access JWT.
- `REFRESH_TTL_DAYS`: expiracion de refresh token opaco.

`JWT_REFRESH_SECRET` fue removida del config y de `.env.example` porque no se usa en esta arquitectura.
