package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/photoprism/photoprism/internal/ai/face"
	"github.com/photoprism/photoprism/internal/auth/acl"
	"github.com/photoprism/photoprism/internal/photoprism/get"
	"github.com/photoprism/photoprism/pkg/txt"
)

// GetFaceRecognitionModels returns face recognition model states.
//
//	@Summary	returns face recognition model states
//	@Id			GetFaceRecognitionModels
//	@Tags		Faces, Config
//	@Produce	json
//	@Success	200				{object}	photoprism.FaceRecognitionModelList
//	@Failure	401,403,429,500	{object}	i18n.Response
//	@Router		/api/v1/faces/models [get]
func GetFaceRecognitionModels(router *gin.RouterGroup) {
	router.GET("/faces/models", func(c *gin.Context) {
		s := Auth(c, acl.ResourceConfig, acl.ActionManage)

		if s.Abort(c) {
			return
		}

		result, err := get.Faces().RecognitionModels(c.Request.Context())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": txt.UpperFirst(err.Error())})
			return
		}

		c.JSON(http.StatusOK, result)
	})
}

// GetFaceRecognitionModel returns one face recognition model state.
//
//	@Summary	returns a face recognition model state
//	@Id			GetFaceRecognitionModel
//	@Tags		Faces, Config
//	@Produce	json
//	@Success	200					{object}	face.RecognitionAvailability
//	@Failure	400,401,403,429,500	{object}	i18n.Response
//	@Param		model				path		string	true	"model key"
//	@Router		/api/v1/faces/models/{model} [get]
func GetFaceRecognitionModel(router *gin.RouterGroup) {
	router.GET("/faces/models/:model", func(c *gin.Context) {
		s := Auth(c, acl.ResourceConfig, acl.ActionManage)

		if s.Abort(c) {
			return
		}

		result, err := get.Faces().RecognitionModel(c.Request.Context(), face.ParseRecognitionModel(c.Param("model")))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": txt.UpperFirst(err.Error())})
			return
		}

		c.JSON(http.StatusOK, result)
	})
}

// RecognizeFaceRecognitionModel prepares candidate state for a recognition model.
//
//	@Summary	prepares candidate face recognition state
//	@Id			RecognizeFaceRecognitionModel
//	@Tags		Faces, Config
//	@Produce	json
//	@Success	200					{object}	entity.FaceModelRun
//	@Failure	400,401,403,409,429	{object}	i18n.Response
//	@Param		model				path		string	true	"model key"
//	@Router		/api/v1/faces/models/{model}/recognize [post]
func RecognizeFaceRecognitionModel(router *gin.RouterGroup) {
	router.POST("/faces/models/:model/recognize", func(c *gin.Context) {
		s := Auth(c, acl.ResourceConfig, acl.ActionManage)

		if s.Abort(c) {
			return
		}

		run, err := get.Faces().RecognizeModel(c.Request.Context(), face.ParseRecognitionModel(c.Param("model")), c.Query("force") == "true")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": txt.UpperFirst(err.Error())})
			return
		}

		c.JSON(http.StatusOK, run)
	})
}

// CompareFaceRecognitionModel compares candidate state with the active model.
//
//	@Summary	compares candidate face recognition state
//	@Id			CompareFaceRecognitionModel
//	@Tags		Faces, Config
//	@Produce	json
//	@Success	200					{object}	photoprism.FaceRecognitionCompareResult
//	@Failure	400,401,403,429,500	{object}	i18n.Response
//	@Param		model				path		string	true	"model key"
//	@Router		/api/v1/faces/models/{model}/compare [get]
func CompareFaceRecognitionModel(router *gin.RouterGroup) {
	router.GET("/faces/models/:model/compare", func(c *gin.Context) {
		s := Auth(c, acl.ResourceConfig, acl.ActionManage)

		if s.Abort(c) {
			return
		}

		result, err := get.Faces().CompareRecognitionModel(face.ParseRecognitionModel(c.Param("model")))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": txt.UpperFirst(err.Error())})
			return
		}

		c.JSON(http.StatusOK, result)
	})
}

// PromoteFaceRecognitionModel promotes a prepared candidate model.
//
//	@Summary	promotes a candidate face recognition model
//	@Id			PromoteFaceRecognitionModel
//	@Tags		Faces, Config
//	@Produce	json
//	@Success	200					{object}	gin.H
//	@Failure	400,401,403,409,429	{object}	i18n.Response
//	@Param		model				path		string	true	"model key"
//	@Router		/api/v1/faces/models/{model}/promote [post]
func PromoteFaceRecognitionModel(router *gin.RouterGroup) {
	router.POST("/faces/models/:model/promote", func(c *gin.Context) {
		s := Auth(c, acl.ResourceConfig, acl.ActionManage)

		if s.Abort(c) {
			return
		}

		modelKey := face.ParseRecognitionModel(c.Param("model"))
		if err := get.Faces().PromoteRecognitionModel(modelKey); err != nil {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": txt.UpperFirst(err.Error())})
			return
		}

		c.JSON(http.StatusOK, gin.H{"model": modelKey, "status": "promoted"})
	})
}
