package photoprism

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/photoprism/photoprism/internal/ai/face"
	"github.com/photoprism/photoprism/internal/config"
	"github.com/photoprism/photoprism/internal/entity"
)

type fakeFaceRecognitionEmbedder struct {
	health FaceRecognitionServiceHealth
}

// Health returns fixed sidecar metadata for tests.
func (f fakeFaceRecognitionEmbedder) Health(context.Context) (FaceRecognitionServiceHealth, error) {
	return f.health, nil
}

// EmbedFaceCrops returns deterministic embeddings for tests.
func (f fakeFaceRecognitionEmbedder) EmbedFaceCrops(_ context.Context, _ string, cropFiles []string) ([]face.Embeddings, error) {
	result := make([]face.Embeddings, len(cropFiles))

	for i := range cropFiles {
		result[i] = testRecognitionEmbeddings(float32(i + 1))
	}

	return result, nil
}

func TestFaces_RecognitionModels(t *testing.T) {
	conf := config.NewMinimalTestConfig(t.TempDir())
	conf.Options().ModelsPath = t.TempDir()
	conf.Options().FaceRecognitionServiceURI = "http://face-recognition:5000/api/v1/vision/face"

	modelDir := filepath.Join(conf.FaceRecognitionModelsPath(), face.RecognitionModelAuraFaceV1)
	require.NoError(t, os.MkdirAll(modelDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(modelDir, "model.onnx"), []byte("onnx"), 0o600))

	restoreProbe := faceRecognitionGPUProbe
	restoreFactory := faceRecognitionEmbedderFactory
	t.Cleanup(func() {
		faceRecognitionGPUProbe = restoreProbe
		faceRecognitionEmbedderFactory = restoreFactory
	})

	faceRecognitionGPUProbe = func(context.Context) (FaceRecognitionServiceHealth, error) {
		return FaceRecognitionServiceHealth{OK: true, GPUName: "NVIDIA GeForce RTX 4060 Laptop GPU", VRAMMiB: 8188}, nil
	}
	faceRecognitionEmbedderFactory = func(string) FaceRecognitionEmbedder {
		return fakeFaceRecognitionEmbedder{health: FaceRecognitionServiceHealth{OK: true, GPUName: "NVIDIA GeForce RTX 4060 Laptop GPU", VRAMMiB: 8188}}
	}

	models, err := NewFaces(conf).RecognitionModels(context.Background())

	require.NoError(t, err)
	assert.Equal(t, face.RecognitionModelFacenet, models.ActiveModel)
	assert.Equal(t, "NVIDIA GeForce RTX 4060 Laptop GPU", models.GPUName)
	assert.Equal(t, 8188, models.VRAMMiB)

	foundAura := false
	for _, model := range models.Models {
		if model.Profile.Key == face.RecognitionModelAuraFaceV1 {
			foundAura = true
			assert.True(t, model.Available)
			assert.True(t, model.Installed)
		}
	}

	assert.True(t, foundAura)
}

func TestFaces_RecognitionModelsUsesSidecarCUDA(t *testing.T) {
	conf := config.NewMinimalTestConfig(t.TempDir())
	conf.Options().ModelsPath = t.TempDir()
	conf.Options().FaceRecognitionServiceURI = "http://face-recognition:5000/api/v1/vision/face"

	modelDir := filepath.Join(conf.FaceRecognitionModelsPath(), face.RecognitionModelAuraFaceV1)
	require.NoError(t, os.MkdirAll(modelDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(modelDir, "model.onnx"), []byte("onnx"), 0o600))

	restoreProbe := faceRecognitionGPUProbe
	restoreFactory := faceRecognitionEmbedderFactory
	t.Cleanup(func() {
		faceRecognitionGPUProbe = restoreProbe
		faceRecognitionEmbedderFactory = restoreFactory
	})

	faceRecognitionGPUProbe = func(context.Context) (FaceRecognitionServiceHealth, error) {
		return FaceRecognitionServiceHealth{}, assert.AnError
	}
	faceRecognitionEmbedderFactory = func(string) FaceRecognitionEmbedder {
		return fakeFaceRecognitionEmbedder{health: FaceRecognitionServiceHealth{
			OK:       true,
			Provider: "CUDAExecutionProvider",
			GPUName:  "NVIDIA GeForce RTX 4060 Laptop GPU",
			VRAMMiB:  8188,
		}}
	}

	models, err := NewFaces(conf).RecognitionModels(context.Background())

	require.NoError(t, err)

	foundAura := false
	for _, model := range models.Models {
		if model.Profile.Key == face.RecognitionModelAuraFaceV1 {
			foundAura = true
			assert.True(t, model.Available)
			assert.Equal(t, "NVIDIA GeForce RTX 4060 Laptop GPU", model.GPUName)
		}
	}

	assert.True(t, foundAura)
}

func TestFaces_PromoteRecognitionModelRequiresPrepared(t *testing.T) {
	conf := config.NewMinimalTestConfigWithDb("faces-model-promote-refuse", t.TempDir())
	defer conf.Shutdown()

	err := NewFaces(conf).PromoteRecognitionModel(face.RecognitionModelAuraFaceV1)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "has not been prepared")
}

func TestFaces_PromoteRecognitionModelKeepsManualSubjects(t *testing.T) {
	conf := config.NewMinimalTestConfigWithDb("faces-model-promote-manual", t.TempDir())
	defer conf.Shutdown()

	require.NoError(t, entity.UnscopedDb().Exec("DELETE FROM face_model_runs").Error)
	require.NoError(t, entity.UnscopedDb().Exec("DELETE FROM face_model_markers").Error)
	require.NoError(t, entity.UnscopedDb().Exec("DELETE FROM face_model_clusters").Error)
	require.NoError(t, entity.UnscopedDb().Exec("DELETE FROM face_model_embeddings").Error)
	require.NoError(t, entity.UnscopedDb().Exec("DELETE FROM markers").Error)
	require.NoError(t, entity.UnscopedDb().Exec("DELETE FROM faces").Error)

	markerUID := "ms6sg6b2wowuy3c2"
	manualSubjUID := "js6sg6b2wowuy3c2"
	candidateSubjUID := "js6sg6b2candidate"

	activeFace := entity.NewFace(manualSubjUID, entity.SrcManual, testRecognitionEmbeddings(7))
	require.NotNil(t, activeFace)
	require.NoError(t, entity.UnscopedDb().Create(activeFace).Error)

	marker := entity.Marker{
		MarkerUID:      markerUID,
		MarkerType:     entity.MarkerFace,
		MarkerSrc:      entity.SrcImage,
		SubjUID:        manualSubjUID,
		SubjSrc:        entity.SrcManual,
		FaceID:         activeFace.ID,
		FaceDist:       0,
		EmbeddingsJSON: testRecognitionEmbeddings(7).JSON(),
		Size:           face.ClusterSizeThreshold,
		Score:          face.ClusterScoreThreshold,
	}
	require.NoError(t, entity.UnscopedDb().Create(&marker).Error)

	cluster, err := entity.NewFaceModelCluster(face.RecognitionModelAuraFaceV1, candidateSubjUID, entity.SrcAuto, testRecognitionEmbeddings(8))
	require.NoError(t, err)
	require.NoError(t, entity.UnscopedDb().Create(cluster).Error)

	modelMarker := entity.FaceModelMarker{
		ModelKey:  face.RecognitionModelAuraFaceV1,
		MarkerUID: markerUID,
		FaceID:    cluster.ID,
		FaceDist:  0.01,
		SubjUID:   candidateSubjUID,
		SubjSrc:   entity.SrcAuto,
	}
	require.NoError(t, entity.UnscopedDb().Create(&modelMarker).Error)

	run := entity.FaceModelRun{
		ModelKey:       face.RecognitionModelAuraFaceV1,
		Status:         FaceModelStatusPrepared,
		MarkerCount:    1,
		EmbeddingCount: 1,
		ClusterCount:   1,
		MatchedCount:   1,
	}
	require.NoError(t, entity.UnscopedDb().Create(&run).Error)

	require.NoError(t, NewFaces(conf).PromoteRecognitionModel(face.RecognitionModelAuraFaceV1))

	updated := entity.FindMarker(markerUID)
	require.NotNil(t, updated)
	assert.Equal(t, manualSubjUID, updated.SubjUID)
	assert.Equal(t, entity.SrcManual, updated.SubjSrc)
	assert.NotEqual(t, activeFace.ID, updated.FaceID)
	assert.NotEmpty(t, updated.FaceID)
	assert.Equal(t, face.RecognitionModelAuraFaceV1, conf.FaceRecognitionModel())
}

// testRecognitionEmbeddings returns a deterministic 512D embedding for tests.
func testRecognitionEmbeddings(seed float32) face.Embeddings {
	values := make([]float32, len(face.NullEmbedding))
	values[0] = seed
	values[1] = seed / 2
	values[2] = 1

	return face.Embeddings{face.NewEmbedding(values)}
}
