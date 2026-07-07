package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

func TestGetFaceRecognitionModels(t *testing.T) {
	app, router, _ := NewApiTest()
	GetFaceRecognitionModels(router)

	r := PerformRequest(app, http.MethodGet, "/api/v1/faces/models")

	assert.Equal(t, http.StatusOK, r.Code)
	assert.Equal(t, "facenet", gjson.Get(r.Body.String(), "activeModel").String())
	assert.Equal(t, "facenet", gjson.Get(r.Body.String(), "models.0.profile.key").String())
}

func TestGetFaceRecognitionModel(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		app, router, _ := NewApiTest()
		GetFaceRecognitionModel(router)

		r := PerformRequest(app, http.MethodGet, "/api/v1/faces/models/facenet")

		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, "facenet", gjson.Get(r.Body.String(), "profile.key").String())
		assert.True(t, gjson.Get(r.Body.String(), "available").Bool())
	})
	t.Run("Unknown", func(t *testing.T) {
		app, router, _ := NewApiTest()
		GetFaceRecognitionModel(router)

		r := PerformRequest(app, http.MethodGet, "/api/v1/faces/models/unknown-model")

		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.Contains(t, gjson.Get(r.Body.String(), "error").String(), "Unknown face recognition model")
	})
}
