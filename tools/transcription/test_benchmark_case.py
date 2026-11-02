import array
import json
from pathlib import Path
import sys
import tempfile
import unittest
import wave

sys.path.insert(0, str(Path(__file__).resolve().parent))
from benchmark_case import compare, edits, load_case, prepare, private_path, record, words, REPO


class BenchmarkCaseTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.source = self.root / "synthetic.wav"
        self.samples = array.array("h", (i % 10000 for i in range(61 * 8000)))
        if sys.byteorder != "little":
            self.samples.byteswap()
        with wave.open(str(self.source), "wb") as f:
            f.setnchannels(1)
            f.setsampwidth(2)
            f.setframerate(8000)
            f.writeframes(self.samples.tobytes())
        self.case = self.root / "case"
        prepare(self.source, self.case)

    def text(self, name, value):
        path = self.root / name
        path.write_text(value, encoding="utf-8")
        return path

    def test_first_minute_is_exact_and_repreparation_is_idempotent(self):
        m = load_case(self.case)
        self.assertEqual(m["frames"], 60 * 8000)
        with wave.open(str(self.case / "audio.wav"), "rb") as f:
            self.assertEqual(f.readframes(f.getnframes()), self.samples[:60 * 8000].tobytes())
        self.assertEqual(m, prepare(self.source, self.case))

    def test_changed_audio_and_different_source_are_rejected(self):
        with (self.case / "audio.wav").open("ab") as f:
            f.write(b"tampered")
        with self.assertRaisesRegex(ValueError, "audio changed"):
            load_case(self.case)
        with self.assertRaises(ValueError):
            prepare(self.source, self.case, 300)

    def test_short_source_never_becomes_valid_case(self):
        with self.assertRaisesRegex(ValueError, "shorter"):
            prepare(self.source, self.root / "long", 300)
        self.assertFalse((self.root / "long/case.json").exists())

    def test_case_duration_cannot_be_relabelled(self):
        path = self.case / "case.json"
        value = json.loads(path.read_text(encoding="utf-8"))
        value["duration_seconds"] = 300
        path.write_text(json.dumps(value), encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "interval or identity"):
            load_case(self.case)

    def test_private_artifacts_cannot_be_written_under_tracked_tools(self):
        with self.assertRaises(ValueError):
            private_path(REPO / "tools" / "patient-case")

    def test_run_retry_preserves_original_and_rejects_changed_text(self):
        p = self.text("one.txt", "Texto ficticio.")
        result = record(self.case, "v1", "windows_dictation", p)
        self.assertEqual(result, record(self.case, "v1", "windows_dictation", p))
        p.write_text("Texto diferente.", encoding="utf-8")
        with self.assertRaises(ValueError):
            record(self.case, "v1", "windows_dictation", p)
        with self.assertRaises(ValueError):
            record(self.case, "../escape", "windows_dictation", p)

    def test_machine_agreement_is_not_called_accuracy(self):
        a = self.text("one.txt", "Hola, mundo.")
        b = self.text("two.txt", "hola mundo!")
        record(self.case, "v1", "windows_dictation", a)
        record(self.case, "v2", "windows_audio_attachment", b)
        result = compare(self.case, "v2", baseline_run="v1")
        self.assertEqual(result["normalized_disagreement_rate"], 0)
        self.assertIsNone(result["word_error_rate"])
        self.assertFalse(result["literal_equal"])
        self.assertTrue(result["diff"])

    def test_reviewed_reference_needs_explicit_assertion(self):
        reference = self.text("reference.txt", "No tengo dolor.")
        hypothesis = self.text("hypothesis.txt", "Tengo dolor.")
        record(self.case, "v1", "windows_dictation", hypothesis)
        with self.assertRaises(ValueError):
            compare(self.case, "v1", reviewed_reference=reference)
        result = compare(self.case, "v1", reviewed_reference=reference, human_reviewed=True)
        self.assertAlmostEqual(result["word_error_rate"], 1 / 3)
        self.assertEqual(words("sí"), ["sí"])
        self.assertNotEqual(words("sí"), words("si"))

    def test_failed_or_tampered_attempt_is_not_scored(self):
        p = self.text("one.txt", "Texto ficticio.")
        record(self.case, "v1", "windows_dictation", p)
        record(self.case, "v2", "windows_audio_attachment", status="unsupported")
        self.assertEqual(compare(self.case, "v2", baseline_run="v1")["comparison_status"], "not_comparable")
        (self.case / "runs/v1/transcript.txt").write_text("changed", encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "transcript changed"):
            compare(self.case, "v1", baseline_run="v1")

    def test_cross_fixture_run_is_rejected(self):
        p = self.text("one.txt", "Texto ficticio.")
        record(self.case, "v1", "windows_dictation", p)
        path = self.case / "runs/v1/result.json"
        result = json.loads(path.read_text(encoding="utf-8"))
        result["case_id"] = "another-case"
        path.write_text(json.dumps(result), encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "another audio"):
            compare(self.case, "v1", baseline_run="v1")

    def test_alignment_counts_meaningful_omissions(self):
        self.assertEqual(edits(["no", "tengo", "dolor"], ["tengo", "dolor"]),
                         {"substitutions": 0, "deletions": 1, "insertions": 0, "total": 1})

    def test_local_engines_share_fixture_but_are_not_accuracy_references(self):
        p = self.text("local.txt", "Texto local ficticio.")
        record(self.case, "whisper", "local_faster_whisper", p)
        record(self.case, "qwen", "local_qwen_asr", p)
        result = compare(self.case, "qwen", baseline_run="whisper")
        self.assertEqual(result["comparison_status"], "compared")
        self.assertEqual(result["normalized_disagreement_rate"], 0)
        self.assertIsNone(result["word_error_rate"])


if __name__ == "__main__":
    unittest.main()
