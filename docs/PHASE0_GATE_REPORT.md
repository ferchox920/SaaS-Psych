# Gate de Fase 0 — hardening clínico y privacidad

Fecha de verificación: 2026-08-10  
Estado: **APROBADO para continuar con la integración local usando el modelo de autorización implementado**.

Este gate verifica invariantes técnicas de SessionFlow. No constituye por sí solo una certificación legal, regulatoria ni una política institucional de historias clínicas.

## Matriz de evidencia

| Requisito | Evidencia autoritativa | Resultado |
|---|---|---|
| Notas privadas por defecto | Request HTTP sin `is_private`, estado inicial del formulario y pruebas de sesión | Pasa |
| Rol administrativo separado del acceso clínico | Asignaciones `treating/supervisor`; pruebas donde `owner` no asignado recibe lista vacía o `403` | Pasa |
| Autoridad diferenciada | `treating` escribe; `supervisor` lee; excepción temporal solo lee | Pasa |
| Aislamiento tenant-aware | Foreign keys compuestas y pruebas PostgreSQL de referencias cruzadas | Pasa |
| Historia de notas | Versiones inmutables, firma irreversible, adenda con motivo y control optimista | Pasa |
| Archivo sin destrucción | Archivo/restauración conservan ID; `DELETE` físico con historia falla por FK | Pasa |
| Auditoría fail-closed | Escrituras clínicas y auditoría comparten transacción; lecturas no se entregan si falla auditoría | Pasa |
| Trazabilidad | Actor, tenant, entidad, acción, tiempo y `request_id`; metadata sin cuerpos clínicos | Pasa |
| Acceso excepcional | Motivo, propósito, máximo 24 h, revocación, solo lectura e historia auditada | Pasa |
| Refresh token no legible por JavaScript | Cookie `HttpOnly`, rotación, CORS explícito, purga de almacenamiento heredado | Pasa |
| Access token no persistente | Estado React en memoria; `localStorage` conserva únicamente el tenant no sensible | Pasa |
| Audio deshabilitado | Sin endpoints ni micrófono; arranque rechaza `CLINICAL_AUDIO_ENABLED=true` | Pasa |
| Contrato y documentación | OpenAPI parseado correctamente; política de audio y diseño actualizados | Pasa |

## Verificaciones ejecutadas

```text
go test ./...
RUN_PG_INTEGRATION=1 go test -count=1 ./internal/http ./internal/infra/db
pnpm --filter web lint
pnpm --filter web build
python yaml.safe_load(docs/openapi.yaml)
git diff --check
```

Las integraciones PostgreSQL se ejecutaron sobre una instancia temporal limpia aplicando migraciones `000001` a `000014`. La instancia se eliminó después de las pruebas y no se tocaron otras bases locales.

## Condiciones que siguen vigentes

- El audio permanece deshabilitado hasta superar el gate de transcripción local de Fase 4.
- Fixtures, benchmarks y calibración de IA deben usar material ficticio o completamente desidentificado.
- Ninguna sugerencia de IA se incorpora automáticamente a la historia clínica.
- Ollama y el futuro transcriptor deben permanecer en loopback y no puede existir fallback a proveedores externos.
