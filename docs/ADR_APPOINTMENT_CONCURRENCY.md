# ADR: reservas atómicas por tenant

Estado: implementado.

## Decisión

La agenda actual reserva un intervalo para todo el tenant. El modelo no tiene un recurso de agenda propio del terapeuta ni un consultorio reservado; por eso no se introduce uno implícitamente. Dos citas activas del mismo tenant se consideran incompatibles si sus intervalos `[starts_at, ends_at)` se intersectan. Las citas adyacentes y las canceladas no bloquean un horario. Tenants distintos pueden usar el mismo intervalo.

`AppointmentRepository.Create` y `Update` toman un `pg_advisory_xact_lock` derivado del ID del tenant antes de comprobar solapamientos dentro de la misma transacción que escribe la cita y su auditoría. El bloqueo se libera al confirmar o revertir. Toda escritura de citas de la aplicación pasa por este repositorio. Las peticiones directas de SQL fuera del repositorio no reciben esta garantía; deben reservar por el mismo camino transaccional.

Se eligió este mecanismo en lugar de una exclusión nueva sobre toda la tabla porque los datos locales anteriores ya contienen intervalos solapados. Una restricción de exclusión rechazaría la migración antes de proteger nuevas escrituras. No se borran ni modifican citas históricas durante la migración. Los solapamientos históricos deben revisarse por separado antes de adoptar una exclusión declarativa.

La migración `000027` añade `revision` a cada cita. La respuesta HTTP la devuelve y las operaciones `PUT /appointments/{id}` y `POST /appointments/{id}/cancel` exigen `expected_revision`. El servicio contrasta la revisión observada y SQL solo actualiza cuando sigue vigente; cada escritura confirma `revision + 1`. Un conflicto devuelve `409` y la interfaz ofrece recargar la agenda. Las operaciones de escritura de auditoría permanecen en la transacción del repositorio.

## Verificación

`TestConcurrentOverlappingAppointmentCreatesCannotBothCommit` fuerza dos creaciones simultáneas y una creación contra una edición con una barrera, después comprueba que solo una confirma y que no queda ningún par incompatible. También comprueba cancelación, adyacencia y aislamiento entre tenants. `TestConcurrentAppointmentEditCannotUndoCancellation` hace competir una edición y una cancelación basadas en la misma revisión. `TestAppointmentUpdateRequiresObservedRevisionHTTP` comprueba `400` cuando falta revisión, `409` para revisiones antiguas y el estado posterior al conflicto.
