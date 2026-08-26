param(
    [string]$PythonPath = ".local/transcription-venv/Scripts/python.exe",
    [ValidateSet("live", "review")]
    [string]$Profile = "live",
    [string]$ModelPath = "",
    [int]$Port = 8091,
    [int]$CpuThreads = 4,
    [int]$BeamSize = 0,
    [ValidateSet("cuda", "cpu")]
    [string]$Device = "cuda"
)

if ([string]::IsNullOrWhiteSpace($ModelPath)) {
    $ModelPath = ".local/transcription/faster-whisper-large-v3"
}
if ($BeamSize -le 0) {
    $BeamSize = if ($Profile -eq "review") { 5 } else { 1 }
}

$python = [System.IO.Path]::GetFullPath((Join-Path (Get-Location) $PythonPath))
$model = [System.IO.Path]::GetFullPath((Join-Path (Get-Location) $ModelPath))
if (-not (Test-Path -LiteralPath $python) -or -not (Test-Path -LiteralPath $model)) {
    throw "Local transcriber runtime or model is missing. Follow docs/LOCAL_TRANSCRIPTION.md."
}

$env:HF_HUB_OFFLINE = "1"
$env:HF_HUB_DISABLE_TELEMETRY = "1"
$computeType = "int8"
if ($Device -eq "cuda") {
    $ollamaCuda = Join-Path $env:LOCALAPPDATA "Programs/Ollama/lib/ollama/cuda_v12"
    if (Test-Path -LiteralPath (Join-Path $ollamaCuda "cublas64_12.dll")) {
        $env:PATH = "$ollamaCuda;$env:PATH"
        # Int8 weights with FP16 compute preserve the tested transcript while
        # leaving more VRAM headroom for qwen3.5:9b on the 6 GB RTX 4050.
        $computeType = "int8_float16"
    } else {
        Write-Warning "Ollama CUDA libraries were not found; falling back to CPU/int8."
        $Device = "cpu"
    }
}
& $python "tools/transcription/faster_whisper_server.py" --model-path $model --port $Port --device $Device --compute-type $computeType --cpu-threads $CpuThreads --beam-size $BeamSize
