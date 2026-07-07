# Face Recognition Sidecar

**Last Updated:** June 25, 2026

This optional service prepares GPU-backed ONNX embeddings for experimental face recognition model candidates. It implements the PhotoPrism Vision payload shape used by `PHOTOPRISM_FACE_RECOGNITION_SERVICE_URI`.

## Model Files

AuraFace v1 is downloaded on first use by default and cached under:

```text
storage/services/face-recognition/models/auraface-v1/model.onnx
```

The download source is `fal/AuraFace-v1` on Hugging Face, using `glintr100.onnx` from the Apache-2.0 model repository. The sidecar verifies the expected SHA-256 digest before installing it. Set `FACE_RECOGNITION_AUTO_DOWNLOAD=false` to require manually managed weights, or set `FACE_RECOGNITION_AURAFACE_V1_URL` to use an internal mirror.

The sidecar also checks `glintr100.onnx` and `auraface-v1.onnx` for AuraFace v1, and `buffalo_l.onnx` for the hidden InsightFace Buffalo-L profile.

## Local Compose

Start the optional overlay with:

```bash
docker compose -f compose.yaml -f compose.face-recognition.yaml --profile face-recognition up -d
```

The overlay sets:

```text
PHOTOPRISM_FACE_RECOGNITION_SERVICE_URI=http://face-recognition:5000/api/v1/vision/face
PHOTOPRISM_FACE_RECOGNITION_MODELS_PATH=/go/src/github.com/photoprism/photoprism/storage/services/face-recognition/models
FACE_RECOGNITION_AUTO_DOWNLOAD=true
```

The sidecar requires the NVIDIA container runtime and defaults to CUDA execution. Set `FACE_RECOGNITION_REQUIRE_CUDA=false` only for local protocol testing; candidate availability in PhotoPrism still expects a CUDA-capable deployment for GPU profiles.
