# Environment Hardening Baseline

Baseline minimo para despliegues de SessionFlow fuera de desarrollo local.

## Auth secrets

- `JWT_ACCESS_SECRET=change-me` existe solo como valor de conveniencia para `APP_ENV=local`.
- Desde startup, la API rechaza arrancar si `APP_ENV` no es `local` y `JWT_ACCESS_SECRET` sigue en `change-me`.
- Definir un secreto unico por entorno con al menos 32 caracteres aleatorios.
- Inyectar secretos desde variables de entorno o un secret manager; no commitearlos en el repositorio.

## Environment classification

- Usar `APP_ENV=local` unicamente para trabajo de desarrollo en la maquina local.
- Configurar `APP_ENV` realista en `dev`, `staging` y `production` para activar validaciones de arranque y observabilidad consistente.

## Operational baseline

- Rotar `JWT_ACCESS_SECRET` mediante procedimiento controlado por entorno.
- Restringir acceso a `.env`, pipelines y sistemas que materializan secretos.
- Mantener rate limiting de login habilitado con Redis en entornos expuestos.
- Revisar periodicamente politicas complementarias pendientes, como password policy y validaciones adicionales de secretos.
