const formats: Record<string, string> = {
  m4a: "audio/mp4", mp4: "audio/mp4", wav: "audio/wav",
  webm: "audio/webm", ogg: "audio/ogg",
};

export function validateAudioFile(file: { name: string; size: number; type: string }) {
  const extension = file.name.split(".").pop()?.toLowerCase() ?? "";
  const mime = formats[extension];
  if (!mime) throw new Error("Formato no admitido. Usa M4A, MP4, WAV, WebM u OGG.");
  if (!file.size) throw new Error("El archivo está vacío.");
  if (file.size > 100 * 1024 * 1024) throw new Error("El archivo supera el límite de esta pantalla (100 MiB). Usa el flujo de sesión para grabaciones grandes.");
  const type = file.type.toLowerCase().split(";")[0];
  if (type && type !== "application/octet-stream" && !type.startsWith("audio/") && !(extension === "mp4" && type === "video/mp4")) {
    throw new Error("El tipo de archivo no corresponde a una grabación de audio.");
  }
  return mime;
}
