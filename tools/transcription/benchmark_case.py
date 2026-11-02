"""Private audio fixtures and transcript comparisons. No network or inference."""
import argparse
from datetime import datetime, timezone
import difflib
import hashlib
import json
import math
from pathlib import Path
import re
import unicodedata
import wave


REPO = Path(__file__).resolve().parents[2]
ROUTES = ("windows_dictation", "windows_audio_attachment", "local_faster_whisper", "local_qwen_asr")
STATUSES = ("transcript_received", "unsupported", "failed")


def digest(path):
    h = hashlib.sha256()
    with Path(path).open("rb") as f:
        for block in iter(lambda: f.read(1024 * 1024), b""):
            h.update(block)
    return h.hexdigest()


def private_path(path):
    path = Path(path).resolve()
    if path.is_relative_to(REPO) and not path.is_relative_to(REPO / ".local"):
        raise ValueError("Store private benchmark artifacts under .local or outside the repository")
    return path


def write_json(path, value):
    with Path(path).open("x", encoding="utf-8") as f:
        json.dump(value, f, ensure_ascii=False, indent=2)


def read_json(path):
    return json.loads(Path(path).read_text(encoding="utf-8"))


def load_case(case):
    case = private_path(case)
    m = read_json(case / "case.json")
    if m["schema_version"] != "transcription-benchmark-case-v1":
        raise ValueError("Unsupported case version")
    identity = {k: m[k] for k in ("source_sha256", "start_sample", "end_sample",
                                "sample_rate", "channels", "recipe")}
    if (m["duration_seconds"] not in (60, 300, 600) or m["start_sample"] != 0
            or m["frames"] != m["end_sample"]
            or m["frames"] != m["duration_seconds"] * m["sample_rate"]
            or m["recipe"] != "decoded-pcm-s16-v1"
            or m["case_id"] != hashlib.sha256(json.dumps(identity, sort_keys=True).encode()).hexdigest()):
        raise ValueError("Fixture interval or identity changed")
    if digest(case / "audio.wav") != m["audio_sha256"]:
        raise ValueError("Fixture audio changed")
    with wave.open(str(case / "audio.wav"), "rb") as audio:
        actual = (audio.getnframes(), audio.getframerate(), audio.getnchannels(), audio.getsampwidth())
    if actual != (m["frames"], m["sample_rate"], m["channels"], 2):
        raise ValueError("Fixture audio metadata changed")
    return m


def prepare(source, case, seconds=60):
    import av

    if seconds not in (60, 300, 600):
        raise ValueError("Choose an explicit 60, 300 or 600 second case")
    source, case = Path(source).resolve(strict=True), private_path(case)
    if not source.is_file():
        raise ValueError("Source must be an audio file")
    source_hash = digest(source)
    if case.exists():
        m = load_case(case)
        if m["source_sha256"] != source_hash or m["duration_seconds"] != seconds:
            raise ValueError("Existing case belongs to a different source or interval")
        return m
    case.mkdir(parents=True, exist_ok=False)
    with av.open(str(source)) as container:
        stream = container.streams.audio[0]
        rate, channels = stream.codec_context.sample_rate, stream.codec_context.channels
        if not rate or not channels:
            raise ValueError("Missing audio sample rate or channel count")
        frames = remaining = seconds * rate
        resampler = av.AudioResampler(format="s16", layout=stream.codec_context.layout, rate=rate)
        with wave.open(str(case / "audio.wav"), "wb") as output:
            output.setnchannels(channels)
            output.setsampwidth(2)
            output.setframerate(rate)
            for decoded in container.decode(stream):
                for converted in resampler.resample(decoded):
                    take = min(converted.samples, remaining)
                    data = converted.to_ndarray().reshape(-1, channels)[:take]
                    output.writeframesraw(data.astype("<i2", copy=False).tobytes())
                    remaining -= take
                if not remaining:
                    break
        if remaining:
            raise ValueError("Source is shorter than the requested interval; incomplete case retained")
    if digest(source) != source_hash:
        raise ValueError("Source changed during preparation")
    identity = {"source_sha256": source_hash, "start_sample": 0, "end_sample": frames,
                "sample_rate": rate, "channels": channels, "recipe": "decoded-pcm-s16-v1"}
    m = {"schema_version": "transcription-benchmark-case-v1", **identity,
         "case_id": hashlib.sha256(json.dumps(identity, sort_keys=True).encode()).hexdigest(),
         "source_file": str(source), "duration_seconds": seconds, "frames": frames,
         "audio_sha256": digest(case / "audio.wav"),
         "reference_status": "no_human_reviewed_reference",
         "created_at": datetime.now(timezone.utc).isoformat()}
    write_json(case / "case.json", m)
    return load_case(case)


def safe_id(value):
    if not re.fullmatch(r"[a-z0-9][a-z0-9_-]{0,79}", value):
        raise ValueError("Use a short lowercase run ID, without paths")
    return value


def record(case, run_id, route, transcript=None, status="transcript_received", settings=None,
           elapsed_seconds=None, reason=None):
    case = private_path(case)
    m = load_case(case)
    safe_id(run_id)
    if route not in ROUTES or status not in STATUSES:
        raise ValueError("Unsupported route or status")
    if elapsed_seconds is not None and (not math.isfinite(elapsed_seconds) or elapsed_seconds < 0):
        raise ValueError("Elapsed time must be finite and nonnegative")
    text = Path(transcript).read_text(encoding="utf-8-sig") if transcript else None
    if status == "transcript_received" and (not text or not text.strip()):
        raise ValueError("A received transcript cannot be empty")
    if status != "transcript_received" and text is not None:
        raise ValueError("Failed or unsupported attempts cannot contain a successful transcript")
    result = {"case_id": m["case_id"], "audio_sha256": m["audio_sha256"],
              "run_id": run_id, "route": route, "status": status,
              "settings": settings or {}, "elapsed_seconds": elapsed_seconds, "reason": reason,
              "transcript_sha256": hashlib.sha256(text.encode("utf-8")).hexdigest() if text else None,
              "human_review_status": "unreviewed"}
    folder = case / "runs" / run_id
    if folder.exists():
        existing = read_json(folder / "result.json")
        if any(existing.get(k) != v for k, v in result.items()):
            raise ValueError("Run ID already exists with different content; use another ID")
        load_run(case, run_id)
        return existing
    folder.mkdir(parents=True, exist_ok=False)
    if text:
        with (folder / "transcript.txt").open("x", encoding="utf-8", newline="") as f:
            f.write(text)
    result["recorded_at"] = datetime.now(timezone.utc).isoformat()
    write_json(folder / "result.json", result)
    return result


def load_run(case, run_id):
    case = private_path(case)
    m = load_case(case)
    result = read_json(case / "runs" / safe_id(run_id) / "result.json")
    if result["case_id"] != m["case_id"] or result["audio_sha256"] != m["audio_sha256"]:
        raise ValueError("Run belongs to another audio fixture")
    text = None
    if result["status"] == "transcript_received":
        path = case / "runs" / run_id / "transcript.txt"
        if digest(path) != result["transcript_sha256"]:
            raise ValueError("Recorded transcript changed")
        text = path.read_text(encoding="utf-8")
    return result, text


def words(text):
    # Keep accents and Spanish negations. Ignore punctuation and case only.
    return re.findall(r"\w+", unicodedata.normalize("NFKC", text).casefold())


def edits(expected, actual):
    # Two-row Levenshtein alignment: substitutions, deletions, insertions.
    previous = [(0, 0, i) for i in range(len(actual) + 1)]
    for i, word in enumerate(expected, 1):
        current = [(0, i, 0)]
        for j, candidate in enumerate(actual, 1):
            s, d, ins = previous[j - 1]
            diagonal = (s + (word != candidate), d, ins)
            s, d, ins = previous[j]
            deletion = (s, d + 1, ins)
            s, d, ins = current[-1]
            insertion = (s, d, ins + 1)
            current.append(min((diagonal, deletion, insertion), key=sum))
        previous = current
    s, d, ins = previous[-1]
    return {"substitutions": s, "deletions": d, "insertions": ins, "total": s + d + ins}


def compare(case, candidate_run, baseline_run=None, reviewed_reference=None, human_reviewed=False):
    if (baseline_run is None) == (reviewed_reference is None):
        raise ValueError("Choose one baseline run or one reviewed reference")
    candidate, actual = load_run(case, candidate_run)
    baseline = None
    if reviewed_reference is not None:
        if not human_reviewed:
            raise ValueError("Explicit human review assertion is required for a reference")
        expected = Path(reviewed_reference).read_text(encoding="utf-8-sig")
        if not words(expected):
            raise ValueError("Reviewed reference cannot be empty")
    else:
        baseline, expected = load_run(case, baseline_run)
    result = {"case_id": candidate["case_id"], "candidate_run": candidate_run,
              "baseline_run": baseline_run, "reference_kind": "human_reviewed" if reviewed_reference else "machine_output",
              "comparison_status": "not_comparable", "word_error_rate": None}
    if actual is None or expected is None:
        result["reason"] = "One attempt did not return a transcript"
        return result
    left, right = words(expected), words(actual)
    counts = edits(left, right)
    rate = counts["total"] / len(left) if left else None
    result.update({"comparison_status": "compared", "reference_words": len(left),
                   "candidate_words": len(right), "word_edits": counts,
                   "normalized_disagreement_rate": rate,
                   "word_error_rate": rate if reviewed_reference else None,
                   "literal_equal": expected == actual,
                   "reference_sha256": hashlib.sha256(expected.encode()).hexdigest(),
                   "diff": "\n".join(difflib.unified_diff(expected.splitlines(), actual.splitlines(),
                                                       fromfile="reference", tofile=candidate_run, lineterm=""))})
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    p = commands.add_parser("prepare")
    p.add_argument("--source", required=True)
    p.add_argument("--case", required=True)
    p.add_argument("--seconds", type=int, choices=(60, 300, 600), default=60)
    p = commands.add_parser("record")
    p.add_argument("--case", required=True)
    p.add_argument("--run-id", required=True)
    p.add_argument("--route", choices=ROUTES, required=True)
    p.add_argument("--transcript")
    p.add_argument("--status", choices=STATUSES, default="transcript_received")
    p.add_argument("--settings-json")
    p.add_argument("--elapsed-seconds", type=float)
    p.add_argument("--reason")
    p = commands.add_parser("compare")
    p.add_argument("--case", required=True)
    p.add_argument("--candidate-run", required=True)
    group = p.add_mutually_exclusive_group(required=True)
    group.add_argument("--baseline-run")
    group.add_argument("--reviewed-reference")
    p.add_argument("--human-reviewed", action="store_true")
    p.add_argument("--output")
    a = parser.parse_args()
    try:
        if a.command == "prepare":
            result = prepare(a.source, a.case, a.seconds)
        elif a.command == "record":
            settings = read_json(a.settings_json) if a.settings_json else {}
            result = record(a.case, a.run_id, a.route, a.transcript, a.status, settings,
                            a.elapsed_seconds, a.reason)
        else:
            result = compare(a.case, a.candidate_run, a.baseline_run,
                             a.reviewed_reference, a.human_reviewed)
            if a.output:
                path = private_path(a.output)
                path.parent.mkdir(parents=True, exist_ok=True)
                write_json(path, result)
        print(json.dumps({k: v for k, v in result.items() if k != "diff"}, ensure_ascii=False))
    except (ValueError, OSError, KeyError) as error:
        parser.exit(1, f"Benchmark refused: {error}\n")


if __name__ == "__main__":
    main()
