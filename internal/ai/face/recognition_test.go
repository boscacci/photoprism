package face

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRecognitionModel(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		assert.Equal(t, RecognitionModelFacenet, ParseRecognitionModel(""))
		assert.Equal(t, RecognitionModelFacenet, ParseRecognitionModel("default"))
		assert.Equal(t, RecognitionModelFacenet, ParseRecognitionModel("Face-Net"))
	})
	t.Run("Candidate", func(t *testing.T) {
		assert.Equal(t, RecognitionModelAuraFaceV1, ParseRecognitionModel(" AuraFace-V1 "))
	})
}

func TestRecognitionProfiles(t *testing.T) {
	t.Run("Public", func(t *testing.T) {
		profiles := RecognitionProfiles(false)

		require.Len(t, profiles, 2)
		assert.Equal(t, RecognitionModelFacenet, profiles[0].Key)
		assert.Equal(t, RecognitionModelAuraFaceV1, profiles[1].Key)
	})
	t.Run("Hidden", func(t *testing.T) {
		profiles := RecognitionProfiles(true)

		require.Len(t, profiles, 3)
		assert.True(t, profiles[2].Hidden)
		assert.Contains(t, profiles[2].Key, RecognitionModelInsightFacePrefix)
	})
}

func TestRecognitionProfileInstalled(t *testing.T) {
	t.Run("Facenet", func(t *testing.T) {
		profile, ok := RecognitionProfileByKey(RecognitionModelFacenet)

		require.True(t, ok)
		assert.True(t, profile.Installed(""))
	})
	t.Run("MissingWeights", func(t *testing.T) {
		profile, ok := RecognitionProfileByKey(RecognitionModelAuraFaceV1)

		require.True(t, ok)
		assert.False(t, profile.Installed(t.TempDir()))
	})
	t.Run("InstalledWeights", func(t *testing.T) {
		modelsPath := t.TempDir()
		profile, ok := RecognitionProfileByKey(RecognitionModelAuraFaceV1)

		require.True(t, ok)
		require.NoError(t, os.MkdirAll(filepath.Join(modelsPath, profile.Key), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(modelsPath, profile.Key, "glintr100.onnx"), []byte("onnx"), 0o644))
		assert.True(t, profile.Installed(modelsPath))
	})
}

func TestRecognitionProfileAvailability(t *testing.T) {
	modelsPath := t.TempDir()
	aura, ok := RecognitionProfileByKey(RecognitionModelAuraFaceV1)

	require.True(t, ok)
	assert.Equal(t, "AuraFace v1 (4x GPU)", aura.Name)
	assert.Contains(t, aura.WeightFiles, "glintr100.onnx")
	require.NoError(t, os.MkdirAll(filepath.Join(modelsPath, aura.Key), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(modelsPath, aura.Key, "model.onnx"), []byte("onnx"), 0o644))

	t.Run("AvailableEightGigabyteGpu", func(t *testing.T) {
		result := aura.Availability(RecognitionProbe{
			ModelsPath:     modelsPath,
			ConfiguredKey:  RecognitionModelFacenet,
			GPUName:        "NVIDIA GeForce RTX 4060 Laptop GPU",
			VRAMMiB:        8188,
			CUDARuntime:    true,
			ServiceURI:     "http://face-recognition:5000/api/v1/vision/face",
			ServiceChecked: true,
			ServiceHealthy: true,
		})

		assert.True(t, result.Installed)
		assert.True(t, result.Available)
		assert.False(t, result.Active)
		assert.Empty(t, result.Reasons)
	})
	t.Run("MissingService", func(t *testing.T) {
		result := aura.Availability(RecognitionProbe{
			ModelsPath:  modelsPath,
			GPUName:     "NVIDIA GeForce RTX 4060 Laptop GPU",
			VRAMMiB:     8188,
			CUDARuntime: true,
		})

		assert.False(t, result.Available)
		assert.Contains(t, result.Reasons, RecognitionReasonMissingService)
	})
	t.Run("SidecarUnhealthy", func(t *testing.T) {
		result := aura.Availability(RecognitionProbe{
			ModelsPath:     modelsPath,
			GPUName:        "NVIDIA GeForce RTX 4060 Laptop GPU",
			VRAMMiB:        8188,
			CUDARuntime:    true,
			ServiceURI:     "http://face-recognition:5000/api/v1/vision/face",
			ServiceChecked: true,
		})

		assert.False(t, result.Available)
		assert.Contains(t, result.Reasons, RecognitionReasonSidecarUnhealthy)
	})
	t.Run("InsufficientVram", func(t *testing.T) {
		insight, ok := RecognitionProfileByKey("insightface-buffalo-l")

		require.True(t, ok)
		require.NoError(t, os.MkdirAll(filepath.Join(modelsPath, insight.Key), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(modelsPath, insight.Key, "model.onnx"), []byte("onnx"), 0o644))

		result := insight.Availability(RecognitionProbe{
			ModelsPath:     modelsPath,
			GPUName:        "NVIDIA GeForce RTX 4060 Laptop GPU",
			VRAMMiB:        8188,
			CUDARuntime:    true,
			ServiceURI:     "http://face-recognition:5000/api/v1/vision/face",
			ServiceChecked: true,
			ServiceHealthy: true,
		})

		assert.False(t, result.Available)
		assert.Contains(t, result.Reasons, RecognitionReasonInsufficientVRAM)
	})
}
