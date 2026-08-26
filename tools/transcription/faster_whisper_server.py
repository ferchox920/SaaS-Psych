import argparse
import json
import os
import tempfile
import threading
import time
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

from faster_whisper import WhisperModel


MAX_AUDIO_BYTES = 25 * 1024 * 1024
ALLOWED_FORMATS = {"wav", "webm", "ogg", "mp4", "m4a"}


class TranscriptionRuntime:
    def __init__(
        self,
        model_path: Path,
        device: str,
        compute_type: str,
        temp_dir: Path,
        cpu_threads: int = 4,
        beam_size: int = 1,
        initial_prompt: str = "Transcripción literal de psicoterapia en español rioplatense, sin resumir ni interpretar.",
    ) -> None:
        if not model_path.is_dir():
            raise RuntimeError("configured local transcription model path does not exist")
        temp_dir.mkdir(parents=True, exist_ok=True)
        self.model_path = model_path
        self.device = device
        self.compute_type = compute_type
        self.temp_dir = temp_dir
        self.cpu_threads = cpu_threads
        self.beam_size = beam_size
        self.initial_prompt = initial_prompt
        self.lock = threading.Lock()
        self.model = WhisperModel(
            str(model_path),
            device=device,
            compute_type=compute_type,
            cpu_threads=cpu_threads,
            num_workers=1,
            local_files_only=True,
        )

    def transcribe(self, audio: bytes, audio_format: str) -> dict:
        if not self.lock.acquire(blocking=False):
            raise BusyError()
        temp_path: Path | None = None
        try:
            with tempfile.NamedTemporaryFile(dir=self.temp_dir, suffix=f".{audio_format}", delete=False) as temporary:
                temporary.write(audio)
                temp_path = Path(temporary.name)
            started = time.perf_counter()
            segments, info = self.model.transcribe(
                str(temp_path),
                language="es",
                beam_size=self.beam_size,
                vad_filter=True,
                vad_parameters={"min_silence_duration_ms": 500, "speech_pad_ms": 250},
                condition_on_previous_text=False,
                initial_prompt=self.initial_prompt,
            )
            text = " ".join(segment.text.strip() for segment in segments).strip()
            elapsed = time.perf_counter() - started
            return {
                "text": text,
                "language": info.language,
                "duration_seconds": round(info.duration, 3),
                "transcription_seconds": round(elapsed, 3),
                "real_time_factor": round(elapsed / max(info.duration, 0.001), 3),
                "engine": "faster-whisper",
                "model": self.model_path.name,
            }
        finally:
            if temp_path is not None:
                temp_path.unlink(missing_ok=True)
            self.lock.release()


class BusyError(Exception):
    pass


class Handler(BaseHTTPRequestHandler):
    runtime: TranscriptionRuntime

    def log_message(self, _format: str, *_args: object) -> None:
        return

    def send_json(self, status: int, payload: dict) -> None:
        encoded = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(encoded)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        try:
            self.wfile.write(encoded)
        except (BrokenPipeError, ConnectionAbortedError, ConnectionResetError):
            return

    def do_GET(self) -> None:
        if self.path != "/health":
            self.send_json(HTTPStatus.NOT_FOUND, {"error": "not_found"})
            return
        self.send_json(HTTPStatus.OK, {
            "available": True,
            "busy": self.runtime.lock.locked(),
            "engine": "faster-whisper",
            "model": self.runtime.model_path.name,
            "device": self.runtime.device,
            "compute_type": self.runtime.compute_type,
            "beam_size": self.runtime.beam_size,
        })

    def do_POST(self) -> None:
        if self.path != "/transcribe":
            self.send_json(HTTPStatus.NOT_FOUND, {"error": "not_found"})
            return
        try:
            length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            length = 0
        audio_format = self.headers.get("X-Audio-Format", "").lower().strip()
        if length <= 0 or length > MAX_AUDIO_BYTES or audio_format not in ALLOWED_FORMATS:
            self.send_json(HTTPStatus.BAD_REQUEST, {"error": "invalid_audio"})
            return
        audio = self.rfile.read(length)
        try:
            result = self.runtime.transcribe(audio, audio_format)
            self.send_json(HTTPStatus.OK, result)
        except BusyError:
            self.send_json(HTTPStatus.CONFLICT, {"error": "busy"})
        except Exception as error:
            print(json.dumps({"event": "transcription_failed", "error_class": type(error).__name__}), flush=True)
            self.send_json(HTTPStatus.INTERNAL_SERVER_ERROR, {"error": "transcription_failed"})


def main() -> None:
    parser = argparse.ArgumentParser(description="Loopback-only faster-whisper service for SessionFlow.")
    parser.add_argument("--model-path", required=True)
    parser.add_argument("--port", type=int, default=8091)
    parser.add_argument("--device", default="cpu")
    parser.add_argument("--compute-type", default="int8")
    parser.add_argument("--cpu-threads", type=int, default=4)
    parser.add_argument("--beam-size", type=int, default=1)
    parser.add_argument("--initial-prompt", default="Transcripción literal de psicoterapia en español rioplatense, sin resumir ni interpretar.")
    parser.add_argument("--temp-dir", default=".local/transcription/tmp")
    args = parser.parse_args()
    runtime = TranscriptionRuntime(
        Path(args.model_path).resolve(),
        args.device,
        args.compute_type,
        Path(args.temp_dir).resolve(),
        cpu_threads=args.cpu_threads,
        beam_size=args.beam_size,
        initial_prompt=args.initial_prompt,
    )
    Handler.runtime = runtime
    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    print(json.dumps({"event": "transcriber_ready", "host": "127.0.0.1", "port": args.port, "model": runtime.model_path.name}), flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
