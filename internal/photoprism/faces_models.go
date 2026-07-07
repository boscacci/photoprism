package photoprism

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize/english"
	"github.com/jinzhu/gorm"

	"github.com/photoprism/photoprism/internal/ai/face"
	"github.com/photoprism/photoprism/internal/ai/vision"
	"github.com/photoprism/photoprism/internal/config"
	"github.com/photoprism/photoprism/internal/entity"
	"github.com/photoprism/photoprism/internal/entity/query"
	"github.com/photoprism/photoprism/internal/mutex"
	"github.com/photoprism/photoprism/internal/thumb/crop"
	"github.com/photoprism/photoprism/pkg/clean"
	"github.com/photoprism/photoprism/pkg/http/scheme"
	"github.com/photoprism/photoprism/pkg/vector/alg"
)

const (
	// FaceModelStatusPreparing indicates that candidate preparation is running.
	FaceModelStatusPreparing = "preparing"
	// FaceModelStatusPrepared indicates that candidate embeddings and clusters are complete.
	FaceModelStatusPrepared = "prepared"
	// FaceModelStatusFailed indicates that candidate preparation failed.
	FaceModelStatusFailed = "failed"
	// FaceModelStatusPromoted indicates that a prepared candidate was promoted.
	FaceModelStatusPromoted = "promoted"
)

const faceRecognitionBatchSize = 32

// FaceRecognitionServiceHealth reports GPU metadata from the sidecar.
type FaceRecognitionServiceHealth struct {
	OK       bool   `json:"ok"`
	Provider string `json:"provider,omitempty"`
	GPUName  string `json:"gpuName,omitempty"`
	VRAMMiB  int    `json:"vramMiB,omitempty"`
}

// FaceRecognitionModelList contains available recognition model states.
type FaceRecognitionModelList struct {
	ActiveModel string                         `json:"activeModel"`
	ServiceURI  string                         `json:"serviceUri,omitempty"`
	GPUName     string                         `json:"gpuName,omitempty"`
	VRAMMiB     int                            `json:"vramMiB,omitempty"`
	Models      []face.RecognitionAvailability `json:"models"`
}

// FaceRecognitionCompareResult summarizes candidate-vs-active marker state.
type FaceRecognitionCompareResult struct {
	ModelKey             string `json:"modelKey"`
	ActiveModel          string `json:"activeModel"`
	Prepared             bool   `json:"prepared"`
	ActiveClusters       int    `json:"activeClusters"`
	CandidateClusters    int    `json:"candidateClusters"`
	ActiveMarkers        int    `json:"activeMarkers"`
	CandidateMarkers     int    `json:"candidateMarkers"`
	ActiveRecognized     int    `json:"activeRecognized"`
	CandidateRecognized  int    `json:"candidateRecognized"`
	ChangedSubjects      int    `json:"changedSubjects"`
	MissingCandidateRows int    `json:"missingCandidateRows"`
}

// FaceRecognitionEmbedder generates embeddings for prepared face crop files.
type FaceRecognitionEmbedder interface {
	EmbedFaceCrops(ctx context.Context, modelKey string, cropFiles []string) ([]face.Embeddings, error)
	Health(ctx context.Context) (FaceRecognitionServiceHealth, error)
}

// HTTPFaceRecognitionEmbedder sends crop files to a Vision API-compatible sidecar.
type HTTPFaceRecognitionEmbedder struct {
	URI string
}

type candidateSample struct {
	MarkerUID  string
	Embeddings face.Embeddings
	Embedding  face.Embedding
}

type candidateCluster struct {
	ID        string
	Embedding face.Embedding
	Radius    float64
}

var (
	faceRecognitionGPUProbe        = probeFaceRecognitionGPU
	faceRecognitionEmbedderFactory = newHTTPFaceRecognitionEmbedder
)

// newHTTPFaceRecognitionEmbedder returns a sidecar embedder for the configured URI.
func newHTTPFaceRecognitionEmbedder(uri string) FaceRecognitionEmbedder {
	return &HTTPFaceRecognitionEmbedder{URI: strings.TrimSpace(uri)}
}

// Health checks the recognition sidecar health endpoint.
func (e *HTTPFaceRecognitionEmbedder) Health(ctx context.Context) (FaceRecognitionServiceHealth, error) {
	if e == nil || strings.TrimSpace(e.URI) == "" {
		return FaceRecognitionServiceHealth{}, fmt.Errorf("face recognition service uri is empty")
	}

	healthURI := faceRecognitionHealthURI(e.URI)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURI, nil)
	if err != nil {
		return FaceRecognitionServiceHealth{}, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return FaceRecognitionServiceHealth{}, err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 300 {
		return FaceRecognitionServiceHealth{}, fmt.Errorf("face recognition service returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return FaceRecognitionServiceHealth{}, err
	}

	if strings.TrimSpace(string(data)) == "" {
		return FaceRecognitionServiceHealth{OK: true}, nil
	}

	health := FaceRecognitionServiceHealth{}
	if err = json.Unmarshal(data, &health); err != nil {
		return FaceRecognitionServiceHealth{}, err
	}

	if !health.OK && health.Provider == "" && health.GPUName == "" && health.VRAMMiB == 0 {
		health.OK = true
	}

	return health, nil
}

// EmbedFaceCrops sends face crop files to the configured sidecar.
func (e *HTTPFaceRecognitionEmbedder) EmbedFaceCrops(ctx context.Context, modelKey string, cropFiles []string) ([]face.Embeddings, error) {
	if e == nil || strings.TrimSpace(e.URI) == "" {
		return nil, fmt.Errorf("face recognition service uri is empty")
	}

	if len(cropFiles) == 0 {
		return nil, nil
	}

	req, err := vision.NewApiRequest(vision.ApiFormatVision, cropFiles, scheme.Data)
	if err != nil {
		return nil, err
	}

	req.Model = face.ParseRecognitionModel(modelKey)

	respCh := make(chan *vision.ApiResponse, 1)
	errCh := make(chan error, 1)

	go func() {
		resp, requestErr := vision.PerformApiRequest(req, e.URI, http.MethodPost, "")
		if requestErr != nil {
			errCh <- requestErr
			return
		}
		respCh <- resp
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err = <-errCh:
		return nil, err
	case resp := <-respCh:
		if resp == nil {
			return nil, fmt.Errorf("face recognition service returned no response")
		}
		if err = resp.Err(); err != nil {
			return nil, err
		}
		return resp.Result.Embeddings, nil
	}
}

// RecognitionModels returns recognition model availability for this deployment.
func (w *Faces) RecognitionModels(ctx context.Context) (FaceRecognitionModelList, error) {
	if w == nil || w.conf == nil {
		return FaceRecognitionModelList{}, fmt.Errorf("faces worker is not configured")
	}

	probe := face.RecognitionProbe{
		ModelsPath:    w.conf.FaceRecognitionModelsPath(),
		ConfiguredKey: w.conf.FaceRecognitionModel(),
		ServiceURI:    w.conf.FaceRecognitionServiceURI(),
	}

	if gpu, err := faceRecognitionGPUProbe(ctx); err == nil {
		probe.GPUName = gpu.GPUName
		probe.VRAMMiB = gpu.VRAMMiB
		probe.CUDARuntime = gpu.OK
	}

	if probe.ServiceURI != "" {
		probe.ServiceChecked = true
		if health, err := faceRecognitionEmbedderFactory(probe.ServiceURI).Health(ctx); err == nil && health.OK {
			probe.ServiceHealthy = true
			if health.Provider == "" || strings.Contains(strings.ToLower(health.Provider), "cuda") {
				probe.CUDARuntime = true
			}
			if health.GPUName != "" {
				probe.GPUName = health.GPUName
			}
			if health.VRAMMiB > 0 {
				probe.VRAMMiB = health.VRAMMiB
			}
		}
	}

	result := FaceRecognitionModelList{
		ActiveModel: probe.ConfiguredKey,
		ServiceURI:  probe.ServiceURI,
		GPUName:     probe.GPUName,
		VRAMMiB:     probe.VRAMMiB,
		Models:      make([]face.RecognitionAvailability, 0, len(face.RecognitionProfiles(true))),
	}

	for _, profile := range face.RecognitionProfiles(true) {
		availability := profile.Availability(probe)
		if profile.Hidden && !availability.Installed && !availability.Active {
			continue
		}

		result.Models = append(result.Models, availability)
	}

	return result, nil
}

// RecognitionModel returns recognition model availability for a model key.
func (w *Faces) RecognitionModel(ctx context.Context, modelKey string) (face.RecognitionAvailability, error) {
	modelKey = face.ParseRecognitionModel(modelKey)

	models, err := w.RecognitionModels(ctx)
	if err != nil {
		return face.RecognitionAvailability{}, err
	}

	for _, model := range models.Models {
		if model.Profile.Key == modelKey {
			return model, nil
		}
	}

	return face.RecognitionAvailability{}, fmt.Errorf("unknown face recognition model %s", clean.Log(modelKey))
}

// RecognizeModel prepares candidate embeddings, clusters, and matches for a model.
func (w *Faces) RecognizeModel(ctx context.Context, modelKey string, force bool) (*entity.FaceModelRun, error) {
	modelKey = face.ParseRecognitionModel(modelKey)

	if modelKey == face.RecognitionModelFacenet {
		return nil, fmt.Errorf("facenet is the active legacy recognition model and does not need candidate preparation")
	}

	availability, err := w.RecognitionModel(ctx, modelKey)
	if err != nil {
		return nil, err
	}

	if !availability.Available {
		return nil, fmt.Errorf("face recognition model %s unavailable: %s", clean.Log(modelKey), strings.Join(availability.Reasons, ", "))
	}

	if err = mutex.FacesWorker.Start(); err != nil {
		return nil, err
	}
	defer mutex.FacesWorker.Stop()

	run, err := w.markRecognitionRun(modelKey, FaceModelStatusPreparing, "")
	if err != nil {
		return nil, err
	}

	embedder := faceRecognitionEmbedderFactory(w.conf.FaceRecognitionServiceURI())

	if force {
		if err = w.clearRecognitionCandidate(modelKey, true); err != nil {
			_, _ = w.markRecognitionRun(modelKey, FaceModelStatusFailed, err.Error())
			return run, err
		}
	} else if err = w.clearRecognitionCandidate(modelKey, false); err != nil {
		_, _ = w.markRecognitionRun(modelKey, FaceModelStatusFailed, err.Error())
		return run, err
	}

	if err = w.prepareRecognitionEmbeddings(ctx, availability.Profile, embedder, force, run); err != nil {
		_, _ = w.markRecognitionRun(modelKey, FaceModelStatusFailed, err.Error())
		return run, err
	}

	if err = w.clusterRecognitionModel(modelKey, run); err != nil {
		_, _ = w.markRecognitionRun(modelKey, FaceModelStatusFailed, err.Error())
		return run, err
	}

	now := entity.TimeStamp()
	run.Status = FaceModelStatusPrepared
	run.Error = ""
	run.PreparedAt = now

	if err = entity.UnscopedDb().Save(run).Error; err != nil {
		return run, err
	}

	log.Infof("faces: prepared %s recognition model with %s and %s", clean.Log(modelKey), english.Plural(run.EmbeddingCount, "embedding", "embeddings"), english.Plural(run.ClusterCount, "cluster", "clusters"))

	return run, nil
}

// CompareRecognitionModel compares candidate state with the active model state.
func (w *Faces) CompareRecognitionModel(modelKey string) (FaceRecognitionCompareResult, error) {
	modelKey = face.ParseRecognitionModel(modelKey)

	result := FaceRecognitionCompareResult{
		ModelKey:    modelKey,
		ActiveModel: w.conf.FaceRecognitionModel(),
	}

	var run entity.FaceModelRun
	if err := entity.Db().Where("model_key = ?", modelKey).First(&run).Error; err == nil {
		result.Prepared = run.Status == FaceModelStatusPrepared || run.Status == FaceModelStatusPromoted
	}

	entity.Db().Model(&entity.Face{}).Count(&result.ActiveClusters)
	entity.Db().Model(&entity.FaceModelCluster{}).Where("model_key = ?", modelKey).Count(&result.CandidateClusters)
	entity.Db().Model(&entity.Marker{}).Where("marker_type = ? AND marker_invalid = 0", entity.MarkerFace).Count(&result.ActiveMarkers)
	entity.Db().Model(&entity.FaceModelMarker{}).Where("model_key = ?", modelKey).Count(&result.CandidateMarkers)
	entity.Db().Model(&entity.Marker{}).Where("marker_type = ? AND marker_invalid = 0 AND subj_uid <> ''", entity.MarkerFace).Count(&result.ActiveRecognized)
	entity.Db().Model(&entity.FaceModelMarker{}).Where("model_key = ? AND subj_uid <> ''", modelKey).Count(&result.CandidateRecognized)

	active := make(map[string]string, result.ActiveMarkers)
	var markers entity.Markers
	if err := entity.Db().Where("marker_type = ? AND marker_invalid = 0", entity.MarkerFace).Find(&markers).Error; err != nil {
		return result, err
	}

	for _, marker := range markers {
		active[marker.MarkerUID] = marker.SubjUID
	}

	var candidates []entity.FaceModelMarker
	if err := entity.Db().Where("model_key = ?", modelKey).Find(&candidates).Error; err != nil {
		return result, err
	}

	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		seen[candidate.MarkerUID] = struct{}{}
		if active[candidate.MarkerUID] != candidate.SubjUID {
			result.ChangedSubjects++
		}
	}

	for markerUID := range active {
		if _, ok := seen[markerUID]; !ok {
			result.MissingCandidateRows++
		}
	}

	return result, nil
}

// PromoteRecognitionModel makes a prepared candidate model the active recognition model.
func (w *Faces) PromoteRecognitionModel(modelKey string) error {
	modelKey = face.ParseRecognitionModel(modelKey)

	if modelKey == face.RecognitionModelFacenet {
		_, err := w.conf.SaveOptionsPatch(config.Values{"FaceRecognitionModel": face.RecognitionModelFacenet})
		return err
	}

	var run entity.FaceModelRun
	if err := entity.Db().Where("model_key = ?", modelKey).First(&run).Error; err != nil {
		return fmt.Errorf("face recognition model %s has not been prepared", clean.Log(modelKey))
	} else if run.Status != FaceModelStatusPrepared && run.Status != FaceModelStatusPromoted {
		return fmt.Errorf("face recognition model %s is not prepared", clean.Log(modelKey))
	}

	var currentMarkers int
	entity.Db().Model(&entity.Marker{}).Where("marker_type = ? AND marker_invalid = 0", entity.MarkerFace).Count(&currentMarkers)
	if run.MarkerCount != currentMarkers || run.EmbeddingCount < currentMarkers {
		return fmt.Errorf("face recognition model %s preparation is incomplete", clean.Log(modelKey))
	}

	tx := entity.UnscopedDb().Begin()
	if tx.Error != nil {
		return tx.Error
	}

	if err := promoteRecognitionModelTx(tx, modelKey); err != nil {
		tx.Rollback()
		return err
	}

	now := entity.TimeStamp()
	if err := tx.Model(&entity.FaceModelRun{}).Where("model_key = ?", modelKey).
		UpdateColumns(entity.Values{"status": FaceModelStatusPromoted, "promoted_at": now, "updated_at": now, "error": ""}).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	if _, err := w.conf.SaveOptionsPatch(config.Values{"FaceRecognitionModel": modelKey}); err != nil {
		return err
	}

	w.conf.Options().FaceRecognitionModel = modelKey
	entity.UpdateFaces.Store(true)

	return nil
}

// prepareRecognitionEmbeddings generates candidate embeddings for valid markers.
func (w *Faces) prepareRecognitionEmbeddings(ctx context.Context, profile face.RecognitionProfile, embedder FaceRecognitionEmbedder, force bool, run *entity.FaceModelRun) error {
	existing := make(map[string]struct{})
	if !force {
		var markerUIDs []string
		if err := entity.Db().Model(&entity.FaceModelEmbedding{}).Where("model_key = ?", profile.Key).Pluck("marker_uid", &markerUIDs).Error; err != nil {
			return err
		}
		for _, markerUID := range markerUIDs {
			existing[markerUID] = struct{}{}
		}
	}

	limit := 500
	offset := 0
	var batchFiles []string
	var batchMarkers []string

	flush := func() error {
		if len(batchFiles) == 0 {
			return nil
		}

		results, err := embedder.EmbedFaceCrops(ctx, profile.Key, batchFiles)
		if err != nil {
			return err
		}

		if len(results) != len(batchMarkers) {
			return fmt.Errorf("face recognition service returned %d embeddings for %d crops", len(results), len(batchMarkers))
		}

		for i, markerUID := range batchMarkers {
			modelEmbedding, embErr := entity.NewFaceModelEmbedding(profile.Key, markerUID, results[i])
			if embErr != nil {
				return embErr
			}

			if err = entity.UnscopedDb().Save(modelEmbedding).Error; err != nil {
				return err
			}

			run.EmbeddingCount++
		}

		log.Infof("faces: prepared %s candidate embeddings", english.Plural(run.EmbeddingCount, "marker", "markers"))
		batchFiles = batchFiles[:0]
		batchMarkers = batchMarkers[:0]

		return nil
	}

	for {
		markers, err := query.FaceMarkers(limit, offset)
		if err != nil {
			return err
		}

		if len(markers) == 0 {
			break
		}

		offset += len(markers)

		for _, marker := range markers {
			if marker.MarkerInvalid || len(marker.EmbeddingsJSON) == 0 {
				continue
			}

			run.MarkerCount++

			if _, ok := existing[marker.MarkerUID]; ok {
				run.EmbeddingCount++
				continue
			}

			cropFile, cropErr := w.recognitionMarkerCrop(marker, profile)
			if cropErr != nil {
				return cropErr
			}

			batchFiles = append(batchFiles, cropFile)
			batchMarkers = append(batchMarkers, marker.MarkerUID)

			if len(batchFiles) >= faceRecognitionBatchSize {
				if err = flush(); err != nil {
					return err
				}
			}
		}
	}

	return flush()
}

// recognitionMarkerCrop returns a cached crop file for a marker.
func (w *Faces) recognitionMarkerCrop(marker entity.Marker, profile face.RecognitionProfile) (string, error) {
	fileHash, areaToken := crop.ParseThumb(marker.Thumb)
	if fileHash == "" {
		return "", fmt.Errorf("marker %s has no thumbnail hash", clean.Log(marker.MarkerUID))
	}

	area := crop.AreaFromString(areaToken)
	if area.Empty() {
		area = crop.NewArea("crop", marker.X, marker.Y, marker.W, marker.H)
	}

	size := crop.Size{
		Name:    crop.Name(fmt.Sprintf("tile_%d", profile.InputSize)),
		Width:   profile.InputSize,
		Height:  profile.InputSize,
		Options: crop.DefaultOptions,
	}

	thumbName, err := crop.ThumbFileName(fileHash, area, size, w.conf.ThumbCachePath())
	if err != nil {
		return "", fmt.Errorf("%s for marker %s", err, clean.Log(marker.MarkerUID))
	}

	_, cropName, err := crop.ImageFromThumb(thumbName, area, size, true)
	if err != nil {
		return "", err
	}

	return cropName, nil
}

// clusterRecognitionModel creates candidate clusters and marker matches.
func (w *Faces) clusterRecognitionModel(modelKey string, run *entity.FaceModelRun) error {
	samples, err := loadCandidateSamples(modelKey)
	if err != nil {
		return err
	}

	if len(samples) < face.ClusterCore {
		return saveCandidateNoise(modelKey, samples, run)
	}

	embeddings := make(face.Embeddings, len(samples))
	for i, sample := range samples {
		embeddings[i] = sample.Embedding
	}

	clusterer, err := alg.DBSCANWithProgress(face.ClusterCore, face.ClusterDist, w.conf.IndexWorkers(), alg.EuclideanDist, 15*time.Minute, func(done, total int) {
		log.Infof("cluster: processing %d of %d candidate embeddings", done, total)
	})
	if err != nil {
		return err
	}

	if err = clusterer.Learn(embeddings.Float64()); err != nil {
		return err
	}

	grouped := make(map[int]face.Embeddings)
	for i, clusterID := range clusterer.Guesses() {
		if clusterID < 1 {
			continue
		}
		grouped[clusterID] = append(grouped[clusterID], samples[i].Embedding)
	}

	clusters := make(map[int]candidateCluster, len(grouped))
	for clusterID, clusterEmbeddings := range grouped {
		cluster, clusterErr := entity.NewFaceModelCluster(modelKey, "", entity.SrcAuto, clusterEmbeddings)
		if clusterErr != nil {
			return clusterErr
		}
		if cluster.Embedding().SkipMatching() {
			continue
		}
		if err = entity.UnscopedDb().Save(cluster).Error; err != nil {
			return err
		}
		run.ClusterCount++
		clusters[clusterID] = candidateCluster{ID: cluster.ID, Embedding: cluster.Embedding(), Radius: cluster.SampleRadius}
	}

	activeSubjects, err := activeMarkerSubjects()
	if err != nil {
		return err
	}

	subjectVotes := make(map[string]map[string]int)
	for i, clusterID := range clusterer.Guesses() {
		modelMarker := entity.FaceModelMarker{
			ModelKey:  modelKey,
			MarkerUID: samples[i].MarkerUID,
			FaceDist:  -1,
		}

		if clusterID > 0 {
			if cluster, ok := clusters[clusterID]; ok {
				modelMarker.FaceID = cluster.ID
				modelMarker.FaceDist = samples[i].Embedding.Dist(cluster.Embedding)
			}
		}

		if subject := activeSubjects[modelMarker.MarkerUID]; subject.SubjUID != "" {
			modelMarker.SubjUID = subject.SubjUID
			modelMarker.SubjSrc = subject.SubjSrc
			if modelMarker.SubjSrc == "" {
				modelMarker.SubjSrc = entity.SrcAuto
			}

			if modelMarker.FaceID != "" {
				if _, ok := subjectVotes[modelMarker.FaceID]; !ok {
					subjectVotes[modelMarker.FaceID] = make(map[string]int)
				}
				subjectVotes[modelMarker.FaceID][modelMarker.SubjUID]++
			}
		}

		if modelMarker.FaceID != "" {
			modelMarker.MatchedAt = entity.TimeStamp()
			run.MatchedCount++
		}

		if err = entity.UnscopedDb().Save(&modelMarker).Error; err != nil {
			return err
		}
	}

	return applyCandidateSubjectVotes(modelKey, subjectVotes)
}

// saveCandidateNoise stores unmatched candidate marker rows when no clusters can be formed.
func saveCandidateNoise(modelKey string, samples []candidateSample, run *entity.FaceModelRun) error {
	activeSubjects, err := activeMarkerSubjects()
	if err != nil {
		return err
	}

	for _, sample := range samples {
		modelMarker := entity.FaceModelMarker{
			ModelKey:  modelKey,
			MarkerUID: sample.MarkerUID,
			FaceDist:  -1,
		}

		if subject := activeSubjects[modelMarker.MarkerUID]; subject.SubjUID != "" {
			modelMarker.SubjUID = subject.SubjUID
			modelMarker.SubjSrc = subject.SubjSrc
			if modelMarker.SubjSrc == "" {
				modelMarker.SubjSrc = entity.SrcAuto
			}
		}

		if err = entity.UnscopedDb().Save(&modelMarker).Error; err != nil {
			return err
		}
	}

	run.ClusterCount = 0
	run.MatchedCount = 0

	return nil
}

// loadCandidateSamples loads candidate embeddings for clustering.
func loadCandidateSamples(modelKey string) ([]candidateSample, error) {
	var rows []entity.FaceModelEmbedding
	if err := entity.Db().Where("model_key = ?", modelKey).Order("marker_uid").Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]candidateSample, 0, len(rows))
	for _, row := range rows {
		embeddings := row.Embeddings()
		if embeddings.Empty() || len(embeddings[0]) == 0 {
			continue
		}
		result = append(result, candidateSample{
			MarkerUID:  row.MarkerUID,
			Embeddings: embeddings,
			Embedding:  embeddings[0],
		})
	}

	return result, nil
}

// activeMarkerSubject stores current marker subject state.
type activeMarkerSubject struct {
	SubjUID string
	SubjSrc string
}

// activeMarkerSubjects returns subject assignments for active markers.
func activeMarkerSubjects() (map[string]activeMarkerSubject, error) {
	var markers entity.Markers
	if err := entity.Db().Where("marker_type = ? AND marker_invalid = 0", entity.MarkerFace).Find(&markers).Error; err != nil {
		return nil, err
	}

	result := make(map[string]activeMarkerSubject, len(markers))
	for _, marker := range markers {
		result[marker.MarkerUID] = activeMarkerSubject{SubjUID: marker.SubjUID, SubjSrc: marker.SubjSrc}
	}

	return result, nil
}

// applyCandidateSubjectVotes assigns majority subjects to candidate clusters and automatic markers.
func applyCandidateSubjectVotes(modelKey string, votes map[string]map[string]int) error {
	for faceID, bySubject := range votes {
		subjUID := ""
		maxVotes := 0
		for candidateSubjUID, count := range bySubject {
			if count > maxVotes {
				subjUID = candidateSubjUID
				maxVotes = count
			}
		}

		if subjUID == "" {
			continue
		}

		if err := entity.UnscopedDb().Model(&entity.FaceModelCluster{}).Where("model_key = ? AND id = ?", modelKey, faceID).
			UpdateColumns(entity.Values{"subj_uid": subjUID}).Error; err != nil {
			return err
		}

		if err := entity.UnscopedDb().Model(&entity.FaceModelMarker{}).
			Where("model_key = ? AND face_id = ? AND (subj_src = '' OR subj_src = ?)", modelKey, faceID, entity.SrcAuto).
			UpdateColumns(entity.Values{"subj_uid": subjUID, "subj_src": entity.SrcAuto}).Error; err != nil {
			return err
		}
	}

	return nil
}

// clearRecognitionCandidate removes candidate rows before a retry.
func (w *Faces) clearRecognitionCandidate(modelKey string, embeddings bool) error {
	if embeddings {
		if err := entity.UnscopedDb().Where("model_key = ?", modelKey).Delete(&entity.FaceModelEmbedding{}).Error; err != nil {
			return err
		}
	}

	if err := entity.UnscopedDb().Where("model_key = ?", modelKey).Delete(&entity.FaceModelCluster{}).Error; err != nil {
		return err
	}

	return entity.UnscopedDb().Where("model_key = ?", modelKey).Delete(&entity.FaceModelMarker{}).Error
}

// markRecognitionRun creates or updates candidate preparation status.
func (w *Faces) markRecognitionRun(modelKey, statusText, errorText string) (*entity.FaceModelRun, error) {
	run := &entity.FaceModelRun{ModelKey: modelKey}
	if err := entity.Db().Where("model_key = ?", modelKey).First(run).Error; err != nil && !gorm.IsRecordNotFoundError(err) {
		return run, err
	}

	if run.ModelKey == "" {
		run.ModelKey = modelKey
	}

	run.Status = statusText
	run.Error = errorText
	run.MarkerCount = 0
	run.EmbeddingCount = 0
	run.ClusterCount = 0
	run.MatchedCount = 0
	run.PreparedAt = nil

	if statusText == FaceModelStatusFailed {
		run.PreparedAt = nil
	}

	return run, entity.UnscopedDb().Save(run).Error
}

// promoteRecognitionModelTx copies candidate state into active face tables.
func promoteRecognitionModelTx(tx *gorm.DB, modelKey string) error {
	if err := tx.Model(&entity.Marker{}).
		Where("subj_src = ? AND marker_type = ?", entity.SrcAuto, entity.MarkerFace).
		UpdateColumns(entity.Values{"marker_name": "", "subj_uid": "", "subj_src": "", "face_id": "", "face_dist": -1.0, "matched_at": nil}).Error; err != nil {
		return err
	}

	if err := tx.Delete(entity.Face{}, "face_src = ?", entity.SrcAuto).Error; err != nil {
		return err
	}

	var clusters []entity.FaceModelCluster
	if err := tx.Where("model_key = ?", modelKey).Find(&clusters).Error; err != nil {
		return err
	}

	faceIDs := make(map[string]string, len(clusters))
	for _, cluster := range clusters {
		active, err := cluster.ToFace()
		if err != nil {
			return err
		}
		active.FaceSrc = entity.SrcAuto

		var existing entity.Face
		if err = tx.Where("id = ?", active.ID).First(&existing).Error; err == nil {
			faceIDs[cluster.ID] = existing.ID
			if existing.SubjUID == "" && active.SubjUID != "" {
				if updateErr := tx.Model(&existing).UpdateColumn("subj_uid", active.SubjUID).Error; updateErr != nil {
					return updateErr
				}
			}
			continue
		} else if !gorm.IsRecordNotFoundError(err) {
			return err
		}

		if err = tx.Create(active).Error; err != nil {
			return err
		}
		faceIDs[cluster.ID] = active.ID
	}

	var candidates []entity.FaceModelMarker
	if err := tx.Where("model_key = ?", modelKey).Find(&candidates).Error; err != nil {
		return err
	}

	now := entity.TimeStamp()
	for _, candidate := range candidates {
		var marker entity.Marker
		if err := tx.Where("marker_uid = ?", candidate.MarkerUID).First(&marker).Error; err != nil {
			continue
		}

		values := entity.Values{
			"face_id":    "",
			"face_dist":  -1.0,
			"matched_at": now,
		}

		if activeFaceID := faceIDs[candidate.FaceID]; activeFaceID != "" {
			values["face_id"] = activeFaceID
			values["face_dist"] = candidate.FaceDist
		}

		if marker.SubjSrc == entity.SrcAuto || marker.SubjUID == "" {
			values["subj_uid"] = candidate.SubjUID
			values["subj_src"] = candidate.SubjSrc
			values["marker_review"] = false
		}

		if err := tx.Model(&entity.Marker{}).Where("marker_uid = ?", candidate.MarkerUID).UpdateColumns(values).Error; err != nil {
			return err
		}
	}

	return nil
}

// probeFaceRecognitionGPU returns the first CUDA-capable GPU reported by nvidia-smi.
func probeFaceRecognitionGPU(ctx context.Context) (FaceRecognitionServiceHealth, error) {
	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits")
	out, err := cmd.Output()
	if err != nil {
		return FaceRecognitionServiceHealth{}, err
	}

	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	if line == "" {
		return FaceRecognitionServiceHealth{}, fmt.Errorf("nvidia-smi returned no gpus")
	}

	parts := strings.Split(line, ",")
	if len(parts) < 2 {
		return FaceRecognitionServiceHealth{}, fmt.Errorf("nvidia-smi returned invalid gpu metadata")
	}

	vram, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return FaceRecognitionServiceHealth{}, err
	}

	return FaceRecognitionServiceHealth{
		OK:       true,
		Provider: "CUDA",
		GPUName:  strings.TrimSpace(parts[0]),
		VRAMMiB:  vram,
	}, nil
}

// faceRecognitionHealthURI returns the sidecar health URI for a configured endpoint.
func faceRecognitionHealthURI(uri string) string {
	uri = strings.TrimRight(strings.TrimSpace(uri), "/")

	for _, suffix := range []string{"/api/v1/vision/face", "/vision/face", "/face"} {
		if strings.HasSuffix(uri, suffix) {
			return strings.TrimSuffix(uri, suffix) + "/health"
		}
	}

	return uri + "/health"
}
