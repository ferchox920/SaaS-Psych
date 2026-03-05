# SPRINTS

Roadmap vivo del proyecto SessionFlow, alineado con `PROGRESS/PROGRESS_INDEX.md`.

## Fuente de verdad

- Registro oficial de ejecucion: `PROGRESS/PROGRESS_INDEX.md`.
- Este archivo (`SPRINTS.md`) es roadmap vivo y resumen por sprint.
- Regla de consistencia: ante cualquier diferencia, prevalece `PROGRESS/PROGRESS_INDEX.md`.

## Estado por sprint

| Sprint | Objetivo | Estado | Evidencia |
| --- | --- | --- | --- |
| S0 | Bootstrap base (estructura, server, DB inicial) | DONE | [S0_1](./PROGRESS/S0/S0_1.md) |
| S1 | Auth + tenancy + RBAC + auditoria auth | DONE | [S1_1](./PROGRESS/S1/S1_1.md), [S1_1.1](./PROGRESS/S1/S1_1.1.md), [S1_3](./PROGRESS/S1/S1_3.md), [S1_5.2](./PROGRESS/S1/S1_5.2.md) |
| S2 | Dominio MVP (clients, appointments, session notes) | DONE | [S2_1](./PROGRESS/S2/S2_1.md), [S2_2](./PROGRESS/S2/S2_2.md), [S2_3](./PROGRESS/S2/S2_3.md) |
| S3 | Cierre de seguridad operativa (rate limit + hardening tenant) | DONE | [S3_1](./PROGRESS/S3/S3_1.md), [S3_2](./PROGRESS/S3/S3_2.md) |
| S4 | Hardening multi-tenant + observabilidad base (/metrics + request logging) | DONE | [S4_1](./PROGRESS/S4/S4_1.md), [S4_4.1](./PROGRESS/S4/S4_4.1.md), [S4_4.2](./PROGRESS/S4/S4_4.2.md), [S4_4.5](./PROGRESS/S4/S4_4.5.md) |
| S5 | Cierre operativo (auditoria dominio + migraciones estandar + CI) | DONE | [S5_5](./PROGRESS/S5/S5_5.md), [S5_5.1](./PROGRESS/S5/S5_5.1.md), [S5_5.2](./PROGRESS/S5/S5_5.2.md) |
| S6 | README ejecutable final + arquitectura + portfolio | DONE | [S6_6](./PROGRESS/S6/S6_6.md) |
| S7 | Tracing OpenTelemetry (HTTP + Postgres + Redis) y stack local con collector + Jaeger | DONE | [S7_7](./PROGRESS/S7/S7_7.md), [S7_7.1](./PROGRESS/S7/S7_7.1.md), [S7_7.2](./PROGRESS/S7/S7_7.2.md), [S7_7.3](./PROGRESS/S7/S7_7.3.md), [S7_7.4](./PROGRESS/S7/S7_7.4.md), [S7_7.5](./PROGRESS/S7/S7_7.5.md) |
| S8 | Correlacion logs-traces + contrato OpenAPI 3.0 + Swagger UI local | DONE | [S8_8](./PROGRESS/S8/S8_8.md), [S8_8.1](./PROGRESS/S8/S8_8.1.md), [S8_8.2](./PROGRESS/S8/S8_8.2.md), [S8_8.3](./PROGRESS/S8/S8_8.3.md) |
| S9 | Hardening final + formalizacion de dominio notes/appointments + stack metricas local | DONE | [S9_9](./PROGRESS/S9/S9_9.md), [S9_9.1](./PROGRESS/S9/S9_9.1.md), [S9_9.2](./PROGRESS/S9/S9_9.2.md), [S9_9.3](./PROGRESS/S9/S9_9.3.md), [S9_9.4](./PROGRESS/S9/S9_9.4.md) |
| S10 | Alineacion documental de roadmap vs ejecucion | DONE | [S10_10](./PROGRESS/S10/S10_10.md) |

## Estado de hitos de cierre

| Hito | Estado | Evidencia |
| --- | --- | --- |
| Multi-tenant (middleware + aislamiento + hardening appointment-client) | DONE | [S3_2](./PROGRESS/S3/S3_2.md), [S4_4.1](./PROGRESS/S4/S4_4.1.md), [S4_4.2](./PROGRESS/S4/S4_4.2.md), [S9_9.1](./PROGRESS/S9/S9_9.1.md) |
| Auth + refresh opaco con rotacion + RBAC | DONE | [S1_1](./PROGRESS/S1/S1_1.md), [S1_1.1](./PROGRESS/S1/S1_1.1.md), [S4_4.3](./PROGRESS/S4/S4_4.3.md) |
| Dominio MVP (clients/appointments/session notes) | DONE | [S2_1](./PROGRESS/S2/S2_1.md), [S2_2](./PROGRESS/S2/S2_2.md), [S2_3](./PROGRESS/S2/S2_3.md), [S9_9.3](./PROGRESS/S9/S9_9.3.md) |
| Auditoria de dominio (usecases) | DONE | [S5_5](./PROGRESS/S5/S5_5.md), [S9_9.3](./PROGRESS/S9/S9_9.3.md) |
| Observabilidad base (request logging + /metrics) | DONE | [S4_1](./PROGRESS/S4/S4_1.md), [S4_4.4](./PROGRESS/S4/S4_4.4.md), [S4_4.5](./PROGRESS/S4/S4_4.5.md) |
| Tracing OpenTelemetry (HTTP + DB + Redis) | DONE | [S7_7.1](./PROGRESS/S7/S7_7.1.md), [S7_7.2](./PROGRESS/S7/S7_7.2.md), [S7_7.3](./PROGRESS/S7/S7_7.3.md) |
| OpenAPI 3.0 y visualizacion Swagger UI | DONE | [S8_8.2](./PROGRESS/S8/S8_8.2.md), [S8_8.3](./PROGRESS/S8/S8_8.3.md) |
| Stack de metricas local (Prometheus + Grafana) | DONE | [S9_9.4](./PROGRESS/S9/S9_9.4.md) |
| CI (test + lint + build + integration DB) | DONE | [S5_5.2](./PROGRESS/S5/S5_5.2.md) |
| README/portfolio final | DONE | [S6_6](./PROGRESS/S6/S6_6.md) |

## Backlog propuesto (post-S10)

| Paso | Alcance | Estado | Evidencia / destino |
| --- | --- | --- | --- |
| S10_10.1 | Estandarizacion de errores HTTP con helper central + `error.details` opcional | DONE | [S10_10.1](./PROGRESS/S10/S10_10.1.md) |
| S10_10.2 | Hardening multi-tenant DB en `user_roles` con FK compuesta `(tenant_id,user_id)` | DONE | [S10_10.2](./PROGRESS/S10/S10_10.2.md) |
| S10_10.3 | Hardening tenant-aware en `audit_logs.actor_user_id` con FK compuesta y backfill seguro | DONE | [S10_10.3](./PROGRESS/S10/S10_10.3.md) |
| S10_10.4 | Reglas de alerting Prometheus versionadas (API down, 5xx, p95 latency) | DONE | [S10_10.4](./PROGRESS/S10/S10_10.4.md) |
| S10_10.5 | Runbook operativo minimo para incident response | DONE | [S10_10.5](./PROGRESS/S10/S10_10.5.md) |
| S10_10.6 | Matriz de cobertura minima por modulo y release checklist | DONE | [S10_10.6](./PROGRESS/S10/S10_10.6.md) |

## Notas

- Este archivo queda sincronizado con `PROGRESS/PROGRESS_INDEX.md` hasta `S10_10.6`.
- Se corrige la inconsistencia previa: `S10_10.1` a `S10_10.6` estaban marcados como `TODO` en este roadmap y ahora figuran `DONE`, en linea con `PROGRESS/PROGRESS_INDEX.md`.
- Fuente de verdad para estados: `PROGRESS/PROGRESS_INDEX.md` manda ante cualquier drift.
