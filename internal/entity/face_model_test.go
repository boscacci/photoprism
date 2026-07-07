package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/photoprism/photoprism/internal/ai/face"
)

func TestFaceModelEmbedding(t *testing.T) {
	t.Run("TableName", func(t *testing.T) {
		assert.Equal(t, "face_model_embeddings", FaceModelEmbedding{}.TableName())
	})
	t.Run("RoundTrip", func(t *testing.T) {
		embeddings := testFaceModelEmbeddings(1)
		result, err := NewFaceModelEmbedding(face.RecognitionModelAuraFaceV1, "ms6sg6b2wowuy3c2", embeddings)

		require.NoError(t, err)
		assert.Equal(t, face.RecognitionModelAuraFaceV1, result.ModelKey)
		assert.Equal(t, "ms6sg6b2wowuy3c2", result.MarkerUID)
		assert.NotEmpty(t, result.EmbeddingsJSON)
		assert.Len(t, result.Embeddings(), 1)
		assert.Len(t, result.Embeddings()[0], len(face.NullEmbedding))
	})
	t.Run("RejectsEmptyEmbeddings", func(t *testing.T) {
		_, err := NewFaceModelEmbedding(face.RecognitionModelAuraFaceV1, "ms6sg6b2wowuy3c2", face.Embeddings{})

		require.Error(t, err)
		assert.Equal(t, "invalid embedding", err.Error())
	})
}

func TestFaceModelCluster(t *testing.T) {
	t.Run("TableName", func(t *testing.T) {
		assert.Equal(t, "face_model_clusters", FaceModelCluster{}.TableName())
		assert.Equal(t, "face_model_markers", FaceModelMarker{}.TableName())
		assert.Equal(t, "face_model_runs", FaceModelRun{}.TableName())
	})
	t.Run("ModelScopedID", func(t *testing.T) {
		embeddings := testFaceModelEmbeddings(2)
		aura, auraErr := NewFaceModelCluster(face.RecognitionModelAuraFaceV1, "js6sg6b2wowuy3c2", SrcAuto, embeddings)
		insight, insightErr := NewFaceModelCluster("insightface-buffalo-l", "js6sg6b2wowuy3c2", SrcAuto, embeddings)

		require.NoError(t, auraErr)
		require.NoError(t, insightErr)
		assert.NotEmpty(t, aura.ID)
		assert.NotEqual(t, aura.ID, insight.ID)
		assert.Equal(t, len(face.NullEmbedding), len(aura.Embedding()))
	})
	t.Run("ToFace", func(t *testing.T) {
		cluster, err := NewFaceModelCluster(face.RecognitionModelAuraFaceV1, "js6sg6b2wowuy3c2", SrcAuto, testFaceModelEmbeddings(3))

		require.NoError(t, err)
		active, err := cluster.ToFace()

		require.NoError(t, err)
		assert.NotEmpty(t, active.ID)
		assert.Equal(t, cluster.SubjUID, active.SubjUID)
		assert.Equal(t, cluster.FaceSrc, active.FaceSrc)
		assert.Equal(t, cluster.FaceKind, active.FaceKind)
	})
}

func testFaceModelEmbeddings(seed float32) face.Embeddings {
	values := make([]float32, len(face.NullEmbedding))
	values[0] = seed
	values[1] = seed / 2
	values[2] = 1

	return face.Embeddings{face.NewEmbedding(values)}
}
