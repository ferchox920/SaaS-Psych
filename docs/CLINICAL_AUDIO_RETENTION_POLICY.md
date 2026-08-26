# Política de audio clínico local

Estado actual: **captura efímera implementada y deshabilitada por defecto**.

## Regla operativa

- SessionFlow solicita micrófono solo después de una acción explícita y una confirmación visible de consentimiento.
- `CLINICAL_AUDIO_ENABLED=false` sigue siendo el valor predeterminado. Habilitarlo exige que el sidecar local esté instalado y activo.
- No se aceptan archivos, blobs, grabaciones, seeds ni fixtures con audio clínico real.
- No existe conservación de audio implícita ni configurable en esta versión.

## Contrato implementado en Fase 4

1. La grabación comienza únicamente por acción explícita y muestra un indicador rojo mientras está activa.
2. El blob permanece en memoria del navegador hasta transcribir o descartar.
3. El backend lo envía solo al sidecar `127.0.0.1`; el sidecar usa un archivo temporal acotado y lo elimina en `finally` ante éxito o error.
4. Cancelar descarta el blob del navegador y cancela la solicitud.
5. Auditoría y logs guardan métricas, formato y tamaño, nunca audio o transcripción.
6. La transcripción vuelve al bloc como texto editable y requiere otra acción manual para analizarse con Qwen.
7. No existe función de conservación de audio. Incorporarla requeriría otro gate, consentimiento documentado, retención y borrado verificable.
8. El runtime fuerza modelo local y modo offline; no hay proveedor externo.

Hasta superar la calibración acústica clínica, solo deben utilizarse audios ficticios o completamente desidentificados.
