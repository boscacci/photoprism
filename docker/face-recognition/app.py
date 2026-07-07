import base64
import binascii
import hashlib
import logging
import os
import subprocess
import threading
from functools import lru_cache
from io import BytesIO
from typing import Any
from urllib.error import URLError
from urllib.request import Request, urlopen

import numpy as np
import onnxruntime as ort
from fastapi import FastAPI, HTTPException
from PIL import Image, ImageOps
from pydantic import BaseModel, Field


app = FastAPI(title="PhotoPrism Face Recognition", version="0.1.0")

logging.basicConfig(level=os.getenv("FACE_RECOGNITION_LOG_LEVEL", "INFO").upper())
log = logging.getLogger("photoprism.face-recognition")

MODEL_FILES = {
    "auraface-v1": ("model.onnx", "glintr100.onnx", "auraface-v1.onnx"),
    "insightface-buffalo-l": ("model.onnx", "buffalo_l.onnx"),
}

MODEL_SOURCES = {
    "auraface-v1": {
        "env": "FACE_RECOGNITION_AURAFACE_V1_URL",
        "file": "model.onnx",
        "sha256": "a7933ea5330113b01c9b60351d8f4c33003f145d8470ac5f0e52ee2effe25c60",
        "size": 260694151,
        "url": "https://huggingface.co/fal/AuraFace-v1/resolve/main/glintr100.onnx",
    },
}

download_lock = threading.Lock()


class VisionRequest(BaseModel):
    id: str = ""
    model: str = ""
    images: list[str] = Field(default_factory=list)


def env_bool(name: str, default: bool) -> bool:
    value = os.getenv(name, "").strip().lower()
    if value == "":
        return default
    return value in {"1", "true", "yes", "on"}


def normalize_model_key(model: str) -> str:
    model = (model or os.getenv("FACE_RECOGNITION_MODEL", "auraface-v1")).strip().lower()
    if model in {"", "default"}:
        return "auraface-v1"
    return model


def model_root() -> str:
    return os.getenv("FACE_RECOGNITION_MODELS_PATH", "/models")


def existing_model_path(model: str) -> str:
    model = normalize_model_key(model)
    names = MODEL_FILES.get(model, ("model.onnx", f"{model}.onnx"))

    for name in names:
        candidate = os.path.join(model_root(), model, name)
        if os.path.isfile(candidate) and os.path.getsize(candidate) > 0:
            return candidate

    return ""


def model_source(model: str) -> dict[str, Any]:
    source = dict(MODEL_SOURCES.get(normalize_model_key(model), {}))
    env_name = source.get("env", "")
    env_url = os.getenv(env_name, "").strip() if env_name else ""

    if env_url:
        source["url"] = env_url

    return source


def remove_partial_file(path: str) -> None:
    try:
        os.remove(path)
    except FileNotFoundError:
        return


def download_model(model: str, source: dict[str, Any]) -> str:
    model = normalize_model_key(model)
    filename = source.get("file") or "model.onnx"
    target_dir = os.path.join(model_root(), model)
    target = os.path.join(target_dir, filename)
    partial = f"{target}.part"
    expected_sha = str(source.get("sha256", "")).strip().lower()
    expected_size = int(source.get("size") or 0)
    url = str(source.get("url", "")).strip()

    if not url:
        raise FileNotFoundError(f"model weights not found for {model}")

    with download_lock:
        if existing := existing_model_path(model):
            return existing

        os.makedirs(target_dir, exist_ok=True)
        remove_partial_file(partial)

        log.info("downloading %s model weights from %s", model, url)
        request = Request(url, headers={"User-Agent": "PhotoPrism Face Recognition"})
        digest = hashlib.sha256()
        received = 0
        next_report = 25

        try:
            with urlopen(request, timeout=30) as response, open(partial, "wb") as file:
                while True:
                    chunk = response.read(1024 * 1024)
                    if not chunk:
                        break

                    file.write(chunk)
                    digest.update(chunk)
                    received += len(chunk)

                    if expected_size and received * 100 // expected_size >= next_report:
                        log.info(
                            "downloaded %s model weights: %d%% (%d/%d MiB)",
                            model,
                            next_report,
                            received // (1024 * 1024),
                            expected_size // (1024 * 1024),
                        )
                        next_report += 25

            if expected_size and received != expected_size:
                raise RuntimeError(f"downloaded {received} bytes, expected {expected_size}")

            actual_sha = digest.hexdigest()
            if expected_sha and actual_sha != expected_sha:
                raise RuntimeError(f"downloaded sha256 {actual_sha}, expected {expected_sha}")

            os.replace(partial, target)
        except Exception:
            remove_partial_file(partial)
            raise

    log.info("installed %s model weights in %s (%d MiB)", model, target, received // (1024 * 1024))
    return target


def model_path(model: str) -> str:
    model = normalize_model_key(model)

    if existing := existing_model_path(model):
        return existing

    if env_bool("FACE_RECOGNITION_AUTO_DOWNLOAD", True):
        if source := model_source(model):
            return download_model(model, source)

    raise FileNotFoundError(f"model weights not found for {model}")


def preferred_providers() -> list[str]:
    providers = ort.get_available_providers()
    require_cuda = env_bool("FACE_RECOGNITION_REQUIRE_CUDA", True)

    if require_cuda and "CUDAExecutionProvider" not in providers:
        raise RuntimeError("CUDAExecutionProvider is not available")

    result = []
    if "CUDAExecutionProvider" in providers:
        result.append("CUDAExecutionProvider")
    if "CPUExecutionProvider" in providers:
        result.append("CPUExecutionProvider")

    return result or providers


def gpu_info() -> tuple[str, int]:
    try:
        output = subprocess.check_output(
            ["nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits"],
            stderr=subprocess.DEVNULL,
            text=True,
            timeout=3,
        ).strip()
    except (OSError, subprocess.SubprocessError):
        return "", 0

    if not output:
        return "", 0

    first = output.splitlines()[0]
    parts = [p.strip() for p in first.split(",", 1)]
    if len(parts) != 2:
        return first, 0

    try:
        return parts[0], int(parts[1])
    except ValueError:
        return parts[0], 0


@lru_cache(maxsize=4)
def session_for(model: str) -> ort.InferenceSession:
    path = model_path(model)
    return ort.InferenceSession(path, providers=preferred_providers())


def decode_image(value: str) -> Image.Image:
    if value.startswith("data:"):
        _, _, encoded = value.partition(",")
        try:
            data = base64.b64decode(encoded, validate=True)
        except binascii.Error as err:
            raise ValueError("invalid data URL image") from err
        return Image.open(BytesIO(data))

    if value.startswith("http://") or value.startswith("https://"):
        request = Request(value, headers={"User-Agent": "PhotoPrism Face Recognition"})
        try:
            with urlopen(request, timeout=15) as response:
                return Image.open(BytesIO(response.read(16 * 1024 * 1024)))
        except URLError as err:
            raise ValueError("image URL could not be loaded") from err

    raise ValueError("unsupported image URL")


def preprocess_image(value: str, size: int = 112) -> np.ndarray:
    image = ImageOps.exif_transpose(decode_image(value)).convert("RGB")
    image = image.resize((size, size), Image.Resampling.BILINEAR)
    array = np.asarray(image).astype(np.float32)
    array = (array - 127.5) / 128.0
    array = np.transpose(array, (2, 0, 1))
    return np.expand_dims(array, axis=0)


def embed_image(session: ort.InferenceSession, value: str) -> list[float]:
    input_name = session.get_inputs()[0].name
    output_name = session.get_outputs()[0].name
    result = session.run([output_name], {input_name: preprocess_image(value)})[0]
    embedding = np.asarray(result).reshape(-1).astype(np.float32)
    norm = np.linalg.norm(embedding)
    if norm > 0:
        embedding = embedding / norm
    return embedding.astype(float).tolist()


def vision_model(model: str) -> dict[str, Any]:
    return {
        "type": "face",
        "name": normalize_model_key(model),
        "version": "onnx",
        "resolution": 112,
    }


@app.get("/health")
def health() -> dict[str, Any]:
    gpu_name, vram_mib = gpu_info()
    providers = ort.get_available_providers()
    ok = "CUDAExecutionProvider" in providers or not env_bool("FACE_RECOGNITION_REQUIRE_CUDA", True)
    model = normalize_model_key("")
    model_installed = False
    model_error = ""

    try:
        model_installed = bool(model_path(model))
    except Exception as err:
        model_error = str(err)

    return {
        "ok": ok,
        "provider": "CUDAExecutionProvider" if "CUDAExecutionProvider" in providers else providers[0] if providers else "",
        "providers": providers,
        "gpuName": gpu_name,
        "vramMiB": vram_mib,
        "model": model,
        "modelInstalled": model_installed,
        "modelError": model_error,
    }


@app.post("/api/v1/vision/face")
def recognize(request: VisionRequest) -> dict[str, Any]:
    if not request.images:
        raise HTTPException(status_code=400, detail="missing images")

    model = normalize_model_key(request.model)

    try:
        session = session_for(model)
        embeddings = [[embed_image(session, image)] for image in request.images]
    except FileNotFoundError as err:
        raise HTTPException(status_code=404, detail=str(err)) from err
    except (RuntimeError, ValueError, URLError, OSError, ort.OnnxRuntimeError) as err:
        raise HTTPException(status_code=503, detail=str(err)) from err

    return {
        "id": request.id,
        "code": 200,
        "model": vision_model(model),
        "result": {
            "embeddings": embeddings,
        },
    }
