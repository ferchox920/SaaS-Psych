# ADR 0001 - Arquitectura de API por capas y tenancy-aware

## Estado

Aprobado

## Contexto

La API se implemento con una separacion clara entre `domain`, `usecase`, `infra` y `http`, pero el arbol final quedo materializado dentro de `apps/api/internal/...` en lugar de replicar literalmente la estructura plana sugerida en `PROJECT_GUIDELINES.md`.

La auditoria marco dos puntos:

- la desviacion estructural existe, aunque no afecta el wiring ni la separacion de responsabilidades;
- no hay un mecanismo automatizado que garantice que toda query nueva preserve el aislamiento por `tenant_id`.

El wiring actual en `apps/api/cmd/server/main.go` y `apps/api/internal/http/server.go` confirma que la composicion central sigue siendo consistente y que los boundaries de la aplicacion estan definidos por capa.

## Decision

Se formaliza como decision arquitectonica que:

1. La estructura valida del backend es `apps/api/internal/{domain,usecase,infra,http,...}`.
2. La recomendacion de `PROJECT_GUIDELINES.md` se interpreta como una arquitectura logica, no como un requisito literal de paths en raiz.
3. Toda query a Postgres o Redis que opere sobre datos de tenant debe ser explicitamente tenant-aware.
4. La revision de PR debe incluir una checklist obligatoria para validar filtros, joins, updates, deletes y tests de aislamiento por tenant.

## Consecuencias

Positivas:

- Se documenta que la desviacion de carpetas es intencional y aceptada.
- Se reduce ambiguedad para futuros cambios de estructura.
- Se incorpora un control de proceso visible para evitar queries cross-tenant.

Limitaciones:

- Esto no reemplaza enforcement automatico en CI o linting.
- El cumplimiento sigue dependiendo de disciplina de revision hasta que exista una regla automatizada.

## Guardrails operativos

Al agregar o modificar acceso a datos:

- incluir `tenant_id` en filtros de lectura y escritura;
- validar joins por claves compuestas o condiciones equivalentes con `tenant_id`;
- evitar lookups por `id` solamente cuando el recurso es tenant-scoped;
- cubrir al menos un caso de aislamiento cross-tenant cuando la superficie cambie.

## Referencias

- `PROJECT_GUIDELINES.md`
- `docs/PR_REVIEW_CHECKLIST.md`
- `apps/api/cmd/server/main.go`
- `apps/api/internal/http/server.go`
