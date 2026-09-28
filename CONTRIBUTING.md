# Evaluar y contribuir

Empieza por [README.md](README.md) y [la demo local](docs/DEMO.md). Usa un checkout limpio y una base descartable; el seed contiene solo identidades ficticias. No subas `.env`, informes generados, bases, trazas ni capturas con información real.

Antes de abrir un PR, ejecuta formato, lint, tests y builds descritos en el README. Los cambios en autorización, concurrencia o revisión humana deben incluir una prueba que reproduzca la brecha y una ejecución contra PostgreSQL real cuando afecten transacciones. Para cambios de interfaz, agrega una prueba Playwright del comportamiento observable.

Describe en el PR qué cambió, qué riesgo protege, comandos ejecutados y límites de la verificación. Mantén las migraciones reversibles cuando sea posible y no modifiques el seed demo para simular resultados clínicos reales. Consulta [SECURITY.md](SECURITY.md) para reportes sensibles.
