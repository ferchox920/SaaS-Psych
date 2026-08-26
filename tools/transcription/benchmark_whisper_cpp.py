import argparse
import json
import subprocess
import threading
import time
from pathlib import Path

import psutil

from benchmark_common import word_error_rate


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--executable", required=True)
    parser.add_argument("--model-path", required=True)
    parser.add_argument("--audio", required=True)
    parser.add_argument("--expected", required=True)
    parser.add_argument("--threads", type=int, default=8)
    args = parser.parse_args()
    output_prefix = Path(args.audio).with_suffix("").with_name("whisper-cpp-output")
    command = [args.executable, "-m", args.model_path, "-f", args.audio, "-l", "es", "-t", str(args.threads), "-bs", "1", "-otxt", "-of", str(output_prefix), "-np"]
    started = time.perf_counter()
    child = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, encoding="utf-8", errors="replace")
    peak_rss = 0
    sampling = True

    def sample() -> None:
        nonlocal peak_rss
        process = psutil.Process(child.pid)
        while sampling and child.poll() is None:
            try:
                peak_rss = max(peak_rss, process.memory_info().rss)
            except psutil.Error:
                break
            time.sleep(0.05)

    sampler = threading.Thread(target=sample, daemon=True)
    sampler.start()
    stdout, stderr = child.communicate()
    sampling = False
    sampler.join(timeout=1)
    finished = time.perf_counter()
    if child.returncode != 0:
        raise SystemExit(f"whisper.cpp failed ({child.returncode}): {stderr[-1000:]}")
    transcript_path = output_prefix.with_suffix(".txt")
    transcript = transcript_path.read_text(encoding="utf-8").strip()
    expected = Path(args.expected).read_text(encoding="utf-8").strip()
    print(json.dumps({
        "engine": "whisper.cpp",
        "model": Path(args.model_path).name,
        "total_seconds": round(finished - started, 3),
        "peak_rss_mib": round(peak_rss / 1024 / 1024, 1),
        "wer": round(word_error_rate(expected, transcript), 4),
        "transcript": transcript,
    }, ensure_ascii=False))


if __name__ == "__main__":
    main()
