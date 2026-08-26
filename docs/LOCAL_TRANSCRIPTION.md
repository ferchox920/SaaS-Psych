# Transcripción clínica local

## Decisión técnica

SessionFlow usa **faster-whisper 1.2.1**, modelo multilingual `large-v3`, CUDA con pesos int8 y cómputo FP16 (`int8_float16`), detrás de un sidecar HTTP fijo en `127.0.0.1:8091`. Reutiliza las bibliotecas CUDA locales de Ollama y cae de forma explícita a CPU/int8 si no están disponibles.

Las fuentes oficiales indican que faster-whisper usa CTranslate2, incluye decodificación mediante PyAV sin exigir FFmpeg del sistema y soporta CPU/int8. whisper.cpp ofrece un binario C/C++ ligero y soporte Windows, pero en este equipo quedó por detrás con el mismo modelo `small`.

- faster-whisper: https://github.com/SYSTRAN/faster-whisper
- whisper.cpp: https://github.com/ggml-org/whisper.cpp

## Instalación explícita

Los modelos no se descargan al ejecutar SessionFlow. La preparación se realiza una sola vez y requiere la bandera explícita `--allow-download`:

```powershell
python -m venv .local/transcription-venv
.\.local\transcription-venv\Scripts\python.exe -m pip install -r tools\transcription\requirements.txt
.\.local\transcription-venv\Scripts\python.exe tools\transcription\download_faster_whisper_model.py `
  --model large-v3 `
  --output .local\transcription\faster-whisper-large-v3 `
  --allow-download
```

Después se inicia el sidecar:

```powershell
powershell -ExecutionPolicy Bypass -File tools\transcription\start_transcriber.ps1
```

El perfil `live` es el predeterminado y usa `beam_size=1` para mantener baja latencia. Para una retranscripción final de máxima precisión, inicia el sidecar con:

```powershell
powershell -ExecutionPolicy Bypass -File tools\transcription\start_transcriber.ps1 -Profile review
```

El perfil `review` usa `beam_size=5`. Ambos perfiles emplean el mismo modelo local `large-v3`, por lo que cambiar de perfil no descarga ni duplica pesos.

Y se habilita en `.env`:

```dotenv
CLINICAL_AUDIO_ENABLED=true
TRANSCRIBER_BASE_URL=http://127.0.0.1:8091
TRANSCRIBER_TIMEOUT_SECONDS=120
TRANSCRIBER_MAX_AUDIO_MB=25
```

El script de arranque establece `HF_HUB_OFFLINE=1` y `HF_HUB_DISABLE_TELEMETRY=1`. El servidor usa exclusivamente un path local y `local_files_only=True`. Los perfiles fijan cuatro hilos CPU, español y un prompt literal de psicoterapia rioplatense; no resumen ni interpretan.

## Flujo y privacidad

1. Fernando selecciona paciente y sesión.
2. Confirma consentimiento y pulsa **Iniciar grabación**.
3. La UI muestra **GRABANDO**, permite pausar, continuar o finalizar y corta el fragmento a los 30 segundos.
4. El audio queda en memoria y requiere **Transcribir localmente**.
5. API y sidecar procesan en loopback; el archivo temporal se elimina siempre.
6. La transcripción aparece en el bloc y puede corregirse.
7. Solo **Analizar localmente** envía el texto revisado a Qwen.

No hay grabación automática, conservación de audio, envío directo a Qwen ni fallback externo.

## Alineación con ClinicalSession

El contrato compatible actual continúa siendo `POST /api/v1/appointments/:appointment_id/transcription`: la autorización y la auditoría se resuelven mediante la cita y todavía no se persiste un artefacto de transcripción. Por lo tanto, la transcripción efímera no puede vincularse hoy de forma durable a `clinical_session_id`.

No se agregó un identificador opcional sin validación porque permitiría declarar una sesión no correspondiente a la cita, al cliente o al tenant. El punto de extensión seguro para la futura capa de ingestión es incorporar un resolver tenant-aware que valide `clinical_session_id → appointment_id + client_id`, conservar el endpoint actual como adaptador compatible y hacer que cualquier artefacto durable nuevo referencie `ClinicalSession`. Esto requiere diseñar antes consentimiento durable y retención del texto; queda deliberadamente fuera de Stage 2A.1.

## Benchmark controlado

Fixture: voz SAPI española generada localmente, texto ficticio, 15,988 s. Qwen `qwen3.5:9b` precargado con ~3,37 GB de VRAM. Inferencia de voz sin red.

| Motor | Modelo/configuración | Transcripción | RAM pico | VRAM adicional | WER |
|---|---|---:|---:|---:|---:|
| faster-whisper | small CPU/int8 | 1,906 s | 548,1 MiB | 0 MiB | 2,78 % |
| whisper.cpp 1.9.2 | small CPU, 8 hilos | 4,090 s total | 651,0 MiB | 0 MiB | 5,56 % |
| faster-whisper | medium CPU/int8 | 5,630 s | 1.497 MiB | 0 MiB | 0 % |
| faster-whisper | medium CUDA/int8_float16, caliente y junto a Qwen | 1,039 s | n/d | ~1,1 GiB | 0 % |
| faster-whisper | large-v3 CUDA/int8_float16, perfil live | 2,663 s | n/d | ~2,9 GiB total | 2,78 %* |
| faster-whisper | large-v3 CUDA/int8_float16, perfil review | 2,688 s | n/d | ~2,9 GiB total | 2,78 %* |

La aceleración CUDA redujo aproximadamente 5,4 veces el tiempo de inferencia frente al baseline CPU del mismo modelo. Con Qwen y Whisper cargados simultáneamente quedaron unos 271 MiB libres de VRAM en la prueba controlada; por eso se mantiene `int8_float16` y no FP16 completo.

\* El único error de palabra del fixture fue `avergenzo` en lugar de `avergüenzo`. Las mediciones de large-v3 se realizaron sin Qwen cargado y dejaron aproximadamente 3 GiB libres en la RTX 4050 de 6 GB.

La voz sintética no sustituye una evaluación con distintos acentos, ruido, solapamientos y habla clínica espontánea. Por eso toda transcripción sigue requiriendo revisión visual antes de análisis.
