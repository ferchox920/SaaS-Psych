"""Run an installed ASR model on a private fixture, with network connections blocked."""
import argparse
from contextlib import contextmanager
from datetime import datetime, timezone
import importlib.metadata
import json
import os
from pathlib import Path
import socket
import subprocess
import sys
import threading
import time

from benchmark_case import digest, load_case, private_path, record, safe_id, write_json


@contextmanager
def no_network():
    """Fail closed on Python network sockets, including library download fallbacks."""
    attempts = []
    originals = {}
    for name in ("connect", "connect_ex", "sendto"):
        originals[name] = getattr(socket.socket, name)

        def blocked(*args, _operation=name, **kwargs):
            attempts.append(_operation)
            raise RuntimeError("Network access is disabled during local ASR inference")

        setattr(socket.socket, name, blocked)
    try:
        yield attempts
    finally:
        for name, original in originals.items():
            setattr(socket.socket, name, original)


def gpu_snapshot():
    try:
        flags = {"creationflags": subprocess.CREATE_NO_WINDOW} if os.name == "nt" else {}
        output = subprocess.check_output(
            ["nvidia-smi", "--query-gpu=name,memory.total,memory.used,memory.free", "--format=csv,noheader,nounits"],
            text=True, stderr=subprocess.DEVNULL, timeout=5, **flags,
        ).strip().splitlines()[0]
        name, total, used, free = (v.strip() for v in output.split(","))
        return {"name": name, "total_mib": int(total), "used_mib": int(used), "free_mib": int(free)}
    except (OSError, ValueError, subprocess.SubprocessError):
        return None


class MemorySampler:
    def __init__(self):
        import psutil
        self.process = psutil.Process()
        self.initial_gpu = gpu_snapshot()
        self.peak_gpu_used_mib = self.initial_gpu["used_mib"] if self.initial_gpu else None
        self.peak_rss = self.process.memory_info().rss
        self.stop = threading.Event()
        self.thread = threading.Thread(target=self.sample, daemon=True)

    def sample(self):
        while not self.stop.is_set():
            self.peak_rss = max(self.peak_rss, self.process.memory_info().rss)
            current = gpu_snapshot()
            if current:
                self.peak_gpu_used_mib = max(self.peak_gpu_used_mib or 0, current["used_mib"])
            self.stop.wait(0.25)

    def finish(self):
        self.stop.set()
        self.thread.join(timeout=6)
        self.peak_rss = max(self.peak_rss, self.process.memory_info().rss)
        current = gpu_snapshot()
        if current:
            self.peak_gpu_used_mib = max(self.peak_gpu_used_mib or 0, current["used_mib"])
        return {"peak_process_rss_mib": round(self.peak_rss / 1024**2, 1),
                "gpu_before": self.initial_gpu, "peak_gpu_device_used_mib": self.peak_gpu_used_mib,
                "gpu_sampling_note": "Device-wide usage, including other apps; sampled every >=250 ms, not per-process allocation"}


def package_versions():
    result = {"python": sys.version.split()[0]}
    for name in ("faster-whisper", "ctranslate2", "torch", "transformers", "accelerate", "av", "numpy"):
        try:
            result[name] = importlib.metadata.version(name)
        except importlib.metadata.PackageNotFoundError:
            pass
    return result


def whisper(args, audio):
    began = time.perf_counter()
    from faster_whisper import WhisperModel
    imported = time.perf_counter()
    model = WhisperModel(str(args.model_path), device="cuda", compute_type=args.whisper_compute_type,
                         cpu_threads=4, num_workers=1, local_files_only=True)
    loaded = time.perf_counter()
    segments, info = model.transcribe(str(audio), language="es", beam_size=5, temperature=0,
                                      vad_filter=True, condition_on_previous_text=True,
                                      initial_prompt=None, word_timestamps=False)
    captured = [{"start": s.start, "end": s.end, "text": s.text,
                 "avg_logprob": s.avg_logprob, "no_speech_prob": s.no_speech_prob} for s in segments]
    finished = time.perf_counter()
    text = " ".join(s["text"].strip() for s in captured).strip()
    return text, {"import_seconds": imported-began, "model_load_seconds": loaded-imported,
                  "processing_seconds": finished-loaded, "pipeline_seconds": finished-began,
                  "segments": captured, "input_duration_seconds": info.duration,
                  "compute_type": model.model.compute_type, "device": "cuda",
                  "language": "es", "beam_size": 5, "temperature": 0,
                  "vad_filter": True, "condition_on_previous_text": True,
                  "initial_prompt": None, "word_timestamps": False}


def qwen(args, audio):
    began = time.perf_counter()
    import torch
    from transformers import AutoProcessor, AutoModelForMultimodalLM
    imported = time.perf_counter()
    if not torch.cuda.is_available():
        raise RuntimeError("CUDA unavailable; refusing to silently switch to CPU")
    torch.set_num_threads(4)
    torch.manual_seed(0)
    torch.cuda.reset_peak_memory_stats()
    processor = AutoProcessor.from_pretrained(str(args.model_path), local_files_only=True, trust_remote_code=False)
    model = AutoModelForMultimodalLM.from_pretrained(
        str(args.model_path), device_map="cuda:0", dtype=torch.bfloat16,
        attn_implementation="sdpa", local_files_only=True, trust_remote_code=False,
    ).eval()
    torch.cuda.synchronize()
    loaded = time.perf_counter()
    with torch.inference_mode():
        inputs = processor.apply_transcription_request(audio=str(audio), language="Spanish").to(model.device, model.dtype)
        output = model.generate(**inputs, max_new_tokens=1024, do_sample=False, num_beams=1)
        generated = output[:, inputs["input_ids"].shape[1]:]
        text = processor.decode(generated, return_format="transcription_only")[0].strip()
        raw = processor.decode(generated)[0]
    torch.cuda.synchronize()
    finished = time.perf_counter()
    return text, {"import_seconds": imported-began, "model_load_seconds": loaded-imported,
                  "processing_seconds": finished-loaded, "pipeline_seconds": finished-began,
                  "raw_model_output": raw, "device": str(model.device), "dtype": str(model.dtype),
                  "attention": "sdpa", "language": "Spanish", "prompt": None,
                  "batch_size": 1, "do_sample": False, "num_beams": 1,
                  "max_new_tokens": 1024, "generated_tokens": int(generated.shape[1]),
                  "token_limit_reached": int(generated.shape[1]) >= 1024,
                  "peak_torch_allocated_mib": torch.cuda.max_memory_allocated()/1024**2,
                  "peak_torch_reserved_mib": torch.cuda.max_memory_reserved()/1024**2}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--case", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--engine", choices=("faster-whisper", "qwen3-asr"), required=True)
    parser.add_argument("--model-path", type=Path, required=True)
    parser.add_argument("--whisper-compute-type", choices=("int8_float16", "float16"), default="int8_float16")
    parser.add_argument("--cuda-library-dir", type=Path, action="append", default=[])
    args = parser.parse_args()
    args.case = private_path(args.case)
    args.model_path = args.model_path.resolve(strict=True)
    if not args.model_path.is_dir():
        parser.error("Model path must be an installed local directory")
    manifest = load_case(args.case)
    safe_id(args.run_id)
    if (args.case / "runs" / args.run_id).exists():
        parser.error("Run already recorded; use a new ID to repeat inference")
    execution = args.case / "execution" / args.run_id
    execution.mkdir(parents=True, exist_ok=False)
    for name, value in {"HF_HUB_OFFLINE": "1", "TRANSFORMERS_OFFLINE": "1", "HF_HUB_DISABLE_TELEMETRY": "1",
                        "DO_NOT_TRACK": "1", "TOKENIZERS_PARALLELISM": "false"}.items():
        os.environ[name] = value
    handles = []
    for folder in args.cuda_library_dir:
        folder = folder.resolve(strict=True)
        os.environ["PATH"] = str(folder) + os.pathsep + os.environ.get("PATH", "")
        if os.name == "nt":
            handles.append(os.add_dll_directory(str(folder)))
    settings = {"engine": args.engine, "model_path": str(args.model_path), "versions": package_versions(),
                "input_duration_seconds": manifest["duration_seconds"], "source_interval_seconds": [0, manifest["duration_seconds"]],
                "model_files_sha256": {p.name: digest(p) for p in sorted(args.model_path.iterdir())
                                       if p.is_file() and p.suffix in (".json", ".safetensors", ".bin", ".jinja")},
                "inference_mode": "local files only; Python socket connections blocked", "paid_api_used": False,
                "dictation_used": False, "machine_reference_supplied": False,
                "started_at": datetime.now(timezone.utc).isoformat(),
                "timing_note": "Pipeline includes library import, model load and one audio inference; excludes installation/download/hash verification; single cold process"}
    provenance = args.model_path / "download-provenance.json"
    if provenance.exists():
        settings["model_provenance"] = json.loads(provenance.read_text(encoding="utf-8"))
    route = "local_faster_whisper" if args.engine == "faster-whisper" else "local_qwen_asr"
    sampler = MemorySampler()
    sampler.thread.start()
    print(json.dumps({"event": "local_inference_started", "run_id": args.run_id, "model": args.model_path.name}), flush=True)
    status, text, reason = "failed", None, None
    with no_network() as attempts:
        try:
            text, metrics = (whisper if args.engine == "faster-whisper" else qwen)(args, args.case / "audio.wav")
            if not text:
                raise RuntimeError("Model returned no transcript")
            if metrics.get("token_limit_reached"):
                raise RuntimeError("Generation reached token limit; do not mark a truncated output complete")
            settings.update(metrics)
            load_case(args.case)
            status = "transcript_received"
        except Exception as error:
            reason = f"{type(error).__name__}: {error}"
            settings["error"] = reason
        finally:
            settings.update(sampler.finish())
            settings["blocked_network_attempts"] = len(attempts)
    settings["completed_at"] = datetime.now(timezone.utc).isoformat()
    write_json(execution / "conditions.json", settings)
    path = None
    if status == "transcript_received":
        path = execution / "transcript-raw.txt"
        with path.open("x", encoding="utf-8", newline="") as f:
            f.write(text)
    result = record(args.case, args.run_id, route, path, status=status, settings=settings,
                    elapsed_seconds=settings.get("pipeline_seconds"), reason=reason)
    print(json.dumps({"event": "local_inference_finished", "run_id": args.run_id, "status": result["status"],
                      "pipeline_seconds": settings.get("pipeline_seconds"), "processing_seconds": settings.get("processing_seconds"),
                      "peak_gpu_device_used_mib": settings.get("peak_gpu_device_used_mib"), "blocked_network_attempts": len(attempts),
                      "error": reason}, ensure_ascii=False), flush=True)
    if status != "transcript_received":
        raise SystemExit(1)


if __name__ == "__main__":
    main()
