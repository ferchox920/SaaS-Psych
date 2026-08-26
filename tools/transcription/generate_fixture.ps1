param(
    [string]$OutputPath = ".local/transcription/fixture-es.wav"
)

$resolvedOutput = [System.IO.Path]::GetFullPath((Join-Path (Get-Location) $OutputPath))
$outputDirectory = [System.IO.Path]::GetDirectoryName($resolvedOutput)
[System.IO.Directory]::CreateDirectory($outputDirectory) | Out-Null

$text = "Paciente ficticio: cuando mi pareja tarda en responder, pienso que ya no le importo. Me pongo ansioso, reviso el telefono y le escribo varias veces. Despues me averguenzo y digo que no deberia necesitar a nadie."
$voice = New-Object -ComObject SAPI.SpVoice
$spanishVoice = $voice.GetVoices() | Where-Object { $_.GetDescription() -match "Spanish|Espa" } | Select-Object -First 1
if (-not $spanishVoice) {
    throw "No local Spanish SAPI voice is installed."
}
$voice.Voice = $spanishVoice
$voice.Rate = -1

$format = New-Object -ComObject SAPI.SpAudioFormat
$format.Type = 18 # SAFT16kHz16BitMono
$stream = New-Object -ComObject SAPI.SpFileStream
$stream.Format = $format
$stream.Open($resolvedOutput, 3, $false) # SSFMCreateForWrite
try {
    $voice.AudioOutputStream = $stream
    [void]$voice.Speak($text)
} finally {
    $stream.Close()
}

Write-Output $resolvedOutput
