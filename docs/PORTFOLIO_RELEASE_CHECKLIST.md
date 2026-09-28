# Estado de publicación de SessionFlow

Registro comprobado el 28 de septiembre de 2026, antes de crear el tag y la GitHub Release `v1.0.0-portfolio`. Este archivo documenta el cierre técnico; los enlaces a los workflows corresponden al commit verificado de `main`.

## Código y seguridad

- [x] El [PR #18](https://github.com/ferchox920/SaaS-Psych/pull/18) se fusionó por squash en `f543273634c6994328d570a2ce802179a1bd0d5c`.
- [x] La [CI posmerge](https://github.com/ferchox920/SaaS-Psych/actions/runs/36426661825) terminó correctamente. Incluyó build y tests de Go, lint, integración PostgreSQL/Redis, race detector, frontend y Python.
- [x] El frontend registró 71 pruebas aprobadas y 2 omisiones deliberadas. El E2E real aprobó 1 recorrido contra PostgreSQL, Redis, API y web, con migraciones desde cero y seed ficticio.
- [x] [CodeQL posmerge](https://github.com/ferchox920/SaaS-Psych/actions/runs/36426661761) terminó correctamente para Go, JavaScript/TypeScript y Python. Las alertas #2 y #3 (`go/allocation-size-overflow`) figuran como `fixed` desde el análisis del 28 de septiembre de 2026; no fueron descartadas.
- [x] `main` mantiene protección: PR obligatorio, rama actualizada, conversaciones resueltas, ocho checks obligatorios, protección para administradores y bloqueo de force push y eliminación.
- [x] Las [capturas del recorrido](screenshots/) muestran únicamente la demo ficticia; la auditoría presenta primero un resumen humano y deja los metadatos en el detalle desplegable.

## Ramas y publicación

Las ramas `portfolio/final-polish` y `portfolio/sessionflow-2026-09-26` se eliminaron después de comprobar que eran ancestros de `main` sin commits exclusivos. `portfolio/release-polish` se eliminó tras confirmar que su árbol de archivos coincidía exactamente con `main` y que el [PR #18](https://github.com/ferchox920/SaaS-Psych/pull/18) estaba fusionado; los cinco commits exclusivos en el grafo eran la consecuencia del squash, no trabajo pendiente. La rama documental `portfolio/release-readiness` se creó desde el `main` verificado para actualizar este registro.

Al momento de editar este archivo todavía no existían el tag ni la GitHub Release `v1.0.0-portfolio`. La publicación está autorizada después de fusionar el PR documental, verificar CI y CodeQL sobre el nuevo `main` y apuntar el tag anotado exactamente a ese SHA.

## Dependabot: nueve PR abiertos

Estado de los checks de cada head consultado el 28 de septiembre de 2026. Los fallos pertenecen a ramas de actualización y no afectan los workflows verdes de `main`. No se fusionó ni cerró ninguno de estos PR.

| PR | Actualización | Estado observado |
| --- | --- | --- |
| [#19](https://github.com/ferchox920/SaaS-Psych/pull/19) | Grupo Go | Falla Lint. |
| [#20](https://github.com/ferchox920/SaaS-Psych/pull/20) | Grupo web | Falla Frontend. |
| [#21](https://github.com/ferchox920/SaaS-Psych/pull/21) | TypeScript 7 | Fallan Frontend y Demo real. |
| [#3](https://github.com/ferchox920/SaaS-Psych/pull/3) | upload-artifact 7 | Checks verdes. |
| [#5](https://github.com/ferchox920/SaaS-Psych/pull/5) | psutil 7.2 | Checks verdes. |
| [#7](https://github.com/ferchox920/SaaS-Psych/pull/7) | setup-go 7 | Checks verdes. |
| [#8](https://github.com/ferchox920/SaaS-Psych/pull/8) | setup-node 7 | Checks verdes. |
| [#14](https://github.com/ferchox920/SaaS-Psych/pull/14) | @types/node 26 | Checks verdes. |
| [#15](https://github.com/ferchox920/SaaS-Psych/pull/15) | checkout 7 | Checks verdes. |

La agrupación nueva de Dependabot reduce propuestas futuras de versiones minor y patch por ecosistema; no modifica retroactivamente los PR ya abiertos.

## Límites del proyecto

SessionFlow es un proyecto de portafolio con datos ficticios. La fixture demo no hace inferencia clínica y el software no está certificado para uso clínico. No se anuncia despliegue público ni video. El análisis libre necesita configuración local. Las mejoras de selectores grandes y paneles clínicos quedan documentadas en el [README](../README.md); no son parte de este cierre editorial.
