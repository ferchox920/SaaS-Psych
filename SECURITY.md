# Seguridad

SessionFlow es un proyecto de portafolio con datos ficticios. No está certificado para uso clínico ni debe recibir información real de pacientes en la demo.

## Reportar una vulnerabilidad

No publiques detalles explotables en un issue. Usa [GitHub private vulnerability reporting](https://github.com/ferchox920/SaaS-Psych/security/advisories/new) si está disponible; de lo contrario, contacta al autor por un canal privado desde su perfil de GitHub. Incluye versión o commit, impacto, pasos mínimos de reproducción con datos ficticios y una propuesta de mitigación si la tienes.

No hay un plazo de respuesta garantizado. Evita pruebas sobre despliegues ajenos y no incluyas datos personales, tokens ni contraseñas reales.

## Límites actuales

El modo demo solo arranca con `APP_ENV=local`; usa credenciales conocidas y una fixture fija. La autenticación, el aislamiento por tenant, las asignaciones clínicas, la revisión humana y la procedencia tienen pruebas automatizadas, pero eso no equivale a una evaluación de seguridad independiente. La retención y el tratamiento de datos reales requieren decisiones legales y operativas fuera del alcance de este portafolio.
