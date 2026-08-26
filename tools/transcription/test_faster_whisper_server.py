import tempfile
import threading
import unittest
from pathlib import Path
from types import SimpleNamespace
import sys

sys.path.insert(0, str(Path(__file__).resolve().parent))
from faster_whisper_server import TranscriptionRuntime


class FakeModel:
    def transcribe(self, path: str, **_kwargs):
        assert Path(path).exists()
        return iter([SimpleNamespace(text=" Texto ficticio ")]), SimpleNamespace(language="es", duration=1.0)


class CleanupTest(unittest.TestCase):
    def test_ephemeral_audio_is_removed_after_transcription(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            runtime = TranscriptionRuntime.__new__(TranscriptionRuntime)
            runtime.model_path = Path("fictitious-model")
            runtime.device = "cpu"
            runtime.compute_type = "int8"
            runtime.cpu_threads = 4
            runtime.beam_size = 1
            runtime.initial_prompt = "Contexto ficticio."
            runtime.temp_dir = Path(directory)
            runtime.lock = threading.Lock()
            runtime.model = FakeModel()
            result = runtime.transcribe(b"fictitious-audio", "wav")
            self.assertEqual(result["text"], "Texto ficticio")
            self.assertEqual(list(Path(directory).iterdir()), [])


if __name__ == "__main__":
    unittest.main()
