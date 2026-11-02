# Prueba progresiva de transcripción

El piloto empieza con los primeros **60 segundos exactos** de un audio local. Las etapas de 300 y 600 segundos se preparan solo al pedirlas explícitamente. El script no graba, reproduce, llama modelos ni envía archivos: prepara una fuente común, registra resultados y compara textos.

Los audios, transcripciones, capturas y resultados reales se guardan en `.local/` (ignorado por Git) o fuera del repositorio. El código y sus tests contienen solo datos sintéticos. Nunca usar una transcripción larga sin tiempos para estimar por cantidad de palabras dónde termina el primer minuto.

## Preparar el caso

Usar el entorno Python de transcripción que ya contiene PyAV y NumPy:

```powershell
.\.local\transcription-venv\Scripts\python.exe tools/transcription/benchmark_case.py prepare --source '<audio-local>' --case '.local/transcription-benchmarks/caso-60s'
```

Genera `audio.wav` PCM de 16 bits a la frecuencia y canales originales: conserva exactamente los primeros N frames decodificados del intervalo. La conversión a PCM de 16 bits queda declarada; no es una copia binaria del AAC original. Se conservan hash del original, hash del recorte, receta, duración y número de muestras. Una repetición verifica la integridad y reutiliza el mismo caso; otro audio o intervalo requiere otra carpeta. Una preparación incompleta no crea un manifiesto válido.

## Versiones

1. `windows_dictation`: reproducir el WAV por la entrada de audio seleccionada y usar **Dictar en la app de Windows**. Registrar texto literal, tiempos, ganancia, ruta de audio, versión de la app si se conoce e interrupciones. El dictado no acredita quién habló. Conservar/restaurar los dispositivos y volúmenes del usuario.
2. `windows_audio_attachment`: siguiente variante, adjuntar **ese mismo WAV** en la app de Windows. Si la interfaz recibe el archivo pero no transcribe, registrar `unsupported`; no sustituirlo silenciosamente con otro motor ni contar el fallo como transcripción vacía de calidad cero.

Los dos recorridos se prueban por separado. El modelo exacto del dictado puede no estar expuesto: declararlo desconocido en vez de inferirlo del selector de chat. Estas pruebas manuales asistidas no constituyen un agente local autónomo.

```powershell
.\.local\transcription-venv\Scripts\python.exe tools/transcription/benchmark_case.py record --case '.local/transcription-benchmarks/caso-60s' --run-id v1-windows-001 --route windows_dictation --transcript '<texto-local>' --settings-json '<condiciones-locales.json>'
# Solo después de ejecutar la variante 2:
.\.local\transcription-venv\Scripts\python.exe tools/transcription/benchmark_case.py record --case '.local/transcription-benchmarks/caso-60s' --run-id v2-archivo-001 --route windows_audio_attachment --transcript '<otro-texto-local>'
.\.local\transcription-venv\Scripts\python.exe tools/transcription/benchmark_case.py compare --case '.local/transcription-benchmarks/caso-60s' --baseline-run v1-windows-001 --candidate-run v2-archivo-001 --output '.local/transcription-benchmarks/caso-60s/comparison-v1-v2.json'
```

Cada resultado tiene un ID de ensayo inmutable, ruta, condiciones y hashes. Repetir la misma importación es idempotente; cambiarla exige otro ID. La preparación del caso no ejecuta ninguna variante. La comparación rechaza resultados de otra fuente o textos modificados después del registro.

## Qué significa la comparación

Entre versiones automáticas se informa **desacuerdo**, no precisión: distancia por palabras, inserciones, omisiones, sustituciones y diff literal. Se ignoran mayúsculas y puntuación para la métrica por palabras, pero se conserva el diff original y los acentos. Dos transcripciones coincidentes pueden compartir errores.

Para medir WER hace falta una referencia escuchada y corregida por una persona, del mismo minuto. Solo se acepta con `--reviewed-reference '<texto>' --human-reviewed` en lugar de `--baseline-run`. Esa bandera es una declaración manual local, no aprobación clínica en SaaS-Psi. Sin referencia humana, `word_error_rate` permanece `null`.

Revisar especialmente negaciones, nombres, cifras, omisiones, agregados y cambios de voz. El archivo original, el texto automático y las correcciones se conservan por separado. No atribuir hablantes ni producir informes aprobados como parte del benchmark. Antes de pasar a 5 minutos y luego 10, comprobar que la interfaz no cierre la captura antes del final.

## Verificación del código

```powershell
.\.local\transcription-venv\Scripts\python.exe -m unittest discover -s tools/transcription -p 'test_*.py' -v
```

Los tests cubren recorte por muestras, integridad, reintentos, identidad de fuente y las diferencias entre coincidencia automática y una referencia humana. No descargan modelos ni ejecutan transcripción clínica.

## Ensayos locales: Whisper y Qwen3-ASR

`benchmark_local_asr.py` ejecuta un modelo ya instalado sobre el `audio.wav` verificado del caso. No recibe un texto esperado ni pistas tomadas de Dictar. Conserva la salida literal en un ensayo nuevo, con ruta `local_faster_whisper` o `local_qwen_asr`. No cambia la configuración del transcriptor de la aplicación ni importa resultados a la historia clínica.

El perfil Whisper usa `large-v3` mediante faster-whisper/CTranslate2, CUDA `int8_float16`, `beam_size=5`, español, VAD y contexto entre segmentos. Qwen usa el checkpoint oficial `Qwen3-ASR-1.7B-hf`, BF16, atención SDPA, un solo audio y generación determinista, sin alineador ni diarización. La primera comparación usa el minuto completo en cada modelo, ejecutados en procesos separados para liberar la memoria de GPU entre ellos. Un resultado vacío o truncado no se registra como transcripción exitosa.

Para controlar el efecto de la cuantización, repetir Whisper con un ID nuevo y `--whisper-compute-type float16`. Mantener las mismas opciones de decodificación y el mismo archivo. Más precisión numérica de los pesos no garantiza una transcripción más fiel.

Instalar Qwen en un entorno separado evita modificar el entorno de Whisper:

```powershell
.\.local\transcription-venv\Scripts\python.exe -m venv .local/qwen-asr-venv
.\.local\qwen-asr-venv\Scripts\python.exe -m pip install torch==2.10.0 --index-url https://download.pytorch.org/whl/cu128
.\.local\qwen-asr-venv\Scripts\python.exe -m pip install -r tools/transcription/requirements-qwen-asr.txt
```

Descargar el checkpoint por separado, con la revisión elegida, desde el repositorio oficial. En el piloto se usó `Qwen/Qwen3-ASR-1.7B-hf`, revisión `bcd2b5b7f32b480ab5790554cfa8347f246a14f3`. Los archivos necesarios son los JSON de configuración/tokenizador, `chat_template.jinja` y `model.safetensors`; no hace falta ejecutar código remoto. Registrar repositorio y revisión en `download-provenance.json` junto a los pesos. Después de esa preparación, el runner impone modo offline de Hugging Face/Transformers, `local_files_only=True` y bloquea conexiones Python TCP/UDP durante la inferencia.

```powershell
# Usar una carpeta privada real en lugar de <caso> y un ID nuevo por ejecución.
.\.local\transcription-venv\Scripts\python.exe tools/transcription/benchmark_local_asr.py --case '<caso>' --run-id whisper-large-v3-001 --engine faster-whisper --model-path .local/transcription/faster-whisper-large-v3 --cuda-library-dir .local/qwen-asr-venv/Lib/site-packages/torch/lib
.\.local\qwen-asr-venv\Scripts\python.exe tools/transcription/benchmark_local_asr.py --case '<caso>' --run-id qwen3-asr-17b-001 --engine qwen3-asr --model-path .local/transcription/qwen3-asr-1.7b-hf
```

Whisper puede reutilizar las DLL CUDA 12/cuDNN 9 incluidas en esa instalación de PyTorch para Windows. La ejecución exige CUDA y no cambia silenciosamente a CPU. Un fallo queda registrado; otro intento debe usar otro ID.

Se miden por separado importación, carga y procesamiento del audio. `pipeline_seconds` incluye esos tres pasos en un proceso nuevo y excluye instalación, descarga y verificación de hashes. La memoria de GPU muestreada con `nvidia-smi` es el consumo **total del dispositivo**, incluyendo otras aplicaciones; Qwen agrega sus máximos propios de PyTorch. Una sola ejecución corta no establece capacidad sostenida ni precisión clínica.

Comparar los ensayos locales con `compare --baseline-run ...` conserva la distinción entre desacuerdo y precisión. La referencia de Dictar también es automática: la coincidencia con ella no acredita fidelidad. Escuchar y corregir una referencia humana antes de informar WER o afirmar que un motor transcribe mejor.

Fuentes de implementación: [faster-whisper](https://github.com/SYSTRAN/faster-whisper), [Qwen3-ASR nativo para Transformers](https://huggingface.co/Qwen/Qwen3-ASR-1.7B-hf), [PyTorch](https://pytorch.org/get-started/locally/).
