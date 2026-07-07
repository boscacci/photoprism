import hashlib
import os
import pathlib
import tempfile
import unittest

import app


class ModelDownloadTest(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp_dir.cleanup)
        self.old_models_path = os.environ.get("FACE_RECOGNITION_MODELS_PATH")
        self.old_auto_download = os.environ.get("FACE_RECOGNITION_AUTO_DOWNLOAD")
        self.old_sources = dict(app.MODEL_SOURCES)
        app.session_for.cache_clear()

    def tearDown(self):
        if self.old_models_path is None:
            os.environ.pop("FACE_RECOGNITION_MODELS_PATH", None)
        else:
            os.environ["FACE_RECOGNITION_MODELS_PATH"] = self.old_models_path

        if self.old_auto_download is None:
            os.environ.pop("FACE_RECOGNITION_AUTO_DOWNLOAD", None)
        else:
            os.environ["FACE_RECOGNITION_AUTO_DOWNLOAD"] = self.old_auto_download

        app.MODEL_SOURCES.clear()
        app.MODEL_SOURCES.update(self.old_sources)
        app.session_for.cache_clear()

    def test_model_path_downloads_and_verifies_known_model(self):
        root = pathlib.Path(self.temp_dir.name)
        source = root / "source.onnx"
        payload = b"fake-onnx"
        source.write_bytes(payload)

        os.environ["FACE_RECOGNITION_MODELS_PATH"] = str(root / "models")
        os.environ["FACE_RECOGNITION_AUTO_DOWNLOAD"] = "true"
        app.MODEL_SOURCES["auraface-v1"] = {
            "file": "model.onnx",
            "sha256": hashlib.sha256(payload).hexdigest(),
            "size": len(payload),
            "url": source.as_uri(),
        }

        result = pathlib.Path(app.model_path("auraface-v1"))

        self.assertEqual(root / "models" / "auraface-v1" / "model.onnx", result)
        self.assertEqual(payload, result.read_bytes())


if __name__ == "__main__":
    unittest.main()
