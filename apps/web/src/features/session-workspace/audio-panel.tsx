"use client";
import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { MAX_AUDIO_BYTES, validateAudio } from "./model";

export function AudioPanel({
  upload,
  verify,
}: {
  upload: (blob: Blob, mime: string) => Promise<void>;
  verify: () => Promise<boolean>;
}) {
  const [phase, setPhase] = useState("idle");
  const [error, setError] = useState("");
  const [audio, setAudio] = useState<Blob | null>(null);
  const recorder = useRef<MediaRecorder | null>(null);
  const stream = useRef<MediaStream | null>(null);
  const alive = useRef(true);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
      clearTimeout(timer.current);
      if (recorder.current?.state !== "inactive") recorder.current?.stop();
      stream.current?.getTracks().forEach((track) => track.stop());
    };
  }, []);
  async function start() {
    setError("");
    setAudio(null);
    setPhase("permission");
    try {
      if (!(await verify())) throw new Error("consent");
      if (!alive.current) return;
      if (
        !navigator.mediaDevices?.getUserMedia ||
        typeof MediaRecorder === "undefined"
      )
        throw new Error("unsupported");
      const mime = [
        "audio/webm;codecs=opus",
        "audio/ogg;codecs=opus",
        "audio/mp4",
      ].find((type) => MediaRecorder.isTypeSupported(type));
      if (!mime) throw new Error("unsupported");
      const media = await navigator.mediaDevices.getUserMedia({ audio: true });
      if (!alive.current) {
        media.getTracks().forEach((track) => track.stop());
        return;
      }
      stream.current = media;
      const instance = new MediaRecorder(media, { mimeType: mime });
      recorder.current = instance;
      const chunks: Blob[] = [];
      let size = 0;
      let overflow = false;
      instance.ondataavailable = (event) => {
        size += event.data.size;
        if (size > MAX_AUDIO_BYTES) {
          overflow = true;
          if (instance.state !== "inactive") instance.stop();
          return;
        }
        chunks.push(event.data);
      };
      instance.onstop = () => {
        clearTimeout(timer.current);
        media.getTracks().forEach((track) => track.stop());
        if (!alive.current) return;
        const blob = new Blob(chunks, { type: mime.split(";")[0] });
        if (overflow || !blob.size) {
          setPhase("failed");
          setError(
            "La captura está vacía o excedió 25 MiB. No se subió audio.",
          );
          return;
        }
        setAudio(blob);
        setPhase("idle");
      };
      instance.onerror = () => {
        overflow = true;
        media.getTracks().forEach((track) => track.stop());
        if (alive.current) {
          setPhase("failed");
          setError("Falló la captura. Usa la alternativa de archivo.");
        }
      };
      instance.start(1000);
      setPhase("capturing");
      timer.current = setTimeout(
        () => {
          if (instance.state !== "inactive") instance.stop();
        },
        10 * 60 * 1000,
      );
    } catch {
      stream.current?.getTracks().forEach((track) => track.stop());
      if (alive.current) {
        setPhase("failed");
        setError(
          "No se pudo habilitar el micrófono. Revisa permiso, consentimiento y compatibilidad del navegador.",
        );
      }
    }
  }
  async function send() {
    if (!audio) return;
    const invalid = validateAudio(audio.size, audio.type);
    if (invalid) {
      setError(invalid);
      return;
    }
    setPhase("uploading");
    setError("");
    try {
      if (!(await verify()) || !alive.current) throw new Error("consent");
      await upload(audio, audio.type);
      if (alive.current) {
        setAudio(null);
        setPhase("uploaded");
      }
    } catch {
      if (alive.current) {
        setPhase("failed");
        setError(
          "No se confirmó el upload. Actualiza los artefactos antes de volver a subirlo.",
        );
      }
    }
  }
  const busy = ["capturing", "paused", "uploading", "permission"].includes(
    phase,
  );
  return (
    <div className="space-y-3">
      <p>
        Audio local · límite 25 MiB. Cada captura se detiene a los 10 minutos;
        no hay streaming continuo ni subida automática.
      </p>
      <div className="flex flex-wrap gap-2">
        <Button disabled={busy} onClick={() => void start()}>
          Grabar con micrófono
        </Button>
        <Button
          variant="secondary"
          disabled={phase !== "capturing" && phase !== "paused"}
          onClick={() => {
            const r = recorder.current;
            if (!r) return;
            if (r.state === "recording") {
              r.pause();
              setPhase("paused");
            } else if (r.state === "paused") {
              r.resume();
              setPhase("capturing");
            }
          }}
        >
          {phase === "paused" ? "Continuar captura" : "Pausar"}
        </Button>
        <Button
          variant="outline"
          disabled={phase !== "capturing" && phase !== "paused"}
          onClick={() => recorder.current?.stop()}
        >
          Detener
        </Button>
      </div>
      <label className="block space-y-2">
        O subir un archivo local
        <Input
          type="file"
          accept="audio/wav,audio/webm,audio/ogg,audio/mp4,audio/x-m4a,.m4a"
          disabled={busy}
          onChange={(event) => {
            const file = event.target.files?.[0];
            if (!file) return;
            const type =
              file.type === "audio/x-wav"
                ? "audio/wav"
                : file.type === "audio/m4a"
                  ? "audio/x-m4a"
                  : file.type;
            const invalid = validateAudio(file.size, type);
            setError(invalid ?? "");
            setAudio(invalid ? null : file.slice(0, file.size, type));
            setPhase(invalid ? "failed" : "idle");
            event.target.value = "";
          }}
        />
      </label>
      {audio && (
        <p>
          Audio pendiente de envío: {(audio.size / 1024 / 1024).toFixed(2)} MiB.
          Se descarta al salir del workspace.
        </p>
      )}
      <Button disabled={!audio || busy} onClick={() => void send()}>
        Subir audio
      </Button>
      <p role="status" aria-live="polite">
        {
          {
            idle: "Sin captura activa",
            permission: "Solicitando permiso…",
            capturing: "Grabando",
            paused: "Captura pausada",
            uploading: "Subiendo…",
            uploaded: "Audio subido. Todavía no implica una transcripción.",
            failed: "Operación no completada",
          }[phase]
        }
      </p>
      {error && <p role="alert">{error}</p>}
    </div>
  );
}
