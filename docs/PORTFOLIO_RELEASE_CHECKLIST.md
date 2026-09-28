# Preparación de la entrega de portafolio

Estado observado el 28 de septiembre de 2026. Este documento registra decisiones de publicación y triaje; no declara una release ni una licencia.

## Notas propuestas para `v1.0.0-portfolio`

**Título sugerido:** SessionFlow — recorrido de portafolio con revisión humana.

**Resumen:** Demo ficticia desde paciente, cita y sesión hasta informe, decisión humana, estado longitudinal y auditoría. Backend Go, frontend Next.js, PostgreSQL y Redis; prueba de navegador full-stack en CI. La fixture de demo no hace inferencia clínica y el producto no está certificado para uso clínico.

Antes de crear el tag o la release:

- [ ] Obtener aprobación explícita del PR de cierre y fusionarlo mediante GitHub.
- [ ] Confirmar CI y CodeQL verdes en el commit de `main` resultante.
- [ ] Comprobar que las alertas de CodeQL #2 y #3 se cerraron por la corrección, sin descartarlas manualmente.
- [ ] Decidir y documentar la licencia. MIT permite reutilización con atribución y sin garantía; sin licencia, el código visible permanece con todos los derechos reservados.
- [ ] Confirmar que las capturas siguen representando la interfaz publicada y que solo contienen datos ficticios.
- [ ] Crear `v1.0.0-portfolio` y publicar la release únicamente con autorización del propietario.

## Gobierno de GitHub

`main` tiene protección aplicada mediante la API de GitHub: exige PR, ramas actualizadas, resolución de conversaciones y los checks Test + Build, Lint, Race, Integration, Frontend, Transcription Python, Demo real y CodeQL. También impide force push y borrado; la protección se aplica a administradores. No exige aprobaciones de terceros (`required_approving_review_count: 0`) para que el propietario pueda mantener el proyecto mediante PR.

No hay tags, releases ni licencia. Las ramas fusionadas `portfolio/final-polish` y `portfolio/sessionflow-2026-09-26` permanecen en remoto y pueden retirarse tras decisión del propietario; no se borraron.

## Triaje de Dependabot

Los 15 PR siguientes estaban abiertos al preparar este cierre. “Verde” significa que los checks del head observado terminaron correctamente; deberán repetirse contra la base vigente antes de integrarlos. Ninguno se fusionó ni cerró aquí.

| PR | Ecosistema | Riesgo y estado observado | Siguiente paso |
| --- | --- | --- | --- |
| #3 upload-artifact 6→7 | Actions | Major; verde | Revisar notas de migración y agrupar con otras Actions compatibles. |
| #7 setup-go 6→7 | Actions | Major; verde | Revisar versión del runtime de la acción. |
| #8 setup-node 6→7 | Actions | Major; verde | Revisar versión del runtime de la acción. |
| #15 checkout 6→7 | Actions | Major; verde | Revisar permisos y comportamiento de checkout. |
| #4 OpenTelemetry 1.41→1.46 | Go | Lint falló: dependencia eleva Go objetivo a 1.25; linter fijado fue compilado con 1.24 | Resolver juntos toolchain y linter antes de reintentar. |
| #6 Prometheus 1.23→1.24 | Go | Mismo fallo de toolchain/linter | Mismo tratamiento; no mezclar con este cierre. |
| #9 go-redis 9.18→9.22 | Go | Verde; afecta integración Redis | Integrar con regresión PostgreSQL/Redis. |
| #10 miniredis 2.37→2.39 | Go | Verde; dependencia de tests | Integrar después de #9 si sigue vigente. |
| #11 pgx 5.7→5.11 | Go | Mismo fallo de toolchain/linter | Resolver toolchain; repetir integración real. |
| #5 psutil 7.0→7.2 | Python | Verde; dependencia de transcripción | Integrar con prueba Python. |
| #12 @hookform/resolvers 5.2→5.9 | npm | Verde; formularios | Integrar con suite web. |
| #13 eslint-config-next 16.1→16.3 | npm | Frontend falló: regla de efectos React rechaza un `setState` síncrono en un efecto | Corregir el patrón señalado por lint y repetir toda la suite; no silenciar la regla. |
| #14 @types/node 20→26 | npm | Major; verde | Revisar alineación con Node 22 y tipos usados antes de integrar. |
| #16 zod 4.3→4.6 | npm | Verde; validación | Integrar con pruebas de formularios y contratos. |
| #17 Tailwind 4.2→4.3 | npm | Verde; posible impacto visual | Revisar capturas y móvil además de CI. |

Orden sugerido: primero los PR verdes de bajo riesgo (#5, #10, #12), después integraciones y UI (#9, #16, #17), luego Actions y majors con revisión manual; dejar #4, #6, #11 y #13 para correcciones específicas. La agrupación nueva de Dependabot reduce futuras propuestas menores y parches, pero no modifica estos PR ya abiertos.

## Deuda posterior

Los archivos `clinical_longitudinal_repository.go`, `clinical_strategy_merge.go`, `gira_semantic.go`, `clinical-workspace.tsx`, `session-workspace/workspace.tsx` y `cmd/server/main.go` concentran responsabilidades. Se dejaron fuera de este PR porque no eran necesarios para corregir las brechas verificadas. Los selectores grandes y la simplificación del panel clínico siguen descritos en el README.
