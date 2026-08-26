# Environment Hardening Baseline

Baseline minimo para despliegues de SessionFlow fuera de desarrollo local.

## Auth secrets

- `JWT_ACCESS_SECRET=change-me` existe solo como valor de conveniencia para `APP_ENV=local`.
- Desde startup, la API rechaza arrancar si `APP_ENV` no es `local` y `JWT_ACCESS_SECRET` sigue en `change-me`.
- Definir un secreto unico por entorno con al menos 32 caracteres aleatorios.
- Inyectar secretos desde variables de entorno o un secret manager; no commitearlos en el repositorio.

## Runtime dependencies

- `DATABASE_URL` no debe usar credenciales de demo ni quedar vacio en entornos compartidos; si falta, la API arranca degradada y deshabilita endpoints de auth/persistencia.
- `REDIS_URL` debe estar configurado en entornos expuestos para mantener activo el rate limit de login; si falta, la API no aplica esa defensa.
- Tratar ausencia de `DATABASE_URL` o `REDIS_URL` fuera de `local` como misconfiguracion operativa, aunque hoy no exista fail-fast automatico para esos casos.

## Observability and data exposure

- `OTEL_TRACES_EXPORTER=otlp` exige `OTEL_EXPORTER_OTLP_ENDPOINT`; la configuracion debe validarse antes de desplegar para evitar arranques fallidos por exporter incompleto.
- Mantener `OTEL_DB_STATEMENT_ENABLED=false` por defecto en entornos con datos sensibles; habilitarlo solo de forma temporal y controlada, porque puede exponer SQL y parametros en trazas.
- Revisar `OTEL_RESOURCE_ATTRIBUTES` para no incluir secretos, tokens ni identificadores sensibles.

## Environment classification

- Usar `APP_ENV=local` unicamente para trabajo de desarrollo en la maquina local.
- Configurar `APP_ENV` realista en `dev`, `staging` y `production` para activar validaciones de arranque y observabilidad consistente.

## Operational baseline

- Rotar `JWT_ACCESS_SECRET` mediante procedimiento controlado por entorno.
- Restringir acceso a `.env`, pipelines y sistemas que materializan secretos.
- Mantener rate limiting de login habilitado con Redis en entornos expuestos.
- Reservar `RUN_PG_INTEGRATION=1` para CI o validacion local intencional; no usarlo como flag permanente de runtime.
- Revisar periodicamente politicas complementarias pendientes, como password policy y validaciones adicionales de secretos.
