import argparse
import json
import subprocess
import threading
import time
from pathlib import Path

import psutil
from faster_whisper import WhisperModel

from benchmark_common import word_error_rate


def gpu_memory_mib() -> int:
    try:
        output = subprocess.check_output(
            ["nvidia-smi", "--query-compute-apps=used_memory", "--format=csv,noheader,nounits"],
            text=True,
            stderr=subprocess.DEVNULL,
        )
        return sum(int(line.strip()) for line in output.splitlines() if line.strip().isdigit())
    except (OSError, subprocess.SubprocessError):
        return 0


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--model-path", required=True)
    parser.add_argument("--audio", required=True)
    parser.add_argument("--expected", required=True)
    parser.add_argument("--device", default="cpu")
    parser.add_argument("--compute-type", default="int8")
    parser.add_argument("--cpu-threads", type=int, default=4)
    parser.add_argument("--beam-size", type=int, default=1)
    parser.add_argument("--condition-on-previous-text", action="store_true")
    parser.add_argument("--initial-prompt", default="")
    args = parser.parse_args()

    process = psutil.Process()
    peak_rss = process.memory_info().rss
    peak_gpu = gpu_memory_mib()
    sampling = True

    def sample() -> None:
        nonlocal peak_rss, peak_gpu
        while sampling:
            peak_rss = max(peak_rss, process.memory_info().rss)
            peak_gpu = max(peak_gpu, gpu_memory_mib())
            time.sleep(0.1)

    sampler = threading.Thread(target=sample, daemon=True)
    sampler.start()
    started = time.perf_counter()
    model = WhisperModel(
        args.model_path,
        device=args.device,
        compute_type=args.compute_type,
        cpu_threads=args.cpu_threads,
        num_workers=1,
        local_files_only=True,
    )
    loaded = time.perf_counter()
    segments, info = model.transcribe(
        args.audio,
        language="es",
        beam_size=args.beam_size,
        vad_filter=True,
        condition_on_previous_text=args.condition_on_previous_text,
        initial_prompt=args.initial_prompt or None,
    )
    transcript = " ".join(segment.text.strip() for segment in segments).strip()
    finished = time.perf_counter()
    sampling = False
    sampler.join(timeout=1)
    expected = Path(args.expected).read_text(encoding="utf-8").strip()
    print(json.dumps({
        "engine": "faster-whisper",
        "model": Path(args.model_path).name,
        "device": args.device,
        "compute_type": args.compute_type,
        "cpu_threads": args.cpu_threads,
        "beam_size": args.beam_size,
        "condition_on_previous_text": args.condition_on_previous_text,
        "model_load_seconds": round(loaded - started, 3),
        "transcription_seconds": round(finished - loaded, 3),
        "total_seconds": round(finished - started, 3),
        "audio_duration_seconds": round(info.duration, 3),
        "real_time_factor": round((finished - loaded) / max(info.duration, 0.001), 3),
        "peak_rss_mib": round(peak_rss / 1024 / 1024, 1),
        "peak_compute_gpu_mib": peak_gpu,
        "wer": round(word_error_rate(expected, transcript), 4),
        "transcript": transcript,
    }, ensure_ascii=False))


if __name__ == "__main__":
    main()
