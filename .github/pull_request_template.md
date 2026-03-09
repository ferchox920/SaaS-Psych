## Summary

- describe el cambio principal
- indica el riesgo principal

## Validation

- [ ] `go test ./...`
- [ ] Se ejecutaron pruebas adicionales relevantes para este cambio

## Tenant-aware review

- [ ] Toda query nueva o modificada filtra por `tenant_id` si corresponde
- [ ] No hay lookups tenant-scoped por `id` solamente
- [ ] `JOIN`, `UPDATE` y `DELETE` preservan aislamiento por tenant
- [ ] Hay evidencia de prueba cross-tenant o se explica por que no aplica

## Docs

- [ ] Actualice la documentacion relevante
- [ ] Revise `docs/PR_REVIEW_CHECKLIST.md` si el cambio toca arquitectura o persistencia
