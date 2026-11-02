# Revisión de posibles pacientes desde Calendar

Primera etapa, solo lectura, disponible en Citas → Google Calendar → Revisar posibles pacientes.

La agenda abre por defecto la semana local de lunes a lunes (fin exclusivo para Calendar). Se reutiliza `GET /api/v1/integrations/google-calendar/events?from=&to=`: devuelve eventos pendientes de asociación del usuario y tenant autenticados. Los pacientes se obtienen mediante el listado autorizado existente, incluidas todas sus páginas. No hay nuevas rutas ni cambios de persistencia.

El botón de análisis utiliza títulos de eventos y pacientes ya cargados. Normaliza acentos, mayúsculas y puntuación; retira prefijos como Sesión o Consulta; agrupa títulos normalizados iguales y busca coincidencias completas o por palabras. Los eventos sin identidad conservan grupos separados. No se fusionan fichas, crean pacientes ni importan citas. Un nombre coincidente siempre requiere confirmación; dos personas con el mismo nombre pueden compartir un grupo de revisión.

Se muestran candidatos existentes, coincidencias ambiguas, posibles altas y eventos sin identidad. La normalización es una heurística inicial, no reconocimiento fiable de personas: no reconoce todos los eventos personales, apodos, faltas de ortografía o títulos complejos. Solo se compara con pacientes visibles; no se afirma que un paciente sea inexistente en toda la organización.

Sin OAuth configurado se puede ejecutar el mismo método con ejemplos ficticios o con títulos pegados, hasta 500 líneas no vacías. Las fuentes se identifican por separado y los títulos pegados no tienen horarios verificados. No se guardan resultados en almacenamiento del navegador. El análisis espera a que se carguen los pacientes y se descarta al cambiar rango, conexión o datos.

La medición mostrada abarca exclusivamente el método local; excluye red, OAuth, base de datos y renderizado. Los índices por nombre y palabra se construyen una vez por ejecución. No hay una consulta por evento; solo se examinan candidatos que comparten la primera palabra. La prueba sintética de 500 eventos es ilustrativa, no un benchmark de integración ni de carga de producción.

Verificación:

```powershell
corepack pnpm --filter web typecheck
corepack pnpm --filter web exec playwright test --config playwright.unit.config.ts
corepack pnpm --filter web lint
```

Siguiente etapa: decisiones persistidas por usuario, asociación de series y creación confirmada con claves idempotentes y transacciones. Antes de implementarla, evaluar falsos positivos y grupos ambiguos con títulos reales revisados por el usuario. La consulta real necesita configurar y autorizar Google Calendar; la conexión de Gmail del asistente es independiente.

También se captura el rechazo de inicio de sesión en LoginForm: AuthContext ya proporciona el mensaje visible y no debe propagarse una promesa rechazada al panel de errores de Next.
