# PR REVIEW CHECKLIST

Checklist de revision para cambios funcionales en la API, con enfasis en aislamiento multi-tenant.

## Tenant-aware queries

- [ ] Toda query nueva o modificada filtra por `tenant_id` si accede a datos tenant-scoped.
- [ ] Ningun lookup tenant-scoped depende solo de `id` sin validar `tenant_id`.
- [ ] Los `JOIN`, `UPDATE` y `DELETE` preservan el aislamiento por tenant.
- [ ] Si el handler recibe `tenant_id` desde contexto, el usecase/repository lo propaga sin reconstruirlo desde input inseguro.
- [ ] Los tests cubren al menos un escenario cross-tenant cuando se agrega o cambia persistencia.
- [ ] Si aparece una consulta global legitima, la excepcion al guard automatizado queda documentada en `tenant_aware_query_guard_test.go`.

## Guia rapida para queries nuevas

- `SELECT`: empezar por la tabla tenant-scoped y exigir `tenant_id` en `WHERE`, incluso cuando `id` sea unico.
- `JOIN`: unir tambien por `tenant_id` o partir desde una fila ya tenant-scoped y verificar que las referencias tengan FK compuesta.
- `UPDATE` y `DELETE`: exigir `tenant_id` en `WHERE` y esperar `not_found` o `0 rows` cuando se intenta operar con otro tenant.
- `INSERT`: si referencia entidades tenant-scoped, validar existencia en el mismo tenant en repository/usecase y respaldarlo con constraints compuestas en DB.
- Tests minimos por tabla critica: un caso same-tenant exitoso y un caso cross-tenant que pruebe aislamiento en lectura, update o referencia.

## Arquitectura por capas

- [ ] La logica de negocio nueva vive en `usecase` y/o `domain`, no en handlers.
- [ ] `infra` implementa detalles tecnicos sin contaminar entidades o reglas del dominio.
- [ ] El wiring nuevo mantiene composicion centralizada en `main.go` y `internal/http/server.go`.

## Contrato y operacion

- [ ] Se actualizo documentacion si cambia arquitectura, contrato HTTP o decisiones relevantes.
- [ ] Logs, errores y trazas no exponen datos sensibles.
- [ ] La evidencia minima de validacion (`go test`, integration o smoke) queda registrada en el PR.
